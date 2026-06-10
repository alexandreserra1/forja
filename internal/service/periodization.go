// Package service: periodization.go é a CAIXA TEAL — o cérebro.
// Aqui mora a regra de negócio. Ele só conhece as INTERFACES de repository,
// nunca a implementação SQLite (Princípio 1 e 2).
//
// O motor da Fase 0 é deliberadamente burro. Na Fase 1 vira periodização de
// verdade — e como tudo aqui está atrás do contrato, troca-se a lógica sem
// tocar em handler, repository nem React.
package service

import (
	"fmt"
	"hash/fnv"
	"strconv"

	"treino/internal/domain"
	"treino/internal/repository"
)

// exercisesPerDay: cada dia recebe esta quantidade de exercícios (dentro da faixa 3–4 do plano).
const exercisesPerDay = 3

// blockRotation (Fase 6C): passo da rotação por bloco. Coprimo com os tamanhos típicos de pool
// (40/47/...) p/ girar bem; cada bloco novo do atleta parte de outra posição -> não repete o anterior.
const blockRotation = 7

// levelRank define a ORDEM dos níveis. É conhecimento de negócio (não de dados):
// beginner < intermediate < advanced. É isto que viabiliza a cascata "nível e abaixo".
var levelRank = map[string]int{
	"beginner":     0,
	"intermediate": 1,
	"advanced":     2,
}

// phaseStimulus mapeia cada fase ao FOCO (estímulo) que predomina nela (Fase 4).
// Placeholder FUNDAMENTADO e calibrável — não verdade cravada: acumulação constrói base/técnica;
// intensificação e realização puxam força; deload volta à técnica leve. O enum de exercise.focus
// só tem technique/strength/conditioning, então o mapa usa esses (ver "Nota de Realidade" no
// plan.md). Promover 'stimulus' a coluna de phase_template (regra como DADO) é trabalho futuro;
// hoje é regra de serviço, declarada aqui de propósito.
var phaseStimulus = map[string]string{
	"accumulation":    "technique",
	"intensification": "strength",
	"realization":     "strength",
	"deload":          "technique",
}

// athleteSeed devolve o DESLOCAMENTO determinístico da seleção por atleta (Fase 6A). É o que
// individualiza: dois atletas com o MESMO perfil partem de posições diferentes no pool da fase, sem
// RNG. O atleta 1 (implícito histórico) tem deslocamento 0 — baseline que preserva as fases
// anteriores. Os demais usam hash(id) (FNV) para espalhar bem. NÃO muda a fase/RPE/deload: só gira
// QUAIS exercícios saem dentro do pool já filtrado pela fase (phase-correct por construção).
func athleteSeed(athleteID int) int {
	if athleteID <= 1 {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(strconv.Itoa(athleteID)))
	return int(h.Sum32())
}

// Service é a aplicação. Recebe o contrato Repository por injeção de dependência.
type Service struct {
	repo repository.Repository
}

// New injeta o repositório (qualquer implementação do contrato).
func New(repo repository.Repository) *Service {
	return &Service{repo: repo}
}

// Questions devolve o questionário para o cliente desenhar.
func (s *Service) Questions() ([]domain.Question, error) {
	return s.repo.ListQuestions()
}

// SaveAnswers grava as respostas do atleta (substitui as anteriores dele).
func (s *Service) SaveAnswers(athleteID int, answers []domain.Answer) error {
	return s.repo.SaveAnswers(athleteID, answers)
}

// CreateAthlete cria um atleta (Fase 6A; sem auth).
func (s *Service) CreateAthlete(name string) (*domain.Athlete, error) {
	return s.repo.CreateAthlete(name)
}

// Athletes lista os atletas (para o seletor).
func (s *Service) Athletes() ([]domain.Athlete, error) {
	return s.repo.ListAthletes()
}

// Workout devolve o último treino gerado.
func (s *Service) Workout() ([]domain.GeneratedWorkout, error) {
	return s.repo.ListWorkout()
}

// Generate é o MOTOR. Lê as respostas, deriva o perfil, escolhe exercícios e
// monta o treino. Salva e devolve.
func (s *Service) Generate(athleteID int) ([]domain.GeneratedWorkout, error) {
	// 1. Deriva o perfil a partir das respostas + regras de interpretação.
	profile, err := s.deriveProfile(athleteID)
	if err != nil {
		return nil, err
	}

	level := profile["level"]
	goal := profile["goal"]
	daysStr := profile["days"]
	if level == "" || goal == "" || daysStr == "" {
		return nil, fmt.Errorf("perfil incompleto (responda o questionário): level=%q goal=%q days=%q",
			level, goal, daysStr)
	}
	days, err := strconv.Atoi(daysStr)
	if err != nil || days < 1 {
		return nil, fmt.Errorf("número de dias inválido: %q", daysStr)
	}

	// 2. Candidatos: exercícios do nível do usuário E ABAIXO (cascata = regra de negócio).
	//    Motor da Fase 0 segue sem filtro de equipamento (nil).
	candidates, err := s.candidatesUpToLevel(level, nil)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("sem exercícios para o nível %q", level)
	}

	// 3. Prioriza pelo foco do objetivo (technique prioriza technique; senão o foco do goal).
	candidates = prioritizeByGoal(candidates, goal)

	// 4. Intercala por padrão de movimento p/ cada dia sair variado.
	ordered := interleaveByPattern(candidates)

	// 5. Distribui em `days` dias com `exercisesPerDay` cada (cicla se faltar candidato).
	workout := distribute(ordered, days, exercisesPerDay)

	// 6. Persiste de forma idempotente: zera o anterior e grava o novo.
	if err := s.repo.ClearWorkout(); err != nil {
		return nil, err
	}
	if err := s.repo.SaveWorkout(workout); err != nil {
		return nil, err
	}
	// Devolve já com nomes resolvidos pelo repositório.
	return s.repo.ListWorkout()
}

// deriveProfile cruza cada resposta DO ATLETA com a regra correspondente -> mapa atributo:valor.
func (s *Service) deriveProfile(athleteID int) (map[string]string, error) {
	answers, err := s.repo.ListAnswers(athleteID)
	if err != nil {
		return nil, err
	}
	rules, err := s.repo.ListAnswerRules()
	if err != nil {
		return nil, err
	}
	// Índice (questionID, optionValue) -> regra, p/ casar em O(1).
	type key struct {
		q int
		v string
	}
	byAnswer := map[key]domain.AnswerRule{}
	for _, r := range rules {
		byAnswer[key{r.QuestionID, r.OptionValue}] = r
	}

	profile := map[string]string{}
	for _, a := range answers {
		if r, ok := byAnswer[key{a.QuestionID, a.AnswerValue}]; ok {
			profile[r.SetsAttribute] = r.AttributeValue
		}
	}
	return profile, nil
}

// candidatesUpToLevel reúne exercícios do nível pedido e de todos abaixo dele.
// equipmentIDs filtra o pool por equipamento disponível NA QUERY (Fase 3): nil/vazio = sem filtro.
// É aqui que a restrição de equipamento entra no FUNIL de seleção, não como swap depois.
func (s *Service) candidatesUpToLevel(level string, equipmentIDs []int) ([]domain.Exercise, error) {
	maxRank, ok := levelRank[level]
	if !ok {
		return nil, fmt.Errorf("nível desconhecido: %q", level)
	}
	var all []domain.Exercise
	// Itera por rank crescente (beginner -> advanced) p/ ordem determinística.
	for _, lvl := range []string{"beginner", "intermediate", "advanced"} {
		if levelRank[lvl] <= maxRank {
			ex, err := s.repo.ListAvailableByLevel(lvl, equipmentIDs)
			if err != nil {
				return nil, err
			}
			all = append(all, ex...)
		}
	}
	return all, nil
}

// candidatesForPhase reúne exercícios do ESTÍMULO da fase (foco), do nível pedido e abaixo
// (cascata = regra de negócio), já filtrados por equipamento NA QUERY (Fase 3+4). Espelha
// candidatesUpToLevel, mas fixando o foco — assim cada fase puxa de um pool diferente.
func (s *Service) candidatesForPhase(stimulus, level string, equipmentIDs []int) ([]domain.Exercise, error) {
	maxRank, ok := levelRank[level]
	if !ok {
		return nil, fmt.Errorf("nível desconhecido: %q", level)
	}
	var all []domain.Exercise
	for _, lvl := range []string{"beginner", "intermediate", "advanced"} {
		if levelRank[lvl] <= maxRank {
			ex, err := s.repo.FindCandidatesForPhase(stimulus, lvl, equipmentIDs)
			if err != nil {
				return nil, err
			}
			all = append(all, ex...)
		}
	}
	return all, nil
}

// prioritizeByGoal põe na frente os exercícios cujo foco casa com o objetivo,
// mantendo os demais como fallback (ordem estável). technique -> foco technique;
// qualquer outro objetivo -> foco igual ao objetivo.
func prioritizeByGoal(exercises []domain.Exercise, goal string) []domain.Exercise {
	var preferred, rest []domain.Exercise
	for _, e := range exercises {
		if e.Focus == goal {
			preferred = append(preferred, e)
		} else {
			rest = append(rest, e)
		}
	}
	return append(preferred, rest...)
}

// interleaveByPattern reordena intercalando padrões de movimento (squat, hinge,
// push, pull, squat, ...), preservando a prioridade dentro de cada padrão.
// Assim, ao fatiar em dias, cada dia tende a misturar padrões diferentes.
func interleaveByPattern(exercises []domain.Exercise) []domain.Exercise {
	buckets := map[string][]domain.Exercise{}
	var order []string // ordem de 1ª aparição de cada padrão (determinístico)
	for _, e := range exercises {
		if _, seen := buckets[e.MovementPattern]; !seen {
			order = append(order, e.MovementPattern)
		}
		buckets[e.MovementPattern] = append(buckets[e.MovementPattern], e)
	}

	var result []domain.Exercise
	for len(result) < len(exercises) {
		for _, p := range order {
			if len(buckets[p]) > 0 {
				result = append(result, buckets[p][0])
				buckets[p] = buckets[p][1:]
			}
		}
	}
	return result
}

// distribute fatia a lista ordenada em `days` dias com `perDay` exercícios cada.
// Se não houver candidatos suficientes, cicla a lista (motor burro da Fase 0).
func distribute(ordered []domain.Exercise, days, perDay int) []domain.GeneratedWorkout {
	var workout []domain.GeneratedWorkout
	idx := 0
	for day := 1; day <= days; day++ {
		for n := 0; n < perDay; n++ {
			ex := ordered[idx%len(ordered)] // cicla se acabar
			workout = append(workout, domain.GeneratedWorkout{
				DayNumber:  day,
				ExerciseID: ex.ID,
			})
			idx++
		}
	}
	return workout
}

// ============================================================
// FASE 1 — gerador de bloco periodizado
// ============================================================

// weekSpec é o esqueleto de uma semana antes de virar sessões/prescrições.
type weekSpec struct {
	number    int
	phase     string
	targetRPE float64
	isDeload  bool
	sets      int
	reps      int
}

// GenerateBlock é o MOTOR da Fase 1: deriva o perfil, monta o esqueleto de fases
// (RPE progressivo, deload na última semana), seleciona exercícios por dia e
// salva a árvore inteira numa transação (arquivando o bloco anterior).
func (s *Service) GenerateBlock(athleteID int) (*domain.BlockOverview, error) {
	profile, err := s.deriveProfile(athleteID)
	if err != nil {
		return nil, err
	}
	goal := profile["goal"]
	level := profile["level"]
	if goal == "" || level == "" || profile["days"] == "" || profile["weeks"] == "" {
		return nil, fmt.Errorf("perfil incompleto (responda o questionário): %v", profile)
	}
	days, err := strconv.Atoi(profile["days"])
	if err != nil || days < 1 {
		return nil, fmt.Errorf("número de dias inválido: %q", profile["days"])
	}
	weeks, err := strconv.Atoi(profile["weeks"])
	if err != nil || weeks < 2 {
		return nil, fmt.Errorf("número de semanas inválido: %q", profile["weeks"])
	}

	templates, err := s.repo.ListPhaseTemplates(goal)
	if err != nil {
		return nil, err
	}
	if len(templates) == 0 {
		return nil, fmt.Errorf("sem moldes de fase para o objetivo %q", goal)
	}

	// Esqueleto das semanas: fases + RPE + deload.
	specs, err := buildWeekSpecs(templates, weeks)
	if err != nil {
		return nil, err
	}

	// Fase 3: o equipamento do atleta FILTRA o pool de candidatos (restrição no funil,
	// não swap depois). Vazio = sem filtro (assume tudo disponível).
	equipIDs, err := s.userEquipmentIDs(athleteID)
	if err != nil {
		return nil, err
	}

	// Fase 6C: exercícios do bloco ANTERIOR do atleta, p/ não repetir o treino. Lido AGORA (antes de
	// arquivar o bloco ativo). Vazio na 1ª geração (nada a evitar).
	recentIDs, err := s.repo.RecentExerciseIDs(athleteID)
	if err != nil {
		return nil, err
	}
	recent := map[int]bool{}
	for _, id := range recentIDs {
		recent[id] = true
	}

	// Fase 5C: movimentos de WOD do bloco anterior, para o compositor não os repetir.
	recentWodIDs, err := s.repo.RecentWodMovementIDs(athleteID)
	if err != nil {
		return nil, err
	}
	recentWod := map[int]bool{}
	for _, id := range recentWodIDs {
		recentWod[id] = true
	}

	// Fase 6B: padrões priorizados pelo atleta (pontos fracos) -> peso na seleção de força.
	priorities, err := s.repo.ListPriorities(athleteID)
	if err != nil {
		return nil, err
	}
	priority := map[string]bool{}
	for _, p := range priorities {
		priority[p.Name] = true
	}

	// Fase 6D: métricas do atleta -> fatores de volume. Nil = ausente = 1.0 (sem regressão).
	metrics, err := s.repo.GetMetrics(athleteID)
	if err != nil {
		return nil, err
	}
	strengthFactor, condFactor := VolumeFactor(metrics)

	// Pool COMPLETO (todos os focos) do nível e abaixo, já filtrado por equipamento. Serve de
	// FALLBACK quando uma fase não acha candidatos do seu estímulo (lacuna de catálogo) — o motor
	// nunca falha por isso — e também garante a regra "nada impossível com o equipamento disponível".
	fullPool, err := s.candidatesUpToLevel(level, equipIDs)
	if err != nil {
		return nil, err
	}
	if len(fullPool) == 0 {
		return nil, fmt.Errorf("nenhum exercício viável para o nível %q com o equipamento disponível", level)
	}

	// Fase 3: relato das substituições por equipamento + substitutos preferidos. Calculado UMA vez
	// (independe da fase): o custo é limitado aos exercícios EXCLUÍDOS por equipamento, e NÃO se
	// multiplica pelas semanas — atende à preocupação de performance ("cuidado nos joins").
	preferred, substitutions, err := s.substitutionReport(level, equipIDs)
	if err != nil {
		return nil, err
	}

	// Fase 4: cada fase escolhe exercícios pelo SEU estímulo. Os pools por estímulo são cacheados
	// (acumulação/deload compartilham 'technique'; intensificação/realização, 'strength'),
	// evitando refazer a query a cada semana.
	poolByStimulus := map[string][]domain.Exercise{}
	orderedForPhase := func(phase string) ([]domain.Exercise, error) {
		stimulus := phaseStimulus[phase]
		if ord, ok := poolByStimulus[stimulus]; ok {
			return ord, nil
		}
		pool, err := s.candidatesForPhase(stimulus, level, equipIDs)
		if err != nil {
			return nil, err
		}
		if len(pool) == 0 {
			pool = fullPool // lacuna: sem exercício do estímulo -> usa o pool inteiro (nunca falha)
		}
		// Fase 6B: peso nos padrões priorizados (duplica) ANTES do interleave (que espalha as cópias).
		// Fase 6C: despriorizar (não excluir) os exercícios do bloco anterior — fresco primeiro.
		weighted := weightByPriority(bumpPreferred(pool, preferred), priority)
		ord := deprioritizeRecent(interleaveByPattern(prioritizeByGoal(weighted, goal)), recent)
		poolByStimulus[stimulus] = ord
		return ord, nil
	}

	// Fase 6A: deslocamento individual do atleta (determinístico) + Fase 6C: rotação por bloco. Gira a
	// posição de partida no pool da fase: o athleteSeed individualiza ENTRE atletas; o blockCount*…
	// faz cada bloco NOVO do mesmo atleta diferir do anterior (não-repetição robusta em toda fase).
	blockCount, err := s.repo.BlockCount(athleteID)
	if err != nil {
		return nil, err
	}
	seed := athleteSeed(athleteID) + blockCount*blockRotation

	// Fase 5B: trilho paralelo de CONDICIONAMENTO. Carrega o substrato do compositor uma vez (nil se
	// o objetivo não tem dose de condicionamento -> bloco só com força).
	cond, err := s.loadConditioner(goal, level, equipIDs, seed, recentWod)
	if err != nil {
		return nil, err
	}
	if cond != nil {
		cond.condFactor = condFactor // Fase 6D: propaga o fator de condicionamento
	}

	// Monta a árvore block -> weeks -> sessions -> prescriptions.
	plan := domain.GeneratedBlock{
		Block: domain.TrainingBlock{AthleteID: athleteID, Goal: goal, TotalWeeks: weeks, DaysPerWeek: days},
	}
	for wi, spec := range specs {
		ordered, err := orderedForPhase(spec.phase)
		if err != nil {
			return nil, err
		}
		gw := domain.GeneratedWeek{
			Week: domain.BlockWeek{
				WeekNumber: spec.number,
				Phase:      spec.phase,
				TargetRPE:  spec.targetRPE,
				IsDeload:   spec.isDeload,
			},
		}
		for day := 1; day <= days; day++ {
			gs := domain.GeneratedSession{Session: domain.BlockSession{DayNumber: day}}
			// Deslocamento varia por ATLETA (seed), SEMANA e DIA — individualiza + varia no bloco.
			start := seed + (wi*days+(day-1))*exercisesPerDay
			for n := 0; n < exercisesPerDay; n++ {
				ex := ordered[(start+n)%len(ordered)]
				gs.Prescriptions = append(gs.Prescriptions, domain.Prescription{
					ExerciseID: ex.ID,
					Sets:       applyFactor(spec.sets, strengthFactor), // Fase 6D
					Reps:       spec.reps,
					TargetRPE:  spec.targetRPE,
					SortOrder:  n + 1,
				})
			}
			// Fase 5B: ao lado da força, o(s) WOD(s) compostos do dia (individualizados por atleta).
			if cond != nil {
				gs.Conditioning = cond.forSession(spec.phase, wi, day, days)
			}
			gw.Sessions = append(gw.Sessions, gs)
		}
		plan.Weeks = append(plan.Weeks, gw)
	}

	// Persiste: arquiva o antigo DO ATLETA, grava o novo (transacional).
	if err := s.repo.ArchiveActiveBlock(athleteID); err != nil {
		return nil, err
	}
	if err := s.repo.SaveGeneratedBlock(plan); err != nil {
		return nil, err
	}
	overview, err := s.ActiveBlock(athleteID)
	if err != nil {
		return nil, err
	}
	overview.Substitutions = substitutions // relato (não persistido) das trocas por equipamento
	return overview, nil
}

// Equipment devolve o catálogo de equipamentos para o questionário marcar (Fase 3).
func (s *Service) Equipment() ([]domain.Equipment, error) {
	return s.repo.ListEquipment()
}

// SaveUserEquipment grava o equipamento que o atleta tem (substitui o anterior dele).
func (s *Service) SaveUserEquipment(athleteID int, equipmentIDs []int) error {
	return s.repo.SetUserEquipment(athleteID, equipmentIDs)
}

// Patterns devolve o catálogo de padrões de movimento (para o seletor de prioridades — Fase 6B).
func (s *Service) Patterns() ([]domain.MovementPattern, error) {
	return s.repo.ListMovementPatterns()
}

// Priorities devolve os padrões priorizados pelo atleta.
func (s *Service) Priorities(athleteID int) ([]domain.MovementPattern, error) {
	return s.repo.ListPriorities(athleteID)
}

// SavePriorities grava as prioridades do atleta (substitui as anteriores dele).
func (s *Service) SavePriorities(athleteID int, patternIDs []int) error {
	return s.repo.SetPriorities(athleteID, patternIDs)
}

// MarkWodDone registra que o atleta completou um WOD (+ RPE opcional — AutoReg WOD).
func (s *Service) MarkWodDone(condPrescriptionID int, actualRPE *float64) error {
	return s.repo.MarkWodDone(condPrescriptionID, actualRPE)
}

// GetMetrics devolve as métricas do atleta (nil se não existirem — Fase 6D).
func (s *Service) GetMetrics(athleteID int) (*domain.AthleteMetrics, error) {
	return s.repo.GetMetrics(athleteID)
}

// SaveMetrics grava as métricas do atleta (UPSERT — Fase 6D).
func (s *Service) SaveMetrics(m domain.AthleteMetrics) error {
	return s.repo.SaveMetrics(m)
}

// userEquipmentIDs devolve os ids do equipamento que o atleta tem (vazio = sem filtro).
func (s *Service) userEquipmentIDs(athleteID int) ([]int, error) {
	owned, err := s.repo.ListUserEquipment(athleteID)
	if err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(owned))
	for _, e := range owned {
		ids = append(ids, e.ID)
	}
	return ids, nil
}

// substitutionReport descobre o que o equipamento EXCLUIU e, quando há regra ESPECÍFICA viável,
// relata a troca (transparência) e marca o substituto como preferido. A busca GENÉRICA já está
// implícita no pool filtrado (o ideal indisponível simplesmente não está nele).
//
// Roda UMA vez por geração (não por semana): devolve o conjunto de IDs preferidos + o relato, que
// o gerador aplica a qualquer pool de fase via bumpPreferred. O custo é limitado ao número de
// exercícios excluídos por equipamento — não se multiplica pelas semanas ("cuidado nos joins").
//
// Só relata trocas específicas (regra existente). Exclusão genérica não é relatada par-a-par
// (qual virou qual) — ver dívida no plan.md. A garantia central (nada impossível) independe disso.
func (s *Service) substitutionReport(level string, equipIDs []int) (map[int]bool, []domain.Substitution, error) {
	if len(equipIDs) == 0 {
		return nil, nil, nil // sem filtro de equipamento => nada foi excluído
	}
	owned := map[int]bool{}
	for _, id := range equipIDs {
		owned[id] = true
	}
	// Pool VIÁVEL (filtrado) p/ saber o que sobrou, e pool COMPLETO p/ achar o que foi excluído.
	pool, err := s.candidatesUpToLevel(level, equipIDs)
	if err != nil {
		return nil, nil, err
	}
	inPool := map[int]bool{}
	for _, e := range pool {
		inPool[e.ID] = true
	}
	full, err := s.candidatesUpToLevel(level, nil)
	if err != nil {
		return nil, nil, err
	}

	preferred := map[int]bool{}
	var subs []domain.Substitution
	for _, ideal := range full {
		if inPool[ideal.ID] {
			continue // não foi excluído por equipamento
		}
		// Qual equipamento faltou? (o primeiro exigido que o atleta não tem)
		req, err := s.repo.ListExerciseEquipment(ideal.ID)
		if err != nil {
			return nil, nil, err
		}
		var missing domain.Equipment
		for _, r := range req {
			if !owned[r.ID] {
				missing = r
				break
			}
		}
		if missing.ID == 0 {
			continue // não foi excluído por equipamento (outro motivo)
		}
		// Existe regra específica (padrão, equipamento faltante)?
		rule, err := s.repo.GetSubstitutionRule(ideal.MovementPattern, missing.ID)
		if err != nil {
			return nil, nil, err
		}
		// Regra só vale se o substituto é viável p/ o atleta (está no pool filtrado).
		if rule == nil || !inPool[rule.ID] {
			continue // sem regra (ou substituto inviável): a busca genérica do pool cobre
		}
		preferred[rule.ID] = true
		subs = append(subs, domain.Substitution{
			Pattern: ideal.MovementPattern, Missing: missing.Name,
			Ideal: ideal.Name, Substitute: rule.Name, Specific: true,
		})
	}
	return preferred, subs, nil
}

// weightByPriority (Fase 6B) DUPLICA os exercícios dos padrões priorizados pelo atleta (pontos
// fracos), dando-lhes ~2× de frequência na distribuição cíclica. As cópias são espalhadas pelo
// interleaveByPattern (ficam ~1 ciclo de padrões apart -> nunca no mesmo dia). Peso 2× = placeholder
// calibrável; vazio = sem mudança.
func weightByPriority(pool []domain.Exercise, priority map[string]bool) []domain.Exercise {
	if len(priority) == 0 {
		return pool
	}
	out := make([]domain.Exercise, 0, len(pool))
	for _, e := range pool {
		out = append(out, e)
		if priority[e.MovementPattern] {
			out = append(out, e) // cópia extra (peso)
		}
	}
	return out
}

// bumpPreferred põe os substitutos preferidos (regra específica) na frente do pool, preservando a
// ordem estável do resto. Aplicado a cada pool de fase pelo gerador.
// deprioritizeRecent (Fase 6C) joga os exercícios usados no bloco ANTERIOR para o FIM do pool (fresco
// primeiro), SEM excluí-los — o motor não repete o treino anterior, mas o pool nunca esvazia num
// catálogo pequeno (degradação graciosa). Ordem estável dentro de cada grupo. Determinístico.
func deprioritizeRecent(pool []domain.Exercise, recent map[int]bool) []domain.Exercise {
	if len(recent) == 0 {
		return pool
	}
	fresh := make([]domain.Exercise, 0, len(pool))
	var used []domain.Exercise
	for _, e := range pool {
		if recent[e.ID] {
			used = append(used, e)
		} else {
			fresh = append(fresh, e)
		}
	}
	return append(fresh, used...)
}

func bumpPreferred(pool []domain.Exercise, preferred map[int]bool) []domain.Exercise {
	if len(preferred) == 0 {
		return pool
	}
	var head, tail []domain.Exercise
	for _, e := range pool {
		if preferred[e.ID] {
			head = append(head, e)
		} else {
			tail = append(tail, e)
		}
	}
	return append(head, tail...)
}

// buildWeekSpecs distribui as `weeks` semanas pelas fases: a ÚLTIMA é sempre deload;
// as demais são repartidas por week_share (acumulação -> intensificação -> realização),
// e o RPE sobe dentro de cada fase conforme rpe_step.
func buildWeekSpecs(templates []domain.PhaseTemplate, weeks int) ([]weekSpec, error) {
	byPhase := map[string]domain.PhaseTemplate{}
	for _, t := range templates {
		byPhase[t.Phase] = t
	}
	deload, ok := byPhase["deload"]
	if !ok {
		return nil, fmt.Errorf("molde de fase sem 'deload'")
	}

	// Fases de trabalho na ordem canônica.
	workPhases := []string{"accumulation", "intensification", "realization"}
	for _, p := range workPhases {
		if _, ok := byPhase[p]; !ok {
			return nil, fmt.Errorf("molde de fase sem %q", p)
		}
	}

	workWeeks := weeks - 1 // a última é deload
	// Conta semanas por fase: floor(share*workWeeks); a sobra vai p/ realização.
	counts := map[string]int{}
	assigned := 0
	for _, p := range workPhases[:2] {
		c := int(byPhase[p].WeekShare * float64(workWeeks))
		if c < 1 {
			c = 1 // garante ao menos 1 semana por fase de trabalho
		}
		counts[p] = c
		assigned += c
	}
	counts["realization"] = workWeeks - assigned
	if counts["realization"] < 1 {
		// Bloco curto demais p/ 3 fases: tira da acumulação p/ garantir realização.
		counts["accumulation"] -= 1 - counts["realization"]
		counts["realization"] = 1
	}

	var specs []weekSpec
	weekNo := 1
	for _, p := range workPhases {
		tmpl := byPhase[p]
		for i := 0; i < counts[p]; i++ {
			specs = append(specs, weekSpec{
				number:    weekNo,
				phase:     p,
				targetRPE: tmpl.BaseRPE + tmpl.RPEStep*float64(i), // sobe dentro da fase
				sets:      tmpl.DefaultSets,
				reps:      tmpl.DefaultReps,
			})
			weekNo++
		}
	}
	// Semana final: deload.
	specs = append(specs, weekSpec{
		number:    weekNo,
		phase:     "deload",
		targetRPE: deload.BaseRPE,
		isDeload:  true,
		sets:      deload.DefaultSets,
		reps:      deload.DefaultReps,
	})
	return specs, nil
}

// ActiveBlock devolve o resumo do bloco ativo do atleta (bloco + semanas), ou nil se não houver.
func (s *Service) ActiveBlock(athleteID int) (*domain.BlockOverview, error) {
	block, err := s.repo.GetActiveBlock(athleteID)
	if err != nil {
		return nil, err
	}
	if block == nil {
		return nil, nil
	}
	weeks, err := s.repo.GetBlockWeeks(block.ID)
	if err != nil {
		return nil, err
	}
	return &domain.BlockOverview{Block: *block, Weeks: weeks}, nil
}

// WeekDetail devolve uma semana do bloco ativo do atleta com sessões e prescrições aninhadas.
func (s *Service) WeekDetail(athleteID, weekNumber int) (*domain.GeneratedWeek, error) {
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
	var target *domain.BlockWeek
	for i := range weeks {
		if weeks[i].WeekNumber == weekNumber {
			target = &weeks[i]
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("semana %d não existe no bloco ativo", weekNumber)
	}

	detail := domain.GeneratedWeek{Week: *target}
	sessions, err := s.repo.GetSessions(target.ID)
	if err != nil {
		return nil, err
	}
	var exerciseIDs []int // ids distintos da semana, p/ resolver conjugados de uma vez
	seen := map[int]bool{}
	for _, sess := range sessions {
		prescriptions, err := s.repo.GetPrescriptions(sess.ID)
		if err != nil {
			return nil, err
		}
		for _, p := range prescriptions {
			if !seen[p.ExerciseID] {
				seen[p.ExerciseID] = true
				exerciseIDs = append(exerciseIDs, p.ExerciseID)
			}
		}
		// Fase 5B: WODs da sessão (já com movimentos resolvidos pelo repositório).
		conditioning, err := s.repo.GetConditioning(sess.ID)
		if err != nil {
			return nil, err
		}
		detail.Sessions = append(detail.Sessions, domain.GeneratedSession{
			Session:       sess,
			Prescriptions: prescriptions,
			Conditioning:  conditioning,
		})
	}

	// Fase 5A: resolve os componentes de TODOS os conjugados da semana numa só query (sem N+1) e
	// anexa a sequência às prescrições que são conjugados. Exercícios simples ficam sem Components.
	components, err := s.repo.ListComponents(exerciseIDs)
	if err != nil {
		return nil, err
	}
	if len(components) > 0 {
		for si := range detail.Sessions {
			ps := detail.Sessions[si].Prescriptions
			for pi := range ps {
				if comps, ok := components[ps[pi].ExerciseID]; ok {
					ps[pi].Components = comps
				}
			}
		}
	}
	return &detail, nil
}

// MarkDone registra uma prescrição como feita (com RPE real e nota opcionais).
func (s *Service) MarkDone(prescriptionID int, actualRPE *float64, notes string) error {
	return s.repo.MarkPrescriptionDone(prescriptionID, actualRPE, notes)
}
