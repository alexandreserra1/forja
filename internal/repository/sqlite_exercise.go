package repository

import (
	"database/sql"
	"fmt"
	"strings"

	"treino/internal/domain"
)

// ---------- ExerciseRepository ----------

// ListByLevel devolve exercícios de EXATAMENTE um nível (acesso puro).
// A cascata "nível e abaixo" é regra de negócio e vive no service, não aqui.
func (r *SQLiteRepo) ListByLevel(level string) ([]domain.Exercise, error) {
	rows, err := r.db.Query(
		`SELECT e.id, e.name, e.movement_pattern_id, mp.name, e.level, e.focus
		   FROM exercise e
		   JOIN movement_pattern mp ON mp.id = e.movement_pattern_id
		  WHERE e.level = ?
		  ORDER BY e.id`, level)
	if err != nil {
		return nil, fmt.Errorf("listar exercícios nível %s: %w", level, err)
	}
	defer rows.Close()

	var exercises []domain.Exercise
	for rows.Next() {
		var e domain.Exercise
		if err := rows.Scan(&e.ID, &e.Name, &e.MovementPatternID,
			&e.MovementPattern, &e.Level, &e.Focus); err != nil {
			return nil, fmt.Errorf("scan exercício: %w", err)
		}
		exercises = append(exercises, e)
	}
	return exercises, rows.Err()
}

// ListAvailableByLevel (Fase 3) devolve exercícios de um nível que o atleta CONSEGUE fazer
// com o equipamento que tem: ou o exercício não exige nada, ou todo equipamento exigido está
// no conjunto disponível. O filtro acontece NA QUERY — o pool já nasce viável.
//
// equipmentIDs vazio = sem filtro (assume tudo disponível), p/ não quebrar blocos sem equipamento.
func (r *SQLiteRepo) ListAvailableByLevel(level string, equipmentIDs []int) ([]domain.Exercise, error) {
	if len(equipmentIDs) == 0 {
		return r.ListByLevel(level) // sem equipamento marcado: comportamento das Fases 1/2
	}
	return r.queryAvailable(level, "", equipmentIDs)
}

// FindCandidatesForPhase (Fase 4) é o ListAvailableByLevel + filtro por FOCO (o estímulo da fase),
// tudo na query. equipmentIDs vazio = sem filtro de equipamento.
func (r *SQLiteRepo) FindCandidatesForPhase(stimulus string, level string, equipmentIDs []int) ([]domain.Exercise, error) {
	return r.queryAvailable(level, stimulus, equipmentIDs)
}

// queryAvailable é o motor de leitura do pool: filtra por nível, opcionalmente por foco, e
// (se houver equipamento marcado) por disponibilidade — sempre NA QUERY (índices em level/focus).
func (r *SQLiteRepo) queryAvailable(level, focus string, equipmentIDs []int) ([]domain.Exercise, error) {
	args := make([]any, 0, len(equipmentIDs)+2)
	where := []string{"e.level = ?"}
	args = append(args, level)
	if focus != "" {
		where = append(where, "e.focus = ?")
		args = append(args, focus)
	}
	if len(equipmentIDs) > 0 {
		placeholders := make([]string, len(equipmentIDs))
		for i, id := range equipmentIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		where = append(where, `NOT EXISTS (
		    SELECT 1 FROM exercise_equipment ee
		     WHERE ee.exercise_id = e.id
		       AND ee.equipment_id NOT IN (`+strings.Join(placeholders, ",")+`))`)
	}
	query := `SELECT e.id, e.name, e.movement_pattern_id, mp.name, e.level, e.focus
	            FROM exercise e
	            JOIN movement_pattern mp ON mp.id = e.movement_pattern_id
	           WHERE ` + strings.Join(where, " AND ") + `
	           ORDER BY e.id`

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("listar pool (nível %s, foco %q): %w", level, focus, err)
	}
	defer rows.Close()

	var exercises []domain.Exercise
	for rows.Next() {
		var e domain.Exercise
		if err := rows.Scan(&e.ID, &e.Name, &e.MovementPatternID,
			&e.MovementPattern, &e.Level, &e.Focus); err != nil {
			return nil, fmt.Errorf("scan exercício do pool: %w", err)
		}
		exercises = append(exercises, e)
	}
	return exercises, rows.Err()
}

// GetSubstitutionRule (Fase 3) devolve o exercício substituto PREFERIDO para um padrão quando falta
// um equipamento, ou nil se não houver regra. (phase é ignorada nesta fase — ver plan.md.)
func (r *SQLiteRepo) GetSubstitutionRule(pattern string, missingEquipmentID int) (*domain.Exercise, error) {
	var e domain.Exercise
	err := r.db.QueryRow(
		`SELECT ex.id, ex.name, ex.movement_pattern_id, mp.name, ex.level, ex.focus
		   FROM substitution_rule sr
		   JOIN movement_pattern mp ON mp.id = sr.movement_pattern_id
		   JOIN exercise ex ON ex.id = sr.substitute_exercise_id
		  WHERE mp.name = ? AND sr.missing_equipment_id = ?
		  ORDER BY sr.id LIMIT 1`, pattern, missingEquipmentID).
		Scan(&e.ID, &e.Name, &e.MovementPatternID, &e.MovementPattern, &e.Level, &e.Focus)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("buscar regra de substituição (%s, equip %d): %w", pattern, missingEquipmentID, err)
	}
	return &e, nil
}

// ListComponents (Fase 5A) devolve os componentes dos conjugados informados, agrupados por id do
// conjugado. UMA query com IN (...) carrega a semana inteira de uma vez — sem N+1. Ids que não são
// conjugados (sem linha em complex_item) simplesmente não aparecem no mapa.
func (r *SQLiteRepo) ListComponents(complexIDs []int) (map[int][]domain.ComplexComponent, error) {
	out := map[int][]domain.ComplexComponent{}
	if len(complexIDs) == 0 {
		return out, nil
	}
	placeholders := make([]string, len(complexIDs))
	args := make([]any, len(complexIDs))
	for i, id := range complexIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	rows, err := r.db.Query(
		`SELECT ci.complex_id, comp.id, comp.name, ci.sort_order, ci.reps
		   FROM complex_item ci
		   JOIN exercise comp ON comp.id = ci.component_exercise_id
		  WHERE ci.complex_id IN (`+strings.Join(placeholders, ",")+`)
		  ORDER BY ci.complex_id, ci.sort_order`, args...)
	if err != nil {
		return nil, fmt.Errorf("listar componentes de conjugados: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var complexID int
		var c domain.ComplexComponent
		if err := rows.Scan(&complexID, &c.ExerciseID, &c.ExerciseName, &c.SortOrder, &c.Reps); err != nil {
			return nil, fmt.Errorf("scan componente de conjugado: %w", err)
		}
		out[complexID] = append(out[complexID], c)
	}
	return out, rows.Err()
}

// ---------- EquipmentRepository (Fase 3) ----------

// ListEquipment devolve o catálogo de equipamentos.
func (r *SQLiteRepo) ListEquipment() ([]domain.Equipment, error) {
	return r.queryEquipment(`SELECT id, name FROM equipment ORDER BY id`)
}

// ListUserEquipment devolve o equipamento que o atleta tem.
func (r *SQLiteRepo) ListUserEquipment(athleteID int) ([]domain.Equipment, error) {
	return r.queryEquipment(
		`SELECT e.id, e.name FROM user_equipment ue
		   JOIN equipment e ON e.id = ue.equipment_id
		  WHERE ue.athlete_id = ? ORDER BY e.id`, athleteID)
}

// ListExerciseEquipment devolve o equipamento que um exercício exige.
func (r *SQLiteRepo) ListExerciseEquipment(exerciseID int) ([]domain.Equipment, error) {
	return r.queryEquipment(
		`SELECT e.id, e.name FROM exercise_equipment ee
		   JOIN equipment e ON e.id = ee.equipment_id
		  WHERE ee.exercise_id = ? ORDER BY e.id`, exerciseID)
}

// queryEquipment é o helper compartilhado das leituras de equipamento.
func (r *SQLiteRepo) queryEquipment(query string, args ...any) ([]domain.Equipment, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("listar equipamento: %w", err)
	}
	defer rows.Close()

	var out []domain.Equipment
	for rows.Next() {
		var e domain.Equipment
		if err := rows.Scan(&e.ID, &e.Name); err != nil {
			return nil, fmt.Errorf("scan equipamento: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// SetUserEquipment substitui o equipamento DO ATLETA (substitui tudo dele, numa transação).
func (r *SQLiteRepo) SetUserEquipment(athleteID int, equipmentIDs []int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("abrir transação: %w", err)
	}
	defer tx.Rollback() // no-op após Commit

	if _, err := tx.Exec(`DELETE FROM user_equipment WHERE athlete_id = ?`, athleteID); err != nil {
		return fmt.Errorf("limpar equipamento do atleta: %w", err)
	}
	stmt, err := tx.Prepare(`INSERT INTO user_equipment (athlete_id, equipment_id) VALUES (?, ?)`)
	if err != nil {
		return fmt.Errorf("preparar insert equipamento: %w", err)
	}
	defer stmt.Close()
	for _, id := range equipmentIDs {
		if _, err := stmt.Exec(athleteID, id); err != nil {
			return fmt.Errorf("inserir equipamento %d: %w", id, err)
		}
	}
	return tx.Commit()
}

// ---------- PriorityRepository (Fase 6B) ----------

// ListMovementPatterns devolve o catálogo de padrões de movimento (para o seletor de prioridades).
func (r *SQLiteRepo) ListMovementPatterns() ([]domain.MovementPattern, error) {
	return r.queryPatterns(`SELECT id, name FROM movement_pattern ORDER BY id`)
}

// ListPriorities devolve os padrões que o atleta priorizou (pontos fracos).
func (r *SQLiteRepo) ListPriorities(athleteID int) ([]domain.MovementPattern, error) {
	return r.queryPatterns(
		`SELECT mp.id, mp.name FROM athlete_priority ap
		   JOIN movement_pattern mp ON mp.id = ap.movement_pattern_id
		  WHERE ap.athlete_id = ? ORDER BY mp.id`, athleteID)
}

// queryPatterns é o helper compartilhado das leituras de padrão de movimento.
func (r *SQLiteRepo) queryPatterns(query string, args ...any) ([]domain.MovementPattern, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("listar padrões: %w", err)
	}
	defer rows.Close()
	var out []domain.MovementPattern
	for rows.Next() {
		var p domain.MovementPattern
		if err := rows.Scan(&p.ID, &p.Name); err != nil {
			return nil, fmt.Errorf("scan padrão: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// SetPriorities substitui as prioridades do atleta (substitui tudo dele, numa transação).
func (r *SQLiteRepo) SetPriorities(athleteID int, patternIDs []int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("abrir transação: %w", err)
	}
	defer tx.Rollback() // no-op após Commit

	if _, err := tx.Exec(`DELETE FROM athlete_priority WHERE athlete_id = ?`, athleteID); err != nil {
		return fmt.Errorf("limpar prioridades do atleta: %w", err)
	}
	stmt, err := tx.Prepare(`INSERT INTO athlete_priority (athlete_id, movement_pattern_id) VALUES (?, ?)`)
	if err != nil {
		return fmt.Errorf("preparar insert prioridade: %w", err)
	}
	defer stmt.Close()
	for _, id := range patternIDs {
		if _, err := stmt.Exec(athleteID, id); err != nil {
			return fmt.Errorf("inserir prioridade %d: %w", id, err)
		}
	}
	return tx.Commit()
}
