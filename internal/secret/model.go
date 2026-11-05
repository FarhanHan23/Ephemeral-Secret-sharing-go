package secret

import (
	"errors"
	"strings"
	"time"
)

// ====================== MODEL ======================
type Secret struct {
	ID            int64     `json:"id"`
	Content       string    `json:"content"`
	ExpiresAt     time.Time `json:"expires_at"`
	RemainingTTL  string    `json:"remaining_ttl,omitempty"`
	BurnAfterRead bool      `json:"burn_after_read"`
	Burned        bool      `json:"burned,omitempty"`
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
