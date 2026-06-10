// Package repository: repository.go é o CONTRATO ROXO — só interfaces.
// É o coração da arquitetura. O service depende DESTAS interfaces, nunca de sqlite.go.
// Trocar SQLite por Postgres amanhã = escrever outra implementação destes contratos,
// sem tocar em service nem handler (Princípio 1).
package repository

import "treino/internal/domain"

// QuestionRepository: leitura do questionário.
type QuestionRepository interface {
	ListQuestions() ([]domain.Question, error)
	ListOptions(questionID int) ([]domain.Option, error)
}

// AnswerRepository: respostas do atleta + regras de interpretação.
// SaveAnswers é plural: o cliente envia o formulário inteiro de uma vez, e a implementação substitui
// as respostas anteriores DAQUELE atleta (Fase 6A: estado escopado por athlete_id). ListAnswerRules
// é catálogo (não escopado).
type AnswerRepository interface {
	SaveAnswers(athleteID int, answers []domain.Answer) error
	ListAnswers(athleteID int) ([]domain.Answer, error)
	ListAnswerRules() ([]domain.AnswerRule, error)
}

// AthleteRepository: identidade do atleta (Fase 6A). Sem auth — só cria e lista.
type AthleteRepository interface {
	CreateAthlete(name string) (*domain.Athlete, error)
	ListAthletes() ([]domain.Athlete, error)
}

// ExerciseRepository: catálogo. ListByLevel devolve EXATAMENTE um nível;
// a cascata "nível e abaixo" é regra de negócio e vive no service.
//
// Fase 3: ListAvailableByLevel filtra o pool por equipamento disponível NA QUERY
// (não carrega o impossível); GetSubstitutionRule traz a preferência específica.
type ExerciseRepository interface {
	ListByLevel(level string) ([]domain.Exercise, error)
	ListAvailableByLevel(level string, equipmentIDs []int) ([]domain.Exercise, error)
	GetSubstitutionRule(pattern string, missingEquipmentID int) (*domain.Exercise, error)
	// FindCandidatesForPhase (Fase 4): pool de UM nível filtrado por foco (estímulo da fase)
	// E por equipamento disponível, tudo na query. O service decide a escolha/variedade.
	FindCandidatesForPhase(stimulus string, level string, equipmentIDs []int) ([]domain.Exercise, error)
	// ListComponents (Fase 5A): componentes dos conjugados informados, em UMA query (IN), agrupados
	// por id do conjugado. Carrega a semana inteira de uma vez (sem N+1). Ids sem componente saem fora.
	ListComponents(complexIDs []int) (map[int][]domain.ComplexComponent, error)
}

// WorkoutRepository: persistência do treino gerado.
// ClearWorkout zera o treino anterior antes de gravar um novo (idempotência da geração).
type WorkoutRepository interface {
	ClearWorkout() error
	SaveWorkout(w []domain.GeneratedWorkout) error
	ListWorkout() ([]domain.GeneratedWorkout, error)
}

// ---- FASE 1 ----

// PhaseTemplateRepository: leitura dos moldes de fase (o dado que o motor lê).
type PhaseTemplateRepository interface {
	ListPhaseTemplates(goal string) ([]domain.PhaseTemplate, error)
}

// BlockRepository: persistência e leitura do bloco materializado.
// Nota: o `CreateBlock` do spec foi absorvido por SaveGeneratedBlock — gravar a
// árvore inteira numa transação é mais seguro que criar o bloco em separado.
// BlockRepository: persistência e leitura do bloco materializado, escopado por atleta (Fase 6A:
// GetActiveBlock/ArchiveActiveBlock filtram por athlete_id; SaveGeneratedBlock grava plan.Block.AthleteID).
// As leituras por id (weeks/sessions/prescriptions) não precisam de athleteID: os ids já pertencem
// ao bloco do atleta. MarkPrescriptionDone idem (a prescrição é única).
type BlockRepository interface {
	GetActiveBlock(athleteID int) (*domain.TrainingBlock, error)
	GetBlockWeeks(blockID int) ([]domain.BlockWeek, error)
	GetSessions(weekID int) ([]domain.BlockSession, error)
	GetPrescriptions(sessionID int) ([]domain.Prescription, error)
	ArchiveActiveBlock(athleteID int) error
	SaveGeneratedBlock(plan domain.GeneratedBlock) error
	MarkPrescriptionDone(prescriptionID int, actualRPE *float64, notes string) error
	// RecentExerciseIDs (Fase 6C): ids distintos de exercício do bloco MAIS RECENTE do atleta, para
	// o motor não repetir o treino anterior. Vazio se o atleta ainda não tem bloco.
	RecentExerciseIDs(athleteID int) ([]int, error)
	// BlockCount (Fase 6C): quantos blocos o atleta já tem (ativo+arquivados). É o índice (0-based) do
	// próximo bloco — usado para ROTACIONAR a seleção a cada bloco (não-repetição robusta em todas as
	// fases, mesmo quando o pool da fase é menor que os slots e não sobra exercício "fresco").
	BlockCount(athleteID int) (int, error)
}

// ---- FASE 2 ----

// ---- FASE 3 ----

// EquipmentRepository: catálogo de equipamento, o que o atleta tem e o que cada exercício exige.
type EquipmentRepository interface {
	ListEquipment() ([]domain.Equipment, error)
	ListUserEquipment(athleteID int) ([]domain.Equipment, error)
	SetUserEquipment(athleteID int, equipmentIDs []int) error
	ListExerciseEquipment(exerciseID int) ([]domain.Equipment, error)
}

// AutoregRepository: leitura do realizado e persistência dos ajustes da autorregulação.
// GetWeekActuals alimenta o detector (previsto vs realizado); ApplyAdjustment reescreve
// a semana alvo E grava o rastro do ajuste numa transação; ListAdjustments é o histórico.
type AutoregRepository interface {
	GetWeekActuals(weekID int) ([]domain.SessionActual, error)
	ApplyAdjustment(adj domain.AutoregAdjustment, updated []domain.Prescription) error
	ListAdjustments(blockID int) ([]domain.AutoregAdjustment, error)
}

// ConditioningRepository (Fase 5B): leituras do substrato do compositor (mapa tempo->sistema, doses
// por fase, formatos, movimentos perfilados M/G/W já filtrados pelo funil) + leitura dos WODs
// materializados (catálogo e por sessão). A GRAVAÇÃO do WOD composto entra na árvore do bloco
// (GeneratedSession.Conditioning), gravada por SaveGeneratedBlock — por isso não há Save aqui.
type ConditioningRepository interface {
	ListEnergySystemMap() ([]domain.EnergySystemBand, error)
	ListPhaseConditioning(goal string) ([]domain.PhaseConditioning, error)
	ListWodFormats() ([]domain.WodFormat, error)
	FindMovementsByModality(modality, level string, equipmentIDs []int) ([]domain.MovementCandidate, error)
	ListWods() ([]domain.Wod, error)
	GetConditioning(sessionID int) ([]domain.ConditioningPrescription, error)
	// RecentWodMovementIDs (Fase 5C): ids de exercício dos movimentos de WOD do bloco MAIS RECENTE do
	// atleta, para o compositor não repetir os mesmos movimentos bloco-a-bloco. Vazio se não houver.
	RecentWodMovementIDs(athleteID int) ([]int, error)
}

// PriorityRepository (Fase 6B): catálogo de padrões de movimento + as prioridades (pontos fracos) do
// atleta, que o service usa para dar peso na seleção. Estado escopado por atleta.
type PriorityRepository interface {
	ListMovementPatterns() ([]domain.MovementPattern, error)
	ListPriorities(athleteID int) ([]domain.MovementPattern, error)
	SetPriorities(athleteID int, patternIDs []int) error
}

// AthleteMetricsRepository (Fase 6D): dados físicos e de esporte do atleta, usados para calibrar
// volume (dose). Todos os campos são opcionais (nil = ausente = fator 1.0). SaveMetrics é UPSERT.
type AthleteMetricsRepository interface {
	SaveMetrics(m domain.AthleteMetrics) error
	GetMetrics(athleteID int) (*domain.AthleteMetrics, error)
}

// AuthRepository (Fase A): credenciais do atleta. Separado do AthleteRepository para que atletas
// criados no modo local (sem senha) continuem funcionando. CreateAuth é chamado só no Register.
type AuthRepository interface {
	CreateAuth(athleteID int, email, passwordHash string) error
	GetAuthByEmail(email string) (*domain.AthleteAuth, error)
}

// WodAutoregRepository: realizado do condicionamento (trilho independente da força).
// MarkWodDone registra que o atleta fez o WOD (+ RPE opcional).
// GetWodActuals devolve os pares previsto/realizado da semana (só os done — base do detector).
// SkipWodPrescription marca uma conditioning_prescription como skipped (autoregulação reduziu dose).
type WodAutoregRepository interface {
	MarkWodDone(condPrescriptionID int, actualRPE *float64) error
	GetWodActuals(weekID int) ([]domain.WodActual, error)
	SkipWodPrescription(condPrescriptionID int) error
}

// Repository agrega todos os contratos — conveniência p/ injetar tudo de uma vez.
type Repository interface {
	QuestionRepository
	AnswerRepository
	AthleteRepository
	AuthRepository
	ExerciseRepository
	WorkoutRepository
	PhaseTemplateRepository
	BlockRepository
	AutoregRepository
	EquipmentRepository
	ConditioningRepository
	PriorityRepository
	AthleteMetricsRepository
	WodAutoregRepository
	OneRMRepository
}

// OneRMRepository: 1RM por exercício (Fase B).
type OneRMRepository interface {
	Save1RM(athleteID, exerciseID int, weightKg float64) error
	List1RMs(athleteID int) ([]domain.OneRM, error)
}

// Assertivas de compilação: se *SQLiteRepo deixar de satisfazer qualquer
// contrato, o build QUEBRA aqui — não em produção.
var (
	_ QuestionRepository      = (*SQLiteRepo)(nil)
	_ AnswerRepository        = (*SQLiteRepo)(nil)
	_ AthleteRepository       = (*SQLiteRepo)(nil)
	_ AuthRepository          = (*SQLiteRepo)(nil)
	_ ExerciseRepository      = (*SQLiteRepo)(nil)
	_ WorkoutRepository       = (*SQLiteRepo)(nil)
	_ PhaseTemplateRepository = (*SQLiteRepo)(nil)
	_ BlockRepository         = (*SQLiteRepo)(nil)
	_ AutoregRepository       = (*SQLiteRepo)(nil)
	_ EquipmentRepository     = (*SQLiteRepo)(nil)
	_ ConditioningRepository  = (*SQLiteRepo)(nil)
	_ PriorityRepository       = (*SQLiteRepo)(nil)
	_ AthleteMetricsRepository = (*SQLiteRepo)(nil)
	_ WodAutoregRepository     = (*SQLiteRepo)(nil)
	_ OneRMRepository          = (*SQLiteRepo)(nil)
	_ Repository               = (*SQLiteRepo)(nil)
)
