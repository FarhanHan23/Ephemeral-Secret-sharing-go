package secret

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrNotFound = errors.New("secret tidak ditemukan")

type SecretStore struct {
	db *sql.DB
}

func NewSecretStore(db *sql.DB) *SecretStore {
	return &SecretStore{db: db}
}

// Repository adalah kontrak penyimpanan todo.
// Service hanya bergantung pada interface ini, bukan pada SQLite.
type Repository interface {
	Create(ctx context.Context, t *Secret) error
	GetByID(ctx context.Context, id int64) (Secret, error)
	List(ctx context.Context) ([]Secret, error)
	Update(ctx context.Context, t Secret) error
	Delete(ctx context.Context, id int64) error
}

const timeLayout = time.RFC3339Nano

// create message
func (s *SecretStore) Create(ctx context.Context, t *Secret) error {
	res, err := s.db.ExecContext(
		ctx,
		`INSERT INTO secrets (content, expires_at, burn_after_read, created_at) VALUES (?, ?, ?, ?)`,
		t.Content,
		t.ExpiresAt.UTC().Format(timeLayout),
		t.BurnAfterRead,
		t.CreatedAt.UTC().Format(timeLayout),
	)

	if err != nil {
		return fmt.Errorf("insert secret: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("last insert id: %w", err)
	}
	t.ID = id
	return nil
}

// scanner adalah interface kecil yang dipenuhi *sql.Row dan *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanSecret(s scanner) (Secret, error) {
	var (
		t       Secret
		expires string
		created string
	)
	if err := s.Scan(&t.ID, &t.Content, &expires, &t.BurnAfterRead, &created); err != nil {
		return Secret{}, err
	}
	var err error
	if t.ExpiresAt, err = time.Parse(timeLayout, expires); err != nil {
		return Secret{}, fmt.Errorf("parse expires_at: %w", err)
	}
	if t.CreatedAt, err = time.Parse(timeLayout, created); err != nil {
		return Secret{}, fmt.Errorf("parse created_at: %w", err)
	}
	return t, nil
}

const selectColumns = `SELECT id, content, expires_at, burn_after_read, created_at FROM secrets`

func (s *SecretStore) GetByID(ctx context.Context, id int64) (Secret, error) {
	row := s.db.QueryRowContext(ctx, selectColumns+` WHERE id = ?`, id)
	sec, err := scanSecret(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Secret{}, ErrNotFound
		}
		return Secret{}, err
	}
	return sec, nil
}

// List mengembalikan daftar secret yang belum expired
func (s *SecretStore) List(ctx context.Context) ([]Secret, error) {
	rows, err := s.db.QueryContext(ctx, selectColumns+` WHERE expires_at > CURRENT_TIMESTAMP ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Secret
	for rows.Next() {
		t, err := scanSecret(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (s *SecretStore) Update(ctx context.Context, t Secret) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE secrets SET content = ?, expires_at = ?, burn_after_read = ?
		 WHERE id = ?`,
		t.Content, t.ExpiresAt.UTC().Format(timeLayout), t.BurnAfterRead, t.ID)
	if err != nil {
		return fmt.Errorf("update secret %d: %w", t.ID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SecretStore) Delete(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM secrets WHERE id = ?`,
		id)
	if err != nil {
		return fmt.Errorf("delete secret %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
