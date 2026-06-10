// Camada de acesso à API. Caminhos relativos: o proxy do Vite (vite.config.js)
// encaminha /api/* para o servidor Go em :8080.

// ---- Auth: token JWT em localStorage ----

const TOKEN_KEY = 'cfit_token'

export function getToken() {
  return localStorage.getItem(TOKEN_KEY)
}

export function saveToken(token) {
  localStorage.setItem(TOKEN_KEY, token)
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY)
}

// Mantido para compatibilidade com dev mode (sem auth) e multi-atleta via query param.
let currentAthlete = 1
export function setAthlete(id) {
  currentAthlete = id
}

// headers monta os cabeçalhos. Quando há token JWT, envia Authorization: Bearer.
// Mantém X-Athlete-Id para dev mode (sem AUTH_SECRET no servidor).
function headers(extra) {
  const tok = getToken()
  const auth = tok ? { Authorization: `Bearer ${tok}` } : {}
  return { 'X-Athlete-Id': String(currentAthlete), ...auth, ...extra }
}

// ---- Fase A: auth ----

export function register(name, email, password) {
  return fetch('/api/auth/register', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, email, password }),
  }).then(jsonOrThrow)
}

export function login(email, password) {
  return fetch('/api/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  }).then(jsonOrThrow)
}

async function jsonOrThrow(res) {
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    throw new Error(data.error || `HTTP ${res.status}`)
  }
  return data
}

// ---- Fase 6A: atletas ----

// GET /api/athletes -> lista de atletas (pode vir null).
export function fetchAthletes() {
  return fetch('/api/athletes', { headers: headers() })
    .then(jsonOrThrow)
    .then((list) => list || [])
}

// POST /api/athletes -> cria um atleta e devolve { id, name, ... }.
export function createAthlete(name) {
  return fetch('/api/athletes', {
    method: 'POST',
    headers: headers({ 'Content-Type': 'application/json' }),
    body: JSON.stringify({ name }),
  }).then(jsonOrThrow)
}

// GET /api/questions -> [{ id, text, type, options: [...] }]
export function fetchQuestions() {
  return fetch('/api/questions', { headers: headers() }).then(jsonOrThrow)
}

// POST /api/answers -> grava as respostas (array de { question_id, answer_value }).
export function submitAnswers(answers) {
  return fetch('/api/answers', {
    method: 'POST',
    headers: headers({ 'Content-Type': 'application/json' }),
    body: JSON.stringify(answers),
  }).then(jsonOrThrow)
}

// POST /api/generate -> roda o motor e devolve o treino gerado.
export function generateWorkout() {
  return fetch('/api/generate', { method: 'POST', headers: headers() }).then(jsonOrThrow)
}

// GET /api/workout -> devolve o último treino gerado.
export function fetchWorkout() {
  return fetch('/api/workout', { headers: headers() }).then(jsonOrThrow)
}

// ---- Fase 1: bloco periodizado ----

// POST /api/block/generate -> gera e devolve o resumo do bloco (block + weeks).
export function generateBlock() {
  return fetch('/api/block/generate', { method: 'POST', headers: headers() }).then(jsonOrThrow)
}

// GET /api/block -> resumo do bloco ativo (ou erro 404 se não houver).
export function fetchBlock() {
  return fetch('/api/block', { headers: headers() }).then(jsonOrThrow)
}

// GET /api/block/week/{n} -> a semana n com sessões + prescrições.
export function fetchWeek(n) {
  return fetch(`/api/block/week/${n}`, { headers: headers() }).then(jsonOrThrow)
}

// POST /api/session/done -> marca uma prescrição como feita.
export function markDone(prescriptionId, actualRpe, notes) {
  return fetch('/api/session/done', {
    method: 'POST',
    headers: headers({ 'Content-Type': 'application/json' }),
    body: JSON.stringify({
      prescription_id: prescriptionId,
      actual_rpe: actualRpe ?? null,
      notes: notes ?? '',
    }),
  }).then(jsonOrThrow)
}

// ---- Fase 2: autorregulação ----

// POST /api/block/evaluate -> avalia a última semana e alivia a próxima se houver estagnação.
export function evaluateBlock() {
  return fetch('/api/block/evaluate', { method: 'POST', headers: headers() }).then(jsonOrThrow)
}

// GET /api/block/adjustments -> histórico de ajustes do bloco ativo (pode vir null).
export function fetchAdjustments() {
  return fetch('/api/block/adjustments', { headers: headers() })
    .then(jsonOrThrow)
    .then((list) => list || [])
}

// ---- Fase 3: equipamento ----

// GET /api/equipment -> catálogo de equipamentos (pode vir null).
export function fetchEquipment() {
  return fetch('/api/equipment', { headers: headers() })
    .then(jsonOrThrow)
    .then((list) => list || [])
}

// POST /api/equipment -> grava o equipamento do atleta (array de ids).
export function saveEquipment(equipmentIds) {
  return fetch('/api/equipment', {
    method: 'POST',
    headers: headers({ 'Content-Type': 'application/json' }),
    body: JSON.stringify(equipmentIds),
  }).then(jsonOrThrow)
}

// ---- Fase 6B: prioridades (pontos fracos) ----

// GET /api/patterns -> catálogo de padrões de movimento (pode vir null).
export function fetchPatterns() {
  return fetch('/api/patterns', { headers: headers() })
    .then(jsonOrThrow)
    .then((list) => list || [])
}

// POST /api/priorities -> grava os padrões priorizados do atleta (array de ids).
export function savePriorities(patternIds) {
  return fetch('/api/priorities', {
    method: 'POST',
    headers: headers({ 'Content-Type': 'application/json' }),
    body: JSON.stringify(patternIds),
  }).then(jsonOrThrow)
}

// ---- Fase 6D: métricas do atleta ----

// ---- AutoReg WOD ----

// POST /api/wod/done -> marca uma conditioning_prescription como feita + RPE opcional.
export function markWodDone(condPrescriptionId, actualRpe) {
  return fetch('/api/wod/done', {
    method: 'POST',
    headers: headers({ 'Content-Type': 'application/json' }),
    body: JSON.stringify({ cond_prescription_id: condPrescriptionId, actual_rpe: actualRpe ?? null }),
  }).then(jsonOrThrow)
}

// ---- Fase B: 1RM ----

// GET /api/1rm → lista os 1RMs do atleta.
export function fetchOneRMs() {
  return fetch('/api/1rm', { headers: headers() })
    .then(jsonOrThrow)
    .then((list) => list || [])
}

// POST /api/1rm → { exercise_id, weight_kg } → UPSERT.
export function saveOneRM(exerciseId, weightKg) {
  return fetch('/api/1rm', {
    method: 'POST',
    headers: headers({ 'Content-Type': 'application/json' }),
    body: JSON.stringify({ exercise_id: exerciseId, weight_kg: weightKg }),
  }).then(jsonOrThrow)
}

// POST /api/metrics -> grava as métricas do atleta (UPSERT).
export function saveMetrics(metricsObj) {
  return fetch('/api/metrics', {
    method: 'POST',
    headers: headers({ 'Content-Type': 'application/json' }),
    body: JSON.stringify(metricsObj),
  }).then(jsonOrThrow)
}
