package repository

import (
	"testing"
)

func TestOneRM_SaveAndList(t *testing.T) {
	repo := newTestDB(t)

	a, _ := repo.CreateAthlete("Atleta 1RM")

	// Salva dois exercícios diferentes (IDs 1 e 2 existem no seed).
	if err := repo.Save1RM(a.ID, 1, 100.0); err != nil {
		t.Fatalf("Save1RM ex1: %v", err)
	}
	if err := repo.Save1RM(a.ID, 2, 80.0); err != nil {
		t.Fatalf("Save1RM ex2: %v", err)
	}

	list, err := repo.List1RMs(a.ID)
	if err != nil {
		t.Fatalf("List1RMs: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("esperava 2 1RMs, veio %d", len(list))
	}
	for _, o := range list {
		if o.ExerciseName == "" {
			t.Errorf("exercise_name vazio no 1RM id=%d", o.ID)
		}
		if o.WeightKg <= 0 {
			t.Errorf("weight_kg inválido: %v", o.WeightKg)
		}
	}
}

func TestOneRM_Upsert(t *testing.T) {
	repo := newTestDB(t)

	a, _ := repo.CreateAthlete("Upsert Test")

	repo.Save1RM(a.ID, 1, 100.0)
	repo.Save1RM(a.ID, 1, 120.0) // atualiza

	list, _ := repo.List1RMs(a.ID)
	if len(list) != 1 {
		t.Fatalf("esperava 1 registro após upsert, veio %d", len(list))
	}
	if list[0].WeightKg != 120.0 {
		t.Errorf("esperava 120kg após upsert, veio %.1f", list[0].WeightKg)
	}
}

func TestOneRM_ListVazio(t *testing.T) {
	repo := newTestDB(t)

	a, _ := repo.CreateAthlete("Sem 1RM")
	list, err := repo.List1RMs(a.ID)
	if err != nil {
		t.Fatalf("List1RMs: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("esperava lista vazia, veio %d", len(list))
	}
}

func TestOneRM_CascadeDeleteAthlete(t *testing.T) {
	repo := newTestDB(t)

	a, _ := repo.CreateAthlete("Deletável")
	repo.Save1RM(a.ID, 1, 100.0)

	repo.db.Exec(`DELETE FROM athlete WHERE id = ?`, a.ID)

	list, err := repo.List1RMs(a.ID)
	if err != nil {
		t.Fatalf("List1RMs após cascade: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("ON DELETE CASCADE deveria ter removido os 1RMs")
	}
}
