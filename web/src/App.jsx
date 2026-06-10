import { useEffect, useState } from 'react'
import Questionnaire from './components/Questionnaire'
import BlockView from './components/BlockView'
import WeekView from './components/WeekView'
import AuthForm from './components/AuthForm'
import ProfileView from './components/ProfileView'
import TimerControls from './components/TimerControls'
import TimerView from './components/TimerView'
import {
  fetchQuestions,
  submitAnswers,
  generateBlock,
  fetchBlock,
  evaluateBlock,
  fetchAdjustments,
  fetchEquipment,
  saveEquipment,
  fetchAthletes,
  createAthlete,
  setAthlete,
  fetchPatterns,
  savePriorities,
  saveMetrics,
  getToken,
  clearToken,
} from './api'
import AthleteBar from './components/AthleteBar'
import './App.css'

export default function App() {
  // authRequired: null = ainda verificando, false = dev mode (sem auth), true = prod (exige JWT)
  const [authRequired, setAuthRequired] = useState(null)
  const [showProfile,     setShowProfile]     = useState(false)
  const [timerPhase,      setTimerPhase]      = useState(null) // null | 'setup' | 'running'
  const [timerConfig,     setTimerConfig]     = useState(null)
  const [authed, setAuthed] = useState(() => getToken() !== null)
  const [athleteName, setAthleteName] = useState('')

  const [questions, setQuestions] = useState([])
  const [selected, setSelected] = useState({}) // { [questionId]: optionValue }
  const [equipment, setEquipment] = useState([]) // catálogo de equipamentos
  const [selectedEquipment, setSelectedEquipment] = useState([]) // ids marcados pelo atleta
  const [substitutions, setSubstitutions] = useState([]) // trocas por equipamento (último generate)
  const [overview, setOverview] = useState(null) // bloco ativo (block + weeks)
  const [activeWeek, setActiveWeek] = useState(null) // número da semana aberta
  const [adjustments, setAdjustments] = useState([]) // histórico de ajustes do motor
  const [evalMsg, setEvalMsg] = useState('') // explicação da última avaliação
  const [evaluating, setEvaluating] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [athletes, setAthletes] = useState([]) // catálogo de atletas (dev mode)
  const [currentAthlete, setCurrentAthlete] = useState(1)
  const [patterns, setPatterns] = useState([]) // catálogo de padrões de movimento (Fase 6B)
  const [selectedPriorities, setSelectedPriorities] = useState([]) // pattern ids priorizados
  const [metrics, setMetrics] = useState({ age_years: '', sex: '', body_weight_kg: '', sport: '' }) // Fase 6D

  // Consulta o servidor se auth é obrigatória. Em dev mode (AUTH_SECRET vazia),
  // pula o AuthForm e carrega o app diretamente.
  useEffect(() => {
    fetch('/api/auth/status')
      .then((r) => r.json())
      .then(({ required }) => {
        setAuthRequired(required)
        if (!required) setAuthed(true)
      })
      .catch(() => setAuthRequired(false)) // fallback: trata como dev mode
  }, [])

  // Ao confirmar autenticação: carrega catálogos e estado do atleta.
  useEffect(() => {
    if (authRequired === null) return // ainda verificando
    if (!authed) return
    setLoading(true)
    Promise.all([
      fetchQuestions().then(setQuestions),
      fetchEquipment().then(setEquipment),
      fetchAthletes().then(setAthletes),
      fetchPatterns().then(setPatterns),
      loadAthleteState(),
    ])
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false))
  }, [authed, authRequired])

  // loadAthleteState recupera o bloco ativo (+ ajustes) do atleta atual, ou cai no questionário.
  function loadAthleteState() {
    return fetchBlock()
      .then((o) => {
        setOverview(o)
        return fetchAdjustments().then(setAdjustments)
      })
      .catch(() => {
        setOverview(null) // 404 = atleta sem bloco -> questionário
        setAdjustments([])
      })
  }

  // Chamado pelo AuthForm após register/login bem-sucedidos.
  function handleAuth(athlete) {
    setAthlete(athlete.id)
    setCurrentAthlete(athlete.id)
    setAthleteName(athlete.name)
    setLoading(true)
    setAuthed(true)
  }

  function handleLogout() {
    clearToken()
    setAuthed(false)
    setAthleteName('')
    setOverview(null)
    setActiveWeek(null)
    setSelected({})
    setSelectedEquipment([])
    setSelectedPriorities([])
    setMetrics({ age_years: '', sex: '', body_weight_kg: '', sport: '' })
    setSubstitutions([])
    setAdjustments([])
    setEvalMsg('')
    setError('')
  }

  // Troca o atleta atual: passa a enviar o id dele e recarrega o estado (bloco ou questionário).
  async function handleSelectAthlete(id) {
    setAthlete(id)
    setCurrentAthlete(id)
    setActiveWeek(null)
    setSelected({})
    setSelectedEquipment([])
    setSelectedPriorities([])
    setMetrics({ age_years: '', sex: '', body_weight_kg: '', sport: '' })
    setSubstitutions([])
    setEvalMsg('')
    setError('')
    await loadAthleteState()
  }

  // Cria um atleta novo e já o seleciona (começa no questionário).
  async function handleCreateAthlete(name) {
    setError('')
    try {
      const a = await createAthlete(name)
      setAthletes((prev) => [...prev, a])
      await handleSelectAthlete(a.id)
    } catch (e) {
      setError(e.message)
    }
  }

  function handleSelect(questionId, value) {
    setSelected((prev) => ({ ...prev, [questionId]: value }))
  }

  function handleToggleEquipment(equipmentId) {
    setSelectedEquipment((prev) =>
      prev.includes(equipmentId)
        ? prev.filter((id) => id !== equipmentId)
        : [...prev, equipmentId],
    )
  }

  function handleTogglePriority(patternId) {
    setSelectedPriorities((prev) =>
      prev.includes(patternId)
        ? prev.filter((id) => id !== patternId)
        : [...prev, patternId],
    )
  }

  // Envia respostas + equipamento -> gera bloco (já filtrado) -> mostra a visão do bloco.
  async function handleSubmit() {
    setError('')
    try {
      const answers = Object.entries(selected).map(([qid, value]) => ({
        question_id: Number(qid),
        answer_value: value,
      }))
      await submitAnswers(answers)
      await saveEquipment(selectedEquipment) // Fase 3: equipamento antes de gerar
      await savePriorities(selectedPriorities) // Fase 6B: prioridades antes de gerar
      // Fase 6D: salva métricas (só os campos preenchidos; converte strings vazias para omitir)
      const metricsPayload = {}
      if (metrics.age_years) metricsPayload.age_years = Number(metrics.age_years)
      if (metrics.sex) metricsPayload.sex = metrics.sex
      if (metrics.body_weight_kg) metricsPayload.body_weight_kg = Number(metrics.body_weight_kg)
      if (metrics.sport) metricsPayload.sport = metrics.sport
      if (Object.keys(metricsPayload).length > 0) await saveMetrics(metricsPayload)
      const generated = await generateBlock()
      setOverview(generated)
      setSubstitutions(generated.substitutions || [])
      setAdjustments([]) // bloco novo: sem ajustes ainda
      setEvalMsg('')
    } catch (e) {
      setError(e.message)
    }
  }

  // Reavalia o bloco: o motor olha o realizado das últimas semanas e alivia se preciso.
  async function handleEvaluate() {
    setEvaluating(true)
    setError('')
    try {
      const result = await evaluateBlock()
      const parts = [result.explanation]
      if (result.wod_explanation) parts.push(result.wod_explanation)
      setEvalMsg(parts.join(' · '))
      const [o, adj] = await Promise.all([fetchBlock(), fetchAdjustments()])
      setOverview(o)
      setAdjustments(adj)
    } catch (e) {
      setError(e.message)
    } finally {
      setEvaluating(false)
    }
  }

  function handleRestart() {
    setOverview(null)
    setActiveWeek(null)
    setSelected({})
    setSelectedEquipment([])
    setSelectedPriorities([])
    setMetrics({ age_years: '', sex: '', body_weight_kg: '', sport: '' })
    setSubstitutions([])
    setAdjustments([])
    setEvalMsg('')
    setError('')
  }

  // Aguardando resposta do /api/auth/status (primeiro render).
  if (authRequired === null) return null

  // Auth obrigatória e sem token válido → mostra formulário de login.
  if (authRequired && !authed) {
    return (
      <main className="mx-auto max-w-2xl px-5 py-10">
        <h1 className="mb-8 text-2xl font-bold text-gray-900">cfit — seu treino</h1>
        <AuthForm onAuth={handleAuth} />
      </main>
    )
  }

  return (
    <main className="mx-auto max-w-2xl px-5 py-10">
      {timerPhase === 'setup' && (
        <TimerControls
          onStart={(cfg) => { setTimerConfig(cfg); setTimerPhase('running') }}
          onClose={() => setTimerPhase(null)}
        />
      )}
      {timerPhase === 'running' && timerConfig && (
        <TimerView
          config={timerConfig}
          onClose={() => setTimerPhase(null)}
          onFinish={() => setTimerPhase(null)}
        />
      )}

      <div className="mb-4 flex items-center justify-between">
        <h1 className="text-2xl font-bold text-gray-900">cfit — seu treino</h1>
        <div className="flex items-center gap-3 text-sm text-gray-600">
          {athleteName && <span>{athleteName}</span>}
          <button
            onClick={() => setTimerPhase('setup')}
            aria-label="Abrir timer"
            className="rounded border border-gray-300 px-3 py-1 text-sm hover:bg-gray-100"
          >
            ⏱
          </button>
          <button
            onClick={() => setShowProfile((v) => !v)}
            className="rounded border border-gray-300 px-3 py-1 text-sm hover:bg-gray-100"
          >
            Perfil
          </button>
          <button
            onClick={handleLogout}
            className="rounded border border-gray-300 px-3 py-1 text-sm hover:bg-gray-100"
          >
            Sair
          </button>
        </div>
      </div>

      {showProfile && (
        <div className="mb-6">
          <ProfileView onClose={() => setShowProfile(false)} />
        </div>
      )}

      <AthleteBar
        athletes={athletes}
        currentAthlete={currentAthlete}
        onSelect={handleSelectAthlete}
        onCreate={handleCreateAthlete}
      />

      {error && (
        <p className="mb-4 rounded-lg bg-red-50 px-3 py-2 text-red-700">⚠ {error}</p>
      )}

      {loading ? (
        <p>Carregando…</p>
      ) : activeWeek != null ? (
        <WeekView weekNumber={activeWeek} onBack={() => setActiveWeek(null)} />
      ) : overview ? (
        <BlockView
          overview={overview}
          onSelectWeek={setActiveWeek}
          onRestart={handleRestart}
          adjustments={adjustments}
          onEvaluate={handleEvaluate}
          evalMsg={evalMsg}
          evaluating={evaluating}
          substitutions={substitutions}
        />
      ) : (
        <Questionnaire
          questions={questions}
          selected={selected}
          onSelect={handleSelect}
          onSubmit={handleSubmit}
          equipment={equipment}
          selectedEquipment={selectedEquipment}
          onToggleEquipment={handleToggleEquipment}
          patterns={patterns}
          selectedPriorities={selectedPriorities}
          onTogglePriority={handleTogglePriority}
          metrics={metrics}
          onChangeMetrics={(field, value) =>
            setMetrics((prev) => ({ ...prev, [field]: value }))
          }
        />
      )}
    </main>
  )
}
