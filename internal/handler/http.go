// Package handler: http.go é a CAIXA AZUL — a borda HTTP.
// Traduz requisição <-> service e devolve JSON. Nenhuma regra de negócio aqui:
// ele só descasca o HTTP e delega ao service.
package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"treino/internal/domain"
	"treino/internal/service"
)

// Handler segura o service (o cérebro) e expõe os endpoints.
type Handler struct {
	svc *service.Service
}

// athleteID resolve o atleta da requisição. Ordem de precedência:
//  1. JWT context (RequireAuth injetou)
//  2. X-Athlete-Id header (testes de integração que não passam pelo middleware)
//  3. ?athlete= query param (testes multi-atleta)
//  4. Default: 1 (atleta do seed)
func (h *Handler) athleteID(r *http.Request) int {
	if id := athleteIDFromCtx(r); id > 1 {
		return id
	}
	if v := r.Header.Get("X-Athlete-Id"); v != "" {
		if id, err := strconv.Atoi(v); err == nil && id > 0 {
			return id
		}
	}
	if v := r.URL.Query().Get("athlete"); v != "" {
		if id, err := strconv.Atoi(v); err == nil && id > 0 {
			return id
		}
	}
	return 1
}

func New(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

// Routes registra os endpoints num ServeMux.
// Rotas públicas (/api/auth/*) não passam pelo RequireAuth.
// Todas as outras são envolvidas com RequireAuth + RateLimit de auth.
func (h *Handler) Routes() *http.ServeMux {
	mux := http.NewServeMux()

	// ---- Rotas PÚBLICAS (sem JWT) ----
	mux.HandleFunc("GET /api/auth/status", h.getAuthStatus)
	authRL := RateLimit(10.0/60, 3) // 10 req/min, burst 3
	mux.Handle("POST /api/auth/register", authRL(http.HandlerFunc(h.postAuthRegister)))
	mux.Handle("POST /api/auth/login", authRL(http.HandlerFunc(h.postAuthLogin)))

	// ---- Rotas PROTEGIDAS (exigem JWT válido) ----
	auth := RequireAuth(h.svc)
	protected := func(method, pattern string, fn http.HandlerFunc) {
		mux.Handle(method+" "+pattern, auth(fn))
	}

	// Fase 6A — atletas
	protected("GET", "/api/athletes", h.getAthletes)
	protected("POST", "/api/athletes", h.postAthletes)
	// Fase 0
	protected("GET", "/api/questions", h.getQuestions)
	protected("POST", "/api/answers", h.postAnswers)
	protected("POST", "/api/generate", h.postGenerate)
	protected("GET", "/api/workout", h.getWorkout)
	// Fase 1 — bloco periodizado
	protected("POST", "/api/block/generate", h.postBlockGenerate)
	protected("GET", "/api/block", h.getBlock)
	protected("GET", "/api/block/week/{n}", h.getBlockWeek)
	protected("POST", "/api/session/done", h.postSessionDone)
	// Fase 2 — autorregulação
	protected("POST", "/api/block/evaluate", h.postBlockEvaluate)
	protected("GET", "/api/block/adjustments", h.getBlockAdjustments)
	// Fase 3 — equipamento
	protected("GET", "/api/equipment", h.getEquipment)
	protected("POST", "/api/equipment", h.postEquipment)
	// Fase 5B — condicionamento
	protected("GET", "/api/wods", h.getWods)
	// Fase 6B — prioridades
	protected("GET", "/api/patterns", h.getPatterns)
	protected("GET", "/api/priorities", h.getPriorities)
	protected("POST", "/api/priorities", h.postPriorities)
	// Fase 6D — métricas
	protected("GET", "/api/metrics", h.getMetrics)
	protected("POST", "/api/metrics", h.postMetrics)
	// AutoReg WOD
	protected("POST", "/api/wod/done", h.postWodDone)
	// Fase B — 1RM
	protected("GET", "/api/1rm", h.get1RM)
	protected("POST", "/api/1rm", h.post1RM)

	return mux
}

// GET /api/questions -> perguntas + opções para o cliente desenhar o questionário.
func (h *Handler) getQuestions(w http.ResponseWriter, r *http.Request) {
	cacheFor1h(w)
	questions, err := h.svc.Questions()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, questions)
}

// POST /api/answers -> recebe o array de respostas e grava (substituindo as anteriores).
func (h *Handler) postAnswers(w http.ResponseWriter, r *http.Request) {
	var answers []domain.Answer
	if err := json.NewDecoder(r.Body).Decode(&answers); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := h.svc.SaveAnswers(h.athleteID(r), answers); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"saved": len(answers)})
}

// POST /api/generate -> roda o motor, salva e devolve o treino.
func (h *Handler) postGenerate(w http.ResponseWriter, r *http.Request) {
	workout, err := h.svc.Generate(h.athleteID(r))
	if err != nil {
		// Perfil incompleto é erro do cliente (400), não do servidor.
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, workout)
}

// GET /api/workout -> devolve o último treino gerado.
func (h *Handler) getWorkout(w http.ResponseWriter, r *http.Request) {
	workout, err := h.svc.Workout()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, workout)
}

// ---------- Fase 1: bloco ----------

// POST /api/block/generate -> roda o motor de bloco, arquiva o antigo, salva o novo.
func (h *Handler) postBlockGenerate(w http.ResponseWriter, r *http.Request) {
	overview, err := h.svc.GenerateBlock(h.athleteID(r))
	if err != nil {
		writeError(w, http.StatusBadRequest, err) // perfil incompleto = erro do cliente
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

// GET /api/block -> resumo do bloco ativo (bloco + semanas). 404 se não houver.
func (h *Handler) getBlock(w http.ResponseWriter, r *http.Request) {
	overview, err := h.svc.ActiveBlock(h.athleteID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if overview == nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("nenhum bloco ativo"))
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

// GET /api/block/week/{n} -> a semana n com sessões + prescrições.
func (h *Handler) getBlockWeek(w http.ResponseWriter, r *http.Request) {
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil || n < 1 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("número de semana inválido"))
		return
	}
	week, err := h.svc.WeekDetail(h.athleteID(r), n)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, week)
}

// POST /api/session/done -> marca uma prescrição como feita (done, actual_rpe, notes).
func (h *Handler) postSessionDone(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PrescriptionID int      `json:"prescription_id"`
		ActualRPE      *float64 `json:"actual_rpe"`
		Notes          string   `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.PrescriptionID == 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("prescription_id obrigatório"))
		return
	}
	if err := h.svc.MarkDone(body.PrescriptionID, body.ActualRPE, body.Notes); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- Fase 2: autorregulação ----------

// POST /api/block/evaluate -> avalia a última semana concluída e ALIVIA a próxima se
// houver estagnação confirmada. Devolve sempre a decisão + explicação modesta.
func (h *Handler) postBlockEvaluate(w http.ResponseWriter, r *http.Request) {
	result, err := h.svc.EvaluateAndAdjust(h.athleteID(r))
	if err != nil {
		// "nenhum bloco ativo" é erro do cliente (não gerou bloco); 400.
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GET /api/block/adjustments -> histórico de ajustes do bloco ativo (tela de transparência).
func (h *Handler) getBlockAdjustments(w http.ResponseWriter, r *http.Request) {
	adjustments, err := h.svc.Adjustments(h.athleteID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, adjustments)
}

// ---------- Fase 3: equipamento ----------

// GET /api/equipment -> catálogo de equipamentos para o questionário marcar.
func (h *Handler) getEquipment(w http.ResponseWriter, r *http.Request) {
	cacheFor1h(w)
	equipment, err := h.svc.Equipment()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, equipment)
}

// POST /api/equipment -> grava o equipamento do atleta (array de ids).
func (h *Handler) postEquipment(w http.ResponseWriter, r *http.Request) {
	var ids []int
	if err := json.NewDecoder(r.Body).Decode(&ids); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := h.svc.SaveUserEquipment(h.athleteID(r), ids); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"saved": len(ids)})
}

// ---------- Fase 6A: atletas ----------

// GET /api/athletes -> lista de atletas (para o seletor; sem auth).
func (h *Handler) getAthletes(w http.ResponseWriter, r *http.Request) {
	athletes, err := h.svc.Athletes()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, athletes)
}

// POST /api/athletes -> cria um atleta { "name": "..." } e devolve com o id.
func (h *Handler) postAthletes(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("name obrigatório"))
		return
	}
	a, err := h.svc.CreateAthlete(body.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, a)
}

// ---------- Fase 5B: condicionamento ----------

// GET /api/wods -> catálogo de WODs (benchmark + gerados). Debug/admin.
func (h *Handler) getWods(w http.ResponseWriter, r *http.Request) {
	wods, err := h.svc.Wods()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, wods)
}

// ---------- Fase 6B: prioridades ----------

// GET /api/patterns -> catálogo de padrões de movimento (para o seletor).
func (h *Handler) getPatterns(w http.ResponseWriter, r *http.Request) {
	cacheFor1h(w)
	patterns, err := h.svc.Patterns()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, patterns)
}

// GET /api/priorities -> padrões priorizados do atleta.
func (h *Handler) getPriorities(w http.ResponseWriter, r *http.Request) {
	priorities, err := h.svc.Priorities(h.athleteID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, priorities)
}

// POST /api/priorities -> grava as prioridades do atleta (array de pattern ids).
func (h *Handler) postPriorities(w http.ResponseWriter, r *http.Request) {
	var ids []int
	if err := json.NewDecoder(r.Body).Decode(&ids); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := h.svc.SavePriorities(h.athleteID(r), ids); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"saved": len(ids)})
}

// ---------- AutoReg WOD ----------

// POST /api/wod/done -> marca um WOD (conditioning_prescription) como feito + RPE opcional.
func (h *Handler) postWodDone(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CondPrescriptionID int      `json:"cond_prescription_id"`
		ActualRPE          *float64 `json:"actual_rpe"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.CondPrescriptionID == 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("cond_prescription_id obrigatório"))
		return
	}
	if err := h.svc.MarkWodDone(body.CondPrescriptionID, body.ActualRPE); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- Fase B: 1RM ----------

// GET /api/1rm → lista os 1RMs do atleta (vazio se não houver nenhum).
func (h *Handler) get1RM(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.List1RMs(h.athleteID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// POST /api/1rm → { exercise_id, weight_kg } → grava ou atualiza o 1RM.
func (h *Handler) post1RM(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ExerciseID int     `json:"exercise_id"`
		WeightKg   float64 `json:"weight_kg"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.ExerciseID == 0 || body.WeightKg <= 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("exercise_id e weight_kg > 0 são obrigatórios"))
		return
	}
	if err := h.svc.Save1RM(h.athleteID(r), body.ExerciseID, body.WeightKg); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"saved": true})
}

// ---------- Fase 6D: métricas ----------

// GET /api/metrics -> métricas do atleta (nil/empty se não existirem).
func (h *Handler) getMetrics(w http.ResponseWriter, r *http.Request) {
	m, err := h.svc.GetMetrics(h.athleteID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if m == nil {
		writeJSON(w, http.StatusOK, map[string]any{})
		return
	}
	writeJSON(w, http.StatusOK, m)
}

// POST /api/metrics -> grava as métricas do atleta (UPSERT).
func (h *Handler) postMetrics(w http.ResponseWriter, r *http.Request) {
	var m domain.AthleteMetrics
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	m.AthleteID = h.athleteID(r)
	if err := h.svc.SaveMetrics(m); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"saved": true})
}

// ---------- Fase A: auth ----------

// GET /api/auth/status → { "required": bool } — informa ao cliente se JWT é obrigatório.
// Em dev (AUTH_SECRET vazia) devolve false; em prod devolve true.
func (h *Handler) getAuthStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"required": os.Getenv("AUTH_SECRET") != ""})
}

// POST /api/auth/register → { name, email, password } → { athlete, token }
func (h *Handler) postAuthRegister(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.Name == "" || body.Email == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("name, email e password são obrigatórios"))
		return
	}
	athlete, token, err := h.svc.Register(body.Name, body.Email, body.Password)
	if err != nil {
		if errors.Is(err, service.ErrEmailTaken) {
			writeError(w, http.StatusConflict, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"athlete": athlete, "token": token})
}

// POST /api/auth/login → { email, password } → { athlete, token }
func (h *Handler) postAuthLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.Email == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("email e password são obrigatórios"))
		return
	}
	athlete, token, err := h.svc.Login(body.Email, body.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"athlete": athlete, "token": token})
}

// ---------- helpers ----------

func cacheFor1h(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "public, max-age=3600")
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	// Slices nil viram null; normaliza p/ [] ser amigável ao cliente.
	if payload == nil {
		payload = []any{}
	}
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

// CORS libera o servidor de dev do React (porta diferente) a falar com a API.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Athlete-Id, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
