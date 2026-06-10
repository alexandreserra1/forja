package repository

import (
	"testing"
)

func TestAuth_CreateAndGet(t *testing.T) {
	repo := newTestDB(t)

	// Cria atleta para ter um athlete_id válido.
	a, err := repo.CreateAthlete("Auth Test")
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}

	if err := repo.CreateAuth(a.ID, "test@cfit.io", "hash_bcrypt"); err != nil {
		t.Fatalf("CreateAuth: %v", err)
	}

	got, err := repo.GetAuthByEmail("test@cfit.io")
	if err != nil {
		t.Fatalf("GetAuthByEmail: %v", err)
	}
	if got == nil {
		t.Fatal("esperava auth, veio nil")
	}
	if got.AthleteID != a.ID {
		t.Errorf("athlete_id: esperava %d, veio %d", a.ID, got.AthleteID)
	}
	if got.Email != "test@cfit.io" {
		t.Errorf("email: esperava test@cfit.io, veio %q", got.Email)
	}
	if got.PasswordHash != "hash_bcrypt" {
		t.Errorf("password_hash não bate")
	}
}

func TestAuth_GetNaoExiste(t *testing.T) {
	repo := newTestDB(t)
	got, err := repo.GetAuthByEmail("nao@existe.io")
	if err != nil {
		t.Fatalf("GetAuthByEmail: %v", err)
	}
	if got != nil {
		t.Error("esperava nil para e-mail inexistente")
	}
}

func TestAuth_EmailUnico(t *testing.T) {
	repo := newTestDB(t)

	a1, _ := repo.CreateAthlete("Atleta 1")
	a2, _ := repo.CreateAthlete("Atleta 2")

	if err := repo.CreateAuth(a1.ID, "mesmo@email.io", "hash1"); err != nil {
		t.Fatalf("primeiro CreateAuth: %v", err)
	}

	err := repo.CreateAuth(a2.ID, "mesmo@email.io", "hash2")
	if err == nil {
		t.Fatal("esperava erro de UNIQUE, veio nil")
	}
	if err != ErrEmailTaken {
		t.Errorf("esperava ErrEmailTaken, veio %v", err)
	}
}

func TestAuth_CascadeDeleteAthlete(t *testing.T) {
	repo := newTestDB(t)

	a, _ := repo.CreateAthlete("Deletável")
	_ = repo.CreateAuth(a.ID, "del@cfit.io", "hash")

	// Apaga o atleta; ON DELETE CASCADE deve remover athlete_auth.
	if _, err := repo.db.Exec(`DELETE FROM athlete WHERE id = ?`, a.ID); err != nil {
		t.Fatalf("delete athlete: %v", err)
	}

	got, err := repo.GetAuthByEmail("del@cfit.io")
	if err != nil {
		t.Fatalf("GetAuthByEmail após cascade: %v", err)
	}
	if got != nil {
		t.Error("auth deveria ter sido removida pelo cascade")
	}
}
