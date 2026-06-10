# cfit — Roadmap de evolução

## Estado atual (tudo verde ✅)

Todas as fases abaixo estão implementadas, testadas (integração + e2e) e com suite verde.

| Feature | Schema | Repo | Service | Handler | React | e2e |
|---|:---:|:---:|:---:|:---:|:---:|:---:|
| Questionário (F0) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Bloco periodizado (F1) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| AutoReg força (F2) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Equipamento + substituição (F3) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Conjugados (F5A) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Condicionamento / WODs (F5B) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| AutoReg WOD — trilho independente | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Multi-atleta + seed determinístico (F6A) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Prioridades / pontos fracos (F6B) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Métricas do atleta / volume scaling (F6D) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Não-repetição por histórico (F6C) | ✅ | ✅ | ✅ | — | — | — |

> **F6C:** `RecentExerciseIDs`, `RecentWodMovementIDs` e `deprioritizeRecent` estão ativos
> internamente no motor. Sem tela dedicada — é internal do service.

---

## Fase A — Auth (próxima — portão antes de usuário real)

**Por quê agora:** sem auth qualquer URL dá acesso aos treinos de qualquer atleta.

### DB
```sql
CREATE TABLE athlete_auth (
    athlete_id    INTEGER PRIMARY KEY REFERENCES athlete(id) ON DELETE CASCADE,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,   -- bcrypt cost 12
    created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_athlete_auth_email ON athlete_auth(email);
```

### Domain (`internal/domain/models.go`)
```go
type AthleteAuth struct {
    AthleteID    int    `json:"athlete_id"`
    Email        string `json:"email"`
    PasswordHash string `json:"-"`
    CreatedAt    string `json:"created_at"`
}
type TokenClaims struct{ AthleteID int `json:"athlete_id"` }
```

### Repository (`internal/repository/repository.go` + `sqlite_auth.go` novo)
```go
type AuthRepository interface {
    CreateAuth(athleteID int, email, passwordHash string) error
    GetAuthByEmail(email string) (*domain.AthleteAuth, error)
}
```

### Service (`internal/service/auth.go` — novo arquivo)
```go
func (s *Service) Register(name, email, password string) (*domain.Athlete, string, error)
func (s *Service) Login(email, password string) (*domain.Athlete, string, error)
func (s *Service) ValidateToken(tokenStr string) (athleteID int, err error)
// jwtSecret lido de env AUTH_SECRET; erro de boot se ausente
```

### Middleware (`internal/handler/auth_middleware.go` — novo)
```go
func RequireAuth(svc *service.Service) func(http.Handler) http.Handler
func athleteIDFromCtx(r *http.Request) int
```

### Handler (`internal/handler/http.go`)
```
Novas rotas públicas:
  POST /api/auth/register  →  postAuthRegister()
  POST /api/auth/login     →  postAuthLogin()
Todas as outras rotas: envolvidas com RequireAuth.
athleteID(r) lê do contexto (não mais do header X-Athlete-Id).
```

### Rate limiter (`internal/handler/ratelimit.go` — novo)
```go
// Token bucket por IP, só para /api/auth/*: 10 req/min, burst 3.
// golang.org/x/time/rate + sync.Map — sem Redis.
func RateLimit(rps float64, burst int) func(http.Handler) http.Handler
```

### React
- `web/src/components/AuthForm.jsx` — login + cadastro (toggle)
- `api.js`: `register(name, email, password)`, `login(email, password)`
  → salvam token em localStorage; `headers()` passa `Authorization: Bearer <token>`
- `App.jsx`: estado `authed bool`; se falso → renderiza `<AuthForm>`

### Dependências Go
```
golang.org/x/crypto           (bcrypt)
github.com/golang-jwt/jwt/v5  (JWT)
golang.org/x/time             (rate limiter)
```

### Testes
- `internal/repository/sqlite_auth_test.go` — CreateAuth, GetAuthByEmail, UNIQUE email
- `internal/handler/http_test.go` — register → login → token → recurso protegido → 401 sem token
- `web/e2e/auth.spec.js` — cadastrar, logar, gerar bloco, deslogar

### Deploy (junto com auth)
```
deploy/
  Caddyfile          ← auto-HTTPS via Let's Encrypt, proxy /api/* → :8080, serve dist/
  docker-compose.yml ← caddy + go server
  Dockerfile.server  ← build Go
```

---

## Fase B — 1RM do atleta

**Por quê:** base para calibração futura de % de carga real a partir do RPE.

### DB
```sql
CREATE TABLE athlete_1rm (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id  INTEGER NOT NULL REFERENCES athlete(id),
    exercise_id INTEGER NOT NULL REFERENCES exercise(id),
    weight_kg   REAL    NOT NULL CHECK (weight_kg > 0),
    recorded_at TEXT    NOT NULL DEFAULT (datetime('now')),
    UNIQUE (athlete_id, exercise_id)   -- UPSERT: só o mais recente
);
CREATE INDEX idx_athlete_1rm_athlete ON athlete_1rm(athlete_id);
```

### Domain
```go
type OneRM struct {
    ID           int     `json:"id"`
    AthleteID    int     `json:"athlete_id"`
    ExerciseID   int     `json:"exercise_id"`
    ExerciseName string  `json:"exercise_name"`
    WeightKg     float64 `json:"weight_kg"`
    RecordedAt   string  `json:"recorded_at"`
}
```

### Repository (`sqlite_1rm.go` — novo)
```go
type OneRMRepository interface {
    Save1RM(athleteID, exerciseID int, weightKg float64) error  // UPSERT
    List1RMs(athleteID int) ([]domain.OneRM, error)             // JOIN exercise
}
```

### Service + Handler
```go
func (s *Service) Save1RM(athleteID, exerciseID int, weightKg float64) error
func (s *Service) List1RMs(athleteID int) ([]domain.OneRM, error)
// GET  /api/1rm
// POST /api/1rm  { exercise_id, weight_kg }
```

### React
- `web/src/components/ProfileView.jsx` — métricas + lista de 1RMs editável
- `api.js`: `fetchOneRMs()`, `saveOneRM(exerciseId, weightKg)`

---

## Fase C — Timer (frontend puro, zero backend)

### Componentes
```
TimerView.jsx    — dígitos grandes (monospace ≥4rem), barra SVG circular, botões ≥56px
                   modes: amrap (countdown) | fortime (stopwatch) | emom (interval + round)
TimerControls.jsx — seletor de modo + duração antes de iniciar
```

### Comportamentos
- Últimos 10s: fundo muda para vermelho
- Sonoro: bipe 3s finais + bipe longo no zero (`AudioContext` — sem arquivo externo)
- Háptico: `navigator.vibrate([200])` por round EMOM; `[500]` no fim
- Toggle mudo + vibração (localStorage)

### Integração com WodCard
- Botão "Iniciar timer" pré-preenche mode/duration do WOD
- Ao fechar timer → WodCard abre input de RPE automaticamente

---

## Fase D — Dashboard (frontend puro, zero backend)

### Componente `DashboardView.jsx`
Dados via `fetchBlock()` + `fetchWeek(n)` já existentes. Seções:
- Badge de fase + RPE-alvo da semana
- Progresso semanal: N/M dias registrados
- Próxima sessão: dia + exercícios pendentes
- Ação rápida "Abrir treino de hoje"

---

## Fase E — Refinamentos do motor (inteligência incremental)

### E1 — Substituição por fase
`substitution_rule.phase` existe no schema mas o service passa `phase=NULL`.
Mudar `GetSubstitutionRule(pattern, phase, missingEquipmentID)` para preferir regras com phase.

### E2 — Viés de RPE por atleta
```go
// service/metrics.go
func (s *Service) RPEBias(athleteID int) (float64, error)
// avg(actual_rpe - target_rpe) dos últimos N logs.
// Se viés > +1.0: descontar na detecção de estagnação.
// GET /api/metrics/bias (informativo)
```

### E3 — Continuidade bloco-a-bloco
```go
// service/metrics.go
func blockTransitionFactor(athleteID int) float64
// Se média actual < target - 1.0 no bloco anterior: +0.25 no base_rpe do novo bloco.
// Rastro em autoreg_adjustment com trigger='block_transition'. Sem schema novo.
```

### E4 — Progresso real via reps_achieved
```sql
ALTER TABLE session_log ADD COLUMN reps_achieved INTEGER;
```
Motor detecta "mesmo RPE + mais reps" = progresso real → não alivia indevidamente.

---

## Decisões de infraestrutura (registradas)

| Decisão | Escolha | Quando reavaliar |
|---|---|---|
| Paginação | ❌ não agora (payloads ≤8KB) | Coaches com 50+ atletas |
| API Gateway | ❌ overhead desnecessário | Múltiplos serviços |
| Proxy reverso | ✅ Caddy (junto com auth) | — |
| Rate limiting | ✅ em memória só em /api/auth/* | Segunda instância → Redis |
| HTTPS | ✅ Caddy auto-HTTPS (Let's Encrypt) | — |
| Redis | ❌ não agora | Rate limit distribuído |
| WebSocket | ❌ não agora | Live coaching |
| Paginação (keyset) | ❌ não agora | `?after_id=X&limit=20` quando chegar |

---

## Verificação de cada fase

```bash
# Fase A (Auth)
go build ./... && go test ./... | grep -E 'ok|FAIL'
go list -deps ./internal/service/ | grep sqlite   # deve ser vazio
npx playwright test --workers=1

# Fase B (1RM)
go test ./internal/repository/ -run TestOneRM
go test ./internal/handler/ -run TestIntegration_1RM

# Fase C (Timer)
npm run build
npx playwright test e2e/timer.spec.js

# Fase D (Dashboard)
npx playwright test e2e/dashboard.spec.js
```

## Dívida declarada (não esquecer)
- Threshold WOD `8.0` e granularidade (pular 1) = calibrar com treinador antes de usuário real.
- `substitution_rule.phase` ignorado pelo service (E1 resolve).
- Fase 0 (`POST /api/generate`) ativa mas obsoleta — remover depois que auth estabilizar.
- Métricas de competição: futuro, aguarda definição do treinador.
