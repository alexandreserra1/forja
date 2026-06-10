package service

import (
	"fmt"
	"sort"

	"treino/internal/domain"
)

// Limiar de RPE e mínimo de WODs feitos para o detector do condicionamento.
// PLACEHOLDER CALIBRÁVEL — validar com treinador antes de usuário real.
const (
	wodRPEThreshold = 8.0 // avg RPE acima disso = condicionamento alto demais
	wodMinDone      = 2   // mínimo de WODs feitos para ter sinal (evita ruído de semana com 1 WOD)
)

// EvaluateWodAutoregulation é o trilho INDEPENDENTE de autorregulação do condicionamento.
// Lê os WOD actuals da última semana com registro suficiente; se avg RPE > limiar E ≥ 2 WODs
// feitos → pula 1 WOD da próxima semana (min 1/semana garantido) + grava rastro.
// Não toca no trilho de força.
func (s *Service) EvaluateWodAutoregulation(athleteID int) (action, explanation string, err error) {
	block, err := s.repo.GetActiveBlock(athleteID)
	if err != nil {
		return "", "", err
	}
	if block == nil {
		return "none", "Sem bloco ativo.", nil
	}
	weeks, err := s.repo.GetBlockWeeks(block.ID)
	if err != nil {
		return "", "", err
	}
	sort.Slice(weeks, func(i, j int) bool { return weeks[i].WeekNumber < weeks[j].WeekNumber })

	// Encontra a última semana com WODs feitos (≥ wodMinDone).
	lastActuals := []domain.WodActual(nil)
	lastWeekIdx := -1
	for i := len(weeks) - 1; i >= 0; i-- {
		a, err := s.repo.GetWodActuals(weeks[i].ID)
		if err != nil {
			return "", "", err
		}
		if len(a) >= wodMinDone {
			lastActuals = a
			lastWeekIdx = i
			break
		}
	}
	if lastWeekIdx < 0 {
		return "needs_log", "Marque os WODs como feitos (com RPE) para o motor poder avaliar o condicionamento.", nil
	}

	// Calcula a média de RPE dos WODs feitos.
	avgRPE := wodAvgRPE(lastActuals)
	if avgRPE <= wodRPEThreshold {
		return "none", fmt.Sprintf("Condicionamento dentro do alvo (RPE médio %.1f).", avgRPE), nil
	}

	// Sinal: avg RPE alto. Procura a próxima semana e pula 1 WOD não-skipado.
	nextIdx := lastWeekIdx + 1
	if nextIdx >= len(weeks) {
		return "none", fmt.Sprintf("RPE médio dos WODs (%.1f) está alto, mas não há próxima semana para ajustar.", avgRPE), nil
	}
	nextWeek := weeks[nextIdx]
	sessions, err := s.repo.GetSessions(nextWeek.ID)
	if err != nil {
		return "", "", err
	}

	// Encontra o primeiro WOD não-skipado da próxima semana.
	skippedID := 0
	for _, sess := range sessions {
		cps, err := s.repo.GetConditioning(sess.ID)
		if err != nil {
			return "", "", err
		}
		if len(cps) > 0 {
			skippedID = cps[0].ID
			break
		}
	}

	// Garante mínimo de 1 WOD na semana antes de skipar.
	if skippedID == 0 {
		return "none", fmt.Sprintf("RPE médio dos WODs (%.1f) está alto, mas a próxima semana já tem apenas 1 WOD ou menos.", avgRPE), nil
	}

	if err := s.repo.SkipWodPrescription(skippedID); err != nil {
		return "", "", err
	}

	expl := fmt.Sprintf(
		"Seu RPE médio nos WODs foi %.1f (acima de %.1f). O motor removeu 1 WOD da semana %d para manter a recuperação.",
		avgRPE, wodRPEThreshold, nextWeek.WeekNumber,
	)
	// Rastro: grava o ajuste (mesma tabela da força, action diferente).
	adj := domain.AutoregAdjustment{
		BlockID:     block.ID,
		WeekID:      nextWeek.ID,
		Trigger:     fmt.Sprintf("wod_rpe_%.1f", avgRPE),
		Action:      "reduce_wod_dose",
		Explanation: expl,
	}
	if err := s.repo.ApplyAdjustment(adj, nil); err != nil {
		return "", "", err
	}

	return "reduce_wod_dose", expl, nil
}

// wodAvgRPE calcula a média de RPE dos WOD actuals (ignora nil — sem RPE registrado).
func wodAvgRPE(actuals []domain.WodActual) float64 {
	sum, n := 0.0, 0
	for _, a := range actuals {
		if a.ActualRPE != nil {
			sum += *a.ActualRPE
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}
