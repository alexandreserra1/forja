package service

import (
	"testing"

	"treino/internal/domain"
)

// wodRepo constrói um fakeRepo com um bloco de 2 semanas e WODs pré-configurados nas semanas.
// semana 1 (weekID=1): 2 WODs done com RPE dado; semana 2 (weekID=2): 1 WOD não-feito (candidato a skip).
func wodRepo(rpe1, rpe2 float64) *fakeRepo {
	repo := blockRepo()
	// Gera o bloco para ter estrutura de semanas.
	svc := New(repo)
	svc.GenerateBlock(1) //nolint
	block, _ := svc.ActiveBlock(1)
	if block == nil || len(block.Weeks) < 2 {
		return repo
	}
	w1ID := block.Weeks[0].ID
	w2ID := block.Weeks[1].ID

	// WOD 1 e 2 da semana 1: feitos com RPE dado.
	repo.wodPrescriptions = []fakeCondPrescription{
		{ConditioningPrescription: domain.ConditioningPrescription{ID: 1001, TargetRPE: 7, Done: true, ActualRPE: &rpe1}, weekID: w1ID},
		{ConditioningPrescription: domain.ConditioningPrescription{ID: 1002, TargetRPE: 7, Done: true, ActualRPE: &rpe2}, weekID: w1ID},
		// WOD da semana 2: não feito, candidato a skip.
		{ConditioningPrescription: domain.ConditioningPrescription{ID: 2001, TargetRPE: 7}, weekID: w2ID},
	}

	// GetConditioning usa condBySession: popula a 1ª sessão da semana 2 com o WOD candidato.
	sessions2, _ := svc.WeekDetail(1, block.Weeks[1].WeekNumber)
	if sessions2 != nil && len(sessions2.Sessions) > 0 {
		sessID := sessions2.Sessions[0].Session.ID
		if repo.condBySession == nil {
			repo.condBySession = map[int][]domain.ConditioningPrescription{}
		}
		repo.condBySession[sessID] = []domain.ConditioningPrescription{
			{ID: 2001, SessionID: sessID, TargetRPE: 7, SortOrder: 1},
		}
	}
	return repo
}

func TestEvaluateWodAutoregulation_RPEAlto(t *testing.T) {
	repo := wodRepo(9.0, 9.5) // avg 9.25 > 8.0 → deve skipar
	svc := New(repo)

	action, expl, err := svc.EvaluateWodAutoregulation(1)
	if err != nil {
		t.Fatalf("EvaluateWodAutoregulation: %v", err)
	}
	if action != "reduce_wod_dose" {
		t.Errorf("esperava reduce_wod_dose, veio %q (expl: %s)", action, expl)
	}
	// WOD 2001 deve estar skipado.
	skipped := false
	for _, cp := range repo.wodPrescriptions {
		if cp.ID == 2001 && cp.skipped {
			skipped = true
		}
	}
	if !skipped {
		t.Error("WOD 2001 deveria estar marcado como skipped")
	}
	// Rastro gravado em adjustments.
	if len(repo.adjustments) == 0 || repo.adjustments[0].Action != "reduce_wod_dose" {
		t.Error("ajuste não foi gravado no rastro")
	}
}

func TestEvaluateWodAutoregulation_RPENormal(t *testing.T) {
	repo := wodRepo(7.0, 7.5) // avg 7.25 <= 8.0 → sem ajuste
	svc := New(repo)

	action, _, err := svc.EvaluateWodAutoregulation(1)
	if err != nil {
		t.Fatalf("EvaluateWodAutoregulation: %v", err)
	}
	if action != "none" {
		t.Errorf("RPE normal não deveria gerar ajuste, veio %q", action)
	}
	for _, cp := range repo.wodPrescriptions {
		if cp.skipped {
			t.Error("nenhum WOD deveria ser skipado com RPE normal")
		}
	}
}

func TestEvaluateWodAutoregulation_SemRegistro(t *testing.T) {
	repo := blockRepo()
	svc := New(repo)
	svc.GenerateBlock(1) //nolint

	action, _, err := svc.EvaluateWodAutoregulation(1)
	if err != nil {
		t.Fatalf("EvaluateWodAutoregulation: %v", err)
	}
	if action != "needs_log" {
		t.Errorf("sem WODs feitos deveria ser needs_log, veio %q", action)
	}
}

func TestEvaluateAndAdjust_ForcaInaleravel(t *testing.T) {
	// Garante que EvaluateAndAdjust (força) continua funcionando e não toca nos WODs.
	repo := blockRepo()
	svc := New(repo)
	svc.GenerateBlock(1) //nolint

	result, err := svc.EvaluateAndAdjust(1)
	if err != nil {
		t.Fatalf("EvaluateAndAdjust: %v", err)
	}
	// Sem registro = needs_log (força intacto).
	if result.Action != "needs_log" {
		t.Errorf("sem log de força deveria ser needs_log, veio %q", result.Action)
	}
}
