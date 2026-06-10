// DashboardView: home do atleta quando há um bloco ativo.
// Mostra fase atual, progresso semanal, próxima sessão e acesso rápido ao treino.

import { useEffect, useState } from 'react'
import { fetchWeek } from '../api'

const PHASE_LABEL = {
  accumulation:    'Acumulação',
  intensification: 'Intensificação',
  realization:     'Realização',
  deload:          'Deload',
}

const PHASE_STYLE = {
  accumulation:    'bg-sky-100 text-sky-800 border-sky-200',
  intensification: 'bg-amber-100 text-amber-800 border-amber-200',
  realization:     'bg-rose-100 text-rose-800 border-rose-200',
  deload:          'bg-emerald-100 text-emerald-800 border-emerald-200',
}

// Verifica se todos os itens treináveis de uma semana estão feitos.
function isWeekDone(weekData) {
  return weekData.sessions.every(
    (s) =>
      s.prescriptions.every((p) => p.done) &&
      (s.conditioning || []).every((c) => c.done),
  )
}

// Conta prescrições feitas e total numa semana.
function weekProgress(weekData) {
  let done = 0, total = 0
  for (const s of weekData.sessions) {
    done  += s.prescriptions.filter((p) => p.done).length
    total += s.prescriptions.length
    done  += (s.conditioning || []).filter((c) => c.done).length
    total += (s.conditioning || []).length
  }
  return { done, total }
}

// Retorna a primeira sessão com ao menos uma prescrição não feita.
function nextSession(weekData) {
  return weekData.sessions.find(
    (s) =>
      s.prescriptions.some((p) => !p.done) ||
      (s.conditioning || []).some((c) => !c.done),
  )
}

export default function DashboardView({
  overview,
  onOpenWeek,
  onOpenBlock,
  onRestart,
  onEvaluate,
  evaluating,
  evalMsg,
  adjustments = [],
}) {
  const { block, weeks } = overview

  const [weekNum,  setWeekNum]  = useState(weeks[0]?.week_number ?? 1)
  const [weekData, setWeekData] = useState(null)
  const [loading,  setLoading]  = useState(true)
  const [error,    setError]    = useState('')

  // Busca a semana; se estiver 100% concluída, avança automaticamente para a próxima.
  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError('')

    async function load(n) {
      const data = await fetchWeek(n)
      if (cancelled) return
      if (isWeekDone(data) && n < weeks[weeks.length - 1].week_number) {
        setWeekNum(n + 1)
        // O próximo useEffect vai buscar n+1; não precisamos fazer nada mais aqui.
      } else {
        setWeekData(data)
        setWeekNum(n)
        setLoading(false)
      }
    }

    load(weekNum).catch((e) => {
      if (!cancelled) { setError(e.message); setLoading(false) }
    })

    return () => { cancelled = true }
  }, [weekNum])

  // --- estados de carregamento / erro ---
  if (loading) {
    return <p className="py-8 text-center text-gray-400">Carregando…</p>
  }
  if (error) {
    return <p className="rounded-lg bg-red-50 px-3 py-2 text-red-700">⚠ {error}</p>
  }
  if (!weekData) return null

  const currentWeek = weekData.week
  const phaseStyle  = PHASE_STYLE[currentWeek.phase] || 'bg-gray-100 text-gray-700 border-gray-200'
  const { done, total } = weekProgress(weekData)
  const pct     = total > 0 ? Math.round((done / total) * 100) : 0
  const next    = nextSession(weekData)
  const allDone = isWeekDone(weekData)

  return (
    <div data-testid="dashboard" className="space-y-6">

      {/* Resumo do bloco */}
      <div>
        <p className="text-sm text-gray-500">
          Bloco de {block.total_weeks} semanas · {block.days_per_week} dias/semana ·{' '}
          <span className="font-medium text-gray-700">{block.goal}</span>
        </p>
      </div>

      {/* Card da semana atual */}
      <div className={`rounded-2xl border p-5 ${phaseStyle}`}>
        <div className="mb-3 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <span className="text-xs font-black uppercase tracking-widest opacity-70">
              Semana {currentWeek.week_number} / {block.total_weeks}
            </span>
            <span className={`rounded-full border px-2 py-0.5 text-xs font-semibold ${phaseStyle}`}>
              {PHASE_LABEL[currentWeek.phase] || currentWeek.phase}
            </span>
          </div>
          <span className="text-sm font-bold">RPE {currentWeek.target_rpe.toFixed(1)}</span>
        </div>

        {/* Barra de progresso */}
        <div className="mb-4">
          <div className="mb-1 flex justify-between text-xs opacity-70">
            <span>Progresso semanal</span>
            <span>{done} / {total} itens feitos</span>
          </div>
          <div className="h-2 w-full overflow-hidden rounded-full bg-black/10">
            <div
              className="h-full rounded-full bg-current transition-all duration-500"
              style={{ width: `${pct}%`, opacity: 0.5 }}
            />
          </div>
        </div>

        {/* Próxima sessão */}
        {allDone ? (
          <p className="text-sm font-medium opacity-80">Semana concluída! 🎉</p>
        ) : next ? (
          <div className="mb-4">
            <p className="mb-1 text-xs font-bold uppercase tracking-wider opacity-60">
              Próxima sessão — Dia {next.session.day_number}
            </p>
            <ul className="space-y-0.5 text-sm opacity-80">
              {next.prescriptions
                .filter((p) => !p.done)
                .slice(0, 3)
                .map((p) => (
                  <li key={p.id}>
                    {p.exercise_name} · {p.sets}×{p.reps}
                  </li>
                ))}
              {(next.conditioning || [])
                .filter((c) => !c.done)
                .slice(0, 1)
                .map((c) => (
                  <li key={c.id}>
                    WOD · {c.wod?.format_name}{' '}
                    {c.wod?.work_sec >= 60
                      ? `${Math.round(c.wod.work_sec / 60)}min`
                      : `${c.wod?.work_sec}s`}
                  </li>
                ))}
            </ul>
          </div>
        ) : null}

        <button
          onClick={() => onOpenWeek(currentWeek.week_number)}
          className="w-full rounded-xl bg-black/10 py-2.5 text-sm font-bold hover:bg-black/20 transition-colors"
        >
          {allDone ? 'Ver semana →' : 'Abrir treino →'}
        </button>
      </div>

      {/* Navegação rápida entre semanas */}
      <div>
        <p className="mb-2 text-xs font-bold uppercase tracking-widest text-gray-400">
          Todas as semanas
        </p>
        <ul className="space-y-1.5">
          {weeks.map((w) => (
            <li key={w.id}>
              <button
                onClick={() => onOpenWeek(w.week_number)}
                className={`flex w-full items-center justify-between rounded-xl border px-4 py-2.5 text-left text-sm transition hover:shadow-sm ${
                  w.week_number === currentWeek.week_number
                    ? `${PHASE_STYLE[w.phase] || 'bg-gray-100 border-gray-200'} font-semibold`
                    : 'border-gray-200 bg-white text-gray-600 hover:bg-gray-50'
                }`}
              >
                <span className="flex items-center gap-2">
                  {w.week_number === currentWeek.week_number && (
                    <span className="inline-block h-1.5 w-1.5 rounded-full bg-current" />
                  )}
                  Semana {w.week_number}
                  <span className="text-xs opacity-60">
                    {PHASE_LABEL[w.phase] || w.phase}
                  </span>
                </span>
                <span className="font-semibold">RPE {w.target_rpe.toFixed(1)}</span>
              </button>
            </li>
          ))}
        </ul>
      </div>

      {/* Ações */}
      <div className="flex flex-wrap items-center gap-3">
        <button
          onClick={onEvaluate}
          disabled={evaluating}
          className="rounded-lg bg-emerald-600 px-4 py-2 text-sm font-medium text-white hover:bg-emerald-700 disabled:bg-emerald-300"
        >
          {evaluating ? 'Avaliando…' : 'Reavaliar com base nos registros'}
        </button>
        <button
          onClick={onRestart}
          className="rounded-lg bg-gray-100 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-200"
        >
          Refazer questionário
        </button>
      </div>

      {evalMsg && (
        <p className="rounded-lg bg-sky-50 px-3 py-2 text-sm text-sky-800">{evalMsg}</p>
      )}

      {adjustments.length > 0 && (
        <details className="rounded-lg border border-gray-200 bg-white">
          <summary className="cursor-pointer px-4 py-3 text-sm font-medium text-gray-600 hover:text-gray-900">
            Histórico de ajustes ({adjustments.length})
          </summary>
          <ul className="divide-y divide-gray-100 px-4 pb-3 text-sm text-gray-600">
            {adjustments.map((a) => (
              <li key={a.id} className="py-2">
                Semana {a.week_number}: RPE ajustado de {a.old_rpe?.toFixed(1)} → {a.new_rpe?.toFixed(1)}
                {a.explanation && <span className="ml-1 text-gray-400">— {a.explanation}</span>}
              </li>
            ))}
          </ul>
        </details>
      )}
    </div>
  )
}
