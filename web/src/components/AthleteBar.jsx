// AthleteBar: seletor de atleta (Fase 6A, sem auth). Escolhe entre os atletas existentes ou cria um
// novo. Trocar o atleta faz o app recarregar o estado dele (bloco ativo ou questionário).

import { useState } from 'react'

export default function AthleteBar({ athletes, currentAthlete, onSelect, onCreate }) {
  const [name, setName] = useState('')

  function handleCreate(e) {
    e.preventDefault()
    const n = name.trim()
    if (n) {
      onCreate(n)
      setName('')
    }
  }

  return (
    <div className="mb-6 flex flex-wrap items-center gap-3 rounded-lg border border-gray-200 bg-gray-50 px-4 py-2">
      <label className="flex items-center gap-2 text-sm text-gray-700">
        Atleta:
        <select
          value={currentAthlete}
          onChange={(e) => onSelect(Number(e.target.value))}
          className="rounded border border-gray-300 bg-white px-2 py-1 text-sm"
        >
          {athletes.map((a) => (
            <option key={a.id} value={a.id}>
              {a.name}
            </option>
          ))}
        </select>
      </label>

      <form onSubmit={handleCreate} className="ml-auto flex items-center gap-2">
        <input
          type="text"
          placeholder="novo atleta"
          value={name}
          onChange={(e) => setName(e.target.value)}
          className="w-32 rounded border border-gray-300 px-2 py-1 text-sm"
        />
        <button
          type="submit"
          disabled={!name.trim()}
          className="rounded bg-gray-800 px-3 py-1 text-sm font-medium text-white hover:bg-gray-900 disabled:bg-gray-300"
        >
          Criar
        </button>
      </form>
    </div>
  )
}
