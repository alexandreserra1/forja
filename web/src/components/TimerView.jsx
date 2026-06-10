import { useState, useEffect, useRef } from 'react'

function playBeep(ctx, freq = 880, dur = 0.1, vol = 0.5) {
  try {
    const osc  = ctx.createOscillator()
    const gain = ctx.createGain()
    osc.connect(gain)
    gain.connect(ctx.destination)
    osc.frequency.value = freq
    gain.gain.setValueAtTime(vol, ctx.currentTime)
    gain.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + dur)
    osc.start(ctx.currentTime)
    osc.stop(ctx.currentTime + dur)
  } catch {
    // AudioContext pode falhar em ambientes restritos — ignora silenciosamente.
  }
}

function Ring({ progress, warn, rest, size = 280 }) {
  const stroke = 14
  const r      = (size - stroke) / 2
  const circ   = 2 * Math.PI * r
  const offset = circ * (1 - Math.max(0, Math.min(1, progress)))
  const color  = rest ? '#60a5fa' : warn ? '#ef4444' : '#22c55e'
  return (
    <svg width={size} height={size} style={{ position: 'absolute' }} aria-hidden="true">
      <circle cx={size / 2} cy={size / 2} r={r} fill="none"
        stroke="rgba(255,255,255,0.08)" strokeWidth={stroke} />
      <circle cx={size / 2} cy={size / 2} r={r} fill="none"
        stroke={color} strokeWidth={stroke} strokeLinecap="round"
        strokeDasharray={circ} strokeDashoffset={offset}
        transform={`rotate(-90 ${size / 2} ${size / 2})`}
        style={{ transition: 'stroke-dashoffset 0.2s linear, stroke 0.3s' }}
      />
    </svg>
  )
}

function fmt(secs) {
  const s = Math.max(0, Math.floor(secs))
  return `${String(Math.floor(s / 60)).padStart(2, '0')}:${String(s % 60).padStart(2, '0')}`
}

// Calcula o estado visual a partir do elapsed e do config.
function compute(elapsed, cfg) {
  const { mode, duration, rounds = 8, workTime = 20, restTime = 10 } = cfg

  if (mode === 'amrap') {
    const rem = Math.max(0, duration - elapsed)
    return {
      time: rem, label: 'AMRAP', sub: null,
      progress: duration > 0 ? rem / duration : 0,
      phase: 'work', round: null, done: rem <= 0,
    }
  }

  if (mode === 'fortime') {
    const done = duration > 0 && elapsed >= duration
    return {
      time: elapsed, label: 'For Time',
      sub: duration > 0 ? `cap ${fmt(duration)}` : null,
      progress: duration > 0 ? Math.min(elapsed / duration, 1) : 0,
      phase: 'work', round: null, done,
    }
  }

  if (mode === 'emom') {
    const total  = rounds * 60
    const r      = Math.min(Math.floor(elapsed / 60) + 1, rounds)
    const within = elapsed % 60
    const rem    = 60 - within
    return {
      time: rem, label: `Round ${r} / ${rounds}`, sub: 'EMOM',
      progress: rem / 60, phase: 'work', round: r, done: elapsed >= total,
    }
  }

  if (mode === 'tabata') {
    const cycle    = workTime + restTime
    const total    = rounds * cycle
    const done     = elapsed >= total
    const r        = Math.min(Math.floor(elapsed / cycle) + 1, rounds)
    const inCycle  = elapsed % cycle
    const isWork   = inCycle < workTime
    const phaseRem = done ? 0 : isWork ? workTime - inCycle : cycle - inCycle
    const phaseTot = isWork ? workTime : restTime
    return {
      time: phaseRem,
      label: isWork ? 'Trabalho' : 'Descanso',
      sub: `Round ${Math.min(r, rounds)} / ${rounds} · Tabata`,
      progress: done ? 0 : phaseRem / phaseTot,
      phase: isWork ? 'work' : 'rest',
      round: Math.min(r, rounds),
      done,
    }
  }

  return { time: 0, label: '', sub: null, progress: 0, phase: 'work', round: null, done: false }
}

export default function TimerView({ config, onClose, onFinish }) {
  const [elapsed, setElapsed] = useState(0)
  const [running, setRunning] = useState(false)
  const [muted,   setMuted]   = useState(() => localStorage.getItem('timer_muted') === '1')
  const [vibeOff, setVibeOff] = useState(() => localStorage.getItem('timer_novibe') === '1')

  const startedAt  = useRef(null) // Date.now() no último resume
  const base       = useRef(0)    // elapsed acumulado antes do último resume
  const ivl        = useRef(null)
  const audioCtx   = useRef(null)
  const lastBeep   = useRef(-1)
  const lastRound  = useRef(0)
  const finishFired = useRef(false)

  function audio() {
    if (!audioCtx.current) {
      audioCtx.current = new (window.AudioContext || window.webkitAudioContext)()
    }
    return audioCtx.current
  }

  function vibrate(pattern) {
    if (!vibeOff) navigator.vibrate?.(pattern)
  }

  const d = compute(elapsed, config)

  // Intervalo: usa Date.now() como fonte de verdade (não acumula drift).
  useEffect(() => {
    if (!running) return
    startedAt.current = Date.now()
    ivl.current = setInterval(() => {
      setElapsed(base.current + (Date.now() - startedAt.current) / 1000)
    }, 100)
    return () => clearInterval(ivl.current)
  }, [running])

  // Beeps e háptico — acionados quando o segundo inteiro muda ou fase/round muda.
  const t = Math.ceil(d.time)
  useEffect(() => {
    if (!running) return

    // Fim do timer
    if (d.done && !finishFired.current) {
      finishFired.current = true
      setRunning(false)
      if (!muted) {
        const ctx = audio()
        playBeep(ctx, 660, 1.0, 0.6)
        setTimeout(() => playBeep(ctx, 880, 0.5, 0.4), 300)
      }
      vibrate([500])
      onFinish?.()
      return
    }

    // 3-2-1 countdown (só modos countdown)
    if (config.mode !== 'fortime' && t >= 1 && t <= 3 && t !== lastBeep.current) {
      lastBeep.current = t
      if (!muted) playBeep(audio(), t === 1 ? 1100 : 880, 0.08, 0.4)
    }

    // EMOM: bipe no início de cada round
    if (config.mode === 'emom' && d.round && d.round !== lastRound.current) {
      lastRound.current = d.round
      if (!muted) playBeep(audio(), 660, 0.15, 0.5)
      vibrate([200])
    }

    // Tabata: bipe na troca de fase/round
    if (config.mode === 'tabata' && d.round && d.round !== lastRound.current) {
      lastRound.current = d.round
      if (!muted) playBeep(audio(), d.phase === 'work' ? 880 : 440, 0.12, 0.4)
      vibrate([150])
    }
  }, [t, d.done, d.round, d.phase])

  function toggleRunning() {
    if (d.done) return
    if (running) {
      base.current = elapsed
      setRunning(false)
    } else {
      setRunning(true)
    }
  }

  function reset() {
    setRunning(false)
    base.current = 0
    setElapsed(0)
    lastBeep.current  = -1
    lastRound.current = 0
    finishFired.current = false
  }

  function nextRound() {
    if (config.mode !== 'emom' || !running) return
    const next = d.round * 60
    base.current = next
    startedAt.current = Date.now()
    setElapsed(next)
  }

  function toggleMute() {
    const v = !muted
    setMuted(v)
    localStorage.setItem('timer_muted', v ? '1' : '0')
  }

  function toggleVibe() {
    const v = !vibeOff
    setVibeOff(v)
    localStorage.setItem('timer_novibe', v ? '1' : '0')
  }

  const warn = config.mode !== 'fortime' && !d.done && t > 0 && t <= 10
  const bg   = d.done          ? 'bg-green-950'
             : d.phase === 'rest' ? 'bg-blue-950'
             : warn             ? 'bg-red-950'
             :                    'bg-gray-950'

  const RING = 280

  return (
    <div
      data-testid="timer-view"
      className={`fixed inset-0 z-50 flex flex-col items-center justify-center ${bg} transition-colors duration-500`}
    >
      {/* Header */}
      <div className="absolute top-0 left-0 right-0 flex items-center justify-between px-5 py-4">
        <span className="text-xs font-black uppercase tracking-widest text-white/50">
          {config.mode === 'amrap'   ? 'AMRAP'    :
           config.mode === 'fortime' ? 'FOR TIME' :
           config.mode === 'emom'    ? 'EMOM'     : 'TABATA'}
        </span>
        <div className="flex items-center gap-4">
          <button onClick={toggleMute}
            className="text-xs font-bold uppercase tracking-wider text-white/40 hover:text-white/80 transition-colors">
            {muted ? 'SOM OFF' : 'SOM ON'}
          </button>
          <button onClick={toggleVibe}
            className="text-xs font-bold uppercase tracking-wider text-white/40 hover:text-white/80 transition-colors">
            {vibeOff ? 'VIB OFF' : 'VIB ON'}
          </button>
          <button onClick={onClose} aria-label="Fechar timer"
            className="text-sm font-medium text-white/40 hover:text-white/80 transition-colors">
            ✕
          </button>
        </div>
      </div>

      {/* Anel de progresso + dígitos */}
      <div className="relative flex items-center justify-center" style={{ width: RING, height: RING }}>
        <Ring progress={d.progress} warn={warn} rest={d.phase === 'rest'} size={RING} />
        <div className="absolute flex select-none flex-col items-center">
          <span
            data-testid="timer-display"
            className={`font-mono font-black tabular-nums leading-none text-white ${
              warn ? 'text-red-300' : d.phase === 'rest' ? 'text-blue-200' : ''
            }`}
            style={{ fontSize: '4.5rem' }}
          >
            {fmt(d.time)}
          </span>
          {d.label && (
            <span className="mt-3 text-sm font-bold uppercase tracking-widest text-white/60">
              {d.label}
            </span>
          )}
          {d.sub && (
            <span className="mt-1 text-xs font-medium uppercase tracking-wider text-white/30">
              {d.sub}
            </span>
          )}
        </div>
      </div>

      {d.done && (
        <p className="mt-4 text-3xl font-black uppercase tracking-widest text-green-300">
          Tempo!
        </p>
      )}

      {/* Controles */}
      <div className="mt-10 flex items-center gap-4">
        <button onClick={reset}
          style={{ minHeight: 56, minWidth: 80 }}
          className="rounded-2xl border border-white/20 px-6 text-sm font-bold uppercase tracking-wider text-white hover:bg-white/10 active:bg-white/20 transition-colors">
          Reset
        </button>

        <button
          data-testid="timer-startstop"
          onClick={toggleRunning}
          disabled={d.done}
          style={{ minHeight: 56, minWidth: 148 }}
          className="rounded-2xl bg-white px-8 text-lg font-black uppercase tracking-wider text-gray-950 hover:bg-gray-100 active:bg-gray-200 disabled:opacity-30 transition-colors"
        >
          {running ? 'Pausar' : d.done ? 'Fim' : elapsed > 0 ? 'Continuar' : 'Iniciar'}
        </button>

        {config.mode === 'emom' && running && !d.done && (
          <button onClick={nextRound}
            style={{ minHeight: 56 }}
            className="rounded-2xl border border-white/20 px-4 text-sm font-bold uppercase tracking-wider text-white hover:bg-white/10 active:bg-white/20 transition-colors">
            Próximo →
          </button>
        )}
      </div>
    </div>
  )
}
