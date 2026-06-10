// Questionnaire: desenha as perguntas vindas da API e coleta uma resposta por pergunta.
// Fase 0: tudo single_choice. O componente é genérico — lê 'options' de cada pergunta.

// Rótulos amigáveis dos padrões de movimento (Fase 6B). Sem rótulo = usa o nome cru.
const PATTERN_LABEL = {
  squat: 'Agachamento',
  hinge: 'Dobradiça de quadril',
  push: 'Empurrar',
  pull: 'Puxar',
  olympic: 'Levantamento olímpico',
  lunge: 'Afundo',
  carry: 'Carregamento',
  core: 'Core',
  cardio: 'Cardio',
}

export default function Questionnaire({
  questions,
  selected,
  onSelect,
  onSubmit,
  equipment = [],
  selectedEquipment = [],
  onToggleEquipment,
  patterns = [],
  selectedPriorities = [],
  onTogglePriority,
  metrics = { age_years: '', sex: '', body_weight_kg: '', sport: '' },
  onChangeMetrics,
}) {
  // Só habilita o botão quando todas as perguntas têm resposta.
  // O equipamento é OPCIONAL (vazio = sem filtro), então não trava o botão.
  const allAnswered = questions.every((q) => selected[q.id] !== undefined)

  return (
    <form
      className="questionnaire"
      onSubmit={(e) => {
        e.preventDefault()
        onSubmit()
      }}
    >
      {questions.map((q) => (
        <fieldset key={q.id} className="question">
          <legend>{q.text}</legend>
          {q.options.map((opt) => (
            <label key={opt.id} className="option">
              <input
                type="radio"
                name={`q-${q.id}`}
                value={opt.value}
                checked={selected[q.id] === opt.value}
                onChange={() => onSelect(q.id, opt.value)}
              />
              {opt.label}
            </label>
          ))}
        </fieldset>
      ))}

      {equipment.length > 0 && (
        <fieldset className="question">
          <legend>Qual equipamento você tem?</legend>
          <p style={{ margin: '0 0 0.5rem', fontSize: '0.85rem', color: '#666' }}>
            Marque o que tem — o treino não vai prescrever nada que você não possa fazer.
            (Não marcar nada = assume tudo disponível.)
          </p>
          {equipment.map((eq) => (
            <label key={eq.id} className="option">
              <input
                type="checkbox"
                checked={selectedEquipment.includes(eq.id)}
                onChange={() => onToggleEquipment(eq.id)}
              />
              {eq.name}
            </label>
          ))}
        </fieldset>
      )}

      {patterns.length > 0 && (
        <fieldset className="question">
          <legend>Quer priorizar algum movimento? (pontos fracos)</legend>
          <p style={{ margin: '0 0 0.5rem', fontSize: '0.85rem', color: '#666' }}>
            Marque o que quer enfatizar — ele aparece mais no bloco. (Não marcar = sem ênfase.)
          </p>
          {patterns.map((pt) => (
            <label key={pt.id} className="option">
              <input
                type="checkbox"
                checked={selectedPriorities.includes(pt.id)}
                onChange={() => onTogglePriority(pt.id)}
              />
              {PATTERN_LABEL[pt.name] || pt.name}
            </label>
          ))}
        </fieldset>
      )}

      {/* Fase 6D: dados opcionais para calibração de dose — todos os campos são opcionais */}
      <fieldset className="question">
        <legend>Dados opcionais (calibração de dose)</legend>
        <p style={{ margin: '0 0 0.75rem', fontSize: '0.85rem', color: '#666' }}>
          Preencha o que quiser — o motor ajusta o volume de acordo. Deixar em branco = sem ajuste.
        </p>
        <div style={{ display: 'grid', gap: '0.75rem' }}>
          <div>
            <label htmlFor="m-age" style={{ display: 'block', fontSize: '0.875rem', marginBottom: '0.25rem' }}>
              Idade (anos)
            </label>
            <input
              id="m-age"
              type="number"
              min="10"
              max="99"
              placeholder="ex: 35"
              value={metrics.age_years}
              onChange={(e) => onChangeMetrics('age_years', e.target.value)}
              style={{ width: '100%', padding: '0.4rem 0.6rem', border: '1px solid #ccc', borderRadius: '4px' }}
            />
          </div>
          <div>
            <label htmlFor="m-sex" style={{ display: 'block', fontSize: '0.875rem', marginBottom: '0.25rem' }}>
              Sexo
            </label>
            <select
              id="m-sex"
              value={metrics.sex}
              onChange={(e) => onChangeMetrics('sex', e.target.value)}
              style={{ width: '100%', padding: '0.4rem 0.6rem', border: '1px solid #ccc', borderRadius: '4px' }}
            >
              <option value="">Prefiro não informar</option>
              <option value="m">Masculino</option>
              <option value="f">Feminino</option>
              <option value="x">Outro</option>
            </select>
          </div>
          <div>
            <label htmlFor="m-weight" style={{ display: 'block', fontSize: '0.875rem', marginBottom: '0.25rem' }}>
              Peso corporal (kg)
            </label>
            <input
              id="m-weight"
              type="number"
              min="30"
              max="250"
              step="0.1"
              placeholder="ex: 80"
              value={metrics.body_weight_kg}
              onChange={(e) => onChangeMetrics('body_weight_kg', e.target.value)}
              style={{ width: '100%', padding: '0.4rem 0.6rem', border: '1px solid #ccc', borderRadius: '4px' }}
            />
          </div>
          <div>
            <label htmlFor="m-sport" style={{ display: 'block', fontSize: '0.875rem', marginBottom: '0.25rem' }}>
              Esporte / modalidade principal
            </label>
            <select
              id="m-sport"
              value={metrics.sport}
              onChange={(e) => onChangeMetrics('sport', e.target.value)}
              style={{ width: '100%', padding: '0.4rem 0.6rem', border: '1px solid #ccc', borderRadius: '4px' }}
            >
              <option value="">Não especificar</option>
              <option value="crossfit">CrossFit</option>
              <option value="general_fitness">Fitness geral</option>
              <option value="weightlifting">Halterofilismo</option>
              <option value="endurance">Endurance (corrida/ciclismo/etc.)</option>
            </select>
          </div>
        </div>
      </fieldset>

      <button type="submit" disabled={!allAnswered}>
        Gerar treino
      </button>
    </form>
  )
}
