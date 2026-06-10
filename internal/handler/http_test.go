// Teste de INTEGRAÇÃO da Fase 3: sobe o stack inteiro (handler -> service -> repository ->
// SQLite real, com schema+seed reais) num httptest.Server e exercita o fluxo de equipamento
// de ponta a ponta. Prova que as camadas funcionam JUNTAS, não só isoladas.
package handler_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"treino/internal/handler"
	"treino/internal/repository"
	"treino/internal/service"
)

// newServer monta o stack real sobre um banco temporário e devolve o servidor de teste.
func newServer(t *testing.T) *httptest.Server {
	srv, _ := newServerDB(t)
	return srv
}

// newServerDB é como newServer mas também devolve o *sql.DB, p/ testes que precisam consultar o
// seed diretamente (ex.: mapear exercício -> foco e validar a seleção por fase).
func newServerDB(t *testing.T) (*httptest.Server, *sql.DB) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		t.Fatalf("abrir db: %v", err)
	}
	for _, f := range []string{"schema.sql", "seed.sql"} {
		b, err := os.ReadFile(filepath.Join("..", "..", "db", f))
		if err != nil {
			t.Fatalf("ler %s: %v", f, err)
		}
		if _, err := db.Exec(string(b)); err != nil {
			t.Fatalf("aplicar %s: %v", f, err)
		}
	}
	srv := httptest.NewServer(handler.CORS(handler.New(service.New(repository.New(db))).Routes()))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { db.Close() })
	return srv, db
}

func getJSON(t *testing.T, url string, out any) {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("GET %s -> %d: %s", url, res.StatusCode, body)
	}
	if out != nil {
		json.NewDecoder(res.Body).Decode(out)
	}
}

func postJSON(t *testing.T, url string, body any, out any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	res, err := http.Post(url, "application/json", &buf)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("POST %s -> %d: %s", url, res.StatusCode, b)
	}
	if out != nil {
		json.NewDecoder(res.Body).Decode(out)
	}
}

func TestIntegration_Fase3_EquipamentoFiltraEGeraSubstituto(t *testing.T) {
	srv := newServer(t)

	// 1. Catálogo de equipamentos chega (seed: 5).
	var catalog []map[string]any
	getJSON(t, srv.URL+"/api/equipment", &catalog)
	if len(catalog) != 13 {
		t.Fatalf("esperava 13 equipamentos (Fase 5B: +Remador/Corda/Bike), veio %d", len(catalog))
	}

	// 2. Perfil: avançado / 4 dias / força / 8 semanas.
	postJSON(t, srv.URL+"/api/answers", []map[string]any{
		{"question_id": 1, "answer_value": "gt_3y"},
		{"question_id": 2, "answer_value": "4"},
		{"question_id": 3, "answer_value": "strength"},
		{"question_id": 4, "answer_value": "8"},
	}, nil)

	// 3. Equipamento: só Barra (id 1) — sem Rack, KB, Argolas, Barra fixa.
	postJSON(t, srv.URL+"/api/equipment", []int{1}, nil)

	// 4. Gera o bloco JÁ filtrado.
	var gen struct {
		Substitutions []struct {
			Ideal, Substitute, Missing string
			Specific                   bool
		} `json:"substitutions"`
	}
	postJSON(t, srv.URL+"/api/block/generate", nil, &gen)

	// O motor relatou a substituição específica (Back Squat -> Overhead Squat por falta de Rack).
	var ok bool
	for _, s := range gen.Substitutions {
		if s.Ideal == "Back Squat" && s.Substitute == "Overhead Squat" && s.Missing == "Rack" && s.Specific {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("esperava substituição Back Squat->Overhead Squat (falta Rack); veio %+v", gen.Substitutions)
	}

	// 5. Nenhuma semana pode conter exercício impossível (Back Squat / KB Swing / Ring Row / Pull-up).
	impossible := map[string]bool{"Back Squat": true, "Kettlebell Swing": true, "Ring Row": true, "Pull-up": true}
	for week := 1; week <= 8; week++ {
		var detail struct {
			Sessions []struct {
				Prescriptions []struct {
					ExerciseName string `json:"exercise_name"`
				} `json:"prescriptions"`
			} `json:"sessions"`
		}
		getJSON(t, srv.URL+"/api/block/week/"+itoa(week), &detail)
		for _, s := range detail.Sessions {
			for _, p := range s.Prescriptions {
				if impossible[p.ExerciseName] {
					t.Errorf("semana %d prescreveu exercício impossível: %s", week, p.ExerciseName)
				}
			}
		}
	}
}

// weekDetail é o shape do JSON de /api/block/week/{n} que estes testes leem — o MESMO que a
// WeekView do front consome (week + sessions + prescriptions com previsto e realizado).
type weekDetail struct {
	Week struct {
		WeekNumber int     `json:"week_number"`
		Phase      string  `json:"phase"`
		IsDeload   bool    `json:"is_deload"`
		TargetRPE  float64 `json:"target_rpe"`
	} `json:"week"`
	Sessions []struct {
		Session struct {
			DayNumber int `json:"day_number"`
		} `json:"session"`
		Prescriptions []struct {
			ID           int      `json:"id"`
			ExerciseID   int      `json:"exercise_id"`
			ExerciseName string   `json:"exercise_name"`
			Sets         int      `json:"sets"`
			Reps         int      `json:"reps"`
			TargetRPE    float64  `json:"target_rpe"`
			Done         bool     `json:"done"`
			ActualRPE    *float64 `json:"actual_rpe"`
			Components   []struct {
				ExerciseName string `json:"exercise_name"`
				SortOrder    int    `json:"sort_order"`
				Reps         int    `json:"reps"`
			} `json:"components"`
		} `json:"prescriptions"`
		Conditioning []struct {
			ID        int     `json:"id"`
			TargetRPE float64 `json:"target_rpe"`
			Done      bool    `json:"done"`
			Wod       struct {
				WorkSec        int    `json:"work_sec"`
				EmphasisSystem string `json:"emphasis_system"`
				FormatName     string `json:"format_name"`
				Source         string `json:"source"`
				Movements      []struct {
					ExerciseName string `json:"exercise_name"`
					Reps         *int   `json:"reps"`
					SortOrder    int    `json:"sort_order"`
				} `json:"movements"`
			} `json:"wod"`
		} `json:"conditioning"`
	} `json:"sessions"`
}

// getWeek faz GET /api/block/week/{n} e devolve o detalhe (como o fetchWeek do front).
func getWeek(t *testing.T, srv *httptest.Server, n int) weekDetail {
	t.Helper()
	var d weekDetail
	getJSON(t, srv.URL+"/api/block/week/"+itoa(n), &d)
	return d
}

// logWeekFull marca TODA a semana como feita com actual_rpe = target_rpe + delta (esforço acima do
// previsto), espelhando o markDone do front em cada prescrição.
func logWeekFull(t *testing.T, srv *httptest.Server, n int, delta float64) {
	t.Helper()
	for _, s := range getWeek(t, srv, n).Sessions {
		for _, p := range s.Prescriptions {
			postJSON(t, srv.URL+"/api/session/done", map[string]any{
				"prescription_id": p.ID,
				"actual_rpe":      p.TargetRPE + delta,
				"notes":           "",
			}, nil)
		}
	}
}

func TestIntegration_Fase4_SelecaoVariaPorFase(t *testing.T) {
	srv, db := newServerDB(t)

	// Foco de cada exercício, lido do seed real — é a verdade contra a qual validamos a seleção.
	focusByID := map[int]string{}
	rows, err := db.Query("SELECT id, focus FROM exercise")
	if err != nil {
		t.Fatalf("consultar focos: %v", err)
	}
	for rows.Next() {
		var id int
		var focus string
		if err := rows.Scan(&id, &focus); err != nil {
			t.Fatalf("scan foco: %v", err)
		}
		focusByID[id] = focus
	}
	rows.Close()

	// Contrato fase -> estímulo (espelha phaseStimulus no service; o teste de integração não enxerga
	// o símbolo não-exportado, então fixa o comportamento documentado aqui).
	stimulus := map[string]string{
		"accumulation":    "technique",
		"intensification": "strength",
		"realization":     "strength",
		"deload":          "technique",
	}

	// Perfil avançado / 4 dias / força / 8 semanas. SEM filtro de equipamento (assume tudo),
	// p/ isolar a variação por fase do funil de equipamento.
	postJSON(t, srv.URL+"/api/answers", []map[string]any{
		{"question_id": 1, "answer_value": "gt_3y"},
		{"question_id": 2, "answer_value": "4"},
		{"question_id": 3, "answer_value": "strength"},
		{"question_id": 4, "answer_value": "8"},
	}, nil)
	postJSON(t, srv.URL+"/api/block/generate", nil, nil)

	// Cada semana só pode prescrever exercícios do foco do estímulo da sua fase.
	byPhaseIDs := map[string]map[int]bool{}
	for week := 1; week <= 8; week++ {
		var detail weekDetail
		getJSON(t, srv.URL+"/api/block/week/"+itoa(week), &detail)
		want, ok := stimulus[detail.Week.Phase]
		if !ok {
			t.Fatalf("semana %d com fase inesperada %q", week, detail.Week.Phase)
		}
		ids := map[int]bool{}
		for _, s := range detail.Sessions {
			for _, p := range s.Prescriptions {
				ids[p.ExerciseID] = true
				if got := focusByID[p.ExerciseID]; got != want {
					t.Errorf("semana %d (fase %s): exercício id=%d tem foco %q, esperava %q",
						week, detail.Week.Phase, p.ExerciseID, got, want)
				}
			}
		}
		if len(ids) == 0 {
			t.Errorf("semana %d sem prescrições", week)
		}
		// Acumula os IDs vistos em cada fase, p/ provar que a seleção muda de fato.
		if byPhaseIDs[detail.Week.Phase] == nil {
			byPhaseIDs[detail.Week.Phase] = map[int]bool{}
		}
		for id := range ids {
			byPhaseIDs[detail.Week.Phase][id] = true
		}
	}

	// A seleção REALMENTE difere entre estímulos diferentes (não é decorativa): nenhum exercício de
	// acumulação (technique) pode aparecer na intensificação (strength), pois os focos são disjuntos.
	for id := range byPhaseIDs["accumulation"] {
		if byPhaseIDs["intensification"][id] {
			t.Errorf("exercício id=%d apareceu em acumulação E intensificação — focos deveriam ser disjuntos", id)
		}
	}
	if len(byPhaseIDs["accumulation"]) == 0 || len(byPhaseIDs["intensification"]) == 0 {
		t.Fatalf("esperava exercícios em acumulação e intensificação; veio %v", byPhaseIDs)
	}
}

func TestIntegration_CicloCompleto_GeraMarcaReleEReavalia(t *testing.T) {
	srv := newServer(t)

	// 1. Perfil: avançado / 4 dias / força / 8 semanas (sem filtro de equipamento).
	postJSON(t, srv.URL+"/api/answers", []map[string]any{
		{"question_id": 1, "answer_value": "gt_3y"},
		{"question_id": 2, "answer_value": "4"},
		{"question_id": 3, "answer_value": "strength"},
		{"question_id": 4, "answer_value": "8"},
	}, nil)

	// 2. Gera o bloco e confere o overview (back->db->resposta com metadados certos).
	var overview struct {
		Block struct {
			TotalWeeks  int    `json:"total_weeks"`
			DaysPerWeek int    `json:"days_per_week"`
			Goal        string `json:"goal"`
		} `json:"block"`
		Weeks []struct {
			WeekNumber int  `json:"week_number"`
			IsDeload   bool `json:"is_deload"`
		} `json:"weeks"`
	}
	postJSON(t, srv.URL+"/api/block/generate", nil, &overview)
	if overview.Block.TotalWeeks != 8 || overview.Block.DaysPerWeek != 4 || overview.Block.Goal != "strength" {
		t.Fatalf("overview com metadados errados: %+v", overview.Block)
	}
	if len(overview.Weeks) != 8 {
		t.Fatalf("esperava 8 semanas no overview, veio %d", len(overview.Weeks))
	}
	if !overview.Weeks[7].IsDeload {
		t.Errorf("última semana deveria ser deload: %+v", overview.Weeks[7])
	}

	// 3. Cada semana devolve treinos BEM FORMADOS: nome resolvido, sets/reps/RPE preenchidos,
	//    e NADA marcado antes de o atleta registrar.
	for week := 1; week <= 8; week++ {
		d := getWeek(t, srv, week)
		if len(d.Sessions) != 4 {
			t.Errorf("semana %d: esperava 4 dias, veio %d", week, len(d.Sessions))
		}
		for _, s := range d.Sessions {
			if len(s.Prescriptions) == 0 {
				t.Errorf("semana %d dia %d sem prescrições", week, s.Session.DayNumber)
			}
			for _, p := range s.Prescriptions {
				if p.ExerciseName == "" || p.Sets <= 0 || p.Reps <= 0 || p.TargetRPE <= 0 {
					t.Errorf("semana %d: prescrição mal formada: %+v", week, p)
				}
				if p.Done || p.ActualRPE != nil {
					t.Errorf("semana %d: prescrição já marcada antes de registrar: %+v", week, p)
				}
			}
		}
	}

	// 4. "Marcados ao final": registra as semanas 1 e 2 inteiras com RPE acima do alvo.
	logWeekFull(t, srv, 1, 1.5)
	logWeekFull(t, srv, 2, 1.5)

	// 5. A RELEITURA da semana 1 (o que o front faz após o POST de markDone) devolve o realizado
	//    persistido: done=true e actual_rpe = target+1.5. Prova o ida-e-volta assíncrono back<->db.
	for _, s := range getWeek(t, srv, 1).Sessions {
		for _, p := range s.Prescriptions {
			if !p.Done {
				t.Errorf("semana 1: prescrição %d deveria estar feita após marcar", p.ID)
			}
			if p.ActualRPE == nil {
				t.Errorf("semana 1: prescrição %d sem actual_rpe após marcar", p.ID)
				continue
			}
			if want := p.TargetRPE + 1.5; absf(*p.ActualRPE-want) > 0.001 {
				t.Errorf("semana 1: prescrição %d actual_rpe=%.2f, esperava %.2f", p.ID, *p.ActualRPE, want)
			}
		}
	}

	// 6. Volume original da semana 3 (alvo do alívio) ANTES de reavaliar.
	origSets := getWeek(t, srv, 3).Sessions[0].Prescriptions[0].Sets

	// 7. Reavalia: 2 semanas consecutivas com esforço acima do alvo => alivia a próxima (semana 3).
	var eval struct {
		Action     string `json:"action"`
		Adjustment *struct {
			Action string `json:"action"`
		} `json:"adjustment"`
	}
	postJSON(t, srv.URL+"/api/block/evaluate", nil, &eval)
	if eval.Action != "reduce_volume" {
		t.Fatalf("esperava ação reduce_volume após 2 semanas em sinal, veio %q", eval.Action)
	}

	// 8. A semana 3 RELIDA reflete o alívio: menos séries que o original (o dado que o front lê
	//    já vem ajustado, sem o cliente recalcular nada).
	for _, s := range getWeek(t, srv, 3).Sessions {
		for _, p := range s.Prescriptions {
			if p.Sets >= origSets {
				t.Errorf("semana 3: volume não reduziu (antes %d, agora %d)", origSets, p.Sets)
			}
		}
	}

	// 9. Histórico de ajustes não vazio (transparência) — outra rota que o front consome.
	var adjustments []map[string]any
	getJSON(t, srv.URL+"/api/block/adjustments", &adjustments)
	if len(adjustments) == 0 {
		t.Errorf("esperava ao menos 1 ajuste no histórico, veio vazio")
	}
}

func TestIntegration_Fase5A_ConjugadoVemComComponentes(t *testing.T) {
	srv := newServer(t)

	// Perfil avançado/força/8 semanas, sem filtro de equipamento (conjugados ficam viáveis).
	postJSON(t, srv.URL+"/api/answers", []map[string]any{
		{"question_id": 1, "answer_value": "gt_3y"},
		{"question_id": 2, "answer_value": "4"},
		{"question_id": 3, "answer_value": "strength"},
		{"question_id": 4, "answer_value": "8"},
	}, nil)
	postJSON(t, srv.URL+"/api/block/generate", nil, nil)

	// Varre o bloco inteiro. Toda prescrição que seja conjugado tem de vir com a sequência coerente;
	// e ao menos um conjugado precisa aparecer (a geração é determinística, então isto é estável).
	complexes := 0
	for week := 1; week <= 8; week++ {
		for _, s := range getWeek(t, srv, week).Sessions {
			for _, p := range s.Prescriptions {
				if len(p.Components) == 0 {
					continue // exercício simples
				}
				complexes++
				if len(p.Components) < 2 {
					t.Errorf("conjugado %q deveria ter ≥2 componentes, veio %d", p.ExerciseName, len(p.Components))
				}
				for i, c := range p.Components {
					if c.ExerciseName == "" || c.Reps <= 0 {
						t.Errorf("componente mal formado em %q: %+v", p.ExerciseName, c)
					}
					if c.SortOrder != i+1 {
						t.Errorf("componente fora de ordem em %q: sort_order=%d, esperava %d", p.ExerciseName, c.SortOrder, i+1)
					}
				}
			}
		}
	}
	if complexes == 0 {
		t.Fatal("esperava ao menos um conjugado prescrito no bloco avançado/força")
	}
	t.Logf("conjugados prescritos no bloco: %d", complexes)
}

func TestIntegration_Fase6A_AtletasRecebemProgramasDistintos(t *testing.T) {
	srv, db := newServerDB(t)

	// Foco de cada exercício (do seed) p/ validar que ambos os atletas seguem phase-correct.
	focusByID := map[int]string{}
	rows, err := db.Query("SELECT id, focus FROM exercise")
	if err != nil {
		t.Fatalf("focos: %v", err)
	}
	for rows.Next() {
		var id int
		var f string
		rows.Scan(&id, &f)
		focusByID[id] = f
	}
	rows.Close()

	// Cria o atleta 2 (o 1 já vem do seed).
	var a2 struct {
		ID int `json:"id"`
	}
	postJSON(t, srv.URL+"/api/athletes", map[string]any{"name": "Atleta 2"}, &a2)
	if a2.ID < 2 {
		t.Fatalf("esperava id de atleta >= 2, veio %d", a2.ID)
	}

	// MESMO perfil para os dois (avançado / 4 dias / força / 8 semanas), via ?athlete=.
	profile := []map[string]any{
		{"question_id": 1, "answer_value": "gt_3y"},
		{"question_id": 2, "answer_value": "4"},
		{"question_id": 3, "answer_value": "strength"},
		{"question_id": 4, "answer_value": "8"},
	}
	for _, a := range []string{"1", itoa(a2.ID)} {
		postJSON(t, srv.URL+"/api/answers?athlete="+a, profile, nil)
		postJSON(t, srv.URL+"/api/block/generate?athlete="+a, nil, nil)
	}

	// Coleta a semana 1 (ordenada) de cada atleta.
	weekIDs := func(athlete string) []int {
		var d weekDetail
		getJSON(t, srv.URL+"/api/block/week/1?athlete="+athlete, &d)
		var ids []int
		for _, s := range d.Sessions {
			for _, p := range s.Prescriptions {
				ids = append(ids, p.ExerciseID)
			}
		}
		return ids
	}
	w1 := weekIDs("1")
	w2 := weekIDs(itoa(a2.ID))
	if len(w1) == 0 || len(w2) == 0 {
		t.Fatal("semana 1 vazia para algum atleta")
	}

	// Programas DISTINTOS: a semana 1 difere entre os atletas (semente por identidade).
	same := len(w1) == len(w2)
	for i := range w1 {
		if i < len(w2) && w1[i] != w2[i] {
			same = false
		}
	}
	if same {
		t.Errorf("atletas 1 e %d receberam a MESMA semana 1 (%v); deveriam diferir", a2.ID, w1)
	}

	// Ambos PHASE-CORRECT: semana 1 é acumulação -> technique (a individualização não quebra a fase).
	for _, id := range append(append([]int{}, w1...), w2...) {
		if focusByID[id] != "technique" {
			t.Errorf("semana 1 deveria ser technique; exercício %d é %q", id, focusByID[id])
		}
	}
}

func TestIntegration_Fase5B_WodCompostoNaSemana(t *testing.T) {
	srv, db := newServerDB(t)

	// Mapa work_sec->sistema e dose por fase, lidos do banco (a verdade contra a qual validamos).
	type band struct {
		max int
		sys string
	}
	var bands []band
	rows, err := db.Query("SELECT max_work_sec, system FROM energy_system_map ORDER BY sort_order")
	if err != nil {
		t.Fatalf("mapa: %v", err)
	}
	for rows.Next() {
		var b band
		rows.Scan(&b.max, &b.sys)
		bands = append(bands, b)
	}
	rows.Close()
	bandSys := func(ws int) string {
		for _, b := range bands {
			if ws <= b.max {
				return b.sys
			}
		}
		return ""
	}
	phaseSys := map[string]string{}
	rows2, _ := db.Query("SELECT phase, emphasis_system FROM phase_conditioning WHERE goal='strength'")
	for rows2.Next() {
		var p, s string
		rows2.Scan(&p, &s)
		phaseSys[p] = s
	}
	rows2.Close()

	// Perfil avançado/força/8 semanas; gera no stack real.
	postJSON(t, srv.URL+"/api/answers", []map[string]any{
		{"question_id": 1, "answer_value": "gt_3y"},
		{"question_id": 2, "answer_value": "4"},
		{"question_id": 3, "answer_value": "strength"},
		{"question_id": 4, "answer_value": "8"},
	}, nil)
	postJSON(t, srv.URL+"/api/block/generate", nil, nil)

	wodCount := 0
	for week := 1; week <= 8; week++ {
		d := getWeek(t, srv, week)
		want := phaseSys[d.Week.Phase]
		for _, s := range d.Sessions {
			for _, c := range s.Conditioning {
				wodCount++
				if c.Wod.Source != "generated" {
					t.Errorf("semana %d: WOD deveria ser generated, veio %q", week, c.Wod.Source)
				}
				// A ênfase do WOD = a do sistema da fase, E o work_sec mapeia a esse sistema.
				if c.Wod.EmphasisSystem != want {
					t.Errorf("semana %d (%s): ênfase %q, esperava %q", week, d.Week.Phase, c.Wod.EmphasisSystem, want)
				}
				if got := bandSys(c.Wod.WorkSec); got != want {
					t.Errorf("semana %d: work_sec %d mapeia %q, esperava %q", week, c.Wod.WorkSec, got, want)
				}
				if c.Wod.FormatName == "" {
					t.Errorf("semana %d: formato do WOD não resolvido", week)
				}
				if len(c.Wod.Movements) == 0 {
					t.Errorf("semana %d: WOD sem movimentos", week)
				}
				for _, m := range c.Wod.Movements {
					if m.ExerciseName == "" || m.Reps == nil || *m.Reps <= 0 {
						t.Errorf("semana %d: movimento mal dosado: %+v", week, m)
					}
				}
			}
		}
	}
	if wodCount == 0 {
		t.Fatal("esperava WODs compostos no bloco (substrato está no seed)")
	}

	// GET /api/wods traz os 12 benchmark + os WODs gerados agora.
	var wods []map[string]any
	getJSON(t, srv.URL+"/api/wods", &wods)
	if len(wods) <= 12 {
		t.Errorf("esperava 12 benchmark + gerados, veio %d", len(wods))
	}
}

func TestIntegration_Fase6C_NaoRepeteBlocoAnterior(t *testing.T) {
	srv := newServer(t)

	// Perfil avançado/força/8 semanas.
	postJSON(t, srv.URL+"/api/answers", []map[string]any{
		{"question_id": 1, "answer_value": "gt_3y"},
		{"question_id": 2, "answer_value": "4"},
		{"question_id": 3, "answer_value": "strength"},
		{"question_id": 4, "answer_value": "8"},
	}, nil)

	// Assinatura do bloco ativo: ids de exercício por sessão, todas as semanas.
	blockSig := func() string {
		sig := ""
		for week := 1; week <= 8; week++ {
			d := getWeek(t, srv, week)
			for _, s := range d.Sessions {
				for _, p := range s.Prescriptions {
					sig += itoa(p.ExerciseID) + ","
				}
				sig += "|"
			}
		}
		return sig
	}

	// Gera o 1º bloco, depois o 2º (mesmo atleta) — o 2º NÃO pode ser idêntico ao 1º.
	postJSON(t, srv.URL+"/api/block/generate", nil, nil)
	b1 := blockSig()
	postJSON(t, srv.URL+"/api/block/generate", nil, nil)
	b2 := blockSig()
	if b1 == b2 {
		t.Errorf("o 2º bloco do atleta deveria diferir do 1º (não-repetição por histórico)")
	}
}

func TestIntegration_Fase6B_PrioridadeAumentaPadrao(t *testing.T) {
	srv, db := newServerDB(t)

	// exercise_id -> padrão de movimento, e o id do padrão 'pull'.
	patternByEx := map[int]string{}
	rows, err := db.Query(`SELECT e.id, mp.name FROM exercise e JOIN movement_pattern mp ON mp.id = e.movement_pattern_id`)
	if err != nil {
		t.Fatalf("padrões: %v", err)
	}
	for rows.Next() {
		var id int
		var p string
		rows.Scan(&id, &p)
		patternByEx[id] = p
	}
	rows.Close()
	var pullID int
	if err := db.QueryRow(`SELECT id FROM movement_pattern WHERE name='pull'`).Scan(&pullID); err != nil {
		t.Fatalf("pull id: %v", err)
	}

	// Atleta 2 (o 1 já existe). Mesmo perfil para os dois; só o 2 prioriza 'pull'.
	var a2 struct {
		ID int `json:"id"`
	}
	postJSON(t, srv.URL+"/api/athletes", map[string]any{"name": "Atleta 2"}, &a2)
	profile := []map[string]any{
		{"question_id": 1, "answer_value": "gt_3y"},
		{"question_id": 2, "answer_value": "4"},
		{"question_id": 3, "answer_value": "strength"},
		{"question_id": 4, "answer_value": "8"},
	}
	for _, a := range []string{"1", itoa(a2.ID)} {
		postJSON(t, srv.URL+"/api/answers?athlete="+a, profile, nil)
	}
	postJSON(t, srv.URL+"/api/priorities?athlete="+itoa(a2.ID), []int{pullID}, nil)
	for _, a := range []string{"1", itoa(a2.ID)} {
		postJSON(t, srv.URL+"/api/block/generate?athlete="+a, nil, nil)
	}

	countPull := func(athlete string) int {
		n := 0
		for week := 1; week <= 8; week++ {
			var d weekDetail
			getJSON(t, srv.URL+"/api/block/week/"+itoa(week)+"?athlete="+athlete, &d)
			for _, s := range d.Sessions {
				for _, p := range s.Prescriptions {
					if patternByEx[p.ExerciseID] == "pull" {
						n++
					}
				}
			}
		}
		return n
	}
	base := countPull("1")
	withPriority := countPull(itoa(a2.ID))
	if withPriority <= base {
		t.Errorf("priorizar pull deveria aumentar a frequência: %d (prioridade) vs %d (sem)", withPriority, base)
	}
}

// absf é |x| para floats (sem importar math por uma linha só).
func absf(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func TestIntegration_WodAutoreg_MarkDoneEAvaliar(t *testing.T) {
	srv, _ := newServerDB(t)

	// Setup: perfil + gerar bloco.
	profile := []map[string]any{
		{"question_id": 1, "answer_value": "gt_3y"},
		{"question_id": 2, "answer_value": "4"},
		{"question_id": 3, "answer_value": "strength"},
		{"question_id": 4, "answer_value": "8"},
	}
	postJSON(t, srv.URL+"/api/answers?athlete=1", profile, nil)
	postJSON(t, srv.URL+"/api/block/generate?athlete=1", nil, nil)

	// Busca WODs da semana 1.
	var w1 weekDetail
	getJSON(t, srv.URL+"/api/block/week/1?athlete=1", &w1)

	var condID int
	for _, s := range w1.Sessions {
		for _, c := range s.Conditioning {
			if c.ID > 0 {
				condID = c.ID
				break
			}
		}
		if condID > 0 {
			break
		}
	}
	if condID == 0 {
		t.Skip("nenhum WOD na semana 1 (bloco sem condicionamento)")
	}

	// Marca WOD feito com RPE 9 (alto).
	rpe := 9.0
	postJSON(t, srv.URL+"/api/wod/done", map[string]any{
		"cond_prescription_id": condID,
		"actual_rpe":           rpe,
	}, nil)

	// Verifica que o WOD foi marcado como feito na releitura.
	var w1after weekDetail
	getJSON(t, srv.URL+"/api/block/week/1?athlete=1", &w1after)
	found := false
	for _, s := range w1after.Sessions {
		for _, c := range s.Conditioning {
			if c.ID == condID && c.Done {
				found = true
			}
		}
	}
	if !found {
		t.Error("WOD deveria estar marcado como done após markWodDone")
	}

	// --- Parte 2: marca TODOS os WODs da semana 1 com RPE individual (trilho WOD independente) ---
	for _, s := range w1after.Sessions {
		for _, c := range s.Conditioning {
			if c.Done {
				continue
			}
			// RPE individual: 8.5, 9.5, 7.5 — valores distintos e plausíveis.
			rpe := 8.5 + float64(c.ID%3)*0.5
			postJSON(t, srv.URL+"/api/wod/done", map[string]any{
				"cond_prescription_id": c.ID,
				"actual_rpe":           rpe,
			}, nil)
		}
	}

	// --- Parte 3: avalia — o motor roda os DOIS trilhos (força + WOD) ---
	var evalResult struct {
		Action         string `json:"action"`
		Explanation    string `json:"explanation"`
		WodAction      string `json:"wod_action"`
		WodExplanation string `json:"wod_explanation"`
	}
	postJSON(t, srv.URL+"/api/block/evaluate?athlete=1", nil, &evalResult)

	if evalResult.WodAction == "" {
		t.Error("evaluate deveria ter preenchido wod_action (trilho WOD não rodou)")
	}
	if evalResult.WodExplanation == "" {
		t.Error("evaluate deveria ter preenchido wod_explanation")
	}
}

// ---------- Fase B: testes de 1RM ----------

func TestIntegration_1RM_SaveAndList(t *testing.T) {
	srv, db := newServerDB(t)

	// Pega o id de um exercício existente no seed.
	var exID int
	if err := db.QueryRow(`SELECT id FROM exercise LIMIT 1`).Scan(&exID); err != nil {
		t.Fatalf("buscar exercise: %v", err)
	}

	// Lista vazia antes de salvar.
	var list []map[string]any
	getJSON(t, srv.URL+"/api/1rm", &list)
	if len(list) != 0 {
		t.Fatalf("esperava lista vazia, veio %d", len(list))
	}

	// Salva o 1RM.
	postJSON(t, srv.URL+"/api/1rm", map[string]any{
		"exercise_id": exID,
		"weight_kg":   100.0,
	}, nil)

	// Lista agora tem 1 item.
	getJSON(t, srv.URL+"/api/1rm", &list)
	if len(list) != 1 {
		t.Fatalf("esperava 1 1RM, veio %d", len(list))
	}
	if list[0]["weight_kg"] != 100.0 {
		t.Errorf("weight_kg: esperava 100, veio %v", list[0]["weight_kg"])
	}
	if list[0]["exercise_name"] == "" {
		t.Error("exercise_name vazio — JOIN não funcionou")
	}

	// Upsert: atualiza para 120kg.
	postJSON(t, srv.URL+"/api/1rm", map[string]any{
		"exercise_id": exID,
		"weight_kg":   120.0,
	}, nil)
	getJSON(t, srv.URL+"/api/1rm", &list)
	if len(list) != 1 {
		t.Fatalf("esperava 1 1RM após upsert, veio %d", len(list))
	}
	if list[0]["weight_kg"] != 120.0 {
		t.Errorf("upsert: esperava 120, veio %v", list[0]["weight_kg"])
	}
}

// ---------- Fase A: testes de auth ----------

// postJSONStatus é como postJSON mas devolve o status HTTP (não falha em !200).
func postJSONStatus(t *testing.T, url string, body any, out any) int {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	res, err := http.Post(url, "application/json", &buf)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer res.Body.Close()
	if out != nil {
		json.NewDecoder(res.Body).Decode(out)
	}
	return res.StatusCode
}

// getWithToken faz GET autenticado e devolve o status HTTP.
func getWithToken(t *testing.T, url, token string) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	res.Body.Close()
	return res.StatusCode
}

// postWithToken faz POST autenticado e devolve o status HTTP.
func postWithToken(t *testing.T, url, token string, body any, out any) int {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(http.MethodPost, url, &buf)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer res.Body.Close()
	if out != nil {
		json.NewDecoder(res.Body).Decode(out)
	}
	return res.StatusCode
}

// TestIntegration_Auth_RegisterLogin exercita o fluxo happy-path:
// registrar → logar → usar token em rota protegida → gerar bloco.
// Cada função de auth usa um servidor fresh (rate limiter limpo).
func TestIntegration_Auth_RegisterLogin(t *testing.T) {
	srv := newServer(t)

	// 1. Registrar: devolve atleta + token (201).
	var regResp struct {
		Athlete struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"athlete"`
		Token string `json:"token"`
	}
	st := postJSONStatus(t, srv.URL+"/api/auth/register", map[string]any{
		"name":     "Auth Test",
		"email":    "auth@cfit.io",
		"password": "senha123",
	}, &regResp)
	if st != http.StatusCreated {
		t.Fatalf("register: esperava 201, veio %d", st)
	}
	if regResp.Token == "" {
		t.Fatal("register: token vazio")
	}
	if regResp.Athlete.ID == 0 {
		t.Fatal("register: athlete.id = 0")
	}

	// 2. Login com as credenciais recém-criadas: devolve atleta + novo token (200).
	var loginResp struct {
		Athlete struct {
			ID int `json:"id"`
		} `json:"athlete"`
		Token string `json:"token"`
	}
	st = postJSONStatus(t, srv.URL+"/api/auth/login", map[string]any{
		"email":    "auth@cfit.io",
		"password": "senha123",
	}, &loginResp)
	if st != http.StatusOK {
		t.Fatalf("login: esperava 200, veio %d", st)
	}
	if loginResp.Token == "" {
		t.Fatal("login: token vazio")
	}
	if loginResp.Athlete.ID != regResp.Athlete.ID {
		t.Errorf("login: athlete_id diferente do registrado (%d vs %d)",
			loginResp.Athlete.ID, regResp.Athlete.ID)
	}

	// 3. Token válido: rota protegida responde 200.
	st = getWithToken(t, srv.URL+"/api/questions", loginResp.Token)
	if st != http.StatusOK {
		t.Errorf("rota protegida com token válido: esperava 200, veio %d", st)
	}

	// 4. Token inválido: rota protegida rejeita com 401 (token malformado é sempre rejeitado).
	st = getWithToken(t, srv.URL+"/api/questions", "token.invalido.mesmo")
	if st != http.StatusUnauthorized {
		t.Errorf("token inválido: esperava 401, veio %d", st)
	}

	// 5. Token válido: gerar bloco completo para o atleta recém-registrado.
	st = postWithToken(t, srv.URL+"/api/answers", loginResp.Token, []map[string]any{
		{"question_id": 1, "answer_value": "gt_3y"},
		{"question_id": 2, "answer_value": "4"},
		{"question_id": 3, "answer_value": "strength"},
		{"question_id": 4, "answer_value": "8"},
	}, nil)
	if st != http.StatusOK {
		t.Errorf("answers com token válido: esperava 200, veio %d", st)
	}
	var blockOverview map[string]any
	st = postWithToken(t, srv.URL+"/api/block/generate", loginResp.Token, nil, &blockOverview)
	if st != http.StatusOK {
		t.Errorf("block/generate com token válido: esperava 200, veio %d", st)
	}
	if blockOverview["block"] == nil {
		t.Error("block/generate: campo 'block' ausente na resposta")
	}
}

// TestIntegration_Auth_BadCredentials confirma rejeição de senha errada (401)
// e de e-mail duplicado (409), em servidor fresh (sem interferência do rate limiter).
func TestIntegration_Auth_BadCredentials(t *testing.T) {
	srv := newServer(t)

	// Registra o atleta para ter credenciais válidas no banco.
	postJSONStatus(t, srv.URL+"/api/auth/register", map[string]any{
		"name":     "Creds Test",
		"email":    "creds@cfit.io",
		"password": "correta123",
	}, nil)

	// Senha errada → 401.
	st := postJSONStatus(t, srv.URL+"/api/auth/login", map[string]any{
		"email":    "creds@cfit.io",
		"password": "errada",
	}, nil)
	if st != http.StatusUnauthorized {
		t.Errorf("senha errada: esperava 401, veio %d", st)
	}
}

// TestIntegration_Auth_EmailDuplicado confirma 409 ao tentar registrar
// o mesmo e-mail duas vezes.
func TestIntegration_Auth_EmailDuplicado(t *testing.T) {
	srv := newServer(t)

	postJSONStatus(t, srv.URL+"/api/auth/register", map[string]any{
		"name": "Primeiro", "email": "dup@cfit.io", "password": "abc123",
	}, nil)

	st := postJSONStatus(t, srv.URL+"/api/auth/register", map[string]any{
		"name": "Segundo", "email": "dup@cfit.io", "password": "xyz789",
	}, nil)
	if st != http.StatusConflict {
		t.Errorf("email duplicado: esperava 409, veio %d", st)
	}
}

// TestIntegration_Auth_ProdMode confirma que sem token o servidor retorna 401
// quando AUTH_SECRET está definida (modo produção).
func TestIntegration_Auth_ProdMode(t *testing.T) {
	t.Setenv("AUTH_SECRET", "chave-de-teste-prod")
	srv := newServer(t)

	// Rota protegida sem token → 401.
	res, err := http.Get(srv.URL + "/api/questions")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Errorf("prod mode sem token: esperava 401, veio %d", res.StatusCode)
	}

	// Rotas públicas (auth) não precisam de token.
	st := postJSONStatus(t, srv.URL+"/api/auth/register", map[string]any{
		"name":     "Prod Test",
		"email":    "prod@cfit.io",
		"password": "senha123",
	}, nil)
	// O endpoint deve aceitar a requisição (2xx ou 5xx, não 401).
	if st == http.StatusUnauthorized {
		t.Errorf("register é rota pública: não deveria retornar 401, veio %d", st)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
