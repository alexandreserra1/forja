package repository

import (
	"fmt"

	"treino/internal/domain"
)

// ---------- QuestionRepository ----------

// ListQuestions devolve as perguntas (ordenadas) já com suas opções aninhadas.
// Usa 2 queries (perguntas + todas as opções) e costura em memória — evita N+1.
func (r *SQLiteRepo) ListQuestions() ([]domain.Question, error) {
	rows, err := r.db.Query(
		`SELECT id, text, type, sort_order, show_when FROM question ORDER BY sort_order`)
	if err != nil {
		return nil, fmt.Errorf("listar perguntas: %w", err)
	}
	defer rows.Close()

	var questions []domain.Question
	index := map[int]int{} // question.id -> posição no slice, p/ anexar opções depois
	for rows.Next() {
		var q domain.Question
		if err := rows.Scan(&q.ID, &q.Text, &q.Type, &q.SortOrder, &q.ShowWhen); err != nil {
			return nil, fmt.Errorf("scan pergunta: %w", err)
		}
		index[q.ID] = len(questions)
		questions = append(questions, q)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	optRows, err := r.db.Query(
		`SELECT id, question_id, label, value, sort_order
		   FROM question_option ORDER BY question_id, sort_order`)
	if err != nil {
		return nil, fmt.Errorf("listar opções: %w", err)
	}
	defer optRows.Close()

	for optRows.Next() {
		var o domain.Option
		if err := optRows.Scan(&o.ID, &o.QuestionID, &o.Label, &o.Value, &o.SortOrder); err != nil {
			return nil, fmt.Errorf("scan opção: %w", err)
		}
		if pos, ok := index[o.QuestionID]; ok {
			questions[pos].Options = append(questions[pos].Options, o)
		}
	}
	return questions, optRows.Err()
}

// ListOptions devolve as opções de uma pergunta específica.
func (r *SQLiteRepo) ListOptions(questionID int) ([]domain.Option, error) {
	rows, err := r.db.Query(
		`SELECT id, question_id, label, value, sort_order
		   FROM question_option WHERE question_id = ? ORDER BY sort_order`, questionID)
	if err != nil {
		return nil, fmt.Errorf("listar opções da pergunta %d: %w", questionID, err)
	}
	defer rows.Close()

	var opts []domain.Option
	for rows.Next() {
		var o domain.Option
		if err := rows.Scan(&o.ID, &o.QuestionID, &o.Label, &o.Value, &o.SortOrder); err != nil {
			return nil, fmt.Errorf("scan opção: %w", err)
		}
		opts = append(opts, o)
	}
	return opts, rows.Err()
}

// ---------- AnswerRepository ----------

// SaveAnswers substitui as respostas do usuário implícito (Fase 0: um só usuário).
// Limpa as antigas e grava as novas numa transação — reenviar o formulário não duplica.
func (r *SQLiteRepo) SaveAnswers(athleteID int, answers []domain.Answer) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("abrir transação: %w", err)
	}
	defer tx.Rollback() // no-op se já houve Commit

	// Substitui só as respostas DESTE atleta (Fase 6A: estado escopado).
	if _, err := tx.Exec(`DELETE FROM user_answer WHERE athlete_id = ?`, athleteID); err != nil {
		return fmt.Errorf("limpar respostas: %w", err)
	}
	stmt, err := tx.Prepare(`INSERT INTO user_answer (athlete_id, question_id, answer_value) VALUES (?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("preparar insert: %w", err)
	}
	defer stmt.Close()
	for _, a := range answers {
		if _, err := stmt.Exec(athleteID, a.QuestionID, a.AnswerValue); err != nil {
			return fmt.Errorf("inserir resposta (q=%d): %w", a.QuestionID, err)
		}
	}
	return tx.Commit()
}

// ListAnswers devolve as respostas gravadas do atleta.
func (r *SQLiteRepo) ListAnswers(athleteID int) ([]domain.Answer, error) {
	rows, err := r.db.Query(`SELECT id, question_id, answer_value FROM user_answer WHERE athlete_id = ?`, athleteID)
	if err != nil {
		return nil, fmt.Errorf("listar respostas: %w", err)
	}
	defer rows.Close()

	var answers []domain.Answer
	for rows.Next() {
		var a domain.Answer
		if err := rows.Scan(&a.ID, &a.QuestionID, &a.AnswerValue); err != nil {
			return nil, fmt.Errorf("scan resposta: %w", err)
		}
		answers = append(answers, a)
	}
	return answers, rows.Err()
}

// ListAnswerRules devolve todas as regras de interpretação.
func (r *SQLiteRepo) ListAnswerRules() ([]domain.AnswerRule, error) {
	rows, err := r.db.Query(
		`SELECT id, question_id, option_value, sets_attribute, attribute_value FROM answer_rule`)
	if err != nil {
		return nil, fmt.Errorf("listar regras: %w", err)
	}
	defer rows.Close()

	var rules []domain.AnswerRule
	for rows.Next() {
		var rule domain.AnswerRule
		if err := rows.Scan(&rule.ID, &rule.QuestionID, &rule.OptionValue,
			&rule.SetsAttribute, &rule.AttributeValue); err != nil {
			return nil, fmt.Errorf("scan regra: %w", err)
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

// ---------- AthleteRepository (Fase 6A) ----------

// CreateAthlete cria um atleta e devolve com o id atribuído.
func (r *SQLiteRepo) CreateAthlete(name string) (*domain.Athlete, error) {
	res, err := r.db.Exec(`INSERT INTO athlete (name) VALUES (?)`, name)
	if err != nil {
		return nil, fmt.Errorf("criar atleta: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	var a domain.Athlete
	if err := r.db.QueryRow(`SELECT id, name, created_at FROM athlete WHERE id = ?`, id).
		Scan(&a.ID, &a.Name, &a.CreatedAt); err != nil {
		return nil, fmt.Errorf("ler atleta criado: %w", err)
	}
	return &a, nil
}

// ListAthletes devolve todos os atletas (para o seletor; sem auth).
func (r *SQLiteRepo) ListAthletes() ([]domain.Athlete, error) {
	rows, err := r.db.Query(`SELECT id, name, created_at FROM athlete ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("listar atletas: %w", err)
	}
	defer rows.Close()
	var out []domain.Athlete
	for rows.Next() {
		var a domain.Athlete
		if err := rows.Scan(&a.ID, &a.Name, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan atleta: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ---------- WorkoutRepository ----------

// ClearWorkout apaga o treino anterior. Chamado antes de gravar um novo (idempotência).
func (r *SQLiteRepo) ClearWorkout() error {
	if _, err := r.db.Exec(`DELETE FROM generated_workout`); err != nil {
		return fmt.Errorf("limpar treino: %w", err)
	}
	return nil
}

// SaveWorkout grava as linhas do treino montado pelo motor.
func (r *SQLiteRepo) SaveWorkout(w []domain.GeneratedWorkout) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("abrir transação: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO generated_workout (day_number, exercise_id) VALUES (?, ?)`)
	if err != nil {
		return fmt.Errorf("preparar insert treino: %w", err)
	}
	defer stmt.Close()
	for _, row := range w {
		if _, err := stmt.Exec(row.DayNumber, row.ExerciseID); err != nil {
			return fmt.Errorf("inserir linha de treino: %w", err)
		}
	}
	return tx.Commit()
}

// ListWorkout devolve o treino gravado, com o nome do exercício resolvido.
func (r *SQLiteRepo) ListWorkout() ([]domain.GeneratedWorkout, error) {
	rows, err := r.db.Query(
		`SELECT gw.id, gw.day_number, gw.exercise_id, e.name
		   FROM generated_workout gw
		   JOIN exercise e ON e.id = gw.exercise_id
		  ORDER BY gw.day_number, gw.id`)
	if err != nil {
		return nil, fmt.Errorf("listar treino: %w", err)
	}
	defer rows.Close()

	var workout []domain.GeneratedWorkout
	for rows.Next() {
		var gw domain.GeneratedWorkout
		if err := rows.Scan(&gw.ID, &gw.DayNumber, &gw.ExerciseID, &gw.ExerciseName); err != nil {
			return nil, fmt.Errorf("scan linha de treino: %w", err)
		}
		workout = append(workout, gw)
	}
	return workout, rows.Err()
}
