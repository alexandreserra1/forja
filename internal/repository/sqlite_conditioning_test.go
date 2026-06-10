// Teste de INTEGRAÇÃO da Fase 5B (M2): as leituras de condicionamento e o round-trip do WOD COMPOSTO
// sobre SQLite real (schema+seed). Prova que SaveGeneratedBlock materializa o WOD (source='generated')
// + movimentos + prescrição na mesma transação, e que GetConditioning os devolve resolvidos.
package repository

import (
	"testing"

	"treino/internal/domain"
)

func intptr(i int) *int { return &i }

func TestRecentWodMovementIDs(t *testing.T) {
	repo := newTestDB(t)
	if ids, _ := repo.RecentWodMovementIDs(1); len(ids) != 0 {
		t.Fatalf("sem bloco deveria ser vazio, veio %v", ids)
	}

	m, _ := repo.FindMovementsByModality("M", "beginner", nil)
	g, _ := repo.FindMovementsByModality("G", "beginner", nil)
	formats, _ := repo.ListWodFormats()
	plan := smallBlock()
	plan.Weeks[0].Sessions[0].Conditioning = []domain.ConditioningPrescription{{
		TargetRPE: 7, SortOrder: 1,
		Wod: domain.Wod{
			Name: "W", FormatID: formats[0].ID, WorkSec: 300, Rounds: 1,
			EmphasisSystem: "oxidative", TargetRPE: 7, Level: "beginner",
			Movements: []domain.WodMovement{
				{ExerciseID: m[0].ExerciseID, SortOrder: 1},
				{ExerciseID: g[0].ExerciseID, SortOrder: 2},
			},
		},
	}}
	if err := repo.SaveGeneratedBlock(plan); err != nil {
		t.Fatalf("SaveGeneratedBlock: %v", err)
	}
	got, err := repo.RecentWodMovementIDs(1)
	if err != nil {
		t.Fatalf("RecentWodMovementIDs: %v", err)
	}
	set := map[int]bool{}
	for _, id := range got {
		set[id] = true
	}
	if len(got) != 2 || !set[m[0].ExerciseID] || !set[g[0].ExerciseID] {
		t.Fatalf("esperava os 2 movimentos do WOD, veio %v", got)
	}
	if ids, _ := repo.RecentWodMovementIDs(2); len(ids) != 0 {
		t.Fatalf("atleta 2 não deveria ter histórico de WOD, veio %v", ids)
	}
}

func TestConditioning_LeiturasERoundTrip(t *testing.T) {
	repo := newTestDB(t)

	// --- Mapa tempo->sistema: 4 bandas, max_work_sec crescente por sort_order ---
	bands, err := repo.ListEnergySystemMap()
	if err != nil {
		t.Fatalf("ListEnergySystemMap: %v", err)
	}
	if len(bands) != 4 {
		t.Fatalf("esperava 4 bandas, veio %d", len(bands))
	}
	for i := 1; i < len(bands); i++ {
		if bands[i].MaxWorkSec <= bands[i-1].MaxWorkSec {
			t.Errorf("bandas fora de ordem crescente: %+v", bands)
		}
	}

	// --- Doses por fase (strength): 4, a 1ª é acumulação enfatizando oxidative (do seed) ---
	doses, err := repo.ListPhaseConditioning("strength")
	if err != nil {
		t.Fatalf("ListPhaseConditioning: %v", err)
	}
	if len(doses) != 4 {
		t.Fatalf("esperava 4 doses p/ strength, veio %d", len(doses))
	}
	if doses[0].Phase != "accumulation" || doses[0].EmphasisSystem != "oxidative" || doses[0].WeeklyWods < 1 {
		t.Errorf("1ª dose inesperada: %+v", doses[0])
	}

	// --- FindMovementsByModality: G/intermediate sem filtro só traz G/intermediate ---
	gAll, err := repo.FindMovementsByModality("G", "intermediate", nil)
	if err != nil {
		t.Fatalf("FindMovementsByModality: %v", err)
	}
	if len(gAll) == 0 {
		t.Fatal("esperava movimentos G/intermediate")
	}
	for _, m := range gAll {
		if m.Modality != "G" || m.Level != "intermediate" {
			t.Errorf("candidato fora do filtro: %+v", m)
		}
		if m.SecsPerRep <= 0 {
			t.Errorf("%s sem secs_per_rep", m.ExerciseName)
		}
	}

	// --- Filtro de equipamento (funil Fase 3): com só Barra fixa, nada pode exigir outro equip ---
	equip, _ := repo.ListEquipment()
	idOf := map[string]int{}
	for _, e := range equip {
		idOf[e.Name] = e.ID
	}
	gBar, _ := repo.FindMovementsByModality("G", "intermediate", []int{idOf["Barra fixa"]})
	for _, m := range gBar {
		req, _ := repo.ListExerciseEquipment(m.ExerciseID)
		for _, q := range req {
			if q.ID != idOf["Barra fixa"] {
				t.Errorf("%s exige %s além de Barra fixa; não deveria estar disponível", m.ExerciseName, q.Name)
			}
		}
	}
	if len(gBar) >= len(gAll) {
		t.Errorf("filtro de equipamento deveria reduzir o pool (%d vs %d)", len(gBar), len(gAll))
	}

	// --- Round-trip: bloco com uma sessão que tem 1 WOD COMPOSTO (3 movimentos M/G/W) ---
	mList, _ := repo.FindMovementsByModality("M", "beginner", nil)
	wList, _ := repo.FindMovementsByModality("W", "beginner", nil)
	if len(mList) == 0 || len(gAll) == 0 || len(wList) == 0 {
		t.Fatal("precisa de candidatos M/G/W p/ montar o WOD de teste")
	}
	formats, _ := repo.ListWodFormats()
	if len(formats) == 0 {
		t.Fatal("sem formatos no seed")
	}

	plan := smallBlock()
	plan.Weeks[0].Sessions[0].Conditioning = []domain.ConditioningPrescription{{
		TargetRPE: 7.0, SortOrder: 1,
		Wod: domain.Wod{
			Name: "WOD Teste", FormatID: formats[0].ID, WorkSec: 720, RestSec: 0, Rounds: 1,
			EmphasisSystem: "mixed", TargetRPE: 7.0, Level: "intermediate",
			Movements: []domain.WodMovement{
				{ExerciseID: mList[0].ExerciseID, Reps: nil, SortOrder: 1},          // M: max/contínuo
				{ExerciseID: gAll[0].ExerciseID, Reps: intptr(10), SortOrder: 2},    // G
				{ExerciseID: wList[0].ExerciseID, Reps: intptr(15), SortOrder: 3},   // W
			},
		},
	}}
	if err := repo.SaveGeneratedBlock(plan); err != nil {
		t.Fatalf("SaveGeneratedBlock: %v", err)
	}

	block, _ := repo.GetActiveBlock(1)
	weeks, _ := repo.GetBlockWeeks(block.ID)
	sessions, _ := repo.GetSessions(weeks[0].ID)
	conds, err := repo.GetConditioning(sessions[0].ID)
	if err != nil {
		t.Fatalf("GetConditioning: %v", err)
	}
	if len(conds) != 1 {
		t.Fatalf("esperava 1 WOD na sessão, veio %d", len(conds))
	}
	c := conds[0]
	if c.Wod.Source != "generated" {
		t.Errorf("WOD montado deveria ter source=generated, veio %q", c.Wod.Source)
	}
	if c.Wod.FormatName == "" {
		t.Error("formato do WOD não foi resolvido")
	}
	if len(c.Wod.Movements) != 3 {
		t.Fatalf("esperava 3 movimentos, veio %d", len(c.Wod.Movements))
	}
	if c.Wod.Movements[0].Reps != nil {
		t.Errorf("1º movimento (M) deveria ser max/contínuo (reps nil), veio %v", *c.Wod.Movements[0].Reps)
	}
	for i, m := range c.Wod.Movements {
		if m.SortOrder != i+1 {
			t.Errorf("movimento %d com sort_order %d", i, m.SortOrder)
		}
		if m.ExerciseName == "" {
			t.Errorf("movimento %d sem nome resolvido", i)
		}
	}

	// --- ListWods: os 12 benchmark do seed + o 1 gerado agora = 13 ---
	wods, err := repo.ListWods()
	if err != nil {
		t.Fatalf("ListWods: %v", err)
	}
	if len(wods) != 13 {
		t.Fatalf("esperava 13 wods (12 benchmark + 1 gerado), veio %d", len(wods))
	}
}

func TestWodAutoreg_MarkDoneGetActualsSkip(t *testing.T) {
	repo := newTestDB(t)

	// Gera bloco com 1 WOD na sessão 1 (mesmo padrão do TestRecentWodMovementIDs).
	m, _ := repo.FindMovementsByModality("M", "beginner", nil)
	g, _ := repo.FindMovementsByModality("G", "beginner", nil)
	formats, _ := repo.ListWodFormats()
	plan := smallBlock()
	plan.Weeks[0].Sessions[0].Conditioning = []domain.ConditioningPrescription{{
		TargetRPE: 7, SortOrder: 1,
		Wod: domain.Wod{
			Name: "AutoRegWod", FormatID: formats[0].ID, WorkSec: 300, Rounds: 1,
			EmphasisSystem: "oxidative", TargetRPE: 7, Level: "beginner",
			Movements: []domain.WodMovement{
				{ExerciseID: m[0].ExerciseID, SortOrder: 1},
				{ExerciseID: g[0].ExerciseID, SortOrder: 2},
			},
		},
	}}
	if err := repo.SaveGeneratedBlock(plan); err != nil {
		t.Fatalf("SaveGeneratedBlock: %v", err)
	}
	block, _ := repo.GetActiveBlock(1)
	weeks, _ := repo.GetBlockWeeks(block.ID)
	if len(weeks) == 0 {
		t.Skip("bloco sem semanas")
	}
	sessions, _ := repo.GetSessions(weeks[0].ID)
	var condID int
	for _, s := range sessions {
		cps, _ := repo.GetConditioning(s.ID)
		if len(cps) > 0 {
			condID = cps[0].ID
			break
		}
	}
	if condID == 0 {
		t.Skip("nenhuma conditioning_prescription no bloco de teste")
	}

	// Actuals antes de marcar = vazio.
	actuals, _ := repo.GetWodActuals(weeks[0].ID)
	if len(actuals) != 0 {
		t.Fatalf("esperava 0 actuals, veio %d", len(actuals))
	}

	// Marca feito com RPE 9.0.
	rpe := 9.0
	if err := repo.MarkWodDone(condID, &rpe); err != nil {
		t.Fatalf("MarkWodDone: %v", err)
	}
	actuals, _ = repo.GetWodActuals(weeks[0].ID)
	if len(actuals) != 1 {
		t.Fatalf("esperava 1 actual, veio %d", len(actuals))
	}
	if actuals[0].ActualRPE == nil || *actuals[0].ActualRPE != 9.0 {
		t.Errorf("RPE incorreto: %v", actuals[0].ActualRPE)
	}

	// Skip: GetConditioning não deve retornar a prescrição skipada.
	if err := repo.SkipWodPrescription(condID); err != nil {
		t.Fatalf("SkipWodPrescription: %v", err)
	}
	for _, s := range sessions {
		cps, _ := repo.GetConditioning(s.ID)
		for _, cp := range cps {
			if cp.ID == condID {
				t.Errorf("prescrição skipada não deveria aparecer em GetConditioning")
			}
		}
	}
}
