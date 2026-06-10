// WeekView: mostra os dias da semana com suas prescrições (sets×reps @ RPE) e
// permite marcar cada uma como feita, com RPE real e nota opcionais.

import { useEffect, useRef, useState } from 'react'
import { fetchWeek, markDone, markWodDone } from '../api'
import TimerControls from './TimerControls'
import TimerView from './TimerView'

const PHASE_LABEL = {
  accumulation: 'Acumulação',
  intensification: 'Intensificação',
  realization: 'Realização',
  deload: 'Deload',
}

// PHASE_STIMULUS espelha o mapa phaseStimulus do service (Fase 4): é o FOCO que cada fase
// enfatiza e que faz a seleção de exercícios variar de uma fase para outra. Rótulo de
// apresentação (placeholder fundamentado, calibrável) — não verdade cravada.
const PHASE_STIMULUS = {
  accumulation: 'ênfase em técnica e base',
  intensification: 'ênfase em força',
  realization: 'ênfase em força',
  deload: 'técnica leve para recuperar',
}

// Rótulo honesto dos sistemas energéticos (Fase 5B): o WOD "enfatiza", nunca "isola".
const SYSTEM_LABEL = {
  phosphagen: 'fosfagênico (potência)',
  glycolytic: 'glicolítico (lático)',
  oxidative: 'oxidativo (aeróbio)',
  mixed: 'misto',
}

export default function WeekView({ weekNumber, onBack }) {
  const [week, setWeek] = useState(null)
  const [error, setError] = useState('')

  // (Re)carrega a semana — usado no mount e após marcar algo como feito.
  function load() {
    fetchWeek(weekNumber)
      .then(setWeek)
      .catch((e) => setError(e.message))
  }

  useEffect(load, [weekNumber])

  if (error) return <p className="text-red-600">⚠ {error}</p>
  if (!week) return <p>Carregando…</p>

  return (
    <div>
      <button
        type="button"
        onClick={onBack}
        className="mb-4 text-sm text-blue-600 hover:underline"
      >
        ← Voltar ao bloco
      </button>

      <h2 className="mb-1 text-xl font-semibold">Semana {week.week.week_number}</h2>
      <p className="mb-1 text-sm text-gray-500">
        {PHASE_LABEL[week.week.phase] || week.week.phase} · RPE-alvo{' '}
        {week.week.target_rpe.toFixed(1)}
      </p>
      {PHASE_STIMULUS[week.week.phase] && (
        <p className="mb-5 text-xs text-gray-400">
          Os exercícios desta semana seguem a {PHASE_STIMULUS[week.week.phase]} — por isso variam
          conforme a fase do bloco.
        </p>
      )}

      <div className="space-y-4">
        {week.sessions.map((s) => (
          <div
            key={s.session.id}
            className="rounded-lg border border-gray-200 bg-white p-4"
          >
            <h3 className="mb-3 font-medium text-gray-700">
              Dia {s.session.day_number}
            </h3>
            <ul className="space-y-3">
              {s.prescriptions.map((p) => (
                <PrescriptionRow key={p.id} p={p} onDone={load} />
              ))}
            </ul>

            {(s.conditioning || []).map((c, i) => (
              <WodCard key={i} c={c} onDone={load} />
            ))}
          </div>
        ))}
      </div>
    </div>
  )
}

// Mapeia o format_name do WOD para o modo do timer.
function wodToTimerInitial(w) {
  const f    = (w.format_name || '').toLowerCase()
  const mode = f.includes('amrap')                          ? 'amrap'
             : f.includes('emom')                           ? 'emom'
             : f.includes('fortime') || f.includes('chipper') ? 'fortime'
             : f.includes('interval') || f.includes('tabata') ? 'tabata'
             : 'amrap'
  return { mode, minutes: Math.max(1, Math.round(w.work_sec / 60)), rounds: w.rounds || 8 }
}

// WodCard: o WOD (condicionamento) montado pelo compositor (Fase 5B). Mostra formato, duração,
// "enfatiza o sistema X" (honesto) e os movimentos. Permite marcar como feito + RPE (AutoReg WOD).
function WodCard({ c, onDone }) {
  const w   = c.wod
  const dur = w.work_sec >= 120 ? `${Math.round(w.work_sec / 60)}min` : `${w.work_sec}s`

  const [rpe,         setRpe]         = useState('')
  const [saving,      setSaving]      = useState(false)
  const [done,        setDone]        = useState(c.done || false)
  const [timerPhase,  setTimerPhase]  = useState(null) // null | 'setup' | 'running'
  const [timerConfig, setTimerConfig] = useState(null)

  const rpeId      = `wod-rpe-${c.id}`
  const rpeInputRef = useRef(null)

  async function handleWodDone() {
    setSaving(true)
    try {
      await markWodDone(c.id, rpe !== '' ? Number(rpe) : null)
      setDone(true)
      if (onDone) onDone()
    } finally {
      setSaving(false)
    }
  }

  function handleTimerDone() {
    setTimerPhase(null)
    // Auto-foca o input de RPE: o atleta acabou o WOD, hora de reportar.
    setTimeout(() => rpeInputRef.current?.focus(), 120)
  }

  return (
    <>
      {timerPhase === 'setup' && (
        <TimerControls
          initial={wodToTimerInitial(w)}
          onStart={(cfg) => { setTimerConfig(cfg); setTimerPhase('running') }}
          onClose={() => setTimerPhase(null)}
        />
      )}
      {timerPhase === 'running' && timerConfig && (
        <TimerView
          config={timerConfig}
          onClose={handleTimerDone}
          onFinish={handleTimerDone}
        />
      )}

      <div
        data-testid="wod-card"
        className={`mt-3 rounded-lg border p-3 ${done ? 'border-green-200 bg-green-50' : 'border-orange-200 bg-orange-50'}`}
      >
        <div className="flex flex-wrap items-baseline gap-2">
          <span
            className={`rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase ${done ? 'bg-green-200 text-green-800' : 'bg-orange-200 text-orange-800'}`}
          >
            {done ? 'WOD ✓' : 'WOD'}
          </span>
          <span className="font-medium text-gray-800">{w.format_name}</span>
          <span className="text-sm text-gray-500">
            {w.rounds > 1 ? `${w.rounds} rounds · ` : ''}
            {dur} · RPE alvo {c.target_rpe.toFixed(1)}
          </span>
          {!done && (
            <button
              onClick={() => setTimerPhase('setup')}
              aria-label="Abrir timer para este WOD"
              className="ml-auto rounded border border-orange-300 px-2 py-0.5 text-xs font-medium text-orange-700 hover:bg-orange-100 transition-colors"
            >
              ⏱ Timer
            </button>
          )}
        </div>
        <p className="mt-1 text-xs text-gray-500">
          Enfatiza o sistema {SYSTEM_LABEL[w.emphasis_system] || w.emphasis_system} — não treina
          exclusivamente.
        </p>
        <ul className="mt-2 space-y-0.5 text-sm text-gray-700">
          {(w.movements || []).map((m, i) => (
            <li key={i}>
              {m.reps != null ? `${m.reps} ` : ''}
              {m.exercise_name}
            </li>
          ))}
        </ul>
        {!done && (
          <div className="mt-3 flex flex-wrap items-center gap-2">
            <label htmlFor={rpeId} className="text-sm text-gray-600">
              RPE real (1–10, opcional):
            </label>
            <input
              ref={rpeInputRef}
              id={rpeId}
              type="number"
              min="1"
              max="10"
              step="0.5"
              placeholder="—"
              value={rpe}
              onChange={(e) => setRpe(e.target.value)}
              style={{ width: '4rem', padding: '0.2rem 0.4rem', border: '1px solid #ccc', borderRadius: '4px', fontSize: '0.875rem' }}
            />
            <button
              onClick={handleWodDone}
              disabled={saving}
              style={{ fontSize: '0.8rem', padding: '0.2rem 0.7rem' }}
            >
              {saving ? 'Salvando…' : 'Marcar feito'}
            </button>
          </div>
        )}
      </div>
    </>
  )
}

function PrescriptionRow({ p, onDone }) {
  const [rpe, setRpe] = useState('')
  const [notes, setNotes] = useState('')
  const [saving, setSaving] = useState(false)

  // RPE real é OBRIGATÓRIO: é o combustível da autorregulação (Fase 2). Sem ele,
  // o motor não tem como comparar previsto vs realizado — então não deixamos marcar feito.
  const rpeValido = rpe !== '' && Number(rpe) >= 1 && Number(rpe) <= 10

  async function handleDone() {
    setSaving(true)
    try {
      await markDone(p.id, Number(rpe), notes)
      onDone()
    } finally {
      setSaving(false)
    }
  }

  // FASE 5A: um conjugado vem com a sequência de componentes; mostramos "N séries de: A ×r + B ×r..."
  // em vez do "sets×reps" simples, para o atleta entender que é uma unidade encadeada.
  const isComplex = Array.isArray(p.components) && p.components.length > 0

  return (
    <li className="flex flex-wrap items-center gap-3">
      <span className="min-w-40 font-medium text-gray-800">
        {p.exercise_name}
        {isComplex && (
          <span className="ml-1 align-middle rounded bg-purple-100 px-1.5 py-0.5 text-[10px] font-semibold text-purple-700">
            conjugado
          </span>
        )}
      </span>
      {isComplex ? (
        <span className="text-sm text-gray-500">
          {p.sets} {p.sets > 1 ? 'séries' : 'série'} de:{' '}
          {p.components.map((c, i) => (
            <span key={i}>
              {i > 0 ? ' + ' : ''}
              {c.exercise_name} ×{c.reps}
            </span>
          ))}{' '}
          @ RPE {p.target_rpe.toFixed(1)}
        </span>
      ) : (
        <span className="text-sm text-gray-500">
          {p.sets}×{p.reps} @ RPE {p.target_rpe.toFixed(1)}
        </span>
      )}

      {p.done ? (
        <span className="rounded-full bg-emerald-100 px-2 py-0.5 text-xs font-medium text-emerald-800">
          ✓ Feito{p.actual_rpe != null ? ` (RPE ${p.actual_rpe})` : ''}
        </span>
      ) : (
        <span className="ml-auto flex items-center gap-2">
          <label className="flex items-center gap-1 text-xs text-gray-600">
            Quão difícil?
            <input
              type="number"
              step="0.5"
              min="1"
              max="10"
              placeholder="RPE 1–10"
              value={rpe}
              onChange={(e) => setRpe(e.target.value)}
              className="w-24 rounded border border-gray-300 px-2 py-1 text-sm"
            />
          </label>
          <input
            type="text"
            placeholder="nota (opcional)"
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            className="w-28 rounded border border-gray-300 px-2 py-1 text-sm"
          />
          <button
            type="button"
            onClick={handleDone}
            disabled={saving || !rpeValido}
            title={rpeValido ? '' : 'Informe o RPE (1–10) para marcar como feito'}
            className="rounded bg-blue-600 px-3 py-1 text-sm font-medium text-white hover:bg-blue-700 disabled:bg-blue-300"
          >
            Marcar feito
          </button>
        </span>
      )}
    </li>
  )
}
