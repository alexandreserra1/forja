// BlockView: lista as semanas do bloco com fase e RPE-alvo. Deload destacado.
// Clicar numa semana abre a WeekView. Na Fase 2 ganha o botão "Reavaliar" e o
// histórico transparente de ajustes do motor.

import Adjustments from './Adjustments'

const PHASE_LABEL = {
  accumulation: 'Acumulação',
  intensification: 'Intensificação',
  realization: 'Realização',
  deload: 'Deload',
}

const PHASE_STYLE = {
  accumulation: 'bg-sky-100 text-sky-800',
  intensification: 'bg-amber-100 text-amber-800',
  realization: 'bg-rose-100 text-rose-800',
  deload: 'bg-emerald-100 text-emerald-800',
}

export default function BlockView({
  overview,
  onSelectWeek,
  onRestart,
  adjustments,
  onEvaluate,
  evalMsg,
  evaluating,
  substitutions = [],
}) {
  const { block, weeks } = overview

  return (
    <div>
      <div className="mb-4 flex items-baseline justify-between">
        <h2 className="text-xl font-semibold">
          Bloco de {block.total_weeks} semanas
        </h2>
        <span className="text-sm text-gray-500">
          {block.days_per_week} dias/semana · objetivo: {block.goal}
        </span>
      </div>

      {substitutions.length > 0 && (
        <div className="mb-4 rounded-lg border border-indigo-200 bg-indigo-50 px-4 py-3">
          <p className="mb-1 text-sm font-medium text-indigo-900">
            Ajustamos {substitutions.length} exercício
            {substitutions.length > 1 ? 's' : ''} ao seu equipamento:
          </p>
          <ul className="space-y-0.5 text-sm text-indigo-800">
            {substitutions.map((s, i) => (
              <li key={i}>
                <strong>{s.substitute}</strong> no lugar de {s.ideal} — você não marcou{' '}
                {s.missing}.
              </li>
            ))}
          </ul>
        </div>
      )}

      <ul className="space-y-2">
        {weeks.map((w) => (
          <li key={w.id}>
            <button
              type="button"
              onClick={() => onSelectWeek(w.week_number)}
              className={`flex w-full items-center justify-between rounded-lg border px-4 py-3 text-left transition hover:shadow ${
                w.is_deload
                  ? 'border-emerald-300 bg-emerald-50'
                  : 'border-gray-200 bg-white'
              }`}
            >
              <span className="flex items-center gap-3">
                <span className="font-medium text-gray-700">
                  Semana {w.week_number}
                </span>
                <span
                  className={`rounded-full px-2 py-0.5 text-xs font-medium ${
                    PHASE_STYLE[w.phase] || 'bg-gray-100 text-gray-700'
                  }`}
                >
                  {PHASE_LABEL[w.phase] || w.phase}
                </span>
              </span>
              <span className="text-sm font-semibold text-gray-600">
                RPE {w.target_rpe.toFixed(1)}
              </span>
            </button>
          </li>
        ))}
      </ul>

      <div className="mt-6 flex flex-wrap items-center gap-3">
        <button
          type="button"
          onClick={onEvaluate}
          disabled={evaluating}
          className="rounded-lg bg-emerald-600 px-4 py-2 text-sm font-medium text-white hover:bg-emerald-700 disabled:bg-emerald-300"
        >
          {evaluating ? 'Avaliando…' : 'Reavaliar com base nos registros'}
        </button>
        <button
          type="button"
          onClick={onRestart}
          className="rounded-lg bg-gray-100 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-200"
        >
          Refazer questionário
        </button>
      </div>

      {evalMsg && (
        <p className="mt-3 rounded-lg bg-sky-50 px-3 py-2 text-sm text-sky-800">
          {evalMsg}
        </p>
      )}

      <Adjustments adjustments={adjustments} />
    </div>
  )
}
