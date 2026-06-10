// Teste de INTEGRAÇÃO da Fase 5A: o service sobre SQLite REAL (schema+seed reais). Monta um bloco
// com uma prescrição apontando para um CONJUGADO do seed e prova que WeekDetail anexa a sequência de
// componentes — determinístico, sem depender da sorte do gerador. Mantém o service sem sqlite na
// build de produção (este import vive só no _test.go; `go list -deps` segue limpo).
package service

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"treino/internal/domain"
	"treino/internal/repository"
)

func realDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		t.Fatalf("abrir db: %v", err)
	}
	for _, f := range []string{"schema.sql", "seed.sql"} {
		b, err := os.ReadFile(filepath.Join("..", "..", "db", f))
		if err != nil {
			t.Fatalf("ler %s: %v", f, err)
		}
		if _, err := db.Exec(string(b)); err != nil {
			t.Fatalf("aplicar %s: %v", f, err)
		}
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestWeekDetail_AnexaComponentesDoConjugado(t *testing.T) {
	db := realDB(t)
	svc := New(repository.New(db))

	// Pega um conjugado (kind='complex') e um exercício simples do seed.
	var complexID, simpleID int
	if err := db.QueryRow("SELECT id FROM exercise WHERE kind='complex' ORDER BY id LIMIT 1").Scan(&complexID); err != nil {
		t.Fatalf("buscar conjugado: %v", err)
	}
	if err := db.QueryRow("SELECT id FROM exercise WHERE kind='simple' ORDER BY id LIMIT 1").Scan(&simpleID); err != nil {
		t.Fatalf("buscar exercício simples: %v", err)
	}

	// Monta um bloco mínimo válido p/ os CHECKs (total_weeks∈{8,10,12}, days_per_week 3–5), mas só
	// materializa a semana 1 — é tudo que o teste lê. 1 dia, 2 prescrições (um conjugado + um simples).
	plan := domain.GeneratedBlock{
		Block: domain.TrainingBlock{AthleteID: 1, Goal: "strength", TotalWeeks: 8, DaysPerWeek: 3},
		Weeks: []domain.GeneratedWeek{{
			Week: domain.BlockWeek{WeekNumber: 1, Phase: "accumulation", TargetRPE: 7},
			Sessions: []domain.GeneratedSession{{
				Session: domain.BlockSession{DayNumber: 1},
				Prescriptions: []domain.Prescription{
					{ExerciseID: complexID, Sets: 3, Reps: 1, TargetRPE: 7, SortOrder: 1},
					{ExerciseID: simpleID, Sets: 4, Reps: 6, TargetRPE: 7, SortOrder: 2},
				},
			}},
		}},
	}
	if err := svc.repo.SaveGeneratedBlock(plan); err != nil {
		t.Fatalf("SaveGeneratedBlock: %v", err)
	}

	week, err := svc.WeekDetail(1, 1)
	if err != nil {
		t.Fatalf("WeekDetail: %v", err)
	}
	if len(week.Sessions) != 1 || len(week.Sessions[0].Prescriptions) != 2 {
		t.Fatalf("estrutura inesperada: %+v", week.Sessions)
	}

	var complexP, simpleP *domain.Prescription
	for i := range week.Sessions[0].Prescriptions {
		p := &week.Sessions[0].Prescriptions[i]
		switch p.ExerciseID {
		case complexID:
			complexP = p
		case simpleID:
			simpleP = p
		}
	}
	if complexP == nil || simpleP == nil {
		t.Fatalf("não achei as duas prescrições")
	}

	// O conjugado vem com a sequência de componentes; o simples, sem nenhum.
	if len(complexP.Components) < 2 {
		t.Fatalf("conjugado deveria trazer seus componentes, veio %d", len(complexP.Components))
	}
	if len(simpleP.Components) != 0 {
		t.Errorf("exercício simples não deveria ter componentes, veio %d", len(simpleP.Components))
	}
	// Componentes em ordem, com nome resolvido e reps.
	for i, c := range complexP.Components {
		if c.SortOrder != i+1 {
			t.Errorf("componente %d com sort_order %d", i, c.SortOrder)
		}
		if c.ExerciseName == "" || c.Reps <= 0 {
			t.Errorf("componente mal formado: %+v", c)
		}
	}
}
