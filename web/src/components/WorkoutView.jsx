// WorkoutView: exibe o treino gerado, agrupado por dia.
// Recebe a lista achatada [{ day_number, exercise_name }] e agrupa por dia.

export default function WorkoutView({ workout, onRestart }) {
  // Agrupa as linhas por day_number, preservando a ordem.
  const days = workout.reduce((acc, row) => {
    ;(acc[row.day_number] ||= []).push(row)
    return acc
  }, {})

  return (
    <div className="workout">
      <h2>Seu treino</h2>
      {Object.entries(days).map(([day, rows]) => (
        <div key={day} className="day">
          <h3>Dia {day}</h3>
          <ul>
            {rows.map((row) => (
              <li key={row.id}>{row.exercise_name}</li>
            ))}
          </ul>
        </div>
      ))}
      <button type="button" onClick={onRestart}>
        Refazer questionário
      </button>
    </div>
  )
}
