package repository

import (
	"database/sql"
	"fmt"

	"treino/internal/domain"
)

// ---------- AthleteMetricsRepository (Fase 6D) ----------

// SaveMetrics grava ou substitui as métricas do atleta (UPSERT por athlete_id).
func (r *SQLiteRepo) SaveMetrics(m domain.AthleteMetrics) error {
	_, err := r.db.Exec(
		`INSERT INTO athlete_metrics (athlete_id, age_years, sex, body_weight_kg, sport)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(athlete_id) DO UPDATE SET
		     age_years      = excluded.age_years,
		     sex            = excluded.sex,
		     body_weight_kg = excluded.body_weight_kg,
		     sport          = excluded.sport`,
		m.AthleteID, m.AgeYears, m.Sex, m.BodyWeightKg, m.Sport,
	)
	if err != nil {
		return fmt.Errorf("salvar métricas: %w", err)
	}
	return nil
}

// GetMetrics devolve as métricas do atleta, ou nil se não existirem.
func (r *SQLiteRepo) GetMetrics(athleteID int) (*domain.AthleteMetrics, error) {
	var m domain.AthleteMetrics
	err := r.db.QueryRow(
		`SELECT athlete_id, age_years, sex, body_weight_kg, sport
		   FROM athlete_metrics WHERE athlete_id = ?`, athleteID,
	).Scan(&m.AthleteID, &m.AgeYears, &m.Sex, &m.BodyWeightKg, &m.Sport)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("carregar métricas: %w", err)
	}
	return &m, nil
}
