import { ref, onMounted, onUnmounted } from 'vue'

export interface PointerState {
  /** Normalized x: -1 (left) .. 1 (right), relative to viewport center */
  x: number
  /** Normalized y: -1 (top) .. 1 (bottom) */
  y: number
  /** Raw pixel coordinates */
  rawX: number
  rawY: number
  /** True when pointer is over the page */
  active: boolean
}

const state = ref<PointerState>({
  x: 0,
  y: 0,
  rawX: 0,
  rawY: 0,
  active: false
})

let lastMoveAt = 0
let rafPending = false
let pendingEvent: PointerEvent | null = null
const IDLE_THRESHOLD_MS = 80

function handleMove(e: PointerEvent) {
  pendingEvent = e
  lastMoveAt = performance.now()
  if (rafPending) return
  rafPending = true
  requestAnimationFrame(flush)
}

function flush() {
  rafPending = false
  if (!pendingEvent) return
  const e = pendingEvent
  pendingEvent = null
  const cx = window.innerWidth / 2
  const cy = window.innerHeight / 2
  state.value = {
    x: Math.max(-1, Math.min(1, (e.clientX - cx) / cx)),
    y: Math.max(-1, Math.min(1, (e.clientY - cy) / cy)),
    rawX: e.clientX,
    rawY: e.clientY,
    active: true
  }
}

function handleLeave() {
  state.value = { ...state.value, active: false }
}

export function usePointer() {
  onMounted(() => {
    window.addEventListener('pointermove', handleMove, { passive: true })
    window.addEventListener('pointerleave', handleLeave)
    window.addEventListener('blur', handleLeave)
  })
  onUnmounted(() => {
    window.removeEventListener('pointermove', handleMove)
    window.removeEventListener('pointerleave', handleLeave)
    window.removeEventListener('blur', handleLeave)
  })
  return state
}

export function getLastMoveAt() {
  return lastMoveAt
}
