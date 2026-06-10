// Teste do M4: prova o motor isolado, com um repositório falso em memória.
// Sem banco, sem HTTP — só a regra de negócio.
package service

import (
	"sort"
	"testing"

	"treino/internal/domain"
)

// fakeRepo implementa repository.Repository em memória.
type fakeRepo struct {
	answers   []domain.Answer
	rules     []domain.AnswerRule
	exercises []domain.Exercise // catálogo completo; ListByLevel filtra
	saved     []domain.GeneratedWorkout

	// Fase 1 — bloco em memória.
	templates     []domain.PhaseTemplate
	blocks        []domain.TrainingBlock
	weeks         []domain.BlockWeek
	sessions      []domain.BlockSession
	prescriptions []domain.Prescription
	logs          map[int]domain.SessionLog // prescriptionID -> log
	nextID        int

	// Fase 2 — ajustes da autorregulação.
	adjustments []domain.AutoregAdjustment

	// Fase 3 — equipamento e substituição.
	equipment     []domain.Equipment
	exerciseEquip map[int][]int // exerciseID -> equipment ids exigidos
	userEquip     []int         // equipment ids do atleta
	subRules      []fakeSubRule

	// Fase 5A — conjugados: exerciseID do conjugado -> componentes.
	components map[int][]domain.ComplexComponent

	// Fase 6A — atletas.
	athletes []domain.Athlete

	// Fase 5B — condicionamento: substrato do compositor + WODs materializados.
	energyBands      []domain.EnergySystemBand
	phaseCond        []domain.PhaseConditioning
	wodFormats       []domain.WodFormat
	movementProfiles []domain.MovementCandidate
	wods             []domain.Wod
	condBySession    map[int][]domain.ConditioningPrescription

	// Fase 6B — prioridades (pontos fracos).
	patterns   []domain.MovementPattern
	priorities []int // pattern ids priorizados (store único do atleta semeado)

	// Fase 6D — métricas do atleta (keyed por athleteID).
	metricsStore map[int]*domain.AthleteMetrics

	// AutoReg WOD — prescrições de condicionamento em memória.
	wodPrescriptions []fakeCondPrescription
}

// fakeCondPrescription espelha conditioning_prescription para o fakeRepo (AutoReg WOD).
type fakeCondPrescription struct {
	domain.ConditioningPrescription
	weekID  int
	skipped bool
}

// fakeSubRule espelha uma linha de substitution_rule (phase ignorada na Fase 3).
type fakeSubRule struct {
	pattern    string
	missing    int
	substitute int // exerciseID
}

func (f *fakeRepo) id() int {
	f.nextID++
	return f.nextID
}

func (f *fakeRepo) ListQuestions() ([]domain.Question, error) { return nil, nil }
func (f *fakeRepo) ListOptions(int) ([]domain.Option, error)  { return nil, nil }

// Fase 6A: respostas/equipamento são um store ÚNICO (o atleta semeado) — devolvido p/ qualquer
// athleteID, o que basta p/ os testes de service (X e Y com MESMO perfil). O isolamento real por
// atleta é coberto pelo teste de integração SQLite.
func (f *fakeRepo) SaveAnswers(_ int, a []domain.Answer) error    { f.answers = a; return nil }
func (f *fakeRepo) ListAnswers(_ int) ([]domain.Answer, error)    { return f.answers, nil }
func (f *fakeRepo) ListAnswerRules() ([]domain.AnswerRule, error) { return f.rules, nil }

// Fase 6A: atletas em memória.
func (f *fakeRepo) CreateAthlete(name string) (*domain.Athlete, error) {
	a := domain.Athlete{ID: f.id(), Name: name}
	f.athletes = append(f.athletes, a)
	return &a, nil
}
func (f *fakeRepo) ListAthletes() ([]domain.Athlete, error) { return f.athletes, nil }

// ---- Fase 6B: prioridades em memória ----

func (f *fakeRepo) ListMovementPatterns() ([]domain.MovementPattern, error) { return f.patterns, nil }

func (f *fakeRepo) ListPriorities(_ int) ([]domain.MovementPattern, error) {
	byID := map[int]domain.MovementPattern{}
	for _, p := range f.patterns {
		byID[p.ID] = p
	}
	var out []domain.MovementPattern
	for _, id := range f.priorities {
		if p, ok := byID[id]; ok {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakeRepo) SetPriorities(_ int, patternIDs []int) error {
	f.priorities = append([]int(nil), patternIDs...)
	return nil
}

// ---- Fase 6D: métricas em memória ----

func (f *fakeRepo) SaveMetrics(m domain.AthleteMetrics) error {
	if f.metricsStore == nil {
		f.metricsStore = map[int]*domain.AthleteMetrics{}
	}
	cp := m
	f.metricsStore[m.AthleteID] = &cp
	return nil
}

func (f *fakeRepo) GetMetrics(athleteID int) (*domain.AthleteMetrics, error) {
	if f.metricsStore == nil {
		return nil, nil
	}
	return f.metricsStore[athleteID], nil
}

// ---- AutoReg WOD: trilho independente em memória ----

func (f *fakeRepo) MarkWodDone(id int, rpe *float64) error {
	for i, cp := range f.wodPrescriptions {
		if cp.ID == id {
			f.wodPrescriptions[i].Done = true
			f.wodPrescriptions[i].ActualRPE = rpe
			return nil
		}
	}
	return nil
}

func (f *fakeRepo) GetWodActuals(weekID int) ([]domain.WodActual, error) {
	var out []domain.WodActual
	for _, cp := range f.wodPrescriptions {
		if cp.weekID == weekID && cp.Done {
			a := domain.WodActual{CondPrescriptionID: cp.ID, TargetRPE: cp.TargetRPE, ActualRPE: cp.ActualRPE, Done: true}
			out = append(out, a)
		}
	}
	return out, nil
}

func (f *fakeRepo) SkipWodPrescription(id int) error {
	for i, cp := range f.wodPrescriptions {
		if cp.ID == id {
			f.wodPrescriptions[i].skipped = true
			return nil
		}
	}
	return nil
}

// ---- Fase 5B: condicionamento em memória ----

func (f *fakeRepo) ListEnergySystemMap() ([]domain.EnergySystemBand, error) { return f.energyBands, nil }
func (f *fakeRepo) ListWodFormats() ([]domain.WodFormat, error)             { return f.wodFormats, nil }
func (f *fakeRepo) ListWods() ([]domain.Wod, error)                         { return f.wods, nil }

func (f *fakeRepo) ListPhaseConditioning(goal string) ([]domain.PhaseConditioning, error) {
	var out []domain.PhaseConditioning
	for _, p := range f.phaseCond {
		if p.Goal == goal {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakeRepo) FindMovementsByModality(modality, level string, equipmentIDs []int) ([]domain.MovementCandidate, error) {
	owned := map[int]bool{}
	for _, id := range equipmentIDs {
		owned[id] = true
	}
	var out []domain.MovementCandidate
	for _, m := range f.movementProfiles {
		if m.Modality != modality || m.Level != level {
			continue
		}
		if len(equipmentIDs) > 0 { // funil de equipamento (mesma lógica do ListAvailableByLevel)
			viable := true
			for _, req := range f.exerciseEquip[m.ExerciseID] {
				if !owned[req] {
					viable = false
					break
				}
			}
			if !viable {
				continue
			}
		}
		out = append(out, m)
	}
	return out, nil
}

func (f *fakeRepo) GetConditioning(sessionID int) ([]domain.ConditioningPrescription, error) {
	return f.condBySession[sessionID], nil
}

func (f *fakeRepo) RecentWodMovementIDs(athleteID int) ([]int, error) {
	recent := 0
	for _, b := range f.blocks {
		if b.AthleteID == athleteID && b.ID > recent {
			recent = b.ID
		}
	}
	if recent == 0 {
		return nil, nil
	}
	weekIDs := map[int]bool{}
	for _, w := range f.weeks {
		if w.BlockID == recent {
			weekIDs[w.ID] = true
		}
	}
	seen := map[int]bool{}
	var ids []int
	for _, s := range f.sessions {
		if !weekIDs[s.WeekID] {
			continue
		}
		for _, c := range f.condBySession[s.ID] {
			for _, m := range c.Wod.Movements {
				if !seen[m.ExerciseID] {
					seen[m.ExerciseID] = true
					ids = append(ids, m.ExerciseID)
				}
			}
		}
	}
	return ids, nil
}

func (f *fakeRepo) ListByLevel(level string) ([]domain.Exercise, error) {
	var out []domain.Exercise
	for _, e := range f.exercises {
		if e.Level == level {
			out = append(out, e)
		}
	}
	return out, nil
}

// ---- Fase 3: equipamento e substituição em memória ----

func (f *fakeRepo) ListAvailableByLevel(level string, equipmentIDs []int) ([]domain.Exercise, error) {
	if len(equipmentIDs) == 0 {
		return f.ListByLevel(level)
	}
	owned := map[int]bool{}
	for _, id := range equipmentIDs {
		owned[id] = true
	}
	var out []domain.Exercise
	for _, e := range f.exercises {
		if e.Level != level {
			continue
		}
		viable := true
		for _, req := range f.exerciseEquip[e.ID] {
			if !owned[req] {
				viable = false
				break
			}
		}
		if viable {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeRepo) FindCandidatesForPhase(stimulus, level string, equipmentIDs []int) ([]domain.Exercise, error) {
	avail, _ := f.ListAvailableByLevel(level, equipmentIDs)
	var out []domain.Exercise
	for _, e := range avail {
		if e.Focus == stimulus {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeRepo) ListComponents(complexIDs []int) (map[int][]domain.ComplexComponent, error) {
	out := map[int][]domain.ComplexComponent{}
	for _, id := range complexIDs {
		if comps, ok := f.components[id]; ok {
			out[id] = comps
		}
	}
	return out, nil
}

func (f *fakeRepo) GetSubstitutionRule(pattern string, missingEquipmentID int) (*domain.Exercise, error) {
	for _, r := range f.subRules {
		if r.pattern == pattern && r.missing == missingEquipmentID {
			for _, e := range f.exercises {
				if e.ID == r.substitute {
					ex := e
					return &ex, nil
				}
			}
		}
	}
	return nil, nil
}

func (f *fakeRepo) ListEquipment() ([]domain.Equipment, error) { return f.equipment, nil }

func (f *fakeRepo) ListUserEquipment(_ int) ([]domain.Equipment, error) {
	byID := map[int]domain.Equipment{}
	for _, e := range f.equipment {
		byID[e.ID] = e
	}
	var out []domain.Equipment
	for _, id := range f.userEquip {
		if e, ok := byID[id]; ok {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeRepo) SetUserEquipment(_ int, equipmentIDs []int) error {
	f.userEquip = append([]int(nil), equipmentIDs...)
	return nil
}

func (f *fakeRepo) ListExerciseEquipment(exerciseID int) ([]domain.Equipment, error) {
	byID := map[int]domain.Equipment{}
	for _, e := range f.equipment {
		byID[e.ID] = e
	}
	var out []domain.Equipment
	for _, id := range f.exerciseEquip[exerciseID] {
		if e, ok := byID[id]; ok {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeRepo) ClearWorkout() error                         { f.saved = nil; return nil }
func (f *fakeRepo) SaveWorkout(w []domain.GeneratedWorkout) error { f.saved = w; return nil }

func (f *fakeRepo) ListWorkout() ([]domain.GeneratedWorkout, error) {
	// Resolve o nome do exercício, como faria o SQL real.
	byID := map[int]string{}
	for _, e := range f.exercises {
		byID[e.ID] = e.Name
	}
	out := make([]domain.GeneratedWorkout, len(f.saved))
	for i, w := range f.saved {
		w.ExerciseName = byID[w.ExerciseID]
		out[i] = w
	}
	return out, nil
}

// ---- Fase 1: PhaseTemplate + Block em memória ----

func (f *fakeRepo) ListPhaseTemplates(goal string) ([]domain.PhaseTemplate, error) {
	var out []domain.PhaseTemplate
	for _, t := range f.templates {
		if t.Goal == goal {
			out = append(out, t)
		}
	}
	return out, nil
}

func (f *fakeRepo) GetActiveBlock(athleteID int) (*domain.TrainingBlock, error) {
	for i := len(f.blocks) - 1; i >= 0; i-- {
		if f.blocks[i].Status == "active" && f.blocks[i].AthleteID == athleteID {
			b := f.blocks[i]
			return &b, nil
		}
	}
	return nil, nil
}

func (f *fakeRepo) GetBlockWeeks(blockID int) ([]domain.BlockWeek, error) {
	var out []domain.BlockWeek
	for _, w := range f.weeks {
		if w.BlockID == blockID {
			out = append(out, w)
		}
	}
	return out, nil
}

func (f *fakeRepo) GetSessions(weekID int) ([]domain.BlockSession, error) {
	var out []domain.BlockSession
	for _, s := range f.sessions {
		if s.WeekID == weekID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *fakeRepo) GetPrescriptions(sessionID int) ([]domain.Prescription, error) {
	byID := map[int]string{}
	for _, e := range f.exercises {
		byID[e.ID] = e.Name
	}
	var out []domain.Prescription
	for _, p := range f.prescriptions {
		if p.SessionID == sessionID {
			p.ExerciseName = byID[p.ExerciseID]
			if log, ok := f.logs[p.ID]; ok {
				p.Done = log.Done
				p.ActualRPE = log.ActualRPE
			}
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakeRepo) ArchiveActiveBlock(athleteID int) error {
	for i := range f.blocks {
		if f.blocks[i].Status == "active" && f.blocks[i].AthleteID == athleteID {
			f.blocks[i].Status = "archived"
		}
	}
	return nil
}

func (f *fakeRepo) SaveGeneratedBlock(plan domain.GeneratedBlock) error {
	block := plan.Block
	block.ID = f.id()
	block.Status = "active"
	f.blocks = append(f.blocks, block)

	for _, gw := range plan.Weeks {
		week := gw.Week
		week.ID = f.id()
		week.BlockID = block.ID
		f.weeks = append(f.weeks, week)
		for _, gs := range gw.Sessions {
			sess := gs.Session
			sess.ID = f.id()
			sess.WeekID = week.ID
			f.sessions = append(f.sessions, sess)
			for _, p := range gs.Prescriptions {
				p.ID = f.id()
				p.SessionID = sess.ID
				f.prescriptions = append(f.prescriptions, p)
			}
			// Fase 5B: materializa o WOD composto (source='generated') + prescrição, como o SQLite.
			for _, c := range gs.Conditioning {
				wod := c.Wod
				wod.ID = f.id()
				wod.Source = "generated"
				f.wods = append(f.wods, wod)
				c.ID = f.id()
				c.SessionID = sess.ID
				c.WodID = wod.ID
				c.Wod = wod
				if f.condBySession == nil {
					f.condBySession = map[int][]domain.ConditioningPrescription{}
				}
				f.condBySession[sess.ID] = append(f.condBySession[sess.ID], c)
			}
		}
	}
	return nil
}

func (f *fakeRepo) MarkPrescriptionDone(prescriptionID int, actualRPE *float64, notes string) error {
	if f.logs == nil {
		f.logs = map[int]domain.SessionLog{}
	}
	f.logs[prescriptionID] = domain.SessionLog{
		PrescriptionID: prescriptionID, Done: true, ActualRPE: actualRPE, Notes: notes,
	}
	return nil
}

// RecentExerciseIDs (Fase 6C): ids do exercício do bloco mais recente (maior id) do atleta.
func (f *fakeRepo) RecentExerciseIDs(athleteID int) ([]int, error) {
	recent := 0
	for _, b := range f.blocks {
		if b.AthleteID == athleteID && b.ID > recent {
			recent = b.ID
		}
	}
	if recent == 0 {
		return nil, nil
	}
	weekIDs := map[int]bool{}
	for _, w := range f.weeks {
		if w.BlockID == recent {
			weekIDs[w.ID] = true
		}
	}
	sessIDs := map[int]bool{}
	for _, s := range f.sessions {
		if weekIDs[s.WeekID] {
			sessIDs[s.ID] = true
		}
	}
	seen := map[int]bool{}
	var ids []int
	for _, p := range f.prescriptions {
		if sessIDs[p.SessionID] && !seen[p.ExerciseID] {
			seen[p.ExerciseID] = true
			ids = append(ids, p.ExerciseID)
		}
	}
	return ids, nil
}

func (f *fakeRepo) BlockCount(athleteID int) (int, error) {
	n := 0
	for _, b := range f.blocks {
		if b.AthleteID == athleteID {
			n++
		}
	}
	return n, nil
}

// ---- Fase 2: read model + ajustes em memória ----

func (f *fakeRepo) GetWeekActuals(weekID int) ([]domain.SessionActual, error) {
	// dia de cada sessão da semana
	day := map[int]int{}
	for _, s := range f.sessions {
		if s.WeekID == weekID {
			day[s.ID] = s.DayNumber
		}
	}
	var out []domain.SessionActual
	for _, p := range f.prescriptions {
		d, ok := day[p.SessionID]
		if !ok {
			continue
		}
		a := domain.SessionActual{
			PrescriptionID: p.ID, SessionID: p.SessionID, DayNumber: d,
			ExerciseID: p.ExerciseID, Sets: p.Sets, Reps: p.Reps, TargetRPE: p.TargetRPE,
		}
		if log, ok := f.logs[p.ID]; ok {
			a.Done = log.Done
			a.ActualRPE = log.ActualRPE
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].DayNumber != out[j].DayNumber {
			return out[i].DayNumber < out[j].DayNumber
		}
		return out[i].PrescriptionID < out[j].PrescriptionID
	})
	return out, nil
}

func (f *fakeRepo) ApplyAdjustment(adj domain.AutoregAdjustment, updated []domain.Prescription) error {
	for _, u := range updated {
		for i := range f.prescriptions {
			if f.prescriptions[i].ID == u.ID {
				f.prescriptions[i].Sets = u.Sets
				f.prescriptions[i].TargetRPE = u.TargetRPE
			}
		}
	}
	adj.ID = f.id()
	f.adjustments = append(f.adjustments, adj)
	return nil
}

func (f *fakeRepo) ListAdjustments(blockID int) ([]domain.AutoregAdjustment, error) {
	var out []domain.AutoregAdjustment
	for i := len(f.adjustments) - 1; i >= 0; i-- {
		if f.adjustments[i].BlockID == blockID {
			out = append(out, f.adjustments[i])
		}
	}
	return out, nil
}

// Fase A — stubs de auth (fakeRepo não usa auth; apenas satisfazem a interface).
func (f *fakeRepo) CreateAuth(_ int, _, _ string) error                   { return nil }
func (f *fakeRepo) GetAuthByEmail(_ string) (*domain.AthleteAuth, error) { return nil, nil }

// Fase B — stubs de 1RM.
func (f *fakeRepo) Save1RM(_ int, _ int, _ float64) error           { return nil }
func (f *fakeRepo) List1RMs(_ int) ([]domain.OneRM, error)          { return nil, nil }

// seedTemplates espelha o seed de phase_template (strength).
func seedTemplates() []domain.PhaseTemplate {
	return []domain.PhaseTemplate{
		{Goal: "strength", Phase: "accumulation", WeekShare: 0.50, BaseRPE: 6.5, RPEStep: 0.25, DefaultSets: 4, DefaultReps: 6, SortOrder: 1},
		{Goal: "strength", Phase: "intensification", WeekShare: 0.33, BaseRPE: 8.0, RPEStep: 0.25, DefaultSets: 4, DefaultReps: 4, SortOrder: 2},
		{Goal: "strength", Phase: "realization", WeekShare: 0.17, BaseRPE: 9.0, RPEStep: 0.0, DefaultSets: 3, DefaultReps: 3, SortOrder: 3},
		{Goal: "strength", Phase: "deload", WeekShare: 0.0, BaseRPE: 5.0, RPEStep: 0.0, DefaultSets: 3, DefaultReps: 5, SortOrder: 4},
	}
}

// catálogo de teste espelhando o seed (níveis e padrões variados).
func seedExercises() []domain.Exercise {
	return []domain.Exercise{
		{ID: 1, Name: "Air Squat", MovementPattern: "squat", Level: "beginner", Focus: "technique"},
		{ID: 2, Name: "Back Squat", MovementPattern: "squat", Level: "intermediate", Focus: "strength"},
		{ID: 3, Name: "Overhead Squat", MovementPattern: "squat", Level: "advanced", Focus: "technique"},
		{ID: 4, Name: "Deadlift", MovementPattern: "hinge", Level: "intermediate", Focus: "strength"},
		{ID: 5, Name: "Kettlebell Swing", MovementPattern: "hinge", Level: "beginner", Focus: "conditioning"},
		{ID: 6, Name: "Push-up", MovementPattern: "push", Level: "beginner", Focus: "technique"},
		{ID: 7, Name: "Strict Press", MovementPattern: "push", Level: "intermediate", Focus: "strength"},
		{ID: 8, Name: "Ring Row", MovementPattern: "pull", Level: "beginner", Focus: "technique"},
		{ID: 9, Name: "Pull-up", MovementPattern: "pull", Level: "intermediate", Focus: "strength"},
	}
}

// regras espelhando o seed (level, days, goal).
func seedRules() []domain.AnswerRule {
	return []domain.AnswerRule{
		{QuestionID: 1, OptionValue: "lt_1y", SetsAttribute: "level", AttributeValue: "beginner"},
		{QuestionID: 1, OptionValue: "gt_3y", SetsAttribute: "level", AttributeValue: "advanced"},
		{QuestionID: 2, OptionValue: "3", SetsAttribute: "days", AttributeValue: "3"},
		{QuestionID: 2, OptionValue: "4", SetsAttribute: "days", AttributeValue: "4"},
		{QuestionID: 3, OptionValue: "technique", SetsAttribute: "goal", AttributeValue: "technique"},
		{QuestionID: 3, OptionValue: "strength", SetsAttribute: "goal", AttributeValue: "strength"},
		{QuestionID: 4, OptionValue: "8", SetsAttribute: "weeks", AttributeValue: "8"},
		{QuestionID: 4, OptionValue: "10", SetsAttribute: "weeks", AttributeValue: "10"},
		{QuestionID: 4, OptionValue: "12", SetsAttribute: "weeks", AttributeValue: "12"},
	}
}

func TestGenerate_BeginnerThreeDaysTechnique(t *testing.T) {
	repo := &fakeRepo{
		rules:     seedRules(),
		exercises: seedExercises(),
		answers: []domain.Answer{
			{QuestionID: 1, AnswerValue: "lt_1y"},     // level=beginner
			{QuestionID: 2, AnswerValue: "3"},         // days=3
			{QuestionID: 3, AnswerValue: "technique"}, // goal=technique
		},
	}
	svc := New(repo)

	workout, err := svc.Generate(1)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// 3 dias × 3 exercícios = 9 linhas.
	if len(workout) != 9 {
		t.Fatalf("esperava 9 linhas (3 dias × 3), veio %d", len(workout))
	}

	// Cada dia tem exatamente 3 exercícios.
	perDay := map[int]int{}
	for _, w := range workout {
		perDay[w.DayNumber]++
		if w.ExerciseName == "" {
			t.Errorf("linha sem nome resolvido: %+v", w)
		}
	}
	for day := 1; day <= 3; day++ {
		if perDay[day] != 3 {
			t.Errorf("dia %d deveria ter 3 exercícios, veio %d", day, perDay[day])
		}
	}

	// beginner só enxerga beginner: todos os exercícios escolhidos devem ser de IDs beginner.
	beginnerIDs := map[int]bool{1: true, 5: true, 6: true, 8: true}
	for _, w := range workout {
		if !beginnerIDs[w.ExerciseID] {
			t.Errorf("beginner não deveria receber exercício id=%d (%s)", w.ExerciseID, w.ExerciseName)
		}
	}
}

func TestGenerate_AdvancedStrengthUsesCascade(t *testing.T) {
	// Não existe exercício advanced+strength no catálogo. A cascata "nível e abaixo"
	// deve garantir candidatos (intermediate/beginner) — sem isso, o motor falharia.
	repo := &fakeRepo{
		rules:     seedRules(),
		exercises: seedExercises(),
		answers: []domain.Answer{
			{QuestionID: 1, AnswerValue: "gt_3y"},    // level=advanced
			{QuestionID: 2, AnswerValue: "4"},        // days=4
			{QuestionID: 3, AnswerValue: "strength"}, // goal=strength
		},
	}
	svc := New(repo)

	workout, err := svc.Generate(1)
	if err != nil {
		t.Fatalf("Generate (advanced+strength): %v", err)
	}
	// 4 dias × 3 = 12 linhas.
	if len(workout) != 12 {
		t.Fatalf("esperava 12 linhas (4 dias × 3), veio %d", len(workout))
	}
	// O primeiro exercício deve ser de foco strength (priorização funcionou).
	first := workout[0]
	if first.ExerciseName != "Back Squat" && first.ExerciseName != "Deadlift" &&
		first.ExerciseName != "Strict Press" && first.ExerciseName != "Pull-up" {
		t.Errorf("esperava 1º exercício de foco strength, veio %q", first.ExerciseName)
	}
}

func TestGenerate_PerfilIncompletoFalha(t *testing.T) {
	repo := &fakeRepo{
		rules:     seedRules(),
		exercises: seedExercises(),
		answers:   []domain.Answer{{QuestionID: 1, AnswerValue: "lt_1y"}}, // só level, falta days e goal
	}
	svc := New(repo)

	if _, err := svc.Generate(1); err == nil {
		t.Fatal("esperava erro de perfil incompleto, veio nil")
	}
}

// ---- Fase 1: motor de bloco ----

func blockRepo() *fakeRepo {
	return &fakeRepo{
		rules:     seedRules(),
		exercises: seedExercises(),
		templates: seedTemplates(),
		// Fase 3: catálogo de equipamento espelhando o seed. userEquip fica VAZIO de propósito
		// (= sem filtro), p/ os testes das Fases 1/2 seguirem idênticos.
		equipment:     seedEquipment(),
		exerciseEquip: seedExerciseEquip(),
		subRules:      seedSubRules(),
		patterns:      seedPatterns(), // Fase 6B (priorities VAZIAS por padrão = sem peso)
		answers: []domain.Answer{
			{QuestionID: 1, AnswerValue: "gt_3y"},    // level=advanced
			{QuestionID: 2, AnswerValue: "4"},        // days=4
			{QuestionID: 3, AnswerValue: "strength"}, // goal=strength
			{QuestionID: 4, AnswerValue: "8"},        // weeks=8
		},
	}
}

// seedEquipment / seedExerciseEquip / seedSubRules espelham o seed da Fase 3.
func seedEquipment() []domain.Equipment {
	return []domain.Equipment{
		{ID: 1, Name: "Barra"}, {ID: 2, Name: "Rack"}, {ID: 3, Name: "Kettlebell"},
		{ID: 4, Name: "Argolas"}, {ID: 5, Name: "Barra fixa"},
	}
}

func seedExerciseEquip() map[int][]int {
	return map[int][]int{
		2: {1, 2}, // Back Squat: Barra + Rack
		3: {1},    // Overhead Squat: Barra
		4: {1},    // Deadlift: Barra
		5: {3},    // Kettlebell Swing: Kettlebell
		7: {1},    // Strict Press: Barra
		8: {4},    // Ring Row: Argolas
		9: {5},    // Pull-up: Barra fixa
	}
}

func seedSubRules() []fakeSubRule {
	return []fakeSubRule{{pattern: "squat", missing: 2, substitute: 3}} // falta Rack -> Overhead Squat
}

// seedPatterns espelha os padrões usados pelo seedExercises (Fase 6B).
func seedPatterns() []domain.MovementPattern {
	return []domain.MovementPattern{
		{ID: 1, Name: "squat"}, {ID: 2, Name: "hinge"}, {ID: 3, Name: "push"}, {ID: 4, Name: "pull"},
	}
}

func TestGenerateBlock_Estrutura(t *testing.T) {
	svc := New(blockRepo())

	overview, err := svc.GenerateBlock(1)
	if err != nil {
		t.Fatalf("GenerateBlock: %v", err)
	}
	if overview.Block.TotalWeeks != 8 || overview.Block.DaysPerWeek != 4 {
		t.Fatalf("bloco com metadados errados: %+v", overview.Block)
	}
	if len(overview.Weeks) != 8 {
		t.Fatalf("esperava 8 semanas, veio %d", len(overview.Weeks))
	}

	// Última semana é deload.
	last := overview.Weeks[len(overview.Weeks)-1]
	if !last.IsDeload || last.Phase != "deload" {
		t.Errorf("última semana deveria ser deload, veio %+v", last)
	}
	// Só a última é deload.
	for _, w := range overview.Weeks[:len(overview.Weeks)-1] {
		if w.IsDeload {
			t.Errorf("semana %d não deveria ser deload", w.WeekNumber)
		}
	}

	// RPE não-decrescente nas semanas de trabalho (1..7) e CAI no deload.
	work := overview.Weeks[:7]
	for i := 1; i < len(work); i++ {
		if work[i].TargetRPE < work[i-1].TargetRPE {
			t.Errorf("RPE caiu dentro do trabalho: semana %d (%.2f) < semana %d (%.2f)",
				work[i].WeekNumber, work[i].TargetRPE, work[i-1].WeekNumber, work[i-1].TargetRPE)
		}
	}
	if last.TargetRPE >= work[len(work)-1].TargetRPE {
		t.Errorf("deload (%.2f) deveria ter RPE menor que a última de trabalho (%.2f)",
			last.TargetRPE, work[len(work)-1].TargetRPE)
	}
}

func TestGenerateBlock_VariaPadraoNoDia(t *testing.T) {
	svc := New(blockRepo())
	if _, err := svc.GenerateBlock(1); err != nil {
		t.Fatalf("GenerateBlock: %v", err)
	}
	// Mapa exercise_id -> padrão, p/ checar variedade dentro de um dia.
	pattern := map[int]string{}
	for _, e := range seedExercises() {
		pattern[e.ID] = e.MovementPattern
	}

	week, err := svc.WeekDetail(1, 1)
	if err != nil {
		t.Fatalf("WeekDetail: %v", err)
	}
	if len(week.Sessions) != 4 {
		t.Fatalf("semana 1 deveria ter 4 dias, veio %d", len(week.Sessions))
	}
	for _, sess := range week.Sessions {
		if len(sess.Prescriptions) == 0 {
			t.Fatalf("dia %d sem prescrições", sess.Session.DayNumber)
		}
		distinct := map[string]bool{}
		for _, p := range sess.Prescriptions {
			distinct[pattern[p.ExerciseID]] = true
			// sets/reps e RPE da semana vieram preenchidos.
			if p.Sets == 0 || p.Reps == 0 {
				t.Errorf("prescrição sem sets/reps: %+v", p)
			}
		}
		if len(distinct) < 2 {
			t.Errorf("dia %d não variou padrão de movimento (%v)", sess.Session.DayNumber, distinct)
		}
	}
}

func TestGenerateBlock_SelecaoPorFase(t *testing.T) {
	// Fase 4: cada fase puxa do pool do SEU estímulo. Com o mapa phaseStimulus
	// (accumulation/deload -> technique; intensification/realization -> strength), as semanas de
	// acumulação só podem trazer exercícios de foco 'technique', e as de intensificação/realização
	// só 'strength'. Valida a LÓGICA (a fase muda a seleção), não números absolutos.
	svc := New(blockRepo())
	if _, err := svc.GenerateBlock(1); err != nil {
		t.Fatalf("GenerateBlock: %v", err)
	}

	focus := map[int]string{}
	for _, e := range seedExercises() {
		focus[e.ID] = e.Focus
	}

	overview, err := svc.ActiveBlock(1)
	if err != nil {
		t.Fatalf("ActiveBlock: %v", err)
	}

	for _, w := range overview.Weeks {
		want := phaseStimulus[w.Phase]
		detail, err := svc.WeekDetail(1, w.WeekNumber)
		if err != nil {
			t.Fatalf("WeekDetail(%d): %v", w.WeekNumber, err)
		}
		seen := false
		for _, sess := range detail.Sessions {
			for _, p := range sess.Prescriptions {
				seen = true
				if focus[p.ExerciseID] != want {
					t.Errorf("semana %d (fase %s, estímulo %s): exercício id=%d tem foco %q, esperava %q",
						w.WeekNumber, w.Phase, want, p.ExerciseID, focus[p.ExerciseID], want)
				}
			}
		}
		if !seen {
			t.Errorf("semana %d sem prescrições", w.WeekNumber)
		}
	}

	// E a seleção REALMENTE varia entre fases distintas: o conjunto de exercícios de uma semana de
	// acumulação difere de uma de intensificação (senão a "seleção por fase" seria decorativa).
	accum, _ := svc.WeekDetail(1, 1)  // accumulation
	intens, _ := svc.WeekDetail(1, 4) // intensification
	accumIDs := exerciseIDset(accum)
	intensIDs := exerciseIDset(intens)
	overlap := 0
	for id := range accumIDs {
		if intensIDs[id] {
			overlap++
		}
	}
	if overlap == len(accumIDs) && len(accumIDs) > 0 {
		t.Errorf("acumulação e intensificação trouxeram exatamente os mesmos exercícios (%v); a fase não mudou a seleção", accumIDs)
	}
}

// weekExerciseIDs coleta os ids de exercício de uma semana, em ORDEM (dia, sort_order).
func weekExerciseIDs(t *testing.T, svc *Service, athleteID, weekNumber int) []int {
	t.Helper()
	w, err := svc.WeekDetail(athleteID, weekNumber)
	if err != nil {
		t.Fatalf("WeekDetail(%d,%d): %v", athleteID, weekNumber, err)
	}
	var ids []int
	for _, s := range w.Sessions {
		for _, p := range s.Prescriptions {
			ids = append(ids, p.ExerciseID)
		}
	}
	return ids
}

func sameOrder(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestGenerateBlock_IndividualizaPorAtleta(t *testing.T) {
	// Dois atletas com o MESMO perfil recebem programas diferentes (semente determinística),
	// ambos phase-correct; e regerar o mesmo atleta dá o mesmo programa (determinístico, sem RNG).
	repo := blockRepo() // advanced / strength / 8 semanas, sem filtro de equipamento
	svc := New(repo)

	if _, err := svc.GenerateBlock(1); err != nil {
		t.Fatalf("GenerateBlock(1): %v", err)
	}
	if _, err := svc.GenerateBlock(2); err != nil {
		t.Fatalf("GenerateBlock(2): %v", err)
	}

	w1 := weekExerciseIDs(t, svc, 1, 1)
	w2 := weekExerciseIDs(t, svc, 2, 1)
	if len(w1) == 0 || len(w2) == 0 {
		t.Fatal("semana 1 vazia para algum atleta")
	}
	// X≠Y: a seleção difere entre atletas (mesmo perfil, semente diferente).
	if sameOrder(w1, w2) {
		t.Errorf("atletas 1 e 2 receberam a MESMA seleção na semana 1 (%v); a semente não individualizou", w1)
	}

	// Ambos phase-correct: semana 1 é acumulação -> estímulo technique.
	focus := map[int]string{}
	for _, e := range seedExercises() {
		focus[e.ID] = e.Focus
	}
	for _, id := range append(append([]int{}, w1...), w2...) {
		if focus[id] != "technique" {
			t.Errorf("semana 1 (acumulação) deveria ser technique; exercício %d é %q", id, focus[id])
		}
	}

	// Fase 6C: regerar NÃO repete — o bloco novo do atleta difere do anterior (não-repetição por
	// histórico/rotação). Continua determinístico (sem RNG), só não é idempotente de propósito.
	if _, err := svc.GenerateBlock(1); err != nil {
		t.Fatalf("regerar atleta 1: %v", err)
	}
	if w1b := weekExerciseIDs(t, svc, 1, 1); sameOrder(w1, w1b) {
		t.Errorf("regeração deveria diferir do bloco anterior (6C), veio igual: %v", w1)
	}
}

// exerciseIDset coleta os IDs de exercício prescritos numa semana.
func exerciseIDset(w *domain.GeneratedWeek) map[int]bool {
	out := map[int]bool{}
	for _, sess := range w.Sessions {
		for _, p := range sess.Prescriptions {
			out[p.ExerciseID] = true
		}
	}
	return out
}

func TestDeprioritizeRecent(t *testing.T) {
	pool := []domain.Exercise{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}}
	got := deprioritizeRecent(pool, map[int]bool{2: true, 4: true})
	want := []int{1, 3, 2, 4} // frescos (1,3) primeiro, recentes (2,4) no fim, ordem estável
	for i, e := range got {
		if e.ID != want[i] {
			t.Fatalf("deprioritize: posição %d id %d, esperava %v", i, e.ID, want)
		}
	}
	if g := deprioritizeRecent(pool, nil); len(g) != 4 || g[0].ID != 1 {
		t.Errorf("recent vazio deveria manter a ordem original")
	}
}

func TestGenerateBlock_NaoRepeteBlocoAnterior(t *testing.T) {
	// Fase 6C: gerar 2 blocos seguidos para o MESMO atleta -> o 2º difere do 1º (não repete o treino
	// anterior), mesmo no pool pequeno (a rotação por bloco garante em todas as fases).
	svc := New(blockRepo())
	if _, err := svc.GenerateBlock(1); err != nil {
		t.Fatalf("1º bloco: %v", err)
	}
	b1 := weekExerciseIDs(t, svc, 1, 1)
	if _, err := svc.GenerateBlock(1); err != nil {
		t.Fatalf("2º bloco: %v", err)
	}
	b2 := weekExerciseIDs(t, svc, 1, 1)
	if sameOrder(b1, b2) {
		t.Errorf("o bloco novo deveria diferir do anterior (não-repetição), veio igual: %v", b1)
	}
}

func TestWeightByPriority(t *testing.T) {
	pool := []domain.Exercise{{ID: 1, MovementPattern: "squat"}, {ID: 2, MovementPattern: "pull"}}
	got := weightByPriority(pool, map[string]bool{"pull": true})
	if len(got) != 3 {
		t.Fatalf("esperava 3 (pull duplicado), veio %d", len(got))
	}
	pulls := 0
	for _, e := range got {
		if e.MovementPattern == "pull" {
			pulls++
		}
	}
	if pulls != 2 {
		t.Errorf("pull deveria aparecer 2x, veio %d", pulls)
	}
	if g := weightByPriority(pool, nil); len(g) != 2 {
		t.Errorf("sem prioridade não deveria duplicar")
	}
}

func TestGenerateBlock_PrioridadeAumentaPadrao(t *testing.T) {
	pattern := map[int]string{}
	for _, e := range seedExercises() {
		pattern[e.ID] = e.MovementPattern
	}
	countPull := func(repo *fakeRepo) int {
		svc := New(repo)
		if _, err := svc.GenerateBlock(1); err != nil {
			t.Fatalf("GenerateBlock: %v", err)
		}
		overview, _ := svc.ActiveBlock(1)
		n := 0
		for _, w := range overview.Weeks {
			d, _ := svc.WeekDetail(1, w.WeekNumber)
			for _, s := range d.Sessions {
				for _, p := range s.Prescriptions {
					if pattern[p.ExerciseID] == "pull" {
						n++
					}
				}
			}
		}
		return n
	}

	base := countPull(blockRepo()) // sem prioridade

	repoP := blockRepo()
	repoP.priorities = []int{4} // pull (id 4 em seedPatterns)
	withPriority := countPull(repoP)

	if withPriority <= base {
		t.Errorf("priorizar pull deveria AUMENTAR a frequência: %d (prioridade) vs %d (base)", withPriority, base)
	}
}

func TestGenerateBlock_MetricasForca(t *testing.T) {
	// weightlifting: força+10% (5 sets base → >=5; arredondamento pode manter 5 em sets pequenos,
	// mas em blocos de 8 semanas o total de sets deve ser >= base)
	countSets := func(repo *fakeRepo) int {
		svc := New(repo)
		if _, err := svc.GenerateBlock(1); err != nil {
			t.Fatalf("GenerateBlock: %v", err)
		}
		total := 0
		overview, _ := svc.ActiveBlock(1)
		for _, w := range overview.Weeks {
			d, _ := svc.WeekDetail(1, w.WeekNumber)
			for _, s := range d.Sessions {
				for _, p := range s.Prescriptions {
					total += p.Sets
				}
			}
		}
		return total
	}

	base := countSets(blockRepo())

	repoW := blockRepo()
	sport := "weightlifting"
	repoW.metricsStore = map[int]*domain.AthleteMetrics{
		1: {AthleteID: 1, Sport: &sport},
	}
	withWeightlifting := countSets(repoW)

	if withWeightlifting < base {
		t.Errorf("weightlifting deveria ter sets >= base: %d vs %d", withWeightlifting, base)
	}

	// endurance: força−10%
	repoE := blockRepo()
	sportE := "endurance"
	repoE.metricsStore = map[int]*domain.AthleteMetrics{
		1: {AthleteID: 1, Sport: &sportE},
	}
	withEndurance := countSets(repoE)
	if withEndurance > base {
		t.Errorf("endurance deveria ter sets <= base: %d vs %d", withEndurance, base)
	}

	// sem métricas = idêntico ao base (verificado por fator 1.0)
	repoBase2 := blockRepo()
	base2 := countSets(repoBase2)
	if base2 != base {
		t.Errorf("sem métricas deveria ser idêntico: %d vs %d", base2, base)
	}
}

func TestGenerateBlock_SegundaGeracaoArquivaPrimeira(t *testing.T) {
	repo := blockRepo()
	svc := New(repo)
	if _, err := svc.GenerateBlock(1); err != nil {
		t.Fatalf("1ª geração: %v", err)
	}
	if _, err := svc.GenerateBlock(1); err != nil {
		t.Fatalf("2ª geração: %v", err)
	}
	active := 0
	for _, b := range repo.blocks {
		if b.Status == "active" {
			active++
		}
	}
	if active != 1 {
		t.Fatalf("deveria haver exatamente 1 bloco ativo, há %d", active)
	}
}

func TestGenerateBlock_PerfilSemWeeksFalha(t *testing.T) {
	repo := blockRepo()
	repo.answers = repo.answers[:3] // remove a resposta de 'weeks'
	svc := New(repo)
	if _, err := svc.GenerateBlock(1); err == nil {
		t.Fatal("esperava erro por falta de 'weeks', veio nil")
	}
}
