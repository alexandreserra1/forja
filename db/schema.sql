-- ============================================================
-- treino / cfit — Fase 0
-- Aplicar: sqlite3 cfit.db < db/schema.sql && sqlite3 cfit.db < db/seed.sql
-- ============================================================

PRAGMA foreign_keys = ON;

-- ============================================================
-- CRIATURA 1: a pergunta (o que aparece na tela)
-- ============================================================
CREATE TABLE question (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    text        TEXT    NOT NULL,              -- "Há quanto tempo você treina?"
    type        TEXT    NOT NULL
                CHECK (type IN ('single_choice','multi_choice','number')),  -- só valores válidos
    sort_order  INTEGER NOT NULL,              -- ordem de exibição
    show_when   TEXT                           -- condição p/ aparecer (NULL = sempre). Fase 0: tudo NULL.
);

CREATE TABLE question_option (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    question_id INTEGER NOT NULL REFERENCES question(id),
    label       TEXT    NOT NULL,              -- "Menos de 1 ano"
    value       TEXT    NOT NULL,              -- "lt_1y" (valor interno estável)
    sort_order  INTEGER NOT NULL
);

-- ============================================================
-- CRIATURA 3: a interpretação (resposta -> atributo do perfil)
-- É isto que alimenta o motor. Editável sem tocar no código.
-- ============================================================
CREATE TABLE answer_rule (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    question_id     INTEGER NOT NULL REFERENCES question(id),
    option_value    TEXT    NOT NULL,          -- "lt_1y"
    sets_attribute  TEXT    NOT NULL,          -- "level"
    attribute_value TEXT    NOT NULL           -- "beginner"
);

-- ============================================================
-- O catálogo de exercícios
-- ============================================================
CREATE TABLE movement_pattern (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE                  -- "squat", "hinge", "push", "pull"
);

CREATE TABLE exercise (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    name                TEXT    NOT NULL,       -- "Back Squat"
    movement_pattern_id INTEGER NOT NULL REFERENCES movement_pattern(id),
    level               TEXT    NOT NULL
                        CHECK (level IN ('beginner','intermediate','advanced')),
    focus               TEXT    NOT NULL
                        CHECK (focus IN ('technique','strength','conditioning')),
    -- FASE 5A: 'simple' (movimento único) ou 'complex' (conjugado = sequência feita como uma unidade).
    -- DEFAULT 'simple' mantém os INSERTs existentes (que listam colunas) intactos. Um conjugado é uma
    -- linha como qualquer outra (entra no MESMO funil de seleção e na MESMA prescription); seus
    -- componentes vivem em complex_item, e seu exercise_equipment é a UNIÃO dos equipamentos deles.
    kind                TEXT    NOT NULL DEFAULT 'simple'
                        CHECK (kind IN ('simple','complex'))
);

-- FASE 5A: os componentes de um conjugado, em ordem. Cada componente é um exercício REAL do catálogo.
-- A viabilidade do conjugado = AND da viabilidade dos componentes — garantida mantendo
-- exercise_equipment(conjugado) = UNIÃO dos equipamentos dos componentes (dado calibrável no seed).
CREATE TABLE complex_item (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    complex_id            INTEGER NOT NULL REFERENCES exercise(id),
    component_exercise_id INTEGER NOT NULL REFERENCES exercise(id),
    sort_order            INTEGER NOT NULL,     -- ordem na sequência (1, 2, 3...)
    reps                  INTEGER NOT NULL,     -- reps do componente por série/round (placeholder calibrável)
    UNIQUE (complex_id, sort_order)
);

-- ============================================================
-- CRIATURA 2: a resposta do usuário + o perfil + o treino gerado
-- (Fase 0: um único usuário implícito; Fase 6A: vira ATLETA, ainda sem auth)
-- ============================================================

-- FASE 6A: o ATLETA. Sem auth (dívida mantida) — escolhido por id. O atleta implícito das fases
-- anteriores vira o id 1 (DEFAULT 1 nas tabelas de estado), p/ os fluxos atuais seguirem iguais.
CREATE TABLE athlete (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE user_answer (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id   INTEGER NOT NULL DEFAULT 1 REFERENCES athlete(id),
    question_id  INTEGER NOT NULL REFERENCES question(id),
    answer_value TEXT    NOT NULL              -- o "value" da opção escolhida
);

-- Perfil = atributos derivados das respostas via answer_rule.
-- Fase 0 pode derivar em memória; tabela opcional para persistir.
CREATE TABLE profile_attribute (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    attribute_name  TEXT NOT NULL,             -- "level"
    attribute_value TEXT NOT NULL              -- "beginner"
);

CREATE TABLE generated_workout (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    day_number  INTEGER NOT NULL,              -- dia 1, 2, 3...
    exercise_id INTEGER NOT NULL REFERENCES exercise(id)
);

-- ============================================================
-- FASE 1 — PERIODIZAÇÃO EM BLOCO
-- Hierarquia: training_block -> block_week -> block_session -> prescription
-- + session_log (o REALIZADO) e phase_template (o molde como dado).
-- ============================================================

-- Os "moldes de fase" como DADO: como cada foco distribui as fases e o RPE.
-- O motor lê isto; nada de números cravados em if's. (números são placeholder
-- fundamentado — ver seção de pesquisa no plan.md)
CREATE TABLE phase_template (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    goal         TEXT    NOT NULL,                 -- 'strength' | 'conditioning' | 'technique'
    phase        TEXT    NOT NULL
                 CHECK (phase IN ('accumulation','intensification','realization','deload')),
    week_share   REAL    NOT NULL,                 -- fração das semanas dedicada à fase (ex 0.5)
    base_rpe     REAL    NOT NULL,                 -- RPE no início da fase
    rpe_step     REAL    NOT NULL,                 -- quanto o RPE sobe por semana dentro da fase
    default_sets INTEGER NOT NULL,
    default_reps INTEGER NOT NULL,
    sort_order   INTEGER NOT NULL
);

-- O BLOCO: o plano de 8/10/12 semanas de um atleta (Fase 6A: vinculado ao atleta).
CREATE TABLE training_block (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id    INTEGER NOT NULL DEFAULT 1 REFERENCES athlete(id),
    goal          TEXT    NOT NULL,                -- do perfil
    total_weeks   INTEGER NOT NULL CHECK (total_weeks IN (8,10,12)),
    days_per_week INTEGER NOT NULL CHECK (days_per_week BETWEEN 3 AND 5),
    status        TEXT    NOT NULL DEFAULT 'active' CHECK (status IN ('active','archived')),
    created_at    TEXT    NOT NULL DEFAULT (datetime('now'))
);

-- A SEMANA: sabe a que fase pertence e seu RPE-alvo.
CREATE TABLE block_week (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    block_id     INTEGER NOT NULL REFERENCES training_block(id),
    week_number  INTEGER NOT NULL,                 -- 1..total_weeks
    phase        TEXT    NOT NULL
                 CHECK (phase IN ('accumulation','intensification','realization','deload')),
    target_rpe   REAL    NOT NULL,
    is_deload    INTEGER NOT NULL DEFAULT 0        -- 0/1
);

-- A SESSÃO: um dia de treino dentro de uma semana.
CREATE TABLE block_session (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    week_id      INTEGER NOT NULL REFERENCES block_week(id),
    day_number   INTEGER NOT NULL                  -- 1..days_per_week
);

-- A PRESCRIÇÃO: o PREVISTO. Um exercício na sessão.
CREATE TABLE prescription (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id   INTEGER NOT NULL REFERENCES block_session(id),
    exercise_id  INTEGER NOT NULL REFERENCES exercise(id),
    sets         INTEGER NOT NULL,
    reps         INTEGER NOT NULL,
    target_rpe   REAL    NOT NULL,                 -- normalmente herda o da semana
    sort_order   INTEGER NOT NULL
);

-- O REGISTRO: o REALIZADO. Base da autorregulação da Fase 2.
-- UNIQUE(prescription_id): um registro por prescrição -> "marcar feito" vira upsert idempotente.
CREATE TABLE session_log (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    prescription_id INTEGER NOT NULL UNIQUE REFERENCES prescription(id),
    done            INTEGER NOT NULL DEFAULT 0,    -- 0/1
    actual_rpe      REAL,                          -- opcional: quão difícil foi de verdade
    notes           TEXT,
    logged_at       TEXT
);

-- ============================================================
-- FASE 2 — AUTORREGULAÇÃO
-- O motor passa a comparar previsto (prescription) vs realizado (session_log)
-- e a ALIVIAR a carga das próximas semanas. Todo ajuste deixa rastro aqui.
-- ============================================================

-- O AJUSTE: registro de toda intervenção da autorregulação.
-- Existe para AUDITAR e EXPLICAR — nunca um ajuste sem rastro. O motor reescreve
-- target_rpe/sets da semana alvo E grava aqui o "antes/depois" + a explicação modesta.
CREATE TABLE autoreg_adjustment (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    block_id      INTEGER NOT NULL REFERENCES training_block(id),
    week_id       INTEGER NOT NULL REFERENCES block_week(id),   -- a semana AJUSTADA
    trigger       TEXT    NOT NULL,                             -- 'stagnation_2w' (o sinal)
    action        TEXT    NOT NULL
                  CHECK (action IN ('reduce_rpe','reduce_volume','reactive_deload','reduce_wod_dose')),
    rpe_before    REAL,
    rpe_after     REAL,
    sets_before   INTEGER,
    sets_after    INTEGER,
    explanation   TEXT    NOT NULL,                             -- a frase mostrada ao atleta
    created_at    TEXT    NOT NULL DEFAULT (datetime('now'))
);

-- ============================================================
-- FASE 3 — SUBSTITUIÇÃO POR EQUIPAMENTO AUSENTE
-- O motor nunca prescreve um exercício que o atleta não pode fazer.
-- Substituir é TAXONOMIA (query), não palpite: mesmo padrão, estímulo coerente, equipamento disponível.
-- ============================================================

-- Catálogo de equipamentos (criado na Fase 3 — não existia antes).
CREATE TABLE equipment (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE                  -- "Barra", "Rack", "Kettlebell"...
);

-- Quais equipamentos cada exercício EXIGE (N:N). Exercício sem linha aqui = não exige nada
-- (peso do corpo) e está SEMPRE disponível.
CREATE TABLE exercise_equipment (
    exercise_id  INTEGER NOT NULL REFERENCES exercise(id),
    equipment_id INTEGER NOT NULL REFERENCES equipment(id),
    PRIMARY KEY (exercise_id, equipment_id)
);

-- O equipamento que o ATLETA tem (vem do questionário). Fase 6A: por atleta.
-- Vazio = sem filtro (assume tudo disponível), p/ não quebrar blocos das Fases 1/2.
CREATE TABLE user_equipment (
    athlete_id   INTEGER NOT NULL DEFAULT 1 REFERENCES athlete(id),
    equipment_id INTEGER NOT NULL REFERENCES equipment(id),
    PRIMARY KEY (athlete_id, equipment_id)
);

-- FASE 6B: padrões de movimento que o atleta quer PRIORIZAR (pontos fracos). O motor dá mais peso a
-- eles na seleção de força. N:N como user_equipment; vazio = sem peso. PK cobre as consultas.
CREATE TABLE athlete_priority (
    athlete_id          INTEGER NOT NULL REFERENCES athlete(id),
    movement_pattern_id INTEGER NOT NULL REFERENCES movement_pattern(id),
    PRIMARY KEY (athlete_id, movement_pattern_id)
);

-- FASE 6D: dados físicos e de esporte do atleta (todos opcionais/nullable). Lidos 1× por
-- GenerateBlock para calibrar volume. PK = athlete_id (1 linha por atleta, UPSERT).
CREATE TABLE athlete_metrics (
    athlete_id      INTEGER PRIMARY KEY REFERENCES athlete(id),
    age_years       INTEGER CHECK (age_years IS NULL OR age_years > 0),
    sex             TEXT    CHECK (sex IS NULL OR sex IN ('m','f','x')),
    body_weight_kg  REAL    CHECK (body_weight_kg IS NULL OR body_weight_kg > 0),
    sport           TEXT    CHECK (sport IS NULL OR
                                   sport IN ('crossfit','general_fitness','weightlifting','endurance'))
);

-- A regra ESPECÍFICA de substituição: dado (padrão, equipamento faltante) -> exercício substituto
-- preferido. Tem prioridade sobre a busca genérica (o pool filtrado). É DADO calibrável.
-- A coluna phase existe para o futuro (substituto por fase); a Fase 3 ainda não a honra (ver plan.md).
CREATE TABLE substitution_rule (
    id                     INTEGER PRIMARY KEY AUTOINCREMENT,
    movement_pattern_id    INTEGER NOT NULL REFERENCES movement_pattern(id),
    phase                  TEXT
                           CHECK (phase IS NULL OR phase IN
                                  ('accumulation','intensification','realization','deload')),
    missing_equipment_id   INTEGER NOT NULL REFERENCES equipment(id),
    substitute_exercise_id INTEGER NOT NULL REFERENCES exercise(id),
    UNIQUE (movement_pattern_id, phase, missing_equipment_id)
);

-- ============================================================
-- FASE 5B — CONDICIONAMENTO (WODs estilo CrossFit) periodizado por sistema energético.
-- A ciência dos 3 sistemas é sólida; os WODs concretos e as faixas exatas são DADO calibrável.
-- O motor "enfatiza" um sistema, nunca afirma "isolar". Tudo aqui é catálogo (lido) + bloco (gravado).
-- ============================================================

-- Catálogo de FORMATOS de WOD (AMRAP, EMOM, For Time, Intervals, Chipper...). DADO calibrável.
CREATE TABLE wod_format (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    name               TEXT NOT NULL UNIQUE,        -- 'AMRAP','EMOM','ForTime','Intervals','Chipper'
    default_domain_sec INTEGER NOT NULL             -- duração típica em segundos (placeholder)
);

-- Catálogo de WODs (placeholder FUNDAMENTADO e calibrável). emphasis_system = sistema ENFATIZADO
-- (não isolado); work_sec é a duração de trabalho alvo (usada p/ validar contra energy_system_map).
CREATE TABLE wod (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT    NOT NULL,
    format_id       INTEGER NOT NULL REFERENCES wod_format(id),
    work_sec        INTEGER NOT NULL,               -- duração de trabalho alvo
    rest_sec        INTEGER NOT NULL DEFAULT 0,
    rounds          INTEGER NOT NULL DEFAULT 1,
    emphasis_system TEXT    NOT NULL
                    CHECK (emphasis_system IN ('phosphagen','glycolytic','oxidative','mixed')),
    target_rpe      REAL    NOT NULL,               -- default do catálogo (a fase manda na prescrição)
    level           TEXT    NOT NULL
                    CHECK (level IN ('beginner','intermediate','advanced')),
    -- FASE 5B (gerador): 'benchmark' = WOD nomeado de referência (os 12 semeados); 'generated' = WOD
    -- montado pelo compositor e materializado na geração do bloco. DEFAULT 'generated'.
    source          TEXT    NOT NULL DEFAULT 'generated'
                    CHECK (source IN ('benchmark','generated'))
);

-- Movimentos de cada WOD (N:N com exercise), p/ respeitar o filtro de equipamento (Fase 3). DADO.
CREATE TABLE wod_movement (
    wod_id      INTEGER NOT NULL REFERENCES wod(id),
    exercise_id INTEGER NOT NULL REFERENCES exercise(id),
    reps        INTEGER,                            -- NULL = "max"/contínuo (ex.: corrida por tempo)
    sort_order  INTEGER NOT NULL,
    PRIMARY KEY (wod_id, exercise_id, sort_order)
);

-- FASE 5B (gerador por composição): PERFIL de cada movimento que o compositor pode usar. 1:1 com
-- exercise (PK = exercise_id), só para os movimentos perfilados — exercício sem perfil não entra no
-- WOD gerado. modality = modalidade CrossFit (M monoestrutural / G ginástico / W halterofilismo);
-- secs_per_rep = estimativa de segundos por rep/unidade (placeholder calibrável, p/ dosar o tempo);
-- skill = exigência técnica (cap de segurança por nível). Sem índice em modality DE PROPÓSITO: a
-- tabela é pequena (~dezenas de linhas) e é lida uma vez por geração — indexar só pesaria nos inserts.
CREATE TABLE movement_profile (
    exercise_id  INTEGER PRIMARY KEY REFERENCES exercise(id),
    modality     TEXT NOT NULL CHECK (modality IN ('M','G','W')),
    secs_per_rep REAL NOT NULL CHECK (secs_per_rep > 0),
    skill        TEXT NOT NULL CHECK (skill IN ('low','med','high'))
);

-- Mapa tempo->sistema enfatizado (FUNDAMENTADO; as faixas exatas são calibráveis). A banda é o
-- LIMITE SUPERIOR de work_sec: a primeira banda (menor max_work_sec) que comporta o work_sec ganha.
CREATE TABLE energy_system_map (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    max_work_sec INTEGER NOT NULL,                  -- limite superior da faixa (segundos)
    system       TEXT NOT NULL
                 CHECK (system IN ('phosphagen','glycolytic','oxidative','mixed')),
    sort_order   INTEGER NOT NULL
);

-- Dose de ênfase de sistema por fase do bloco (placeholder calibrável). DADO.
-- wod_target_rpe manda na prescrição da fase; weekly_wods = quantos WODs/semana naquela fase.
CREATE TABLE phase_conditioning (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    goal            TEXT NOT NULL,
    phase           TEXT NOT NULL
                    CHECK (phase IN ('accumulation','intensification','realization','deload')),
    emphasis_system TEXT NOT NULL
                    CHECK (emphasis_system IN ('phosphagen','glycolytic','oxidative','mixed')),
    wod_target_rpe  REAL NOT NULL,
    weekly_wods     INTEGER NOT NULL,
    sort_order      INTEGER NOT NULL,
    UNIQUE (goal, phase)
);

-- Prescrição de condicionamento materializada no bloco (IRMÃ da prescription de força). Gravada na
-- MESMA transação do bloco (o session_id vem das sessões criadas em SaveGeneratedBlock).
CREATE TABLE conditioning_prescription (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id INTEGER NOT NULL REFERENCES block_session(id),
    wod_id     INTEGER NOT NULL REFERENCES wod(id),
    target_rpe REAL    NOT NULL,
    sort_order INTEGER NOT NULL,
    -- AutoReg WOD: realizado (trilho independente da força)
    done       INTEGER NOT NULL DEFAULT 0,   -- 0/1: atleta marcou como feito
    actual_rpe REAL,                         -- RPE real reportado (opcional)
    skipped    INTEGER NOT NULL DEFAULT 0    -- 1: autoregulação pulou este WOD
);

-- ============================================================
-- ÍNDICES — mantêm as queries quentes rápidas conforme o catálogo/os blocos crescem.
-- Join em coluna indexada é barato; o custo é scan em coluna sem índice (Fase 4).
-- NÃO precisam de índice extra (já cobertos): exercise_equipment(exercise_id) e
-- user_equipment(equipment_id) pela PK; equipment(name)/movement_pattern(name) pelo UNIQUE;
-- session_log(prescription_id) pelo UNIQUE; substitution_rule(...) pelo UNIQUE.
-- ============================================================
-- Seleção de exercício (pool por nível+foco e por padrão).
CREATE INDEX idx_exercise_level_focus ON exercise(level, focus);
CREATE INDEX idx_exercise_pattern     ON exercise(movement_pattern_id);
-- Travessia do bloco materializado (lida a cada leitura/geração; cresce com blocos arquivados).
CREATE INDEX idx_block_week_block     ON block_week(block_id);
CREATE INDEX idx_block_session_week   ON block_session(week_id);
CREATE INDEX idx_prescription_session ON prescription(session_id);
CREATE INDEX idx_autoreg_block        ON autoreg_adjustment(block_id);
-- Componentes de um conjugado, carregados em lote (IN) ao montar a semana (Fase 5A).
CREATE INDEX idx_complex_item_complex ON complex_item(complex_id);
-- Movimentos dos WODs da semana, carregados em lote (IN); prescrição de condicionamento por sessão.
CREATE INDEX idx_wod_movement_wod         ON wod_movement(wod_id);
CREATE INDEX idx_cond_prescription_session ON conditioning_prescription(session_id);
-- Estado por atleta (Fase 6A): respostas e blocos filtrados por athlete_id. user_equipment já é
-- coberto pela PK (athlete_id, equipment_id).
CREATE INDEX idx_user_answer_athlete    ON user_answer(athlete_id);
CREATE INDEX idx_training_block_athlete ON training_block(athlete_id);

-- ============================================================
-- FASE A — AUTH (email + senha, JWT)
-- Separada do athlete para manter o atleta criável sem credenciais (modo local/dev).
-- ON DELETE CASCADE: apagar o atleta limpa as credenciais automaticamente.
-- ============================================================
CREATE TABLE athlete_auth (
    athlete_id    INTEGER PRIMARY KEY REFERENCES athlete(id) ON DELETE CASCADE,
    email         TEXT    NOT NULL UNIQUE,
    password_hash TEXT    NOT NULL,   -- bcrypt cost 12
    created_at    TEXT    NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_athlete_auth_email ON athlete_auth(email);

-- ============================================================
-- FASE B — 1RM do atleta (base para calibração de % de carga)
-- UNIQUE(athlete_id, exercise_id): só o 1RM mais recente por exercício (UPSERT).
-- ============================================================
CREATE TABLE athlete_1rm (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id  INTEGER NOT NULL REFERENCES athlete(id) ON DELETE CASCADE,
    exercise_id INTEGER NOT NULL REFERENCES exercise(id),
    weight_kg   REAL    NOT NULL CHECK (weight_kg > 0),
    recorded_at TEXT    NOT NULL DEFAULT (datetime('now')),
    UNIQUE (athlete_id, exercise_id)
);
CREATE INDEX idx_athlete_1rm_athlete ON athlete_1rm(athlete_id);
