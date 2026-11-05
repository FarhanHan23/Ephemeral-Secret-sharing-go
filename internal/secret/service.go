package secret

import (
	"context"
	"log"
	"time"
)

// Service berisi logika bisnis. Ia tidak tahu apa-apa soal HTTP maupun SQL.
type Service struct {
	repo Repository
	now  func() time.Time // bisa diganti di test agar waktu deterministik
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

func (s *Service) Create(ctx context.Context, in SecretInput) (Secret, error) {
	if err := in.Validate(); err != nil {
		return Secret{}, err
	}

	dur, err := time.ParseDuration(in.TTL)
	if err != nil {
		return Secret{}, err
	}

	now := s.now()
	sec := Secret{
		Content:       in.Content,
		RemainingTTL:  in.TTL,
		ExpiresAt:     now.Add(dur),
		BurnAfterRead: in.BurnAfterRead,
		CreatedAt:     now,
	}

	if err := s.repo.Create(ctx, &sec); err != nil {
		return Secret{}, err
	}
	return sec, nil
}

func (s *Service) Get(ctx context.Context, id int64) (Secret, error) {
	// 1. Ambil data dari repository
	secret, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Secret{}, err // NotFound / DB error
	}

	now := s.now()
	// 2. Cek expired
	if secret.ExpiresAt.Before(now) {
		// hapus otomatis jika sudah melewati batas waktu
		_ = s.repo.Delete(ctx, id)
		return Secret{}, ErrNotFound
	}

	// 3. Log & hanguskan kalau BurnAfterRead
	log.Printf("Secret %d viewed. BurnAfterRead=%t", id, secret.BurnAfterRead)

	if secret.BurnAfterRead {
		log.Printf("Secret %d deleted after read (BurnAfterRead=true)", id)
		if err := s.repo.Delete(ctx, id); err != nil {
			log.Printf("WARNING: delete failed for secret %d: %v", id, err)
		}
		// set flag supaya consumer/front-end tahu dia sudah dibakar
		secret.Burned = true
	}

	// 4. Hitung sisa waktu dinamis
	secret.RemainingTTL = time.Until(secret.ExpiresAt).Round(time.Second).String()

	return secret, nil
}

func (s *Service) List(ctx context.Context) ([]Secret, error) {
	secrets, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	// Hitung remaining TTL untuk masing-masing secret yang aktif
	for i := range secrets {
		secrets[i].RemainingTTL = time.Until(secrets[i].ExpiresAt).Round(time.Second).String()
	}
	return secrets, nil
}

func (s *Service) Update(ctx context.Context, id int64, in SecretInput) (Secret, error) {
	if err := in.Validate(); err != nil {
		return Secret{}, err
	}

	dur, err := time.ParseDuration(in.TTL)
	if err != nil {
		return Secret{}, err
	}

	now := s.now()
	sec := Secret{
		ID:            id,
		Content:       in.Content,
		RemainingTTL:  in.TTL,
		ExpiresAt:     now.Add(dur),
		BurnAfterRead: in.BurnAfterRead,
	}

	if err := s.repo.Update(ctx, sec); err != nil {
		return Secret{}, err
	}
	return sec, nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}