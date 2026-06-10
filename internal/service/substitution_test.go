// Testes da Fase 3: pool equipment-aware (M3) e seleção com substituição (M4).
// Lógica isolada, com o fakeRepo em memória.
package service

import (
	"testing"

	"treino/internal/domain"
)

// blockExerciseIDs reúne todos os exercise_id prescritos no bloco gerado.
func blockExerciseIDs(f *fakeRepo) map[int]bool {
	ids := map[int]bool{}
	for _, p := range f.prescriptions {
		ids[p.ExerciseID] = true
	}
	return ids
}

func TestGenerateBlock_PoolFiltradoPorEquipamento(t *testing.T) {
	repo := blockRepo()
	repo.userEquip = []int{1} // só Barra: sem Rack, KB, Argolas, Barra fixa
	svc := New(repo)

	if _, err := svc.GenerateBlock(1); err != nil {
		t.Fatalf("GenerateBlock: %v", err)
	}

	ids := blockExerciseIDs(repo)
	// Impossíveis sem o equipamento: Back Squat(2, +Rack), KB Swing(5), Ring Row(8), Pull-up(9).
	for _, bad := range []int{2, 5, 8, 9} {
		if ids[bad] {
			t.Errorf("exercício %d exige equipamento ausente e não deveria ter sido prescrito", bad)
		}
	}
	if len(ids) == 0 {
		t.Fatal("bloco não prescreveu nenhum exercício")
	}
}

func TestGenerateBlock_SubstituicaoEspecifica(t *testing.T) {
	repo := blockRepo()
	repo.userEquip = []int{1} // Barra, sem Rack -> regra squat+Rack -> Overhead Squat(3)
	svc := New(repo)

	overview, err := svc.GenerateBlock(1)
	if err != nil {
		t.Fatalf("GenerateBlock: %v", err)
	}

	// O relato de substituição específica deve aparecer.
	var found *domain.Substitution
	for i := range overview.Substitutions {
		if overview.Substitutions[i].Ideal == "Back Squat" {
			found = &overview.Substitutions[i]
		}
	}
	if found == nil {
		t.Fatalf("esperava substituição de Back Squat, veio %+v", overview.Substitutions)
	}
	if found.Substitute != "Overhead Squat" || found.Missing != "Rack" || !found.Specific {
		t.Errorf("substituição errada: %+v", *found)
	}
	// E o substituto está mesmo no bloco.
	if !blockExerciseIDs(repo)[3] {
		t.Error("Overhead Squat (substituto) deveria estar no bloco")
	}
}

func TestGenerateBlock_ComTudoNaoSubstitui(t *testing.T) {
	repo := blockRepo()
	repo.userEquip = []int{1, 2, 3, 4, 5} // tem tudo
	svc := New(repo)

	overview, err := svc.GenerateBlock(1)
	if err != nil {
		t.Fatalf("GenerateBlock: %v", err)
	}
	if len(overview.Substitutions) != 0 {
		t.Fatalf("com todo equipamento não deveria substituir, veio %+v", overview.Substitutions)
	}
	// O ideal (Back Squat, strength) está disponível.
	if !blockExerciseIDs(repo)[2] {
		t.Error("Back Squat deveria estar no bloco quando há Rack")
	}
}

func TestGenerateBlock_SemEquipamentoMarcadoNaoFiltra(t *testing.T) {
	repo := blockRepo() // userEquip vazio = sem filtro (compat Fases 1/2)
	svc := New(repo)

	overview, err := svc.GenerateBlock(1)
	if err != nil {
		t.Fatalf("GenerateBlock: %v", err)
	}
	if len(overview.Substitutions) != 0 {
		t.Fatalf("sem equipamento marcado não deveria substituir, veio %+v", overview.Substitutions)
	}
	if !blockExerciseIDs(repo)[2] {
		t.Error("sem filtro, Back Squat deveria estar disponível")
	}
}

func TestGenerateBlock_LacunaPoolVazioFalha(t *testing.T) {
	// Todos os exercícios exigem Barra(1); atleta só tem Rack(2) -> nenhum viável -> erro explícito.
	repo := blockRepo()
	repo.exercises = []domain.Exercise{
		{ID: 1, Name: "X", MovementPattern: "squat", Level: "beginner", Focus: "strength"},
		{ID: 2, Name: "Y", MovementPattern: "hinge", Level: "beginner", Focus: "strength"},
	}
	repo.exerciseEquip = map[int][]int{1: {1}, 2: {1}}
	repo.subRules = nil
	repo.userEquip = []int{2} // tem Rack, não tem Barra
	svc := New(repo)

	if _, err := svc.GenerateBlock(1); err == nil {
		t.Fatal("esperava erro de lacuna (nenhum exercício viável), veio nil")
	}
}
