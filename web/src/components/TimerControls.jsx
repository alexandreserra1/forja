import { useState } from 'react'

const MODES = [
  { id: 'amrap',   label: 'AMRAP' },
  { id: 'fortime', label: 'For Time' },
  { id: 'emom',    label: 'EMOM' },
  { id: 'tabata',  label: 'Tabata' },
]

export default function TimerControls({ initial = {}, onStart, onClose }) {
  const [mode, setMode]         = useState(initial.mode || 'amrap')
  const [minutes, setMinutes]   = useState(initial.minutes ?? 12)
  const [rounds, setRounds]     = useState(initial.rounds ?? 8)
  const [workSecs, setWorkSecs] = useState(initial.workSecs ?? 20)
  const [restSecs, setRestSecs] = useState(initial.restSecs ?? 10)

  function handleStart() {
    onStart({ mode, duration: minutes * 60, rounds, workTime: workSecs, restTime: restSecs })
  }

  return (
    <div data-testid="timer-controls" className="fixed inset-0 z-50 flex items-center justify-center bg-gray-950 p-6">
      <div className="w-full max-w-sm">
        <div className="mb-8 flex items-center justify-between">
          <span className="text-xs font-black uppercase tracking-widest text-white/50">Timer</span>
          <button onClick={onClose} aria-label="Fechar timer" className="text-lg text-white/40 hover:text-white/80 transition-colors">✕</button>
        </div>

        <p className="mb-3 text-xs font-bold uppercase tracking-widest text-white/40">Modo</p>
        <div className="mb-8 grid grid-cols-2 gap-2">
          {MODES.map(m => (
            <button key={m.id} onClick={() => setMode(m.id)}
              className={`rounded-xl py-3 text-sm font-bold uppercase tracking-wider transition-colors ${
                mode === m.id
                  ? 'bg-white text-gray-950'
                  : 'border border-white/20 text-white hover:bg-white/10'
              }`}>
              {m.label}
            </button>
          ))}
        </div>

        {(mode === 'amrap' || mode === 'fortime') && (
          <label className="mb-6 block">
            <span className="mb-2 block text-xs font-bold uppercase tracking-widest text-white/40">
              {mode === 'fortime' ? 'Cap (minutos, 0 = sem limite)' : 'Duração (minutos)'}
            </span>
            <input type="number" min="1" max="90" value={minutes}
              onChange={e => setMinutes(Number(e.target.value))}
              className="w-full rounded-xl border border-white/20 bg-transparent px-4 py-3 text-3xl font-mono font-bold text-white focus:outline-none focus:border-white/60"
            />
          </label>
        )}

        {mode === 'emom' && (
          <div className="mb-6 grid grid-cols-2 gap-4">
            <label>
              <span className="mb-2 block text-xs font-bold uppercase tracking-widest text-white/40">Minutos</span>
              <input type="number" min="1" max="60" value={minutes}
                onChange={e => setMinutes(Number(e.target.value))}
                className="w-full rounded-xl border border-white/20 bg-transparent px-4 py-3 text-2xl font-mono font-bold text-white focus:outline-none focus:border-white/60"
              />
            </label>
            <label>
              <span className="mb-2 block text-xs font-bold uppercase tracking-widest text-white/40">Rounds</span>
              <input type="number" min="1" max="60" value={rounds}
                onChange={e => setRounds(Number(e.target.value))}
                className="w-full rounded-xl border border-white/20 bg-transparent px-4 py-3 text-2xl font-mono font-bold text-white focus:outline-none focus:border-white/60"
              />
            </label>
          </div>
        )}

        {mode === 'tabata' && (
          <div className="mb-6 grid grid-cols-3 gap-3">
            <label>
              <span className="mb-2 block text-xs font-bold uppercase tracking-widest text-white/40">Trab (s)</span>
              <input type="number" min="5" max="120" value={workSecs}
                onChange={e => setWorkSecs(Number(e.target.value))}
                className="w-full rounded-xl border border-white/20 bg-transparent px-3 py-3 text-xl font-mono font-bold text-white focus:outline-none focus:border-white/60"
              />
            </label>
            <label>
              <span className="mb-2 block text-xs font-bold uppercase tracking-widest text-white/40">Desc (s)</span>
              <input type="number" min="5" max="120" value={restSecs}
                onChange={e => setRestSecs(Number(e.target.value))}
                className="w-full rounded-xl border border-white/20 bg-transparent px-3 py-3 text-xl font-mono font-bold text-white focus:outline-none focus:border-white/60"
              />
            </label>
            <label>
              <span className="mb-2 block text-xs font-bold uppercase tracking-widest text-white/40">Rounds</span>
              <input type="number" min="1" max="30" value={rounds}
                onChange={e => setRounds(Number(e.target.value))}
                className="w-full rounded-xl border border-white/20 bg-transparent px-3 py-3 text-xl font-mono font-bold text-white focus:outline-none focus:border-white/60"
              />
            </label>
          </div>
        )}

        <button onClick={handleStart}
          style={{ minHeight: 64 }}
          className="w-full rounded-2xl bg-white text-xl font-black uppercase tracking-wider text-gray-950 hover:bg-gray-100 active:bg-gray-200 transition-colors">
          Iniciar
        </button>
      </div>
    </div>
  )
}
