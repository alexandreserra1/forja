package repository

import (
	"fmt"

	"treino/internal/domain"
)

// ---------- WodAutoregRepository ----------

// MarkWodDone registra que o atleta completou um WOD (+ RPE opcional).
func (r *SQLiteRepo) MarkWodDone(condPrescriptionID int, actualRPE *float64) error {
	_, err := r.db.Exec(
		`UPDATE conditioning_prescription SET done = 1, actual_rpe = ? WHERE id = ?`,
		actualRPE, condPrescriptionID,
	)
	if err != nil {
		return fmt.Errorf("marcar wod feito: %w", err)
	}
	return nil
}

// GetWodActuals devolve os pares previsto/realizado dos WODs FEITOS na semana.
// Só lê os done=1 — base do detector de estagnação do condicionamento.
func (r *SQLiteRepo) GetWodActuals(weekID int) ([]domain.WodActual, error) {
	rows, err := r.db.Query(
		`SELECT cp.id, cp.target_rpe, cp.actual_rpe, cp.done
		   FROM conditioning_prescription cp
		   JOIN block_session bs ON bs.id = cp.session_id
		  WHERE bs.week_id = ? AND cp.done = 1 AND cp.skipped = 0`, weekID)
	if err != nil {
		return nil, fmt.Errorf("actuals dos wods da semana %d: %w", weekID, err)
	}
	defer rows.Close()
	var out []domain.WodActual
	for rows.Next() {
		var a domain.WodActual
		var doneBit int
		if err := rows.Scan(&a.CondPrescriptionID, &a.TargetRPE, &a.ActualRPE, &doneBit); err != nil {
			return nil, fmt.Errorf("scan wod actual: %w", err)
		}
		a.Done = doneBit == 1
		out = append(out, a)
	}
	return out, rows.Err()
}

// SkipWodPrescription marca uma conditioning_prescription como skipped (autoreg reduziu dose).
// A prescrição permanece no banco (auditável) mas não aparece mais em GetConditioning.
func (r *SQLiteRepo) SkipWodPrescription(condPrescriptionID int) error {
	_, err := r.db.Exec(
		`UPDATE conditioning_prescription SET skipped = 1 WHERE id = ?`,
		condPrescriptionID,
	)
	if err != nil {
		return fmt.Errorf("skip wod prescription %d: %w", condPrescriptionID, err)
	}
	return nil
}
