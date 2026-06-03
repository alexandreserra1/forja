# Fase 0 — Fundação

> Objetivo: uma fatia vertical fina que atravessa as três zonas.
> O usuário responde perguntas → vira perfil → motor MÍNIMO monta um treino básico → aparece na tela.
> O motor começa burro de propósito. A inteligência vem nas fases seguintes.

---

## O que esta fase prova

Que as três zonas conversam de ponta a ponta:
- **Dados** (SQLite): perguntas, opções, regras de interpretação, exercícios.
- **Servidor** (Go em camadas): handler → serviço → repository (interface) → SQLite.
- **Cliente** (React): mostra perguntas, envia respostas, exibe o treino gerado.

Quando você abrir o navegador, responder as perguntas e ver um treino aparecer, a Fase 0 está concluída.

---

## Princípios que NÃO podem ser violados (definem a arquitetura)

1. **A interface congela a fachada e liberta as entranhas.** O serviço depende do contrato `Repository`, nunca do SQLite concreto. Trocar SQLite por Postgres amanhã = trocar só a implementação.
2. **A regra de negócio nunca mora junto do acesso a dados.** Lógica de montar treino mora no serviço (teal). SQL mora no repository (cinza). Se SQL vazar pro serviço, ou regra vazar pro repository, está errado.
3. **Questionário é dado, não código.** Perguntas, opções e interpretações vivem em tabelas. O código é um leitor genérico.
4. **Três criaturas separadas:** pergunta (o que se pergunta) ≠ resposta do usuário (o que ele respondeu) ≠ interpretação (o que a resposta significa pro treino).

---

## Esquema do banco (SQLite)

```sql
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
```

### Dados de seed (mínimo para a fatia funcionar)

```sql
-- Perguntas básicas
INSERT INTO question (text, type, sort_order, show_when) VALUES
  ('Há quanto tempo você treina?', 'single_choice', 1, NULL),
  ('Quantos dias por semana você quer treinar?', 'single_choice', 2, NULL),
  ('Qual seu principal objetivo?', 'single_choice', 3, NULL);

-- Opções da pergunta 1 (tempo de treino)
INSERT INTO question_option (question_id, label, value, sort_order) VALUES
  (1, 'Menos de 1 ano', 'lt_1y', 1),
  (1, '1 a 3 anos', '1_3y', 2),
  (1, 'Mais de 3 anos', 'gt_3y', 3);

-- Opções da pergunta 2 (dias)
INSERT INTO question_option (question_id, label, value, sort_order) VALUES
  (2, '3 dias', '3', 1),
  (2, '4 dias', '4', 2);

-- Opções da pergunta 3 (objetivo)
INSERT INTO question_option (question_id, label, value, sort_order) VALUES
  (3, 'Força', 'strength', 1),
  (3, 'Condicionamento', 'conditioning', 2),
  (3, 'Técnica', 'technique', 3);

-- Interpretações: resposta -> atributo
INSERT INTO answer_rule (question_id, option_value, sets_attribute, attribute_value) VALUES
  (1, 'lt_1y', 'level', 'beginner'),
  (1, '1_3y',  'level', 'intermediate'),
  (1, 'gt_3y', 'level', 'advanced'),
  (2, '3',     'days',  '3'),
  (2, '4',     'days',  '4'),
  (3, 'strength',     'goal', 'strength'),
  (3, 'conditioning', 'goal', 'conditioning'),
  (3, 'technique',    'goal', 'technique');

-- Padrões de movimento
INSERT INTO movement_pattern (name) VALUES ('squat'), ('hinge'), ('push'), ('pull');

-- Exercícios (alguns por nível/foco para o motor mínimo ter o que escolher)
INSERT INTO exercise (name, movement_pattern_id, level, focus) VALUES
  ('Air Squat',        1, 'beginner',     'technique'),
  ('Back Squat',       1, 'intermediate', 'strength'),
  ('Overhead Squat',   1, 'advanced',     'technique'),
  ('Deadlift',         2, 'intermediate', 'strength'),
  ('Kettlebell Swing', 2, 'beginner',     'conditioning'),
  ('Push-up',          3, 'beginner',     'technique'),
  ('Strict Press',     3, 'intermediate', 'strength'),
  ('Ring Row',         4, 'beginner',     'technique'),
  ('Pull-up',          4, 'intermediate', 'strength');
```

---

## O motor MÍNIMO (regra burra, de propósito)

Mora no serviço (camada teal). Lógica da Fase 0, sem inteligência:

1. Lê os atributos do perfil: `level`, `days`, `goal`.
2. Filtra exercícios: pega os do `level` do usuário **e abaixo** (advanced enxerga advanced+intermediate+beginner). Isso garante candidatos mesmo p/ combinações sem exercício próprio (ex: advanced+strength).
   - Se `goal = technique`, prioriza `focus = technique`. Senão prioriza o foco do objetivo.
3. Distribui em `days` dias, 3–4 exercícios por dia, variando o padrão de movimento.
4. Salva em `generated_workout` e devolve.

> Isso é deliberadamente simplório. Na Fase 1 ele vira o motor de periodização de verdade — e como mora todo atrás do contrato, você troca a lógica sem mexer em handler, repository nem React.

---

## Estrutura de pastas (Go)

```
treino/
├── go.mod
├── cmd/
│   └── server/
│       └── main.go              # liga tudo: abre DB, injeta repo no serviço, sobe HTTP
├── internal/
│   ├── domain/
│   │   └── models.go            # structs: Question, Option, Exercise, Workout, Profile
│   ├── repository/
│   │   ├── repository.go        # AS INTERFACES (o contrato roxo)
│   │   └── sqlite.go            # implementação concreta (a caixa cinza)
│   ├── service/
│   │   └── periodization.go     # o motor mínimo (a caixa teal — o cérebro)
│   └── handler/
│       └── http.go              # handlers HTTP (a caixa azul)
├── db/
│   ├── schema.sql               # o CREATE TABLE acima
│   └── seed.sql                 # os INSERT acima
└── web/                         # o React vem aqui (pode ser projeto separado também)
```

### O contrato (repository.go) — o coração da arquitetura

```go
package repository

import "treino/internal/domain"

type QuestionRepository interface {
    ListQuestions() ([]domain.Question, error)
    ListOptions(questionID int) ([]domain.Option, error)
}

type AnswerRepository interface {
    SaveAnswer(questionID int, value string) error
    ListAnswers() ([]domain.Answer, error)
    ListAnswerRules() ([]domain.AnswerRule, error)
}

type ExerciseRepository interface {
    ListByLevel(level string) ([]domain.Exercise, error)
}

type WorkoutRepository interface {
    SaveWorkout(w []domain.GeneratedWorkout) error
    ListWorkout() ([]domain.GeneratedWorkout, error)
}
```

> O serviço recebe essas interfaces no construtor (injeção de dependência). Nunca importa `sqlite.go` diretamente.

---

## Endpoints da API (Fase 0)

```
GET  /api/questions          -> lista perguntas + opções (React desenha o questionário)
POST /api/answers            -> recebe as respostas do usuário
POST /api/generate           -> roda o motor mínimo, salva e devolve o treino
GET  /api/workout            -> devolve o treino gerado
```

---

## Marcos de validação (faça nesta ordem)

- [ ] **M1 — Banco de pé.** `schema.sql` + `seed.sql` rodam no SQLite sem erro. `SELECT` retorna perguntas e exercícios.
- [ ] **M2 — Repository lê.** Em Go, a implementação SQLite lista perguntas e exercícios. Teste isso com um `main` temporário ou um teste.
- [ ] **M3 — Contrato no lugar.** O serviço recebe as interfaces (não o SQLite concreto). Confirme: `service` não importa `sqlite`.
- [ ] **M4 — Motor mínimo gera.** Dado um perfil fixo (ex: beginner, 3 dias, technique), o serviço devolve um treino coerente. Teste sem HTTP ainda.
- [ ] **M5 — API responde.** `GET /api/questions` devolve JSON. `POST /api/generate` devolve um treino em JSON. Teste com curl.
- [ ] **M6 — React mostra.** Tela lista as perguntas, envia respostas, chama generate, exibe o treino. **Fim da Fase 0.**

---

## Atalhos conscientes da Fase 0 (dívida que pagamos depois)

- Sem autenticação — um único usuário implícito.
- `show_when` todo NULL — questionário ainda não ramifica (vira adaptativo na Fase 1+).
- Motor burro — sem periodização real ainda.
- Sem cache, sem Postgres, sem Docker.
- Perfil pode ser derivado em memória a cada request (a tabela `profile_attribute` é opcional nesta fase).

Cada atalho é uma porta que a arquitetura em camadas mantém aberta para evoluir sem reescrever.

---

## Quando terminar

Quando o M6 estiver verde — você responde no navegador e vê o treino — me avise e avançamos para a **Fase 1**: transformar o motor mínimo em periodização de verdade e tornar o questionário adaptativo (ativar o `show_when`).