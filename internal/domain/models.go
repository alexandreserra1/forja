// Package domain contém as structs puras do negócio — sem SQL, sem HTTP.
// É o vocabulário compartilhado por todas as camadas (repository, service, handler).
package domain

// Athlete é o dono do estado (respostas, equipamento, blocos) — Fase 6A.
type Athlete struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

// AthleteAuth são as credenciais de login (Fase A). Separado do Athlete: atletas criados no
// modo local/dev não precisam de senha. PasswordHash nunca é serializado para JSON.
type AthleteAuth struct {
	AthleteID    int    `json:"athlete_id"`
	Email        string `json:"email"`
	PasswordHash string `json:"-"`
	CreatedAt    string `json:"created_at"`
}

// AthleteMetrics são os dados físicos e de esporte do atleta (Fase 6D). Todos opcionais (nil =
// ausente = fator 1.0 = comportamento atual). Usados só para calibrar volume, nunca para SQL.
type AthleteMetrics struct {
	AthleteID     int      `json:"athlete_id"`
	AgeYears      *int     `json:"age_years,omitempty"`
	Sex           *string  `json:"sex,omitempty"`
	BodyWeightKg  *float64 `json:"body_weight_kg,omitempty"`
	Sport         *string  `json:"sport,omitempty"`
}

// MovementPattern é um padrão de movimento (squat/hinge/push/pull/olympic…). Catálogo p/ o seletor de
// prioridades (Fase 6B).
type MovementPattern struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Question é a CRIATURA 1: o que aparece na tela.
// Options vem preenchido quando o repository monta a pergunta completa.
type Question struct {
	ID        int      `json:"id"`
	Text      string   `json:"text"`
	Type      string   `json:"type"`       // single_choice | multi_choice | number
	SortOrder int      `json:"sort_order"`
	ShowWhen  *string  `json:"show_when"`  // NULL no banco = sempre exibe (Fase 0: tudo nil)
	Options   []Option `json:"options"`
}

// Option é uma alternativa de resposta de uma Question.
type Option struct {
	ID         int    `json:"id"`
	QuestionID int    `json:"question_id"`
	Label      string `json:"label"` // texto exibido: "Menos de 1 ano"
	Value      string `json:"value"` // valor interno estável: "lt_1y"
	SortOrder  int    `json:"sort_order"`
}

// Answer é a CRIATURA 2: o que o usuário respondeu.
type Answer struct {
	ID          int    `json:"id"`
	QuestionID  int    `json:"question_id"`
	AnswerValue string `json:"answer_value"` // o "value" da opção escolhida
}

// AnswerRule é a CRIATURA 3: o que a resposta significa pro motor.
// Mapeia (pergunta, valor escolhido) -> (atributo do perfil, valor do atributo).
type AnswerRule struct {
	ID             int    `json:"id"`
	QuestionID     int    `json:"question_id"`
	OptionValue    string `json:"option_value"`    // "lt_1y"
	SetsAttribute  string `json:"sets_attribute"`  // "level"
	AttributeValue string `json:"attribute_value"` // "beginner"
}

// Exercise é uma entrada do catálogo. MovementPattern vem resolvido (nome) p/ o motor
// conseguir variar o padrão de movimento sem outra query.
type Exercise struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	MovementPatternID int    `json:"movement_pattern_id"`
	MovementPattern   string `json:"movement_pattern"` // nome resolvido: "squat", "hinge"...
	Level             string `json:"level"`            // beginner | intermediate | advanced
	Focus             string `json:"focus"`            // technique | strength | conditioning
}

// GeneratedWorkout é uma linha do treino montado pelo motor: um exercício num dia.
// ExerciseName vem resolvido p/ o cliente exibir sem outra query.
type GeneratedWorkout struct {
	ID           int    `json:"id"`
	DayNumber    int    `json:"day_number"`
	ExerciseID   int    `json:"exercise_id"`
	ExerciseName string `json:"exercise_name"`
}

// ============================================================
// FASE 1 — periodização em bloco
// ============================================================

// PhaseTemplate é o MOLDE de uma fase para um objetivo: quanto das semanas ela
// ocupa, o RPE e os esquemas de série. É dado, não código.
type PhaseTemplate struct {
	ID          int     `json:"id"`
	Goal        string  `json:"goal"`
	Phase       string  `json:"phase"` // accumulation | intensification | realization | deload
	WeekShare   float64 `json:"week_share"`
	BaseRPE     float64 `json:"base_rpe"`
	RPEStep     float64 `json:"rpe_step"`
	DefaultSets int     `json:"default_sets"`
	DefaultReps int     `json:"default_reps"`
	SortOrder   int     `json:"sort_order"`
}

// TrainingBlock é o plano de 8/10/12 semanas de um atleta.
type TrainingBlock struct {
	ID          int    `json:"id"`
	AthleteID   int    `json:"athlete_id"` // Fase 6A: a quem o bloco pertence
	Goal        string `json:"goal"`
	TotalWeeks  int    `json:"total_weeks"`
	DaysPerWeek int    `json:"days_per_week"`
	Status      string `json:"status"` // active | archived
	CreatedAt   string `json:"created_at"`
}

// BlockWeek é uma semana do bloco: sabe sua fase e o RPE-alvo.
type BlockWeek struct {
	ID         int     `json:"id"`
	BlockID    int     `json:"block_id"`
	WeekNumber int     `json:"week_number"`
	Phase      string  `json:"phase"`
	TargetRPE  float64 `json:"target_rpe"`
	IsDeload   bool    `json:"is_deload"`
}

// BlockSession é um dia de treino dentro de uma semana.
type BlockSession struct {
	ID        int `json:"id"`
	WeekID    int `json:"week_id"`
	DayNumber int `json:"day_number"`
}

// Prescription é o PREVISTO: um exercício prescrito numa sessão.
// ExerciseName, Done e ActualRPE vêm resolvidos na leitura (JOIN/LEFT JOIN).
type Prescription struct {
	ID           int      `json:"id"`
	SessionID    int      `json:"session_id"`
	ExerciseID   int      `json:"exercise_id"`
	ExerciseName string   `json:"exercise_name"`
	Sets         int      `json:"sets"`
	Reps         int      `json:"reps"`
	TargetRPE    float64  `json:"target_rpe"`
	SortOrder    int      `json:"sort_order"`
	Done         bool     `json:"done"`
	ActualRPE    *float64 `json:"actual_rpe"`
	// FASE 5A: só vem preenchido quando o exercício é um CONJUGADO (kind='complex') — a sequência
	// de componentes a executar como uma unidade. Para exercícios simples, fica vazio (omitido).
	Components []ComplexComponent `json:"components,omitempty"`
}

// ComplexComponent é um movimento dentro de um conjugado (Fase 5A): o componente e suas reps, na
// ordem da sequência. Resolvido na leitura (JOIN no exercício do componente).
type ComplexComponent struct {
	ExerciseID   int    `json:"exercise_id"`
	ExerciseName string `json:"exercise_name"`
	SortOrder    int    `json:"sort_order"`
	Reps         int    `json:"reps"`
}

// SessionLog é o REALIZADO: o registro do atleta sobre uma prescrição.
type SessionLog struct {
	ID             int      `json:"id"`
	PrescriptionID int      `json:"prescription_id"`
	Done           bool     `json:"done"`
	ActualRPE      *float64 `json:"actual_rpe"`
	Notes          string   `json:"notes"`
	LoggedAt       string   `json:"logged_at"`
}

// ---- Árvore materializada que o motor monta antes de persistir ----

// GeneratedBlock é o bloco inteiro em memória: block + semanas aninhadas.
type GeneratedBlock struct {
	Block TrainingBlock   `json:"block"`
	Weeks []GeneratedWeek `json:"weeks"`
}

// GeneratedWeek é uma semana com suas sessões.
type GeneratedWeek struct {
	Week     BlockWeek          `json:"week"`
	Sessions []GeneratedSession `json:"sessions"`
}

// GeneratedSession é uma sessão com suas prescrições.
type GeneratedSession struct {
	Session       BlockSession   `json:"session"`
	Prescriptions []Prescription `json:"prescriptions"`
	// FASE 5B: WODs (condicionamento) materializados na sessão, ao lado da força. Na geração, cada um
	// traz o WOD COMPOSTO a gravar; na leitura (WeekDetail), vem resolvido com seus movimentos.
	Conditioning []ConditioningPrescription `json:"conditioning,omitempty"`
}

// ============================================================
// FASE 5B — condicionamento (WOD) por composição
// ============================================================

// EnergySystemBand é uma faixa do mapa tempo->sistema. A 1ª banda (por sort_order) cujo MaxWorkSec
// >= work_sec vence — define o sistema ENFATIZADO de um WOD pela sua duração de trabalho.
type EnergySystemBand struct {
	MaxWorkSec int    `json:"max_work_sec"`
	System     string `json:"system"`
	SortOrder  int    `json:"sort_order"`
}

// PhaseConditioning é a DOSE de condicionamento de uma fase: que sistema enfatizar, o RPE-alvo do
// WOD e quantos WODs por semana. DADO calibrável.
type PhaseConditioning struct {
	Goal           string  `json:"goal"`
	Phase          string  `json:"phase"`
	EmphasisSystem string  `json:"emphasis_system"`
	WodTargetRPE   float64 `json:"wod_target_rpe"`
	WeeklyWods     int     `json:"weekly_wods"`
	SortOrder      int     `json:"sort_order"`
}

// WodFormat é um formato de WOD (AMRAP/EMOM/ForTime/Intervals/Chipper) com sua duração típica.
type WodFormat struct {
	ID               int    `json:"id"`
	Name             string `json:"name"`
	DefaultDomainSec int    `json:"default_domain_sec"`
}

// MovementCandidate é um movimento PERFILADO (M/G/W) elegível para compor um WOD — o que o compositor
// recebe do funil (já filtrado por modalidade, nível e equipamento).
type MovementCandidate struct {
	ExerciseID   int     `json:"exercise_id"`
	ExerciseName string  `json:"exercise_name"`
	Modality     string  `json:"modality"`     // M | G | W
	SecsPerRep   float64 `json:"secs_per_rep"` // estimativa p/ dosar tempo (placeholder calibrável)
	Skill        string  `json:"skill"`        // low | med | high
	Level        string  `json:"level"`
}

// WodMovement é um movimento dentro de um WOD, com reps (nil = "max"/contínuo, ex.: corrida por tempo).
type WodMovement struct {
	ExerciseID   int    `json:"exercise_id"`
	ExerciseName string `json:"exercise_name"`
	Reps         *int   `json:"reps"`
	SortOrder    int    `json:"sort_order"`
}

// Wod é um WOD — 'benchmark' do catálogo OU 'generated' pelo compositor. Movements vem resolvido na
// leitura (e preenchido pelo compositor na geração).
type Wod struct {
	ID             int           `json:"id"`
	Name           string        `json:"name"`
	FormatID       int           `json:"format_id"`
	FormatName     string        `json:"format_name"`
	WorkSec        int           `json:"work_sec"`
	RestSec        int           `json:"rest_sec"`
	Rounds         int           `json:"rounds"`
	EmphasisSystem string        `json:"emphasis_system"`
	TargetRPE      float64       `json:"target_rpe"`
	Level          string        `json:"level"`
	Source         string        `json:"source"` // benchmark | generated
	Movements      []WodMovement `json:"movements,omitempty"`
}

// ConditioningPrescription é o WOD materializado numa sessão (irmã da Prescription de força). Na
// GERAÇÃO, Wod traz o WOD composto a gravar (sem id); na LEITURA, vem resolvido (com Movements).
type ConditioningPrescription struct {
	ID        int     `json:"id"`
	SessionID int     `json:"session_id"`
	WodID     int     `json:"wod_id"`
	TargetRPE float64 `json:"target_rpe"`
	SortOrder int     `json:"sort_order"`
	Wod       Wod     `json:"wod"`
	// AutoReg WOD: realizado (trilho independente da força).
	Done      bool     `json:"done"`
	ActualRPE *float64 `json:"actual_rpe,omitempty"`
}

// WodActual é o par previsto/realizado de um WOD (análogo a SessionActual para força).
// Usado pelo detector de estagnação do condicionamento.
type WodActual struct {
	CondPrescriptionID int      `json:"cond_prescription_id"`
	TargetRPE          float64  `json:"target_rpe"`
	ActualRPE          *float64 `json:"actual_rpe"`
	Done               bool     `json:"done"`
}

// BlockOverview é o resumo do bloco ativo para a tela de visão: bloco + semanas.
// Substitutions só vem preenchido na RESPOSTA do generate (não persistido) — Fase 3.
type BlockOverview struct {
	Block         TrainingBlock  `json:"block"`
	Weeks         []BlockWeek    `json:"weeks"`
	Substitutions []Substitution `json:"substitutions,omitempty"`
}

// ============================================================
// FASE 2 — autorregulação
// ============================================================

// SessionActual coloca o PREVISTO e o REALIZADO de uma prescrição lado a lado.
// É o tijolo do read model que o detector de estagnação lê (previsto vs realizado).
type SessionActual struct {
	PrescriptionID int      `json:"prescription_id"`
	SessionID      int      `json:"session_id"`
	DayNumber      int      `json:"day_number"`
	ExerciseID     int      `json:"exercise_id"`
	Sets           int      `json:"sets"`
	Reps           int      `json:"reps"`
	TargetRPE      float64  `json:"target_rpe"` // previsto
	ActualRPE      *float64 `json:"actual_rpe"` // realizado (nil = sem registro)
	Done           bool     `json:"done"`
}

// ============================================================
// FASE 3 — substituição por equipamento ausente
// ============================================================

// Equipment é uma entrada do catálogo de equipamentos.
type Equipment struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Substitution é o relato (não persistido) de uma troca feita na geração por falta de
// equipamento: o motor pôs Substitute no lugar do Ideal porque falta Missing. Devolvido na
// resposta do generate (em memória), para a transparência da Fase 3.
type Substitution struct {
	Pattern    string `json:"pattern"`     // padrão de movimento afetado
	Missing    string `json:"missing"`     // equipamento que o atleta não tem
	Ideal      string `json:"ideal"`       // exercício que seria o ideal (indisponível)
	Substitute string `json:"substitute"`  // exercício escolhido no lugar
	Specific   bool   `json:"specific"`    // true = veio de substitution_rule; false = busca genérica
}

// AutoregAdjustment é o RASTRO de uma intervenção da autorregulação: o que mudou,
// de quanto para quanto, e a explicação modesta mostrada ao atleta. Auditável.
type AutoregAdjustment struct {
	ID          int      `json:"id"`
	BlockID     int      `json:"block_id"`
	WeekID      int      `json:"week_id"`     // a semana AJUSTADA
	Trigger     string   `json:"trigger"`     // 'stagnation_2w'
	Action      string   `json:"action"`      // reduce_rpe | reduce_volume | reactive_deload
	RPEBefore   *float64 `json:"rpe_before"`
	RPEAfter    *float64 `json:"rpe_after"`
	SetsBefore  *int     `json:"sets_before"`
	SetsAfter   *int     `json:"sets_after"`
	Explanation string   `json:"explanation"` // linguagem modesta, sempre presente
	CreatedAt   string   `json:"created_at"`
}

// OneRM é o 1RM de um exercício específico para um atleta (Fase B).
// UPSERT: só o valor mais recente é mantido por par (athlete_id, exercise_id).
type OneRM struct {
	ID           int     `json:"id"`
	AthleteID    int     `json:"athlete_id"`
	ExerciseID   int     `json:"exercise_id"`
	ExerciseName string  `json:"exercise_name"`
	WeightKg     float64 `json:"weight_kg"`
	RecordedAt   string  `json:"recorded_at"`
}
