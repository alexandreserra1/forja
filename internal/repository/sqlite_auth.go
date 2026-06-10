package repository

import (
	"database/sql"
	"errors"

	"treino/internal/domain"
)

// ErrEmailTaken é devolvido quando o e-mail já está cadastrado.
var ErrEmailTaken = errors.New("e-mail já cadastrado")

func (r *SQLiteRepo) CreateAuth(athleteID int, email, passwordHash string) error {
	_, err := r.db.Exec(
		`INSERT INTO athlete_auth (athlete_id, email, password_hash) VALUES (?, ?, ?)`,
		athleteID, email, passwordHash,
	)
	if err != nil && isUniqueConstraint(err) {
		return ErrEmailTaken
	}
	return err
}

func (r *SQLiteRepo) GetAuthByEmail(email string) (*domain.AthleteAuth, error) {
	var a domain.AthleteAuth
	err := r.db.QueryRow(
		`SELECT athlete_id, email, password_hash, created_at FROM athlete_auth WHERE email = ?`,
		email,
	).Scan(&a.AthleteID, &a.Email, &a.PasswordHash, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}
