// Package service: conditioning.go é a Fase 5B — o COMPOSITOR de WODs. Monta o WOD a partir da
// análise dos movimentos (modalidade M/G/W + sistema energético da fase + dosagem por tempo), de
// forma DETERMINÍSTICA e INDIVIDUALIZADA por atleta (usa a semente da Fase 6A). Trilho paralelo ao
// gerador de força — não muda força/RPE/deload/autorregulação.
//
// HONESTIDADE: a ciência dos 3 sistemas energéticos (tempo->sistema) é sólida; a COMPOSIÇÃO exata
// (shape por sistema, secs_por_rep, caps, dosagem) é HEURÍSTICA / placeholder calibrável, a validar
// com treinador antes de usuário real. O motor "enfatiza" um sistema, nunca "isola".
package service

import (
	"fmt"
	"math"

	"treino/internal/domain"
)

// Parâmetros de composição (placeholder calibrável; os testes validam a LÓGICA, não estes números).
const (
	condMinReps    = 1
	condMaxReps    = 60 // teto de reps por movimento (realismo); a dosagem aproxima o tempo-alvo
	condMaxRound   = 30 // teto de rounds
	maxHighPerWod  = 1  // FASE 5C: no máximo 1 movimento de ALTA skill por WOD (segurança)
)

// skillRepCap (Fase 5C): teto de reps por nível de skill. Movimento técnico não vai a volume alto
// ("50 snatches for time" não acontece). Placeholder calibrável; não mexe no work_sec (desacoplado).
var skillRepCap = map[string]int{"low": 50, "med": 25, "high": 12}

// systemShape: as modalidades de UM round por sistema. O triplet M/G/W é o WOD "misto" (o clássico);
// sistemas focados usam menos modalidades — mais fiel à programação real que um triplet sempre.
var systemShape = map[string][]string{
	"phosphagen": {"W"},           // potência: 1 levantamento explosivo, poucas reps, intervalado
	"glycolytic": {"G", "W"},      // lático: couplet ginástico + carga
	"oxidative":  {"M"},           // aeróbio: motor monoestrutural sustentado
	"mixed":      {"M", "G", "W"}, // o WOD clássico: 1 de cada modalidade
}

// systemTargetWork: duração-alvo do round (s) por sistema. Cada valor cai DENTRO da banda do
// energy_system_map, então o work_sec do WOD mapeia ao sistema por construção (a ênfase da fase é
// garantida). As reps são dosadas para aproximar este alvo.
var systemTargetWork = map[string]int{
	"phosphagen": 12,
	"glycolytic": 75,
	"oxidative":  300,
	"mixed":      700,
}

// conditioner carrega o substrato UMA vez por geração e compõe WODs por sessão (determinístico).
type conditioner struct {
	seed        int
	level       string
	doseByPhase map[string]domain.PhaseConditioning
	formats     []domain.WodFormat
	movByMod    map[string][]domain.MovementCandidate // M/G/W -> candidatos (nível e abaixo, equip-filtrado)
	condFactor  float64                               // Fase 6D: multiplica WeeklyWods (1.0 = sem alteração)
}

// Wods devolve o catálogo de WODs (benchmark + gerados) — para GET /api/wods (debug/admin).
func (s *Service) Wods() ([]domain.Wod, error) {
	return s.repo.ListWods()
}

// deprioritizeMovements (Fase 5C) joga os movimentos usados no WOD do bloco anterior para o FIM do
// pool da modalidade (fresco primeiro), sem excluir — o compositor não repete os mesmos movimentos.
func deprioritizeMovements(pool []domain.MovementCandidate, recent map[int]bool) []domain.MovementCandidate {
	if len(recent) == 0 {
		return pool
	}
	fresh := make([]domain.MovementCandidate, 0, len(pool))
	var used []domain.MovementCandidate
	for _, m := range pool {
		if recent[m.ExerciseID] {
			used = append(used, m)
		} else {
			fresh = append(fresh, m)
		}
	}
	return append(fresh, used...)
}

// loadConditioner monta o substrato do compositor. Devolve nil (sem erro) se o objetivo não tem dose
// de condicionamento — aí o bloco sai só com força (degradação graciosa). recentWod despriorisa os
// movimentos do WOD do bloco anterior (Fase 5C: não repetir).
func (s *Service) loadConditioner(goal, level string, equipIDs []int, seed int, recentWod map[int]bool) (*conditioner, error) {
	doses, err := s.repo.ListPhaseConditioning(goal)
	if err != nil {
		return nil, err
	}
	formats, err := s.repo.ListWodFormats()
	if err != nil {
		return nil, err
	}
	if len(doses) == 0 || len(formats) == 0 {
		return nil, nil // sem substrato -> sem condicionamento
	}
	doseByPhase := map[string]domain.PhaseConditioning{}
	for _, d := range doses {
		doseByPhase[d.Phase] = d
	}
	movByMod := map[string][]domain.MovementCandidate{}
	for _, mod := range []string{"M", "G", "W"} {
		ms, err := s.movementsForModality(mod, level, equipIDs)
		if err != nil {
			return nil, err
		}
		movByMod[mod] = deprioritizeMovements(ms, recentWod) // Fase 5C: frescos primeiro
	}
	return &conditioner{seed: seed, level: level, doseByPhase: doseByPhase, formats: formats, movByMod: movByMod}, nil
}

// movementsForModality reúne os movimentos perfilados de uma modalidade, do nível pedido e ABAIXO
// (cascata = regra de negócio), já filtrados por equipamento NA QUERY (funil Fase 3).
func (s *Service) movementsForModality(modality, level string, equipIDs []int) ([]domain.MovementCandidate, error) {
	maxRank, ok := levelRank[level]
	if !ok {
		return nil, fmt.Errorf("nível desconhecido: %q", level)
	}
	var all []domain.MovementCandidate
	for _, lvl := range []string{"beginner", "intermediate", "advanced"} {
		if levelRank[lvl] <= maxRank {
			ms, err := s.repo.FindMovementsByModality(modality, lvl, equipIDs)
			if err != nil {
				return nil, err
			}
			all = append(all, ms...)
		}
	}
	return all, nil
}

// forSession devolve os WODs da sessão (0 ou 1 no M4). Usa a fase (dose), a semana e o dia + a
// semente do atleta para escolher de forma determinística e variada. Sem movimento viável de alguma
// modalidade do shape -> LACUNA (nil): não prescreve WOD inválido.
func (c *conditioner) forSession(phase string, wi, day, days int) []domain.ConditioningPrescription {
	dose, ok := c.doseByPhase[phase]
	if !ok || dose.WeeklyWods <= 0 {
		return nil
	}
	// Fase 6D: escala o número de WODs pelo fator de condicionamento do atleta (min 1 se dose>0).
	if c.condFactor != 0 && c.condFactor != 1.0 {
		dose.WeeklyWods = applyFactor(dose.WeeklyWods, c.condFactor)
	}
	if !wodDays(dose.WeeklyWods, days)[day] {
		return nil // este dia não recebe WOD nesta fase
	}
	wod, ok := c.compose(dose, wi, day)
	if !ok {
		return nil // lacuna
	}
	return []domain.ConditioningPrescription{{TargetRPE: dose.WodTargetRPE, SortOrder: 1, Wod: wod}}
}

// wodDays distribui weekly_wods entre os `days` dias, espalhando (não em dias seguidos).
func wodDays(weeklyWods, days int) map[int]bool {
	out := map[int]bool{}
	if weeklyWods <= 0 || days <= 0 {
		return out
	}
	if weeklyWods >= days {
		for d := 1; d <= days; d++ {
			out[d] = true
		}
		return out
	}
	for k := 0; k < weeklyWods; k++ {
		out[k*days/weeklyWods+1] = true
	}
	return out
}

// compose MONTA o WOD: 1 movimento por modalidade do shape do sistema (escolha por índice
// determinístico), dosado para o tempo-alvo do sistema. work_sec = alvo do sistema (na banda),
// então o WOD mapeia ao sistema enfatizado por construção.
func (c *conditioner) compose(dose domain.PhaseConditioning, wi, day int) (domain.Wod, bool) {
	system := dose.EmphasisSystem
	shape := systemShape[system]
	target := systemTargetWork[system]
	if len(shape) == 0 || target == 0 {
		return domain.Wod{}, false
	}
	// Índice determinístico do slot: varia por ATLETA (seed), SEMANA e DIA.
	k := c.seed + wi*7 + day*3

	// 1 movimento por modalidade do shape (offset por posição p/ variar entre modalidades). Fase 5C:
	// no máx 1 movimento de ALTA skill por WOD — se já houver um, escolhe um de skill menor.
	picks := make([]domain.MovementCandidate, 0, len(shape))
	highCount := 0
	for i, mod := range shape {
		pool := c.movByMod[mod]
		if len(pool) == 0 {
			return domain.Wod{}, false // lacuna: sem movimento viável dessa modalidade
		}
		mv := pickMovement(pool, k+i*5, highCount >= maxHighPerWod)
		if mv.Skill == "high" {
			highCount++
		}
		picks = append(picks, mv)
	}

	movements := doseRound(picks, target)
	format := c.formats[k%len(c.formats)]
	rounds := deriveRounds(format.DefaultDomainSec, target)

	return domain.Wod{
		Name:           fmt.Sprintf("%s — ênfase %s", format.Name, system),
		FormatID:       format.ID,
		FormatName:     format.Name,
		WorkSec:        target,
		RestSec:        0,
		Rounds:         rounds,
		EmphasisSystem: system,
		TargetRPE:      dose.WodTargetRPE,
		Level:          c.level,
		Source:         "generated",
		Movements:      movements,
	}, true
}

// doseRound escolhe as reps de cada movimento para o round aproximar o tempo-alvo (orçamento de tempo
// dividido pelas modalidades, reps ~ orçamento/segundos-por-rep), com clamp de realismo E teto por
// skill (Fase 5C): movimento técnico não vai a volume alto.
func doseRound(picks []domain.MovementCandidate, target int) []domain.WodMovement {
	budget := float64(target) / float64(len(picks))
	movements := make([]domain.WodMovement, 0, len(picks))
	for i, p := range picks {
		reps := int(math.Round(budget / p.SecsPerRep))
		if reps < condMinReps {
			reps = condMinReps
		}
		repCap := condMaxReps
		if sc, ok := skillRepCap[p.Skill]; ok && sc < repCap {
			repCap = sc // teto por skill (segurança)
		}
		if reps > repCap {
			reps = repCap
		}
		r := reps // cópia: cada movimento aponta para SEU valor
		movements = append(movements, domain.WodMovement{
			ExerciseID: p.ExerciseID, ExerciseName: p.ExerciseName, Reps: &r, SortOrder: i + 1,
		})
	}
	return movements
}

// pickMovement escolhe um movimento do pool pelo índice (determinístico); se avoidHigh, pula para o
// próximo de skill < high (no máx 1 high por WOD). Se TODOS forem high, mantém o do índice.
func pickMovement(pool []domain.MovementCandidate, idx int, avoidHigh bool) domain.MovementCandidate {
	n := len(pool)
	base := ((idx % n) + n) % n
	for off := 0; off < n; off++ {
		m := pool[(base+off)%n]
		if !avoidHigh || m.Skill != "high" {
			return m
		}
	}
	return pool[base]
}

// deriveRounds estima quantos rounds cabem na duração típica do formato (placeholder calibrável).
func deriveRounds(formatDefaultSec, target int) int {
	if target <= 0 {
		return 1
	}
	r := int(math.Round(float64(formatDefaultSec) / float64(target)))
	if r < 1 {
		r = 1
	}
	if r > condMaxRound {
		r = condMaxRound
	}
	return r
}
