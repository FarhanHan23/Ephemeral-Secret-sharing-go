package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ====================== MODEL ======================
type Secret struct {
	ID            int       `json:"id"`
	Content       string    `json:"content"`
	ExpiresAt     time.Time `json:"expires_at"`
	RemainingTTL  string    `json:"remaining_ttl,omitempty"`
	BurnAfterRead bool      `json:"burn_after_read"`
	CreatedAt     time.Time `json:"created_at"`
}

type SecretInput struct {
	Content       string `json:"content"`
	TTL           string `json:"ttl"`             // Contoh input: "10m", "1h", "24h"
	BurnAfterRead bool   `json:"burn_after_read"` // true jika ingin 1x baca langsung hancur
}

func (in SecretInput) Validate() error {
	var errs []error
	// 1. Validasi Content tidak boleh kosong
	if strings.TrimSpace(in.Content) == "" {
		errs = append(errs, errors.New("content wajib diisi"))
	}
	// 2. Validasi TTL tidak boleh kosong
	if strings.TrimSpace(in.TTL) == "" {
		errs = append(errs, errors.New("durasi masa aktif (TTL) wajib diisi"))
	} else {
		// 3. Validasi apakah format TTL bisa dipahami oleh time.ParseDuration
		dur, err := time.ParseDuration(in.TTL)
		if err != nil {
			errs = append(errs, errors.New("format TTL tidak valid (contoh: 10m, 1h, 24h)"))
		} else if dur <= 0 {
			errs = append(errs, errors.New("durasi TTL harus lebih dari 0 detik"))
		} else if dur > 24*time.Hour {
			errs = append(errs, errors.New("durasi TTL maksimal 24 jam"))
		}
	}
	return errors.Join(errs...) // untuk mengumpulkan semua error validasi sekaligus!
}

// ====================== STORE (in-memory + mutex) ======================
var ErrNotFound = errors.New("secret tidak ditemukan")

type SecretStore struct {
	mu     sync.RWMutex
	data   map[int]Secret
	nextID int
}

// fungsi untuk membuat object dengan tipe SecretStore
func NewSecretStore() *SecretStore {
	return &SecretStore{
		data:   make(map[int]Secret),
		nextID: 1,
	}
}

// List mengembalikan daftar secret yang belum expired
func (s *SecretStore) List() []Secret {
	// 1. Kunci untuk membaca (goroutine lain yang mau baca juga tetap boleh masuk)
	s.mu.RLock()
	// 2. Pastikan kunci dilepas otomatis saat fungsi selesai
	defer s.mu.RUnlock()

	now := time.Now()
	var result []Secret

	// 3. Aman melakukan perulangan pada map s.data
	for _, secret := range s.data {
		if now.Before(secret.ExpiresAt) {
			result = append(result, secret)
		}
	}

	return result
}

func (s *SecretStore) Get(ID int) (Secret, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.data[ID]
	if !ok {
		return Secret{}, ErrNotFound
	}
	// 1. Jika sudah kedaluwarsa, hapus dan return error
	if time.Now().After(b.ExpiresAt) {
		delete(s.data, ID)
		return Secret{}, ErrNotFound
	}
	// 2. Jika fitur Burn After Read aktif, musnahkan setelah dibaca!
	if b.BurnAfterRead {
		delete(s.data, ID)
	}
	return b, nil
}

func (s *SecretStore) Create(in SecretInput) Secret {
	s.mu.Lock()
	defer s.mu.Unlock()

	dur, _ := time.ParseDuration(in.TTL)
	now := time.Now()

	b := Secret{
		ID:            s.nextID,
		Content:       in.Content,
		RemainingTTL:  in.TTL,
		ExpiresAt:     now.Add(dur),
		BurnAfterRead: in.BurnAfterRead,
		CreatedAt:     now,
	}
	s.data[b.ID] = b
	s.nextID++
	return b
}

func (s *SecretStore) Update(id int, in SecretInput) (Secret, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.data[id]
	if !ok {
		return Secret{}, ErrNotFound
	}
	dur, _ := time.ParseDuration(in.TTL)
	b.Content = in.Content
	b.RemainingTTL = in.TTL
	b.BurnAfterRead = in.BurnAfterRead
	b.ExpiresAt = time.Now().Add(dur) // Hitung ulang waktu kedaluwarsa baru
	s.data[id] = b
	return b, nil
}

func (s *SecretStore) Delete(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[id]; !ok {
		return ErrNotFound
	}
	delete(s.data, id)
	return nil
}

// ====================== HELPER JSON ======================
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func readJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

// ====================== HANDLER ======================
type SecretHandler struct {
	store *SecretStore
}

func (s *SecretHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/secret", s.list)
	mux.HandleFunc("POST /api/secret", s.create)
	mux.HandleFunc("GET /api/secret/{id}", s.get)
	mux.HandleFunc("PUT /api/secret/{id}", s.update)
	mux.HandleFunc("DELETE /api/secret/{id}", s.delete)
}

func parseID(r *http.Request) (int, error) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("id %q tidak valid", r.PathValue("id"))
	}
	return id, nil
}

func (h *SecretHandler) list(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.store.List())
}

func (h *SecretHandler) get(w http.ResponseWriter, r *http.Request) {
	// 1. Ambil ID
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// 2. Panggil store
	secret, err := h.store.Get(id)
	if err != nil {
		// Pembedaan error yang lebih baik untuk UI
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "Secret tidak ditemukan atau sudah expired/dihanguskan.")
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	// 3. Output JSON
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"content":    secret.Content,
			"expired_at": secret.ExpiresAt,
			"burned":     secret.BurnAfterRead, // Mengkonfirmasi apakah ia baru saja dihapus karena dibaca
		},
	})
}

func (h *SecretHandler) create(w http.ResponseWriter, r *http.Request) {
	var in SecretInput
	if err := readJSON(w, r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "Format JSON tidak valid")
		return
	}
	// Gunakan validator yang sudah kita buat!
	if err := in.Validate(); err != nil {
		// Kita convert error menjadi string biasa agar JSON-friendly
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	secret := h.store.Create(in)
	// Berikan respons location header dan ID
	w.Header().Set("Location", fmt.Sprintf("/api/secret/%d", secret.ID))
	writeJSON(w, http.StatusCreated, map[string]any{
		"success": true,
		"data": map[string]any{
			"id":              secret.ID,
			"expires_at":      secret.ExpiresAt,
			"burn_after_read": secret.BurnAfterRead,
		},
	})
}

func (h *SecretHandler) update(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var in SecretInput
	if err := readJSON(w, r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "Format JSON tidak valid")
		return
	}

	if err := in.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	updated, err := h.store.Update(id, in)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "Secret tidak ditemukan")
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    updated,
	})
}

func (h *SecretHandler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.store.Delete(id); err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "Secret tidak ditemukan")
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ====================== MIDDLEWARE ======================
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func logging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mulai := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		logger.Info("request", "method", r.Method, "path", r.URL.Path,
			"status", rec.status, "durasi", time.Since(mulai).String())
	})
}

func recovery(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logger.Error("panic", "err", err)
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func run(logger *slog.Logger) error {
	store := NewSecretStore()
	mux := http.NewServeMux()
	(&SecretHandler{store: store}).Register(mux)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	addr := ":" + cmp.Or(os.Getenv("PORT"), "8080")
	srv := &http.Server{
		Addr:              addr,
		Handler:           logging(logger, recovery(logger, mux)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server mulai", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutdown dimulai")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	logger.Info("shutdown selesai")
	return nil
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("fatal", "err", err)
		os.Exit(1)
	}
}
