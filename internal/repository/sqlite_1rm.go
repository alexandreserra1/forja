package repository

import (
	"database/sql"
	"errors"

	"treino/internal/domain"
)

func (r *SQLiteRepo) Save1RM(athleteID, exerciseID int, weightKg float64) error {
	_, err := r.db.Exec(`
		INSERT INTO athlete_1rm (athlete_id, exercise_id, weight_kg)
		VALUES (?, ?, ?)
		ON CONFLICT(athlete_id, exercise_id)
		DO UPDATE SET weight_kg = excluded.weight_kg, recorded_at = datetime('now')`,
		athleteID, exerciseID, weightKg,
	)
	return err
}

func (r *SQLiteRepo) List1RMs(athleteID int) ([]domain.OneRM, error) {
	rows, err := r.db.Query(`
		SELECT a.id, a.athlete_id, a.exercise_id, e.name, a.weight_kg, a.recorded_at
		FROM athlete_1rm a
		JOIN exercise e ON e.id = a.exercise_id
		WHERE a.athlete_id = ?
		ORDER BY e.name`,
		athleteID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.OneRM
	for rows.Next() {
		var o domain.OneRM
		if err := rows.Scan(&o.ID, &o.AthleteID, &o.ExerciseID, &o.ExerciseName, &o.WeightKg, &o.RecordedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if errors.Is(err, sql.ErrNoRows) || out == nil {
		return []domain.OneRM{}, nil
	}
	return out, nil
}
