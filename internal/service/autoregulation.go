// Package service: autoregulation.go é a inteligência da FASE 2 — a CAIXA TEAL ganha
// MEMÓRIA ATIVA. O motor passa a comparar o PREVISTO (prescription) com o REALIZADO
// (session_log) e a ALIVIAR a carga das próximas semanas quando há sinal de estagnação.
//
// Princípios inegociáveis (ver plan.md):
//   - Reage a TENDÊNCIA (2 semanas), nunca a um ponto.
//   - SÓ alivia: nunca eleva RPE, nunca leva a RPE 10, nunca age com < 2 semanas de dado.
//   - Sem registro suficiente => pede registro, não infere estagnação.
//   - Todo ajuste é LOCAL (a próxima semana) e deixa RASTRO explicável (autoreg_adjustment).
//
// O gerador da Fase 1 (periodization.go) NÃO muda. Esta é uma camada nova, atrás da fachada.
package service

import (
	"fmt"
	"sort"

	"treino/internal/domain"
)

// Parâmetros do detector e do ajuste. São CALIBRÁVEIS (placeholder fundamentado):
// os testes validam a LÓGICA (dispara no momento certo, respeita tetos), não estes números.
const (
	rpeSignalDelta       = 1.0  // actual_rpe médio >= target + 1.0 => esforço subindo p/ o mesmo trabalho
	incompleteSignalFrac = 0.34 // > ~1/3 das prescrições sem registro => sinal de não-conclusão
	minLoggedFrac        = 0.5  // < metade registrada => inconclusivo (pede registro, não pune)
	severeStagnationRun  = 3    // 3+ semanas seguidas de sinal => deload reativo (alívio maior)
	minSets              = 2    // piso de séries ao reduzir volume
	rpeReliefStep        = 0.5  // micro-alívio de RPE quando o volume já é mínimo
	deloadRPEDrop        = 1.0  // alívio de RPE num deload reativo
)

// weekSignal classifica o que uma semana diz sobre o atleta.
type weekSignal int

const (
	signalInconclusive weekSignal = iota // registro insuficiente: não dá p/ concluir
	signalOK                             // dentro do esperado
	signalStagnation                     // sinal de estagnação (esforço subindo ou não-conclusão)
)

// weekSummary resume o previsto-vs-realizado de uma semana (read model, M2).
type weekSummary struct {
	total      int     // prescrições na semana
	logged     int     // quantas têm registro (actual_rpe != nil)
	meanActual float64 // média do RPE realizado (entre as logadas)
	meanTarget float64 // média do RPE previsto (entre as logadas) — comparação maçã-com-maçã
}

// composeWeek monta o resumo a partir do realizado cru. Função pura, sem efeitos.
func composeWeek(actuals []domain.SessionActual) weekSummary {
	s := weekSummary{total: len(actuals)}
	var sumActual, sumTarget float64
	for _, a := range actuals {
		if a.ActualRPE != nil {
			s.logged++
			sumActual += *a.ActualRPE
			sumTarget += a.TargetRPE
		}
	}
	if s.logged > 0 {
		s.meanActual = sumActual / float64(s.logged)
		s.meanTarget = sumTarget / float64(s.logged)
	}
	return s
}

// classify aplica a regra do detector a UMA semana (M3). A confirmação de estagnação
// (2 semanas consecutivas) é decidida por quem chama, não aqui.
func (s weekSummary) classify() weekSignal {
	if s.total == 0 {
		return signalInconclusive
	}
	loggedFrac := float64(s.logged) / float64(s.total)
	if loggedFrac < minLoggedFrac {
		return signalInconclusive // atleta mal registrou: pede registro, não infere
	}
	effortUp := s.meanActual >= s.meanTarget+rpeSignalDelta
	incomplete := (1 - loggedFrac) > incompleteSignalFrac
	if effortUp || incomplete {
		return signalStagnation
	}
	return signalOK
}

// EvaluationResult é o que o endpoint /evaluate devolve: a decisão + a explicação modesta.
// Contém os dois trilhos (força e WOD), cada um independente.
type EvaluationResult struct {
	Action      string                    `json:"action"` // none | needs_log | reduce_volume | reduce_rpe | reactive_deload
	Explanation string                    `json:"explanation"`
	Adjustment  *domain.AutoregAdjustment `json:"adjustment,omitempty"`
	WodAction   string                    `json:"wod_action,omitempty"` // AutoReg WOD: none | needs_log | reduce_wod_dose
	WodExplanation string                 `json:"wod_explanation,omitempty"`
}

// EvaluateAndAdjust é o orquestrador da Fase 2 (M4): lê o bloco ativo, roda Medir+Detectar
// em cada semana, confirma estagnação (2 consecutivas) e, se houver, ALIVIA a próxima semana
// dentro dos tetos de segurança — gravando o rastro. Devolve sempre uma explicação.
func (s *Service) EvaluateAndAdjust(athleteID int) (*EvaluationResult, error) {
	block, err := s.repo.GetActiveBlock(athleteID)
	if err != nil {
		return nil, err
	}
	if block == nil {
		return nil, fmt.Errorf("nenhum bloco ativo")
	}
	weeks, err := s.repo.GetBlockWeeks(block.ID)
	if err != nil {
		return nil, err
	}
	sort.Slice(weeks, func(i, j int) bool { return weeks[i].WeekNumber < weeks[j].WeekNumber })

	// Classifica cada semana (previsto vs realizado).
	signals := make([]weekSignal, len(weeks))
	for i, w := range weeks {
		actuals, err := s.repo.GetWeekActuals(w.ID)
		if err != nil {
			return nil, err
		}
		signals[i] = composeWeek(actuals).classify()
	}

	// Última semana avaliável (com registro suficiente). Sem nenhuma => pede registro.
	last := -1
	for i := len(weeks) - 1; i >= 0; i-- {
		if signals[i] != signalInconclusive {
			last = i
			break
		}
	}
	if last < 0 {
		r := &EvaluationResult{Action: "needs_log", Explanation: "Registre seus treinos (com o RPE real) para o motor poder avaliar a semana."}
		s.enrichWithWod(athleteID, r)
		return r, nil
	}

	// Estagnação confirmada só com 2 semanas CONSECUTIVAS de sinal terminando na última avaliável.
	if last < 1 || signals[last] != signalStagnation || signals[last-1] != signalStagnation {
		r := &EvaluationResult{Action: "none", Explanation: "Seus registros não indicam estagnação. Seguimos com o bloco como planejado."}
		s.enrichWithWod(athleteID, r)
		return r, nil
	}

	// Conta a sequência de sinal (p/ decidir a intensidade do alívio).
	run := 0
	for i := last; i >= 0 && signals[i] == signalStagnation; i-- {
		run++
	}

	// Alvo do ajuste: a primeira semana DEPOIS da última avaliável que não seja deload.
	if last+1 >= len(weeks) {
		r := &EvaluationResult{Action: "none", Explanation: "Há sinal de estagnação, mas o bloco está terminando — não há próxima semana para aliviar."}
		s.enrichWithWod(athleteID, r)
		return r, nil
	}
	target := weeks[last+1]
	if target.IsDeload {
		r := &EvaluationResult{Action: "none", Explanation: "Há sinal de estagnação, mas a próxima semana já é um deload — o alívio já está previsto."}
		s.enrichWithWod(athleteID, r)
		return r, nil
	}

	// Idempotência: não empilhar ajustes sobre a mesma semana em reavaliações repetidas.
	existing, err := s.repo.ListAdjustments(block.ID)
	if err != nil {
		return nil, err
	}
	for _, a := range existing {
		if a.WeekID == target.ID {
			r := &EvaluationResult{Action: "none", Explanation: "A próxima semana já foi ajustada anteriormente. Nenhuma mudança nova."}
			s.enrichWithWod(athleteID, r)
			return r, nil
		}
	}

	// Lê o previsto da semana alvo p/ montar o ajuste.
	targetActuals, err := s.repo.GetWeekActuals(target.ID)
	if err != nil {
		return nil, err
	}
	if len(targetActuals) == 0 {
		r := &EvaluationResult{Action: "none", Explanation: "Há sinal de estagnação, mas a próxima semana não tem prescrições para ajustar."}
		s.enrichWithWod(athleteID, r)
		return r, nil
	}

	adj, updated := planAdjustment(block.ID, target, targetActuals, run)

	// TETOS DE SEGURANÇA (defesa em profundidade — este motor decide a carga de uma
	// pessoa real, então a garantia tem de ser LEGÍVEL, não implícita):
	//
	//   "a autorregulação SÓ ALIVIA: o RPE-alvo e o volume nunca sobem reativamente."
	//
	// Como a ação só reduz, isto também satisfaz POR CONSTRUÇÃO o teto do plan.md
	// "nunca ultrapassar o RPE-topo da fase" (não se pode estourar um topo reduzindo).
	// Por isso não lemos o topo da fase: a invariante "não aumentar" é mais forte e
	// mais simples. Se um dia o motor ganhar poder de SUBIR carga, este ponto quebra
	// de propósito — forçando uma decisão consciente e uma checagem de topo de verdade.
	for i, p := range updated {
		orig := targetActuals[i]
		if p.TargetRPE > orig.TargetRPE || p.Sets > orig.Sets {
			return nil, fmt.Errorf("trava de segurança: ajuste tentou AUMENTAR carga (prescrição %d)", p.ID)
		}
	}

	if err := s.repo.ApplyAdjustment(adj, updated); err != nil {
		return nil, err
	}
	result := &EvaluationResult{Action: adj.Action, Explanation: adj.Explanation, Adjustment: &adj}
	// AutoReg WOD: trilho independente — roda sempre junto e adiciona o resultado.
	wodAction, wodExpl, err := s.EvaluateWodAutoregulation(athleteID)
	if err == nil {
		result.WodAction = wodAction
		result.WodExplanation = wodExpl
	}
	return result, nil
}

// planAdjustment escolhe a ação (preferência da literatura: reduzir volume antes de RPE;
// sinal persistente => deload reativo) e monta as prescrições reescritas + o rastro.
// Não toca no banco — é pura e testável.
func planAdjustment(blockID int, target domain.BlockWeek, actuals []domain.SessionActual, run int) (domain.AutoregAdjustment, []domain.Prescription) {
	// Valores representativos da semana (uniformes: vêm do molde da fase).
	setsBefore := actuals[0].Sets
	rpeBefore := actuals[0].TargetRPE

	adj := domain.AutoregAdjustment{
		BlockID: blockID,
		WeekID:  target.ID,
		Trigger: "stagnation_2w",
	}
	updated := make([]domain.Prescription, len(actuals))

	switch {
	case run >= severeStagnationRun:
		// Deload reativo: corta volume ao piso E alivia o RPE. Alívio maior p/ sinal persistente.
		setsAfter := minSets
		rpeAfter := rpeBefore - deloadRPEDrop
		for i, a := range actuals {
			s := a.Sets
			if s > minSets {
				s = minSets
			}
			updated[i] = domain.Prescription{ID: a.PrescriptionID, Sets: s, TargetRPE: a.TargetRPE - deloadRPEDrop}
		}
		adj.Action = "reactive_deload"
		adj.SetsBefore, adj.SetsAfter = &setsBefore, &setsAfter
		adj.RPEBefore, adj.RPEAfter = &rpeBefore, &rpeAfter
		adj.Explanation = fmt.Sprintf(
			"Seus registros das últimas semanas sugerem uma semana mais leve. Transformamos a semana %d num alívio (menos séries e menor esforço-alvo) para você recuperar.",
			target.WeekNumber)

	case setsBefore > minSets:
		// Preferência da literatura: reduzir VOLUME mantendo a intensidade.
		setsAfter := setsBefore - 1
		for i, a := range actuals {
			s := a.Sets - 1
			if s < minSets {
				s = minSets
			}
			updated[i] = domain.Prescription{ID: a.PrescriptionID, Sets: s, TargetRPE: a.TargetRPE}
		}
		adj.Action = "reduce_volume"
		adj.SetsBefore, adj.SetsAfter = &setsBefore, &setsAfter
		adj.Explanation = fmt.Sprintf(
			"Seus registros das últimas 2 semanas sugerem aliviar a carga. Reduzimos o volume da semana %d (de %d para %d séries) para você recuperar.",
			target.WeekNumber, setsBefore, setsAfter)

	default:
		// Volume já é mínimo: micro-alívio de RPE.
		rpeAfter := rpeBefore - rpeReliefStep
		for i, a := range actuals {
			updated[i] = domain.Prescription{ID: a.PrescriptionID, Sets: a.Sets, TargetRPE: a.TargetRPE - rpeReliefStep}
		}
		adj.Action = "reduce_rpe"
		adj.RPEBefore, adj.RPEAfter = &rpeBefore, &rpeAfter
		adj.Explanation = fmt.Sprintf(
			"Seus registros das últimas 2 semanas sugerem aliviar a carga. Reduzimos o esforço-alvo da semana %d (RPE %.1f para %.1f) para você recuperar.",
			target.WeekNumber, rpeBefore, rpeAfter)
	}

	return adj, updated
}

// Adjustments devolve o histórico de ajustes do bloco ativo do atleta (tela de transparência).
func (s *Service) Adjustments(athleteID int) ([]domain.AutoregAdjustment, error) {
	block, err := s.repo.GetActiveBlock(athleteID)
	if err != nil {
		return nil, err
	}
	if block == nil {
		return nil, nil
	}
	return s.repo.ListAdjustments(block.ID)
}

// enrichWithWod roda o trilho de WOD autoregulation e adiciona os campos ao resultado existente.
// Erros do trilho WOD são silenciados (não falham a avaliação de força).
func (s *Service) enrichWithWod(athleteID int, r *EvaluationResult) {
	action, expl, err := s.EvaluateWodAutoregulation(athleteID)
	if err == nil {
		r.WodAction = action
		r.WodExplanation = expl
	}
}
