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
    type        TEXT    NOT NULL,              -- 'single_choice' | 'multi_choice' | 'number'
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
    level               TEXT    NOT NULL,       -- 'beginner' | 'intermediate' | 'advanced'
    focus               TEXT    NOT NULL        -- 'technique' | 'strength' | 'conditioning'
);

-- ============================================================
-- CRIATURA 2: a resposta do usuário + o perfil + o treino gerado
-- (Fase 0: um único usuário implícito, sem auth ainda)
-- ============================================================
CREATE TABLE user_answer (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
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
