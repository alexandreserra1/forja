package repository

import (
	"database/sql"
	"fmt"

	"treino/internal/domain"
)

// ---------- PhaseTemplateRepository (Fase 1) ----------

// ListPhaseTemplates devolve os moldes de fase de um objetivo, em ordem.
func (r *SQLiteRepo) ListPhaseTemplates(goal string) ([]domain.PhaseTemplate, error) {
	rows, err := r.db.Query(
		`SELECT id, goal, phase, week_share, base_rpe, rpe_step, default_sets, default_reps, sort_order
		   FROM phase_template WHERE goal = ? ORDER BY sort_order`, goal)
	if err != nil {
		return nil, fmt.Errorf("listar moldes de fase (%s): %w", goal, err)
	}
	defer rows.Close()

	var templates []domain.PhaseTemplate
	for rows.Next() {
		var t domain.PhaseTemplate
		if err := rows.Scan(&t.ID, &t.Goal, &t.Phase, &t.WeekShare, &t.BaseRPE,
			&t.RPEStep, &t.DefaultSets, &t.DefaultReps, &t.SortOrder); err != nil {
			return nil, fmt.Errorf("scan molde de fase: %w", err)
		}
		templates = append(templates, t)
	}
	return templates, rows.Err()
}

// ---------- BlockRepository (Fase 1) ----------

// GetActiveBlock devolve o bloco ativo DO ATLETA, ou nil se não houver.
func (r *SQLiteRepo) GetActiveBlock(athleteID int) (*domain.TrainingBlock, error) {
	var b domain.TrainingBlock
	err := r.db.QueryRow(
		`SELECT id, athlete_id, goal, total_weeks, days_per_week, status, created_at
		   FROM training_block WHERE athlete_id = ? AND status = 'active'
		  ORDER BY id DESC LIMIT 1`, athleteID).
		Scan(&b.ID, &b.AthleteID, &b.Goal, &b.TotalWeeks, &b.DaysPerWeek, &b.Status, &b.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("buscar bloco ativo: %w", err)
	}
	return &b, nil
}

// GetBlockWeeks devolve as semanas do bloco, em ordem.
func (r *SQLiteRepo) GetBlockWeeks(blockID int) ([]domain.BlockWeek, error) {
	rows, err := r.db.Query(
		`SELECT id, block_id, week_number, phase, target_rpe, is_deload
		   FROM block_week WHERE block_id = ? ORDER BY week_number`, blockID)
	if err != nil {
		return nil, fmt.Errorf("listar semanas do bloco %d: %w", blockID, err)
	}
	defer rows.Close()

	var weeks []domain.BlockWeek
	for rows.Next() {
		var w domain.BlockWeek
		if err := rows.Scan(&w.ID, &w.BlockID, &w.WeekNumber, &w.Phase, &w.TargetRPE, &w.IsDeload); err != nil {
			return nil, fmt.Errorf("scan semana: %w", err)
		}
		weeks = append(weeks, w)
	}
	return weeks, rows.Err()
}

// GetSessions devolve as sessões de uma semana, em ordem de dia.
func (r *SQLiteRepo) GetSessions(weekID int) ([]domain.BlockSession, error) {
	rows, err := r.db.Query(
		`SELECT id, week_id, day_number FROM block_session WHERE week_id = ? ORDER BY day_number`, weekID)
	if err != nil {
		return nil, fmt.Errorf("listar sessões da semana %d: %w", weekID, err)
	}
	defer rows.Close()

	var sessions []domain.BlockSession
	for rows.Next() {
		var s domain.BlockSession
		if err := rows.Scan(&s.ID, &s.WeekID, &s.DayNumber); err != nil {
			return nil, fmt.Errorf("scan sessão: %w", err)
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

// GetPrescriptions devolve as prescrições de uma sessão com nome do exercício
// resolvido e o estado do registro (done/actual_rpe) via LEFT JOIN.
func (r *SQLiteRepo) GetPrescriptions(sessionID int) ([]domain.Prescription, error) {
	rows, err := r.db.Query(
		`SELECT p.id, p.session_id, p.exercise_id, e.name, p.sets, p.reps,
		        p.target_rpe, p.sort_order,
		        COALESCE(sl.done, 0), sl.actual_rpe
		   FROM prescription p
		   JOIN exercise e ON e.id = p.exercise_id
		   LEFT JOIN session_log sl ON sl.prescription_id = p.id
		  WHERE p.session_id = ?
		  ORDER BY p.sort_order`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("listar prescrições da sessão %d: %w", sessionID, err)
	}
	defer rows.Close()

	var prescriptions []domain.Prescription
	for rows.Next() {
		var p domain.Prescription
		var done int
		if err := rows.Scan(&p.ID, &p.SessionID, &p.ExerciseID, &p.ExerciseName,
			&p.Sets, &p.Reps, &p.TargetRPE, &p.SortOrder, &done, &p.ActualRPE); err != nil {
			return nil, fmt.Errorf("scan prescrição: %w", err)
		}
		p.Done = done == 1
		prescriptions = append(prescriptions, p)
	}
	return prescriptions, rows.Err()
}

// ArchiveActiveBlock arquiva o bloco ativo DO ATLETA (se houver). Chamado antes de gerar um novo.
func (r *SQLiteRepo) ArchiveActiveBlock(athleteID int) error {
	if _, err := r.db.Exec(
		`UPDATE training_block SET status = 'archived' WHERE athlete_id = ? AND status = 'active'`,
		athleteID); err != nil {
		return fmt.Errorf("arquivar bloco ativo: %w", err)
	}
	return nil
}

// SaveGeneratedBlock grava a árvore inteira (block -> weeks -> sessions ->
// prescriptions) numa única transação. Falha no meio = nada gravado (rollback).
func (r *SQLiteRepo) SaveGeneratedBlock(plan domain.GeneratedBlock) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("abrir transação: %w", err)
	}
	defer tx.Rollback() // no-op após Commit

	res, err := tx.Exec(
		`INSERT INTO training_block (athlete_id, goal, total_weeks, days_per_week, status)
		 VALUES (?, ?, ?, ?, 'active')`,
		plan.Block.AthleteID, plan.Block.Goal, plan.Block.TotalWeeks, plan.Block.DaysPerWeek)
	if err != nil {
		return fmt.Errorf("inserir bloco: %w", err)
	}
	blockID, err := res.LastInsertId()
	if err != nil {
		return err
	}

	for _, gw := range plan.Weeks {
		res, err := tx.Exec(
			`INSERT INTO block_week (block_id, week_number, phase, target_rpe, is_deload)
			 VALUES (?, ?, ?, ?, ?)`,
			blockID, gw.Week.WeekNumber, gw.Week.Phase, gw.Week.TargetRPE, boolToInt(gw.Week.IsDeload))
		if err != nil {
			return fmt.Errorf("inserir semana %d: %w", gw.Week.WeekNumber, err)
		}
		weekID, err := res.LastInsertId()
		if err != nil {
			return err
		}

		for _, gs := range gw.Sessions {
			res, err := tx.Exec(
				`INSERT INTO block_session (week_id, day_number) VALUES (?, ?)`,
				weekID, gs.Session.DayNumber)
			if err != nil {
				return fmt.Errorf("inserir sessão dia %d: %w", gs.Session.DayNumber, err)
			}
			sessionID, err := res.LastInsertId()
			if err != nil {
				return err
			}

			for _, p := range gs.Prescriptions {
				if _, err := tx.Exec(
					`INSERT INTO prescription (session_id, exercise_id, sets, reps, target_rpe, sort_order)
					 VALUES (?, ?, ?, ?, ?, ?)`,
					sessionID, p.ExerciseID, p.Sets, p.Reps, p.TargetRPE, p.SortOrder); err != nil {
					return fmt.Errorf("inserir prescrição: %w", err)
				}
			}

			// FASE 5B: o condicionamento (WOD composto) é gravado na MESMA transação da força.
			for _, c := range gs.Conditioning {
				if err := insertConditioning(tx, sessionID, c); err != nil {
					return err
				}
			}
		}
	}
	return tx.Commit()
}

// insertConditioning grava um WOD COMPOSTO (source='generated') + seus movimentos + a prescrição de
// condicionamento da sessão, dentro da transação do bloco (Fase 5B).
func insertConditioning(tx *sql.Tx, sessionID int64, c domain.ConditioningPrescription) error {
	res, err := tx.Exec(
		`INSERT INTO wod (name, format_id, work_sec, rest_sec, rounds, emphasis_system, target_rpe, level, source)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'generated')`,
		c.Wod.Name, c.Wod.FormatID, c.Wod.WorkSec, c.Wod.RestSec, c.Wod.Rounds,
		c.Wod.EmphasisSystem, c.Wod.TargetRPE, c.Wod.Level)
	if err != nil {
		return fmt.Errorf("inserir wod gerado: %w", err)
	}
	wodID, err := res.LastInsertId()
	if err != nil {
		return err
	}
	for _, m := range c.Wod.Movements {
		if _, err := tx.Exec(
			`INSERT INTO wod_movement (wod_id, exercise_id, reps, sort_order) VALUES (?, ?, ?, ?)`,
			wodID, m.ExerciseID, m.Reps, m.SortOrder); err != nil {
			return fmt.Errorf("inserir movimento do wod: %w", err)
		}
	}
	if _, err := tx.Exec(
		`INSERT INTO conditioning_prescription (session_id, wod_id, target_rpe, sort_order) VALUES (?, ?, ?, ?)`,
		sessionID, wodID, c.TargetRPE, c.SortOrder); err != nil {
		return fmt.Errorf("inserir prescrição de condicionamento: %w", err)
	}
	return nil
}

// MarkPrescriptionDone registra (ou atualiza) o realizado de uma prescrição.
// Upsert via UNIQUE(prescription_id): marcar de novo não duplica.
func (r *SQLiteRepo) MarkPrescriptionDone(prescriptionID int, actualRPE *float64, notes string) error {
	_, err := r.db.Exec(
		`INSERT INTO session_log (prescription_id, done, actual_rpe, notes, logged_at)
		 VALUES (?, 1, ?, ?, datetime('now'))
		 ON CONFLICT(prescription_id) DO UPDATE SET
		     done = 1,
		     actual_rpe = excluded.actual_rpe,
		     notes = excluded.notes,
		     logged_at = excluded.logged_at`,
		prescriptionID, actualRPE, notes)
	if err != nil {
		return fmt.Errorf("marcar prescrição %d como feita: %w", prescriptionID, err)
	}
	return nil
}

// RecentExerciseIDs (Fase 6C) devolve os ids distintos de exercício prescritos no bloco MAIS RECENTE
// do atleta (maior id de training_block dele) — para o motor não repetir o treino anterior. Vazio se
// o atleta ainda não tem bloco. Uma query, sobre índices já existentes.
func (r *SQLiteRepo) RecentExerciseIDs(athleteID int) ([]int, error) {
	rows, err := r.db.Query(
		`SELECT DISTINCT p.exercise_id
		   FROM prescription p
		   JOIN block_session bs ON bs.id = p.session_id
		   JOIN block_week    bw ON bw.id = bs.week_id
		  WHERE bw.block_id = (
		      SELECT id FROM training_block WHERE athlete_id = ? ORDER BY id DESC LIMIT 1)`,
		athleteID)
	if err != nil {
		return nil, fmt.Errorf("ids recentes do atleta %d: %w", athleteID, err)
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan id recente: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// BlockCount (Fase 6C) devolve quantos blocos o atleta já tem (ativo + arquivados).
func (r *SQLiteRepo) BlockCount(athleteID int) (int, error) {
	var n int
	if err := r.db.QueryRow(
		`SELECT count(*) FROM training_block WHERE athlete_id = ?`, athleteID).Scan(&n); err != nil {
		return 0, fmt.Errorf("contar blocos do atleta %d: %w", athleteID, err)
	}
	return n, nil
}

// ---------- AutoregRepository (Fase 2) ----------

// GetWeekActuals devolve, para uma semana, cada prescrição (previsto) com seu
// registro (realizado) ao lado, via LEFT JOIN. É o read model que o detector lê.
func (r *SQLiteRepo) GetWeekActuals(weekID int) ([]domain.SessionActual, error) {
	rows, err := r.db.Query(
		`SELECT p.id, p.session_id, bs.day_number, p.exercise_id, p.sets, p.reps,
		        p.target_rpe, COALESCE(sl.done, 0), sl.actual_rpe
		   FROM block_session bs
		   JOIN prescription p ON p.session_id = bs.id
		   LEFT JOIN session_log sl ON sl.prescription_id = p.id
		  WHERE bs.week_id = ?
		  ORDER BY bs.day_number, p.sort_order`, weekID)
	if err != nil {
		return nil, fmt.Errorf("listar realizado da semana %d: %w", weekID, err)
	}
	defer rows.Close()

	var actuals []domain.SessionActual
	for rows.Next() {
		var a domain.SessionActual
		var done int
		if err := rows.Scan(&a.PrescriptionID, &a.SessionID, &a.DayNumber, &a.ExerciseID,
			&a.Sets, &a.Reps, &a.TargetRPE, &done, &a.ActualRPE); err != nil {
			return nil, fmt.Errorf("scan realizado: %w", err)
		}
		a.Done = done == 1
		actuals = append(actuals, a)
	}
	return actuals, rows.Err()
}

// ApplyAdjustment aplica o ajuste numa única transação: reescreve target_rpe/sets
// de cada prescrição da semana alvo E grava o rastro em autoreg_adjustment.
// Falha no meio = nada muda (rollback) — a prescrição fica intacta e nenhum ajuste grava.
func (r *SQLiteRepo) ApplyAdjustment(adj domain.AutoregAdjustment, updated []domain.Prescription) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("abrir transação: %w", err)
	}
	defer tx.Rollback() // no-op após Commit

	for _, p := range updated {
		if _, err := tx.Exec(
			`UPDATE prescription SET target_rpe = ?, sets = ? WHERE id = ?`,
			p.TargetRPE, p.Sets, p.ID); err != nil {
			return fmt.Errorf("reescrever prescrição %d: %w", p.ID, err)
		}
	}

	if _, err := tx.Exec(
		`INSERT INTO autoreg_adjustment
		   (block_id, week_id, trigger, action, rpe_before, rpe_after, sets_before, sets_after, explanation)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		adj.BlockID, adj.WeekID, adj.Trigger, adj.Action,
		adj.RPEBefore, adj.RPEAfter, adj.SetsBefore, adj.SetsAfter, adj.Explanation); err != nil {
		return fmt.Errorf("gravar ajuste: %w", err)
	}
	return tx.Commit()
}

// ListAdjustments devolve o histórico de ajustes de um bloco, do mais recente ao mais antigo.
func (r *SQLiteRepo) ListAdjustments(blockID int) ([]domain.AutoregAdjustment, error) {
	rows, err := r.db.Query(
		`SELECT id, block_id, week_id, trigger, action,
		        rpe_before, rpe_after, sets_before, sets_after, explanation, created_at
		   FROM autoreg_adjustment WHERE block_id = ? ORDER BY id DESC`, blockID)
	if err != nil {
		return nil, fmt.Errorf("listar ajustes do bloco %d: %w", blockID, err)
	}
	defer rows.Close()

	var adjustments []domain.AutoregAdjustment
	for rows.Next() {
		var a domain.AutoregAdjustment
		if err := rows.Scan(&a.ID, &a.BlockID, &a.WeekID, &a.Trigger, &a.Action,
			&a.RPEBefore, &a.RPEAfter, &a.SetsBefore, &a.SetsAfter, &a.Explanation, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan ajuste: %w", err)
		}
		adjustments = append(adjustments, a)
	}
	return adjustments, rows.Err()
}
