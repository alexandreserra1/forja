// Teste do M4 (Fase 5B): o COMPOSITOR isolado, com fakeRepo. Prova que o WOD é montado por fase
// (sistema energético), individualizado por atleta e determinístico — validando a LÓGICA, não os
// números (que são placeholder calibrável).
package service

import (
	"testing"

	"treino/internal/domain"
)

// --- substrato de condicionamento em memória (espelha o seed, em escala) ---

func seedEnergyMap() []domain.EnergySystemBand {
	return []domain.EnergySystemBand{
		{MaxWorkSec: 15, System: "phosphagen", SortOrder: 1},
		{MaxWorkSec: 120, System: "glycolytic", SortOrder: 2},
		{MaxWorkSec: 480, System: "oxidative", SortOrder: 3},
		{MaxWorkSec: 3600, System: "mixed", SortOrder: 4},
	}
}

func seedPhaseConditioningStrength() []domain.PhaseConditioning {
	return []domain.PhaseConditioning{
		{Goal: "strength", Phase: "accumulation", EmphasisSystem: "oxidative", WodTargetRPE: 6.0, WeeklyWods: 2, SortOrder: 1},
		{Goal: "strength", Phase: "intensification", EmphasisSystem: "glycolytic", WodTargetRPE: 7.5, WeeklyWods: 2, SortOrder: 2},
		{Goal: "strength", Phase: "realization", EmphasisSystem: "phosphagen", WodTargetRPE: 8.0, WeeklyWods: 1, SortOrder: 3},
		{Goal: "strength", Phase: "deload", EmphasisSystem: "oxidative", WodTargetRPE: 5.0, WeeklyWods: 1, SortOrder: 4},
	}
}

func seedWodFormats() []domain.WodFormat {
	return []domain.WodFormat{
		{ID: 1, Name: "AMRAP", DefaultDomainSec: 720},
		{ID: 2, Name: "EMOM", DefaultDomainSec: 600},
		{ID: 3, Name: "ForTime", DefaultDomainSec: 300},
	}
}

func seedMovementProfiles() []domain.MovementCandidate {
	return []domain.MovementCandidate{
		{ExerciseID: 101, ExerciseName: "Run", Modality: "M", SecsPerRep: 3.5, Skill: "low", Level: "beginner"},
		{ExerciseID: 102, ExerciseName: "Row", Modality: "M", SecsPerRep: 3.0, Skill: "low", Level: "beginner"},
		{ExerciseID: 103, ExerciseName: "Air Squat", Modality: "G", SecsPerRep: 1.5, Skill: "low", Level: "beginner"},
		{ExerciseID: 104, ExerciseName: "Pull-up", Modality: "G", SecsPerRep: 2.0, Skill: "med", Level: "intermediate"},
		{ExerciseID: 105, ExerciseName: "Kettlebell Swing", Modality: "W", SecsPerRep: 2.0, Skill: "low", Level: "beginner"},
		{ExerciseID: 106, ExerciseName: "Thruster", Modality: "W", SecsPerRep: 3.0, Skill: "med", Level: "intermediate"},
		{ExerciseID: 107, ExerciseName: "Power Clean", Modality: "W", SecsPerRep: 3.5, Skill: "med", Level: "intermediate"},
		// high-skill (advanced) p/ exercitar os caps de segurança da Fase 5C.
		{ExerciseID: 108, ExerciseName: "Ring Muscle-up", Modality: "G", SecsPerRep: 5.0, Skill: "high", Level: "advanced"},
		{ExerciseID: 109, ExerciseName: "Snatch", Modality: "W", SecsPerRep: 4.0, Skill: "high", Level: "advanced"},
	}
}

// condRepo = o blockRepo (advanced/strength/8 semanas, sem equipamento) + substrato de condicionamento.
func condRepo() *fakeRepo {
	r := blockRepo()
	r.energyBands = seedEnergyMap()
	r.phaseCond = seedPhaseConditioningStrength()
	r.wodFormats = seedWodFormats()
	r.movementProfiles = seedMovementProfiles()
	return r
}

// bandSystem replica a regra do mapa: a 1ª banda cujo max >= work_sec vence.
func bandSystem(workSec int) string {
	for _, b := range seedEnergyMap() {
		if workSec <= b.MaxWorkSec {
			return b.System
		}
	}
	return ""
}

// blockWodNames concatena os nomes dos movimentos de todos os WODs do bloco do atleta (assinatura).
func blockWodNames(t *testing.T, svc *Service, athleteID int) string {
	t.Helper()
	overview, err := svc.ActiveBlock(athleteID)
	if err != nil || overview == nil {
		t.Fatalf("ActiveBlock(%d): %v", athleteID, err)
	}
	sig := ""
	for _, w := range overview.Weeks {
		d, _ := svc.WeekDetail(athleteID, w.WeekNumber)
		for _, s := range d.Sessions {
			for _, c := range s.Conditioning {
				for _, m := range c.Wod.Movements {
					sig += m.ExerciseName + ","
				}
				sig += "|"
			}
		}
	}
	return sig
}

func TestDoseRound_TetoPorSkill(t *testing.T) {
	// Alvo grande + secs baixo dariam reps enormes; o teto por skill limita (técnico = menos volume).
	picks := []domain.MovementCandidate{
		{ExerciseID: 1, ExerciseName: "Snatch", Modality: "W", SecsPerRep: 1.0, Skill: "high"},
		{ExerciseID: 2, ExerciseName: "Air Squat", Modality: "G", SecsPerRep: 1.0, Skill: "low"},
	}
	movs := doseRound(picks, 600)
	if *movs[0].Reps > skillRepCap["high"] {
		t.Errorf("high passou do teto: %d > %d", *movs[0].Reps, skillRepCap["high"])
	}
	if *movs[1].Reps > skillRepCap["low"] {
		t.Errorf("low passou do teto: %d > %d", *movs[1].Reps, skillRepCap["low"])
	}
	if skillRepCap["high"] >= skillRepCap["low"] {
		t.Errorf("teto de high (%d) deveria ser menor que low (%d)", skillRepCap["high"], skillRepCap["low"])
	}
}

func TestPickMovement_EvitaHigh(t *testing.T) {
	pool := []domain.MovementCandidate{{ExerciseID: 1, Skill: "high"}, {ExerciseID: 2, Skill: "low"}}
	if m := pickMovement(pool, 0, true); m.Skill == "high" {
		t.Errorf("avoidHigh deveria pular o high, veio %+v", m)
	}
	if m := pickMovement(pool, 0, false); m.ExerciseID != 1 {
		t.Errorf("sem avoid deveria pegar o do índice (id 1), veio %+v", m)
	}
	allHigh := []domain.MovementCandidate{{ExerciseID: 1, Skill: "high"}, {ExerciseID: 2, Skill: "high"}}
	if m := pickMovement(allHigh, 0, true); m.ExerciseID != 1 {
		t.Errorf("todos high deveria manter o do índice, veio %+v", m)
	}
}

func TestGenerateBlock_WodRespeitaCaps(t *testing.T) {
	// Fase 5C: nenhum WOD do bloco viola o teto de reps por skill nem o máx de 1 movimento high.
	svc := New(condRepo())
	if _, err := svc.GenerateBlock(1); err != nil {
		t.Fatalf("GenerateBlock: %v", err)
	}
	skillByID := map[int]string{}
	for _, m := range seedMovementProfiles() {
		skillByID[m.ExerciseID] = m.Skill
	}
	overview, _ := svc.ActiveBlock(1)
	for _, w := range overview.Weeks {
		d, _ := svc.WeekDetail(1, w.WeekNumber)
		for _, s := range d.Sessions {
			for _, c := range s.Conditioning {
				high := 0
				for _, m := range c.Wod.Movements {
					skill := skillByID[m.ExerciseID]
					if skill == "high" {
						high++
					}
					if cap, ok := skillRepCap[skill]; ok && m.Reps != nil && *m.Reps > cap {
						t.Errorf("semana %d: %s (%s) com %d reps > teto %d", w.WeekNumber, m.ExerciseName, skill, *m.Reps, cap)
					}
				}
				if high > maxHighPerWod {
					t.Errorf("semana %d: WOD com %d movimentos high (máx %d)", w.WeekNumber, high, maxHighPerWod)
				}
			}
		}
	}
}

func TestDeprioritizeMovements(t *testing.T) {
	pool := []domain.MovementCandidate{{ExerciseID: 1}, {ExerciseID: 2}, {ExerciseID: 3}}
	got := deprioritizeMovements(pool, map[int]bool{2: true})
	want := []int{1, 3, 2} // frescos (1,3) primeiro, recente (2) no fim
	for i, m := range got {
		if m.ExerciseID != want[i] {
			t.Fatalf("pos %d: id %d, esperava %v", i, m.ExerciseID, want)
		}
	}
	if g := deprioritizeMovements(pool, nil); g[0].ExerciseID != 1 {
		t.Errorf("recent vazio deveria manter a ordem")
	}
}

func TestGenerateBlock_ComporWodPorFase(t *testing.T) {
	svc := New(condRepo())
	if _, err := svc.GenerateBlock(1); err != nil {
		t.Fatalf("GenerateBlock: %v", err)
	}

	phaseSystem := map[string]string{}
	phaseRPE := map[string]float64{}
	for _, d := range seedPhaseConditioningStrength() {
		phaseSystem[d.Phase] = d.EmphasisSystem
		phaseRPE[d.Phase] = d.WodTargetRPE
	}
	modByID := map[int]string{}
	for _, m := range seedMovementProfiles() {
		modByID[m.ExerciseID] = m.Modality
	}

	overview, _ := svc.ActiveBlock(1)
	wodCount := 0
	for _, w := range overview.Weeks {
		detail, err := svc.WeekDetail(1, w.WeekNumber)
		if err != nil {
			t.Fatalf("WeekDetail(%d): %v", w.WeekNumber, err)
		}
		wantSystem := phaseSystem[w.Phase]
		shape := systemShape[wantSystem]
		for _, s := range detail.Sessions {
			for _, c := range s.Conditioning {
				wodCount++
				wod := c.Wod
				// 1) work_sec mapeia ao sistema ENFATIZADO da fase (a periodização do condicionamento).
				if got := bandSystem(wod.WorkSec); got != wantSystem {
					t.Errorf("semana %d (%s): work_sec %d mapeia %q, esperava %q", w.WeekNumber, w.Phase, wod.WorkSec, got, wantSystem)
				}
				if wod.EmphasisSystem != wantSystem {
					t.Errorf("semana %d: emphasis_system %q, esperava %q", w.WeekNumber, wod.EmphasisSystem, wantSystem)
				}
				// 2) RPE-alvo = o da dose da fase.
				if wod.TargetRPE != phaseRPE[w.Phase] {
					t.Errorf("semana %d: RPE %.1f, esperava %.1f", w.WeekNumber, wod.TargetRPE, phaseRPE[w.Phase])
				}
				if wod.Source != "generated" {
					t.Errorf("WOD montado deveria ser generated, veio %q", wod.Source)
				}
				// 3) 1 movimento por modalidade do shape, na ordem, com reps>0 e nome.
				if len(wod.Movements) != len(shape) {
					t.Fatalf("semana %d (%s): %d movimentos, esperava %d (shape %v)", w.WeekNumber, w.Phase, len(wod.Movements), len(shape), shape)
				}
				for i, m := range wod.Movements {
					if modByID[m.ExerciseID] != shape[i] {
						t.Errorf("movimento %d é modalidade %q, esperava %q", i, modByID[m.ExerciseID], shape[i])
					}
					if m.Reps == nil || *m.Reps <= 0 || m.ExerciseName == "" {
						t.Errorf("movimento mal dosado: %+v", m)
					}
				}
			}
		}
	}
	if wodCount == 0 {
		t.Fatal("esperava WODs compostos no bloco (substrato seedado)")
	}

	// --- Individualização: atleta 2 (mesmo perfil) recebe WODs diferentes ---
	if _, err := svc.GenerateBlock(2); err != nil {
		t.Fatalf("GenerateBlock(2): %v", err)
	}
	a1 := blockWodNames(t, svc, 1)
	a2 := blockWodNames(t, svc, 2)
	if a1 == a2 {
		t.Errorf("atletas 1 e 2 receberam os MESMOS WODs; a semente não individualizou")
	}

	// --- Fase 6C: regerar NÃO repete — os WODs do bloco novo diferem do anterior ---
	if _, err := svc.GenerateBlock(1); err != nil {
		t.Fatalf("regerar atleta 1: %v", err)
	}
	if a1b := blockWodNames(t, svc, 1); a1b == a1 {
		t.Errorf("os WODs do bloco novo deveriam diferir do anterior (6C não-repetição)")
	}
}
