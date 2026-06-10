import { useEffect, useState } from 'react'
import { fetchOneRMs, saveOneRM } from '../api'

// Exercícios principais do seed que vale registrar 1RM.
// O usuário pode adicionar qualquer ID manualmente também.
const KEY_EXERCISES = [
  { id: 1,  name: 'Back Squat' },
  { id: 3,  name: 'Deadlift' },
  { id: 5,  name: 'Bench Press' },
  { id: 7,  name: 'Overhead Press' },
  { id: 9,  name: 'Power Clean' },
  { id: 11, name: 'Power Snatch' },
]

export default function ProfileView({ onClose }) {
  const [oneRMs, setOneRMs] = useState([])
  const [drafts, setDrafts] = useState({}) // exerciseId → weightKg (string)
  const [saving, setSaving] = useState(null) // exerciseId sendo salvo
  const [error, setError] = useState('')

  useEffect(() => {
    fetchOneRMs()
      .then((list) => {
        setOneRMs(list)
        // Pré-preenche os campos com valores já salvos.
        const pre = {}
        list.forEach((o) => { pre[o.exercise_id] = String(o.weight_kg) })
        setDrafts(pre)
      })
      .catch((e) => setError(e.message))
  }, [])

  async function handleSave(exerciseId) {
    const kg = parseFloat(drafts[exerciseId])
    if (!kg || kg <= 0) return
    setSaving(exerciseId)
    setError('')
    try {
      await saveOneRM(exerciseId, kg)
      setOneRMs((prev) => {
        const idx = prev.findIndex((o) => o.exercise_id === exerciseId)
        const entry = { exercise_id: exerciseId, weight_kg: kg }
        if (idx >= 0) {
          const next = [...prev]
          next[idx] = { ...next[idx], ...entry }
          return next
        }
        return [...prev, entry]
      })
    } catch (e) {
      setError(e.message)
    } finally {
      setSaving(null)
    }
  }

  function current(exerciseId) {
    return oneRMs.find((o) => o.exercise_id === exerciseId)
  }

  return (
    <div className="rounded-xl border border-gray-200 bg-white p-6">
      <div className="mb-5 flex items-center justify-between">
        <h2 className="text-lg font-semibold text-gray-900">Perfil — 1RM</h2>
        {onClose && (
          <button
            onClick={onClose}
            className="text-sm text-gray-500 hover:text-gray-800"
          >
            Fechar
          </button>
        )}
      </div>

      {error && (
        <p className="mb-4 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>
      )}

      <p className="mb-4 text-sm text-gray-500">
        Registre seu 1RM (carga máxima numa repetição) para calibrar o treino ao longo do tempo.
      </p>

      <ul className="flex flex-col gap-3">
        {KEY_EXERCISES.map(({ id, name }) => {
          const saved = current(id)
          const draft = drafts[id] ?? ''
          return (
            <li key={id} className="flex items-center gap-3">
              <span className="w-44 text-sm font-medium text-gray-800">{name}</span>
              <input
                type="number"
                min="0"
                step="0.5"
                value={draft}
                onChange={(e) => setDrafts((prev) => ({ ...prev, [id]: e.target.value }))}
                placeholder="kg"
                className="w-24 rounded-lg border border-gray-300 px-2 py-1 text-sm focus:outline-none focus:ring-2 focus:ring-gray-800"
              />
              <button
                onClick={() => handleSave(id)}
                disabled={saving === id || !drafts[id]}
                className="rounded-lg bg-gray-900 px-3 py-1 text-sm font-medium text-white hover:bg-gray-700 disabled:opacity-40"
              >
                {saving === id ? '…' : saved ? 'Atualizar' : 'Salvar'}
              </button>
              {saved && (
                <span className="text-xs text-gray-400">
                  salvo: {saved.weight_kg} kg
                </span>
              )}
            </li>
          )
        })}
      </ul>
    </div>
  )
}
