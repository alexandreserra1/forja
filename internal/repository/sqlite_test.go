// Teste do M2: prova que a implementação SQLite lê o banco de ponta a ponta.
// Constrói um banco temporário aplicando o schema + seed reais e exercita cada método.
package repository

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3" // registra o driver "sqlite3"

	"treino/internal/domain"
)

// newTestDB cria um banco temporário com schema + seed aplicados.
func newTestDB(t *testing.T) *SQLiteRepo {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	// _foreign_keys=on: sem isto o SQLite ignora as FKs e o teste de rollback não dispara.
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		t.Fatalf("abrir db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	for _, f := range []string{"schema.sql", "seed.sql"} {
		sqlBytes, err := os.ReadFile(filepath.Join("..", "..", "db", f))
		if err != nil {
			t.Fatalf("ler %s: %v", f, err)
		}
		if _, err := db.Exec(string(sqlBytes)); err != nil {
			t.Fatalf("aplicar %s: %v", f, err)
		}
	}
	return New(db)
}

func TestListQuestions(t *testing.T) {
	repo := newTestDB(t)
	questions, err := repo.ListQuestions()
	if err != nil {
		t.Fatalf("ListQuestions: %v", err)
	}
	if len(questions) != 4 {
		t.Fatalf("esperava 4 perguntas, veio %d", len(questions))
	}
	// Ordenadas por sort_order: a primeira é "Há quanto tempo você treina?"
	if questions[0].SortOrder != 1 {
		t.Errorf("primeira pergunta deveria ter sort_order 1, veio %d", questions[0].SortOrder)
	}
	// A primeira pergunta tem 3 opções aninhadas.
	if len(questions[0].Options) != 3 {
		t.Errorf("pergunta 1 deveria ter 3 opções, veio %d", len(questions[0].Options))
	}
	// show_when é NULL no seed da Fase 0.
	if questions[0].ShowWhen != nil {
		t.Errorf("show_when deveria ser nil na Fase 0, veio %v", *questions[0].ShowWhen)
	}
}

func TestListByLevel(t *testing.T) {
	repo := newTestDB(t)
	// Nível exato (sem cascata — a cascata é regra do service).
	beginner, err := repo.ListByLevel("beginner")
	if err != nil {
		t.Fatalf("ListByLevel: %v", err)
	}
	if len(beginner) == 0 {
		t.Fatal("esperava exercícios beginner no seed")
	}
	for _, e := range beginner {
		if e.Level != "beginner" {
			t.Errorf("%s não é beginner (é %s)", e.Name, e.Level)
		}
		if e.MovementPattern == "" {
			t.Errorf("%s veio sem padrão de movimento resolvido", e.Name)
		}
	}
}

func TestSaveAndListAnswers(t *testing.T) {
	repo := newTestDB(t)
	in := []domain.Answer{
		{QuestionID: 1, AnswerValue: "lt_1y"},
		{QuestionID: 2, AnswerValue: "3"},
		{QuestionID: 3, AnswerValue: "technique"},
	}
	if err := repo.SaveAnswers(1, in); err != nil {
		t.Fatalf("SaveAnswers: %v", err)
	}
	out, err := repo.ListAnswers(1)
	if err != nil {
		t.Fatalf("ListAnswers: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("esperava 3 respostas, veio %d", len(out))
	}
	// Reenviar substitui (não duplica): grava só 1 e confere que sobrou 1.
	if err := repo.SaveAnswers(1, []domain.Answer{{QuestionID: 1, AnswerValue: "gt_3y"}}); err != nil {
		t.Fatalf("SaveAnswers (2ª vez): %v", err)
	}
	out, _ = repo.ListAnswers(1)
	if len(out) != 1 {
		t.Fatalf("reenvio deveria substituir; esperava 1, veio %d", len(out))
	}
}

func TestListAnswerRules(t *testing.T) {
	repo := newTestDB(t)
	rules, err := repo.ListAnswerRules()
	if err != nil {
		t.Fatalf("ListAnswerRules: %v", err)
	}
	// 8 da Fase 0 + 3 de 'weeks' da Fase 1 = 11.
	if len(rules) != 11 {
		t.Fatalf("esperava 11 regras no seed, veio %d", len(rules))
	}
}

// ---- Fase 1 ----

func TestListPhaseTemplates(t *testing.T) {
	repo := newTestDB(t)
	templates, err := repo.ListPhaseTemplates("strength")
	if err != nil {
		t.Fatalf("ListPhaseTemplates: %v", err)
	}
	if len(templates) != 4 {
		t.Fatalf("esperava 4 moldes p/ strength, veio %d", len(templates))
	}
	// Em ordem de sort_order: accumulation é o primeiro.
	if templates[0].Phase != "accumulation" {
		t.Errorf("1ª fase deveria ser accumulation, veio %q", templates[0].Phase)
	}
}

// smallBlock monta uma árvore mínima p/ os testes de persistência.
func smallBlock() domain.GeneratedBlock {
	rpe := 6.5
	return domain.GeneratedBlock{
		Block: domain.TrainingBlock{AthleteID: 1, Goal: "strength", TotalWeeks: 8, DaysPerWeek: 3},
		Weeks: []domain.GeneratedWeek{
			{
				Week: domain.BlockWeek{WeekNumber: 1, Phase: "accumulation", TargetRPE: rpe},
				Sessions: []domain.GeneratedSession{
					{
						Session: domain.BlockSession{DayNumber: 1},
						Prescriptions: []domain.Prescription{
							{ExerciseID: 1, Sets: 4, Reps: 6, TargetRPE: rpe, SortOrder: 1},
							{ExerciseID: 5, Sets: 4, Reps: 6, TargetRPE: rpe, SortOrder: 2},
						},
					},
				},
			},
		},
	}
}

func TestSaveGeneratedBlockRoundTrip(t *testing.T) {
	repo := newTestDB(t)
	if err := repo.SaveGeneratedBlock(smallBlock()); err != nil {
		t.Fatalf("SaveGeneratedBlock: %v", err)
	}

	block, err := repo.GetActiveBlock(1)
	if err != nil || block == nil {
		t.Fatalf("GetActiveBlock: %v (block=%v)", err, block)
	}
	if block.Goal != "strength" || block.TotalWeeks != 8 {
		t.Errorf("bloco lido errado: %+v", block)
	}

	weeks, err := repo.GetBlockWeeks(block.ID)
	if err != nil || len(weeks) != 1 {
		t.Fatalf("GetBlockWeeks: %v (n=%d)", err, len(weeks))
	}
	sessions, err := repo.GetSessions(weeks[0].ID)
	if err != nil || len(sessions) != 1 {
		t.Fatalf("GetSessions: %v (n=%d)", err, len(sessions))
	}
	prescriptions, err := repo.GetPrescriptions(sessions[0].ID)
	if err != nil || len(prescriptions) != 2 {
		t.Fatalf("GetPrescriptions: %v (n=%d)", err, len(prescriptions))
	}
	if prescriptions[0].ExerciseName == "" {
		t.Error("prescrição deveria ter nome do exercício resolvido")
	}
	if prescriptions[0].Done {
		t.Error("prescrição recém-criada não deveria estar 'done'")
	}
}

func TestSaveGeneratedBlockIsTransactional(t *testing.T) {
	repo := newTestDB(t)
	// Injeta uma prescrição com exercise_id inexistente -> viola a FK no meio da árvore.
	bad := smallBlock()
	bad.Weeks[0].Sessions[0].Prescriptions[1].ExerciseID = 99999
	if err := repo.SaveGeneratedBlock(bad); err == nil {
		t.Fatal("esperava erro de FK, veio nil")
	}
	// Nada pode ter sido gravado (rollback).
	block, err := repo.GetActiveBlock(1)
	if err != nil {
		t.Fatalf("GetActiveBlock: %v", err)
	}
	if block != nil {
		t.Fatalf("transação deveria ter feito rollback; bloco não deveria existir: %+v", block)
	}
}

func TestArchiveAndMarkDone(t *testing.T) {
	repo := newTestDB(t)
	if err := repo.SaveGeneratedBlock(smallBlock()); err != nil {
		t.Fatalf("SaveGeneratedBlock: %v", err)
	}

	// Arquivar: depois de arquivar, não há bloco ativo.
	if err := repo.ArchiveActiveBlock(1); err != nil {
		t.Fatalf("ArchiveActiveBlock: %v", err)
	}
	block, _ := repo.GetActiveBlock(1)
	if block != nil {
		t.Fatalf("após arquivar não deveria haver bloco ativo, veio %+v", block)
	}

	// Gera outro p/ testar MarkPrescriptionDone (upsert).
	if err := repo.SaveGeneratedBlock(smallBlock()); err != nil {
		t.Fatalf("SaveGeneratedBlock 2: %v", err)
	}
	block, _ = repo.GetActiveBlock(1)
	weeks, _ := repo.GetBlockWeeks(block.ID)
	sessions, _ := repo.GetSessions(weeks[0].ID)
	prescriptions, _ := repo.GetPrescriptions(sessions[0].ID)
	pid := prescriptions[0].ID

	rpe := 7.5
	if err := repo.MarkPrescriptionDone(pid, &rpe, "ok"); err != nil {
		t.Fatalf("MarkPrescriptionDone: %v", err)
	}
	// Marcar de novo (upsert) não duplica nem falha.
	if err := repo.MarkPrescriptionDone(pid, &rpe, "de novo"); err != nil {
		t.Fatalf("MarkPrescriptionDone 2: %v", err)
	}
	prescriptions, _ = repo.GetPrescriptions(sessions[0].ID)
	if !prescriptions[0].Done {
		t.Error("prescrição deveria estar marcada como feita")
	}
	if prescriptions[0].ActualRPE == nil || *prescriptions[0].ActualRPE != 7.5 {
		t.Errorf("actual_rpe deveria ser 7.5, veio %v", prescriptions[0].ActualRPE)
	}
}

// ---- Fase 2 ----

// activeSmallBlock grava o smallBlock e devolve o id da prescrição 1 e da semana 1.
func activeSmallBlock(t *testing.T, repo *SQLiteRepo) (weekID, pid int) {
	t.Helper()
	if err := repo.SaveGeneratedBlock(smallBlock()); err != nil {
		t.Fatalf("SaveGeneratedBlock: %v", err)
	}
	block, _ := repo.GetActiveBlock(1)
	weeks, _ := repo.GetBlockWeeks(block.ID)
	sessions, _ := repo.GetSessions(weeks[0].ID)
	prescriptions, _ := repo.GetPrescriptions(sessions[0].ID)
	return weeks[0].ID, prescriptions[0].ID
}

func TestGetWeekActuals(t *testing.T) {
	repo := newTestDB(t)
	weekID, pid := activeSmallBlock(t, repo)

	// Antes de logar: 2 prescrições, ambas sem registro.
	actuals, err := repo.GetWeekActuals(weekID)
	if err != nil {
		t.Fatalf("GetWeekActuals: %v", err)
	}
	if len(actuals) != 2 {
		t.Fatalf("esperava 2 prescrições na semana, veio %d", len(actuals))
	}
	for _, a := range actuals {
		if a.Done || a.ActualRPE != nil {
			t.Errorf("sem registro, não deveria estar done nem ter actual_rpe: %+v", a)
		}
	}

	// Depois de logar uma: o realizado aparece ao lado do previsto.
	rpe := 8.5
	if err := repo.MarkPrescriptionDone(pid, &rpe, "puxado"); err != nil {
		t.Fatalf("MarkPrescriptionDone: %v", err)
	}
	actuals, _ = repo.GetWeekActuals(weekID)
	var logged int
	for _, a := range actuals {
		if a.PrescriptionID == pid {
			if !a.Done || a.ActualRPE == nil || *a.ActualRPE != 8.5 {
				t.Errorf("prescrição logada veio errada: %+v", a)
			}
			logged++
		}
	}
	if logged != 1 {
		t.Errorf("esperava exatamente 1 prescrição logada, veio %d", logged)
	}
}

func TestApplyAdjustmentRoundTrip(t *testing.T) {
	repo := newTestDB(t)
	weekID, pid := activeSmallBlock(t, repo)
	block, _ := repo.GetActiveBlock(1)

	before, _ := repo.GetWeekActuals(weekID)
	setsBefore := before[0].Sets

	sb, sa := setsBefore, setsBefore-1
	adj := domain.AutoregAdjustment{
		BlockID: block.ID, WeekID: weekID, Trigger: "stagnation_2w", Action: "reduce_volume",
		SetsBefore: &sb, SetsAfter: &sa, Explanation: "reduzimos o volume",
	}
	updated := []domain.Prescription{{ID: pid, Sets: setsBefore - 1, TargetRPE: before[0].TargetRPE}}
	if err := repo.ApplyAdjustment(adj, updated); err != nil {
		t.Fatalf("ApplyAdjustment: %v", err)
	}

	// A prescrição foi reescrita.
	after, _ := repo.GetWeekActuals(weekID)
	for _, a := range after {
		if a.PrescriptionID == pid && a.Sets != setsBefore-1 {
			t.Errorf("séries deveriam ter caído p/ %d, veio %d", setsBefore-1, a.Sets)
		}
	}
	// O rastro ficou no histórico.
	adjs, err := repo.ListAdjustments(block.ID)
	if err != nil {
		t.Fatalf("ListAdjustments: %v", err)
	}
	if len(adjs) != 1 || adjs[0].Explanation == "" || adjs[0].CreatedAt == "" {
		t.Fatalf("esperava 1 ajuste com explicação e data, veio %+v", adjs)
	}
}

func TestApplyAdjustmentIsTransactional(t *testing.T) {
	repo := newTestDB(t)
	weekID, pid := activeSmallBlock(t, repo)
	block, _ := repo.GetActiveBlock(1)

	// week_id inválido viola a FK ao gravar o rastro -> a reescrita da prescrição deve reverter.
	adj := domain.AutoregAdjustment{
		BlockID: block.ID, WeekID: 999999, Trigger: "stagnation_2w", Action: "reduce_volume",
		Explanation: "deveria falhar",
	}
	updated := []domain.Prescription{{ID: pid, Sets: 1, TargetRPE: 1}}
	if err := repo.ApplyAdjustment(adj, updated); err == nil {
		t.Fatal("esperava erro de FK no week_id, veio nil")
	}

	// Rollback: a prescrição NÃO pode ter sido alterada e nenhum ajuste pode existir.
	after, _ := repo.GetWeekActuals(weekID)
	for _, a := range after {
		if a.PrescriptionID == pid && (a.Sets == 1 || a.TargetRPE == 1) {
			t.Errorf("prescrição foi alterada apesar do rollback: %+v", a)
		}
	}
	if adjs, _ := repo.ListAdjustments(block.ID); len(adjs) != 0 {
		t.Fatalf("não deveria haver ajuste após rollback, veio %d", len(adjs))
	}
}

// ---- Fase 3 ----

func TestEquipmentCatalogAndUser(t *testing.T) {
	repo := newTestDB(t)

	cat, err := repo.ListEquipment()
	if err != nil {
		t.Fatalf("ListEquipment: %v", err)
	}
	if len(cat) != 13 {
		t.Fatalf("esperava 13 equipamentos no seed (Fase 5B: +Remador/Corda/Bike), veio %d", len(cat))
	}

	// Atleta sem equipamento ainda.
	if u, _ := repo.ListUserEquipment(1); len(u) != 0 {
		t.Fatalf("atleta não deveria ter equipamento ainda, veio %d", len(u))
	}
	// Grava Barra(1) + Rack(2); regrava só Barra (substitui, não acumula).
	if err := repo.SetUserEquipment(1, []int{1, 2}); err != nil {
		t.Fatalf("SetUserEquipment: %v", err)
	}
	if u, _ := repo.ListUserEquipment(1); len(u) != 2 {
		t.Fatalf("esperava 2 equipamentos, veio %d", len(u))
	}
	if err := repo.SetUserEquipment(1, []int{1}); err != nil {
		t.Fatalf("SetUserEquipment 2: %v", err)
	}
	if u, _ := repo.ListUserEquipment(1); len(u) != 1 || u[0].ID != 1 {
		t.Fatalf("regravar deveria substituir; veio %+v", u)
	}
}

func TestListExerciseEquipment(t *testing.T) {
	repo := newTestDB(t)
	// Back Squat (id 2) exige Barra + Rack.
	eq, err := repo.ListExerciseEquipment(2)
	if err != nil {
		t.Fatalf("ListExerciseEquipment: %v", err)
	}
	if len(eq) != 2 {
		t.Fatalf("Back Squat deveria exigir 2 equipamentos, veio %d", len(eq))
	}
	// Air Squat (id 1) não exige nada (peso do corpo).
	if eq, _ := repo.ListExerciseEquipment(1); len(eq) != 0 {
		t.Fatalf("Air Squat não deveria exigir equipamento, veio %d", len(eq))
	}
}

func TestListAvailableByLevel(t *testing.T) {
	repo := newTestDB(t)

	// Sem equipamento marcado -> sem filtro (= ListByLevel).
	full, _ := repo.ListByLevel("intermediate")
	avail, err := repo.ListAvailableByLevel("intermediate", nil)
	if err != nil {
		t.Fatalf("ListAvailableByLevel(nil): %v", err)
	}
	if len(avail) != len(full) {
		t.Fatalf("equipamento vazio deveria ser sem filtro: %d vs %d", len(avail), len(full))
	}

	// Atleta com Barra(1) mas SEM Rack(2): Back Squat (exige Barra+Rack) some;
	// Deadlift e Strict Press (só Barra) ficam.
	avail, _ = repo.ListAvailableByLevel("intermediate", []int{1})
	names := map[string]bool{}
	for _, e := range avail {
		names[e.Name] = true
	}
	if names["Back Squat"] {
		t.Error("Back Squat exige Rack; não deveria estar disponível sem Rack")
	}
	if !names["Deadlift"] || !names["Strict Press"] {
		t.Errorf("Deadlift/Strict Press (só Barra) deveriam estar disponíveis: %v", names)
	}
}

func TestGetSubstitutionRule(t *testing.T) {
	repo := newTestDB(t)
	// squat + falta Rack(2) -> Overhead Squat (seed).
	sub, err := repo.GetSubstitutionRule("squat", 2)
	if err != nil {
		t.Fatalf("GetSubstitutionRule: %v", err)
	}
	if sub == nil || sub.Name != "Overhead Squat" {
		t.Fatalf("esperava Overhead Squat como substituto, veio %v", sub)
	}
	// Sem regra para esse caso -> nil, sem erro.
	none, err := repo.GetSubstitutionRule("hinge", 99)
	if err != nil {
		t.Fatalf("GetSubstitutionRule (sem regra): %v", err)
	}
	if none != nil {
		t.Fatalf("não deveria haver regra; veio %+v", none)
	}
}

func TestFindCandidatesForPhase(t *testing.T) {
	repo := newTestDB(t)

	// Estímulo strength, nível intermediate, sem filtro de equipamento: só exercícios strength.
	cands, err := repo.FindCandidatesForPhase("strength", "intermediate", nil)
	if err != nil {
		t.Fatalf("FindCandidatesForPhase: %v", err)
	}
	if len(cands) == 0 {
		t.Fatal("esperava candidatos strength/intermediate no catálogo")
	}
	for _, e := range cands {
		if e.Focus != "strength" || e.Level != "intermediate" {
			t.Errorf("candidato fora do filtro: %s (focus=%s level=%s)", e.Name, e.Focus, e.Level)
		}
	}

	// Estímulo technique, com equipamento só Barra(1): nenhum candidato pode exigir outro equip.
	tech, err := repo.FindCandidatesForPhase("technique", "advanced", []int{1})
	if err != nil {
		t.Fatalf("FindCandidatesForPhase (technique, Barra): %v", err)
	}
	for _, e := range tech {
		if e.Focus != "technique" {
			t.Errorf("%s não é technique", e.Name)
		}
		req, _ := repo.ListExerciseEquipment(e.ID)
		for _, q := range req {
			if q.ID != 1 {
				t.Errorf("%s exige %s (id %d) além de Barra — não deveria estar disponível", e.Name, q.Name, q.ID)
			}
		}
	}
}

func TestAthleteScoping(t *testing.T) {
	repo := newTestDB(t)
	// Atleta 1 já vem do seed; cria o atleta 2.
	a2, err := repo.CreateAthlete("Atleta 2")
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}
	if a2.ID == 1 {
		t.Fatalf("atleta 2 não deveria ter id 1")
	}

	// Respostas isoladas por atleta.
	if err := repo.SaveAnswers(1, []domain.Answer{{QuestionID: 1, AnswerValue: "lt_1y"}}); err != nil {
		t.Fatalf("SaveAnswers a1: %v", err)
	}
	if err := repo.SaveAnswers(a2.ID, []domain.Answer{
		{QuestionID: 1, AnswerValue: "gt_3y"}, {QuestionID: 2, AnswerValue: "4"}}); err != nil {
		t.Fatalf("SaveAnswers a2: %v", err)
	}
	if got, _ := repo.ListAnswers(1); len(got) != 1 || got[0].AnswerValue != "lt_1y" {
		t.Errorf("respostas do atleta 1 contaminadas: %+v", got)
	}
	if got, _ := repo.ListAnswers(a2.ID); len(got) != 2 {
		t.Errorf("atleta 2 deveria ter 2 respostas, veio %+v", got)
	}

	// Equipamento isolado por atleta.
	if err := repo.SetUserEquipment(1, []int{1, 2}); err != nil {
		t.Fatalf("SetUserEquipment a1: %v", err)
	}
	if err := repo.SetUserEquipment(a2.ID, []int{3}); err != nil {
		t.Fatalf("SetUserEquipment a2: %v", err)
	}
	if got, _ := repo.ListUserEquipment(1); len(got) != 2 {
		t.Errorf("equipamento do atleta 1: esperava 2, veio %+v", got)
	}
	if got, _ := repo.ListUserEquipment(a2.ID); len(got) != 1 || got[0].ID != 3 {
		t.Errorf("equipamento do atleta 2: esperava só id 3, veio %+v", got)
	}

	// Blocos ativos SIMULTÂNEOS, um por atleta.
	if err := repo.SaveGeneratedBlock(smallBlock()); err != nil { // atleta 1
		t.Fatalf("SaveGeneratedBlock a1: %v", err)
	}
	b2 := smallBlock()
	b2.Block.AthleteID = a2.ID
	if err := repo.SaveGeneratedBlock(b2); err != nil {
		t.Fatalf("SaveGeneratedBlock a2: %v", err)
	}
	if b, _ := repo.GetActiveBlock(1); b == nil || b.AthleteID != 1 {
		t.Errorf("bloco ativo do atleta 1 errado: %+v", b)
	}
	if b, _ := repo.GetActiveBlock(a2.ID); b == nil || b.AthleteID != a2.ID {
		t.Errorf("bloco ativo do atleta 2 errado: %+v", b)
	}

	// Arquivar o bloco do atleta 1 NÃO afeta o do atleta 2.
	if err := repo.ArchiveActiveBlock(1); err != nil {
		t.Fatalf("ArchiveActiveBlock a1: %v", err)
	}
	if b, _ := repo.GetActiveBlock(1); b != nil {
		t.Errorf("atleta 1 não deveria ter bloco ativo após arquivar: %+v", b)
	}
	if b, _ := repo.GetActiveBlock(a2.ID); b == nil {
		t.Error("atleta 2 deveria manter seu bloco ativo (arquivar o do 1 não o afeta)")
	}
}

func TestRecentExerciseIDs(t *testing.T) {
	repo := newTestDB(t)

	// Atleta sem bloco -> vazio (1ª geração não tem o que evitar).
	if ids, err := repo.RecentExerciseIDs(1); err != nil || len(ids) != 0 {
		t.Fatalf("sem bloco deveria ser vazio, veio %v (err %v)", ids, err)
	}

	// Grava um bloco (exercícios 1 e 5) para o atleta 1.
	if err := repo.SaveGeneratedBlock(smallBlock()); err != nil {
		t.Fatalf("SaveGeneratedBlock: %v", err)
	}
	got, err := repo.RecentExerciseIDs(1)
	if err != nil {
		t.Fatalf("RecentExerciseIDs: %v", err)
	}
	set := map[int]bool{}
	for _, id := range got {
		set[id] = true
	}
	if !set[1] || !set[5] || len(got) != 2 {
		t.Fatalf("esperava {1,5} do bloco, veio %v", got)
	}

	// Outro atleta não enxerga o histórico do atleta 1.
	if ids, _ := repo.RecentExerciseIDs(2); len(ids) != 0 {
		t.Fatalf("atleta 2 não deveria ter histórico, veio %v", ids)
	}

	// Um bloco MAIS RECENTE (só exercício 1) substitui a janela: agora o recente é {1}.
	recent := smallBlock()
	recent.Weeks[0].Sessions[0].Prescriptions = recent.Weeks[0].Sessions[0].Prescriptions[:1] // só ex 1
	if err := repo.SaveGeneratedBlock(recent); err != nil {
		t.Fatalf("SaveGeneratedBlock recente: %v", err)
	}
	got2, _ := repo.RecentExerciseIDs(1)
	if len(got2) != 1 || got2[0] != 1 {
		t.Fatalf("janela deveria ser o bloco mais recente {1}, veio %v", got2)
	}
}

func TestMovementPatternsAndPriorities(t *testing.T) {
	repo := newTestDB(t)

	// Catálogo de padrões: o seed tem ao menos squat/hinge/push/pull + os da Fase 4/5B.
	patterns, err := repo.ListMovementPatterns()
	if err != nil {
		t.Fatalf("ListMovementPatterns: %v", err)
	}
	byName := map[string]int{}
	for _, p := range patterns {
		byName[p.Name] = p.ID
	}
	if byName["pull"] == 0 || byName["hinge"] == 0 {
		t.Fatalf("catálogo de padrões incompleto: %+v", patterns)
	}

	// Sem prioridade -> vazio.
	if ps, _ := repo.ListPriorities(1); len(ps) != 0 {
		t.Fatalf("atleta 1 não deveria ter prioridade, veio %+v", ps)
	}

	// Define prioridades do atleta 1 (pull + hinge); atleta 2 fica isolado.
	if err := repo.SetPriorities(1, []int{byName["pull"], byName["hinge"]}); err != nil {
		t.Fatalf("SetPriorities: %v", err)
	}
	got, _ := repo.ListPriorities(1)
	names := map[string]bool{}
	for _, p := range got {
		names[p.Name] = true
	}
	if len(got) != 2 || !names["pull"] || !names["hinge"] {
		t.Fatalf("esperava {pull,hinge}, veio %+v", got)
	}
	if ps, _ := repo.ListPriorities(2); len(ps) != 0 {
		t.Fatalf("atleta 2 não deveria herdar prioridades, veio %+v", ps)
	}

	// Reenvio substitui.
	if err := repo.SetPriorities(1, []int{byName["pull"]}); err != nil {
		t.Fatalf("SetPriorities 2: %v", err)
	}
	if ps, _ := repo.ListPriorities(1); len(ps) != 1 || ps[0].Name != "pull" {
		t.Fatalf("reenvio deveria deixar só {pull}, veio %+v", ps)
	}
}

func TestAthleteMetrics(t *testing.T) {
	repo := newTestDB(t)

	// Sem métricas ainda -> nil.
	if m, err := repo.GetMetrics(1); err != nil || m != nil {
		t.Fatalf("esperava nil para atleta sem métricas, veio %+v %v", m, err)
	}

	age := 35
	sport := "weightlifting"
	if err := repo.SaveMetrics(domain.AthleteMetrics{
		AthleteID: 1, AgeYears: &age, Sport: &sport,
	}); err != nil {
		t.Fatalf("SaveMetrics: %v", err)
	}
	m, err := repo.GetMetrics(1)
	if err != nil || m == nil {
		t.Fatalf("GetMetrics após save: %v %v", m, err)
	}
	if *m.AgeYears != 35 || *m.Sport != "weightlifting" || m.Sex != nil || m.BodyWeightKg != nil {
		t.Fatalf("dados inesperados: %+v", m)
	}

	// UPSERT: atualizar age, esporte vira nil.
	age2 := 40
	if err := repo.SaveMetrics(domain.AthleteMetrics{AthleteID: 1, AgeYears: &age2}); err != nil {
		t.Fatalf("SaveMetrics UPSERT: %v", err)
	}
	m2, _ := repo.GetMetrics(1)
	if *m2.AgeYears != 40 || m2.Sport != nil {
		t.Fatalf("UPSERT não sobrescreveu: %+v", m2)
	}

	// Isolamento por atleta.
	if m3, _ := repo.GetMetrics(2); m3 != nil {
		t.Fatalf("atleta 2 não deveria ter métricas, veio %+v", m3)
	}
}

func TestComplexCatalogAndComponents(t *testing.T) {
	repo := newTestDB(t)

	// Um conjugado do seed é um exercício SELECIONÁVEL como outro qualquer: aparece no pool do seu
	// nível/foco (advanced/strength) — prova que entra no MESMO funil, sem tratamento especial.
	cands, err := repo.FindCandidatesForPhase("strength", "advanced", nil)
	if err != nil {
		t.Fatalf("FindCandidatesForPhase: %v", err)
	}
	var complexID int
	for _, e := range cands {
		if e.Name == "Complexo de Arremesso (Clean Pull + Power Clean + Push Jerk)" {
			complexID = e.ID
		}
	}
	if complexID == 0 {
		t.Fatal("o conjugado de arremesso deveria ser selecionável em strength/advanced")
	}

	// ListComponents resolve a sequência na ordem certa, com reps e nomes; um exercício simples (Air
	// Squat, id 1) não tem componentes. Uma única chamada resolve múltiplos ids (sem N+1).
	byComplex, err := repo.ListComponents([]int{complexID, 1})
	if err != nil {
		t.Fatalf("ListComponents: %v", err)
	}
	if _, ok := byComplex[1]; ok {
		t.Error("exercício simples (id 1) não deveria ter componentes")
	}
	comps := byComplex[complexID]
	if len(comps) != 3 {
		t.Fatalf("esperava 3 componentes no conjugado, veio %d", len(comps))
	}
	wantNames := []string{"Clean Pull", "Power Clean", "Push Jerk"}
	for i, c := range comps {
		if c.SortOrder != i+1 {
			t.Errorf("componente %d com sort_order %d, esperava %d", i, c.SortOrder, i+1)
		}
		if c.ExerciseName != wantNames[i] {
			t.Errorf("componente %d: nome %q, esperava %q", i, c.ExerciseName, wantNames[i])
		}
		if c.Reps <= 0 {
			t.Errorf("componente %q sem reps", c.ExerciseName)
		}
	}

	// Lista vazia -> mapa vazio, sem erro (caminho do exercício simples na semana).
	empty, err := repo.ListComponents(nil)
	if err != nil || len(empty) != 0 {
		t.Fatalf("ListComponents(nil) deveria ser vazio sem erro, veio %v err=%v", empty, err)
	}
}

func TestWorkoutRoundTrip(t *testing.T) {
	repo := newTestDB(t)
	if err := repo.ClearWorkout(); err != nil {
		t.Fatalf("ClearWorkout: %v", err)
	}
	rows := []domain.GeneratedWorkout{
		{DayNumber: 1, ExerciseID: 1},
		{DayNumber: 2, ExerciseID: 1},
	}
	if err := repo.SaveWorkout(rows); err != nil {
		t.Fatalf("SaveWorkout: %v", err)
	}
	out, err := repo.ListWorkout()
	if err != nil {
		t.Fatalf("ListWorkout: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("esperava 2 linhas de treino, veio %d", len(out))
	}
	if out[0].ExerciseName == "" {
		t.Error("ListWorkout deveria resolver o nome do exercício")
	}
	// ClearWorkout limpa de fato.
	if err := repo.ClearWorkout(); err != nil {
		t.Fatalf("ClearWorkout 2: %v", err)
	}
	out, _ = repo.ListWorkout()
	if len(out) != 0 {
		t.Fatalf("após ClearWorkout esperava 0, veio %d", len(out))
	}
}
