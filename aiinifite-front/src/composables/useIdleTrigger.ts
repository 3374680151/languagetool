import { onMounted, onUnmounted } from 'vue'

export function useIdleTrigger(
  callback: () => void,
  options: { minMs?: number; maxMs?: number; onlyWhenActive?: () => boolean } = {}
) {
  const { minMs = 6000, maxMs = 10000, onlyWhenActive } = options
  let timer: number | null = null

  function schedule() {
    if (timer) clearTimeout(timer)
    const delay = minMs + Math.random() * (maxMs - minMs)
    timer = window.setTimeout(() => {
      if (onlyWhenActive && !onlyWhenActive()) {
        schedule()
        return
      }
      callback()
      schedule()
    }, delay)
  }

  onMounted(schedule)
  onUnmounted(() => {
    if (timer) clearTimeout(timer)
  })
}
