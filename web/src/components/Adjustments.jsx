// Adjustments: a TRANSPARÊNCIA da Fase 2. Mostra, em linguagem modesta, todo ajuste
// que o motor fez na carga — o quê, em qual semana e por quê. Nunca "detectamos
// overtraining"; sempre "seus registros sugerem aliviar". Confiança = ver o porquê.

const ACTION_LABEL = {
  reduce_volume: 'Volume reduzido',
  reduce_rpe: 'Esforço-alvo reduzido',
  reactive_deload: 'Semana de alívio',
}

export default function Adjustments({ adjustments }) {
  if (!adjustments || adjustments.length === 0) return null

  return (
    <section className="mt-6">
      <h3 className="mb-2 text-sm font-semibold text-gray-700">
        Ajustes do motor ({adjustments.length})
      </h3>
      <ul className="space-y-2">
        {adjustments.map((a) => (
          <li
            key={a.id}
            className="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3"
          >
            <div className="mb-1 flex items-center gap-2">
              <span className="rounded-full bg-amber-200 px-2 py-0.5 text-xs font-medium text-amber-900">
                {ACTION_LABEL[a.action] || a.action}
              </span>
              <span className="text-xs text-gray-500">{a.created_at}</span>
            </div>
            <p className="text-sm text-amber-900">{a.explanation}</p>
          </li>
        ))}
      </ul>
    </section>
  )
}
