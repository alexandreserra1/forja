// Testes da Fase 2: read model (M2), detector de estagnação (M3) e ajuste com
// tetos de segurança (M4). Tudo isolado, com o fakeRepo em memória — sem HTTP, sem banco.
package service

import (
	"testing"

	"treino/internal/domain"
)

func fptr(v float64) *float64 { return &v }

// ---- M2: read model previsto-vs-realizado ----

func TestComposeWeek_MediasEContagem(t *testing.T) {
	actuals := []domain.SessionActual{
		{PrescriptionID: 1, TargetRPE: 7, ActualRPE: fptr(8), Done: true},
		{PrescriptionID: 2, TargetRPE: 7, ActualRPE: fptr(9), Done: true},
		{PrescriptionID: 3, TargetRPE: 7, ActualRPE: nil}, // sem registro
	}
	s := composeWeek(actuals)
	if s.total != 3 || s.logged != 2 {
		t.Fatalf("contagem errada: total=%d logged=%d", s.total, s.logged)
	}
	if s.meanActual != 8.5 || s.meanTarget != 7 {
		t.Fatalf("médias erradas: actual=%.2f target=%.2f", s.meanActual, s.meanTarget)
	}
}

// ---- M3: detector (regra das 2 semanas é de quem orquestra; aqui é por semana) ----

func TestClassify_EsforcoSubindoEhSinal(t *testing.T) {
	// actual médio 1 ponto acima do alvo, tudo registrado => estagnação.
	a := []domain.SessionActual{
		{TargetRPE: 7, ActualRPE: fptr(8), Done: true},
		{TargetRPE: 7, ActualRPE: fptr(8), Done: true},
	}
	if got := composeWeek(a).classify(); got != signalStagnation {
		t.Fatalf("esperava signalStagnation, veio %v", got)
	}
}

func TestClassify_DentroDoEsperadoEhOK(t *testing.T) {
	a := []domain.SessionActual{
		{TargetRPE: 7, ActualRPE: fptr(7), Done: true},
		{TargetRPE: 7, ActualRPE: fptr(7), Done: true},
	}
	if got := composeWeek(a).classify(); got != signalOK {
		t.Fatalf("esperava signalOK, veio %v", got)
	}
}

func TestClassify_PoucoRegistroEhInconclusivo(t *testing.T) {
	// 1 de 4 registrado (< 50%) => não infere, pede registro.
	a := []domain.SessionActual{
		{TargetRPE: 7, ActualRPE: fptr(9), Done: true},
		{TargetRPE: 7, ActualRPE: nil},
		{TargetRPE: 7, ActualRPE: nil},
		{TargetRPE: 7, ActualRPE: nil},
	}
	if got := composeWeek(a).classify(); got != signalInconclusive {
		t.Fatalf("esperava signalInconclusive, veio %v", got)
	}
}

// ---- M3/M4 via orquestrador: detector + ajuste de ponta a ponta (sem HTTP) ----

// weekPrescriptions devolve as prescrições de uma semana do bloco gerado no fakeRepo.
func weekPrescriptions(f *fakeRepo, weekNumber int) []domain.Prescription {
	var weekID int
	for _, w := range f.weeks {
		if w.WeekNumber == weekNumber {
			weekID = w.ID
		}
	}
	sess := map[int]bool{}
	for _, s := range f.sessions {
		if s.WeekID == weekID {
			sess[s.ID] = true
		}
	}
	var ps []domain.Prescription
	for _, p := range f.prescriptions {
		if sess[p.SessionID] {
			ps = append(ps, p)
		}
	}
	return ps
}

// logWeek marca todas as prescrições da semana como feitas com actual_rpe = target + delta.
func logWeek(t *testing.T, svc *Service, f *fakeRepo, weekNumber int, delta float64) {
	t.Helper()
	for _, p := range weekPrescriptions(f, weekNumber) {
		if err := svc.MarkDone(p.ID, fptr(p.TargetRPE+delta), ""); err != nil {
			t.Fatalf("MarkDone semana %d: %v", weekNumber, err)
		}
	}
}

func TestEvaluate_DuasSemanasDeSinalAliviaProxima(t *testing.T) {
	repo := blockRepo()
	svc := New(repo)
	if _, err := svc.GenerateBlock(1); err != nil { // 8 semanas, strength
		t.Fatalf("GenerateBlock: %v", err)
	}
	// Semanas 1 e 2 com esforço 1 ponto acima do alvo => sinal nas duas.
	logWeek(t, svc, repo, 1, rpeSignalDelta)
	logWeek(t, svc, repo, 2, rpeSignalDelta)

	// Guarda o sets previsto da semana 3 ANTES do ajuste.
	before := weekPrescriptions(repo, 3)
	if len(before) == 0 {
		t.Fatal("semana 3 sem prescrições")
	}
	setsBefore, rpeBefore := before[0].Sets, before[0].TargetRPE

	res, err := svc.EvaluateAndAdjust(1)
	if err != nil {
		t.Fatalf("EvaluateAndAdjust: %v", err)
	}
	if res.Action != "reduce_volume" {
		t.Fatalf("esperava reduce_volume, veio %q (%s)", res.Action, res.Explanation)
	}

	// A semana 3 deve ter sido ALIVIADA: menos séries, RPE não maior.
	after := weekPrescriptions(repo, 3)
	for _, p := range after {
		if p.Sets >= setsBefore {
			t.Errorf("séries não reduziram: antes=%d depois=%d", setsBefore, p.Sets)
		}
		if p.TargetRPE > rpeBefore {
			t.Errorf("TETO VIOLADO: RPE subiu de %.1f para %.1f", rpeBefore, p.TargetRPE)
		}
	}
	// Rastro gravado.
	adjs, _ := svc.Adjustments(1)
	if len(adjs) != 1 || adjs[0].Explanation == "" {
		t.Fatalf("esperava 1 ajuste com explicação, veio %+v", adjs)
	}
}

func TestEvaluate_UmaSemanaRuimNaoDispara(t *testing.T) {
	repo := blockRepo()
	svc := New(repo)
	if _, err := svc.GenerateBlock(1); err != nil {
		t.Fatalf("GenerateBlock: %v", err)
	}
	logWeek(t, svc, repo, 1, rpeSignalDelta) // só uma semana de sinal
	logWeek(t, svc, repo, 2, 0)              // semana 2 dentro do esperado

	res, err := svc.EvaluateAndAdjust(1)
	if err != nil {
		t.Fatalf("EvaluateAndAdjust: %v", err)
	}
	if res.Action != "none" {
		t.Fatalf("uma semana ruim não deveria disparar; veio %q", res.Action)
	}
	if adjs, _ := svc.Adjustments(1); len(adjs) != 0 {
		t.Fatalf("não deveria haver ajuste, veio %d", len(adjs))
	}
}

func TestEvaluate_SemRegistroPedeRegistro(t *testing.T) {
	repo := blockRepo()
	svc := New(repo)
	if _, err := svc.GenerateBlock(1); err != nil {
		t.Fatalf("GenerateBlock: %v", err)
	}
	// Nada logado.
	res, err := svc.EvaluateAndAdjust(1)
	if err != nil {
		t.Fatalf("EvaluateAndAdjust: %v", err)
	}
	if res.Action != "needs_log" {
		t.Fatalf("sem registro deveria pedir registro; veio %q", res.Action)
	}
}

func TestEvaluate_Idempotente(t *testing.T) {
	repo := blockRepo()
	svc := New(repo)
	if _, err := svc.GenerateBlock(1); err != nil {
		t.Fatalf("GenerateBlock: %v", err)
	}
	logWeek(t, svc, repo, 1, rpeSignalDelta)
	logWeek(t, svc, repo, 2, rpeSignalDelta)

	if _, err := svc.EvaluateAndAdjust(1); err != nil {
		t.Fatalf("1ª avaliação: %v", err)
	}
	res, err := svc.EvaluateAndAdjust(1) // reavaliar não deve empilhar
	if err != nil {
		t.Fatalf("2ª avaliação: %v", err)
	}
	if res.Action != "none" {
		t.Fatalf("reavaliar deveria ser no-op, veio %q", res.Action)
	}
	if adjs, _ := svc.Adjustments(1); len(adjs) != 1 {
		t.Fatalf("esperava exatamente 1 ajuste após reavaliar, veio %d", len(adjs))
	}
}

// ---- M4: planAdjustment escolhe a ação certa e NUNCA sobe carga ----

func TestPlanAdjustment_VolumeMinimoReduzRPE(t *testing.T) {
	target := domain.BlockWeek{ID: 50, WeekNumber: 4}
	actuals := []domain.SessionActual{
		{PrescriptionID: 1, Sets: minSets, TargetRPE: 9},
		{PrescriptionID: 2, Sets: minSets, TargetRPE: 9},
	}
	adj, updated := planAdjustment(7, target, actuals, 2)
	if adj.Action != "reduce_rpe" {
		t.Fatalf("volume mínimo deveria reduzir RPE, veio %q", adj.Action)
	}
	for _, p := range updated {
		if p.Sets != minSets {
			t.Errorf("não deveria mexer no volume já mínimo: %d", p.Sets)
		}
		if p.TargetRPE >= 9 {
			t.Errorf("RPE deveria cair, veio %.1f", p.TargetRPE)
		}
	}
}

func TestPlanAdjustment_SinalPersistenteDeloadReativo(t *testing.T) {
	target := domain.BlockWeek{ID: 60, WeekNumber: 5}
	actuals := []domain.SessionActual{
		{PrescriptionID: 1, Sets: 4, TargetRPE: 8},
		{PrescriptionID: 2, Sets: 4, TargetRPE: 8},
	}
	adj, updated := planAdjustment(7, target, actuals, severeStagnationRun) // 3 semanas
	if adj.Action != "reactive_deload" {
		t.Fatalf("sinal persistente deveria virar deload reativo, veio %q", adj.Action)
	}
	for _, p := range updated {
		if p.Sets > minSets || p.TargetRPE >= 8 {
			t.Errorf("deload reativo deveria cortar volume e RPE: sets=%d rpe=%.1f", p.Sets, p.TargetRPE)
		}
	}
}
