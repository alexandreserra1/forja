package repository

import (
	"fmt"
	"strings"

	"treino/internal/domain"
)

// ---------- ConditioningRepository (Fase 5B) ----------

// ListEnergySystemMap devolve as faixas tempo->sistema, em ordem (a 1ª que comporta o work_sec vence).
func (r *SQLiteRepo) ListEnergySystemMap() ([]domain.EnergySystemBand, error) {
	rows, err := r.db.Query(
		`SELECT max_work_sec, system, sort_order FROM energy_system_map ORDER BY sort_order`)
	if err != nil {
		return nil, fmt.Errorf("listar mapa de sistemas: %w", err)
	}
	defer rows.Close()
	var out []domain.EnergySystemBand
	for rows.Next() {
		var b domain.EnergySystemBand
		if err := rows.Scan(&b.MaxWorkSec, &b.System, &b.SortOrder); err != nil {
			return nil, fmt.Errorf("scan banda: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ListPhaseConditioning devolve a dose de condicionamento por fase do objetivo, em ordem.
func (r *SQLiteRepo) ListPhaseConditioning(goal string) ([]domain.PhaseConditioning, error) {
	rows, err := r.db.Query(
		`SELECT goal, phase, emphasis_system, wod_target_rpe, weekly_wods, sort_order
		   FROM phase_conditioning WHERE goal = ? ORDER BY sort_order`, goal)
	if err != nil {
		return nil, fmt.Errorf("listar dose por fase (%s): %w", goal, err)
	}
	defer rows.Close()
	var out []domain.PhaseConditioning
	for rows.Next() {
		var p domain.PhaseConditioning
		if err := rows.Scan(&p.Goal, &p.Phase, &p.EmphasisSystem, &p.WodTargetRPE, &p.WeeklyWods, &p.SortOrder); err != nil {
			return nil, fmt.Errorf("scan dose: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListWodFormats devolve os formatos de WOD (o compositor escolhe um e atribui ao WOD gerado).
func (r *SQLiteRepo) ListWodFormats() ([]domain.WodFormat, error) {
	rows, err := r.db.Query(`SELECT id, name, default_domain_sec FROM wod_format ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("listar formatos: %w", err)
	}
	defer rows.Close()
	var out []domain.WodFormat
	for rows.Next() {
		var f domain.WodFormat
		if err := rows.Scan(&f.ID, &f.Name, &f.DefaultDomainSec); err != nil {
			return nil, fmt.Errorf("scan formato: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// FindMovementsByModality devolve os movimentos PERFILADOS de uma modalidade, de EXATAMENTE um nível
// (a cascata é regra do service) e — se houver equipamento marcado — só os viáveis (funil Fase 3).
func (r *SQLiteRepo) FindMovementsByModality(modality, level string, equipmentIDs []int) ([]domain.MovementCandidate, error) {
	args := []any{modality, level}
	where := "p.modality = ? AND e.level = ?"
	if len(equipmentIDs) > 0 {
		ph := make([]string, len(equipmentIDs))
		for i, id := range equipmentIDs {
			ph[i] = "?"
			args = append(args, id)
		}
		where += ` AND NOT EXISTS (
		    SELECT 1 FROM exercise_equipment ee
		     WHERE ee.exercise_id = e.id
		       AND ee.equipment_id NOT IN (` + strings.Join(ph, ",") + `))`
	}
	rows, err := r.db.Query(
		`SELECT e.id, e.name, p.modality, p.secs_per_rep, p.skill, e.level
		   FROM movement_profile p
		   JOIN exercise e ON e.id = p.exercise_id
		  WHERE `+where+`
		  ORDER BY e.id`, args...)
	if err != nil {
		return nil, fmt.Errorf("listar movimentos (modalidade %s, nível %s): %w", modality, level, err)
	}
	defer rows.Close()
	var out []domain.MovementCandidate
	for rows.Next() {
		var m domain.MovementCandidate
		if err := rows.Scan(&m.ExerciseID, &m.ExerciseName, &m.Modality, &m.SecsPerRep, &m.Skill, &m.Level); err != nil {
			return nil, fmt.Errorf("scan movimento: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ListWods devolve o catálogo de WODs (benchmark + gerados), sem movimentos — p/ GET /api/wods.
func (r *SQLiteRepo) ListWods() ([]domain.Wod, error) {
	rows, err := r.db.Query(
		`SELECT w.id, w.name, w.format_id, f.name, w.work_sec, w.rest_sec, w.rounds,
		        w.emphasis_system, w.target_rpe, w.level, w.source
		   FROM wod w JOIN wod_format f ON f.id = w.format_id
		  ORDER BY w.id`)
	if err != nil {
		return nil, fmt.Errorf("listar wods: %w", err)
	}
	defer rows.Close()
	var out []domain.Wod
	for rows.Next() {
		var w domain.Wod
		if err := rows.Scan(&w.ID, &w.Name, &w.FormatID, &w.FormatName, &w.WorkSec, &w.RestSec,
			&w.Rounds, &w.EmphasisSystem, &w.TargetRPE, &w.Level, &w.Source); err != nil {
			return nil, fmt.Errorf("scan wod: %w", err)
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// GetConditioning devolve os WODs prescritos numa sessão, JÁ com seus movimentos resolvidos. Duas
// queries (prescrições+wod, depois movimentos em lote por IN) — sem N+1 dentro da sessão.
// WODs marcados como skipped=1 pela autorregulação são excluídos da resposta.
func (r *SQLiteRepo) GetConditioning(sessionID int) ([]domain.ConditioningPrescription, error) {
	rows, err := r.db.Query(
		`SELECT cp.id, cp.session_id, cp.wod_id, cp.target_rpe, cp.sort_order,
		        cp.done, cp.actual_rpe,
		        w.name, w.format_id, f.name, w.work_sec, w.rest_sec, w.rounds,
		        w.emphasis_system, w.target_rpe, w.level, w.source
		   FROM conditioning_prescription cp
		   JOIN wod w ON w.id = cp.wod_id
		   JOIN wod_format f ON f.id = w.format_id
		  WHERE cp.session_id = ? AND cp.skipped = 0
		  ORDER BY cp.sort_order`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("listar condicionamento da sessão %d: %w", sessionID, err)
	}
	defer rows.Close()

	var out []domain.ConditioningPrescription
	var wodIDs []int
	for rows.Next() {
		var c domain.ConditioningPrescription
		var doneBit int
		if err := rows.Scan(&c.ID, &c.SessionID, &c.WodID, &c.TargetRPE, &c.SortOrder,
			&doneBit, &c.ActualRPE,
			&c.Wod.Name, &c.Wod.FormatID, &c.Wod.FormatName, &c.Wod.WorkSec, &c.Wod.RestSec,
			&c.Wod.Rounds, &c.Wod.EmphasisSystem, &c.Wod.TargetRPE, &c.Wod.Level, &c.Wod.Source); err != nil {
			return nil, fmt.Errorf("scan prescrição de condicionamento: %w", err)
		}
		c.Done = doneBit == 1
		c.Wod.ID = c.WodID
		out = append(out, c)
		wodIDs = append(wodIDs, c.WodID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}

	// Movimentos de TODOS os wods da sessão numa só query (sem N+1), agrupados por wod_id.
	byWod, err := r.wodMovements(wodIDs)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Wod.Movements = byWod[out[i].WodID]
	}
	return out, nil
}

// RecentWodMovementIDs (Fase 5C) devolve os ids de exercício dos movimentos de WOD do bloco MAIS
// RECENTE do atleta — para o compositor não repetir os mesmos movimentos de um bloco para o outro.
func (r *SQLiteRepo) RecentWodMovementIDs(athleteID int) ([]int, error) {
	rows, err := r.db.Query(
		`SELECT DISTINCT wm.exercise_id
		   FROM wod_movement wm
		   JOIN conditioning_prescription cp ON cp.wod_id = wm.wod_id
		   JOIN block_session bs ON bs.id = cp.session_id
		   JOIN block_week    bw ON bw.id = bs.week_id
		  WHERE bw.block_id = (
		      SELECT id FROM training_block WHERE athlete_id = ? ORDER BY id DESC LIMIT 1)`,
		athleteID)
	if err != nil {
		return nil, fmt.Errorf("movimentos de WOD recentes do atleta %d: %w", athleteID, err)
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan id de movimento recente: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// wodMovements carrega os movimentos dos wods informados em UMA query (IN), agrupados por wod_id.
func (r *SQLiteRepo) wodMovements(wodIDs []int) (map[int][]domain.WodMovement, error) {
	out := map[int][]domain.WodMovement{}
	if len(wodIDs) == 0 {
		return out, nil
	}
	ph := make([]string, len(wodIDs))
	args := make([]any, len(wodIDs))
	for i, id := range wodIDs {
		ph[i] = "?"
		args[i] = id
	}
	rows, err := r.db.Query(
		`SELECT wm.wod_id, wm.exercise_id, e.name, wm.reps, wm.sort_order
		   FROM wod_movement wm
		   JOIN exercise e ON e.id = wm.exercise_id
		  WHERE wm.wod_id IN (`+strings.Join(ph, ",")+`)
		  ORDER BY wm.wod_id, wm.sort_order`, args...)
	if err != nil {
		return nil, fmt.Errorf("listar movimentos dos wods: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var wodID int
		var m domain.WodMovement
		if err := rows.Scan(&wodID, &m.ExerciseID, &m.ExerciseName, &m.Reps, &m.SortOrder); err != nil {
			return nil, fmt.Errorf("scan movimento do wod: %w", err)
		}
		out[wodID] = append(out[wodID], m)
	}
	return out, rows.Err()
}
