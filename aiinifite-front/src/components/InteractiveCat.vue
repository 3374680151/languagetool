<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, watch } from 'vue'
import { gsap } from 'gsap'
import { usePointer } from '../composables/usePointer'
import { useIdleTrigger } from '../composables/useIdleTrigger'
import CatShadow from './CatShadow.vue'
import ClickParticles from './ClickParticles.vue'

const root = ref<HTMLDivElement | null>(null)
const catEl = ref<HTMLDivElement | null>(null)
const bgEl = ref<HTMLDivElement | null>(null)
const isHover = ref(false)
const isExcited = ref(false)
const clickBursts = ref<Record<number, { id: number; x: number; y: number }>>({})
let burstId = 0

const pointer = usePointer()
const tiltMax = 10

let pointerUnwatch: (() => void) | null = null
onMounted(() => {
  startExcited()
  if (catEl.value && bgEl.value) {
    const rotY = gsap.quickTo(catEl.value, 'rotationY', { duration: 0.45, ease: 'power2.out' })
    const rotX = gsap.quickTo(catEl.value, 'rotationX', { duration: 0.45, ease: 'power2.out' })
    const bgRotY = gsap.quickTo(bgEl.value, 'rotationY', { duration: 0.7, ease: 'power2.out' })
    const bgRotX = gsap.quickTo(bgEl.value, 'rotationX', { duration: 0.7, ease: 'power2.out' })
    const bgX = gsap.quickTo(bgEl.value, 'x', { duration: 0.7, ease: 'power2.out' })
    const bgY = gsap.quickTo(bgEl.value, 'y', { duration: 0.7, ease: 'power2.out' })

    let lastX = 2
    let lastY = 2
    const tick = () => {
      const { x, y } = pointer.value
      if (Math.abs(x - lastX) < 0.002 && Math.abs(y - lastY) < 0.002) return
      lastX = x
      lastY = y
      rotY(x * tiltMax)
      rotX(-y * tiltMax * 0.6)
      bgRotY(-x * tiltMax * 0.4)
      bgRotX(y * tiltMax * 0.24)
      bgX(-x * 18)
      bgY(-y * 18)
    }
    gsap.ticker.add(tick)
    pointerUnwatch = () => gsap.ticker.remove(tick)
  }
})

let excitedTimer: number | null = null
let clickCount = 0
let clickResetTimer: number | null = null

watch(isExcited, (v) => (v ? startExcited() : stopExcited()))

function triggerExcited() {
  isExcited.value = true
  if (excitedTimer) clearTimeout(excitedTimer)
  excitedTimer = window.setTimeout(() => {
    isExcited.value = false
  }, 5000)
}

function onClick(e: MouseEvent) {
  e.stopPropagation()
  if (!catEl.value) return
  clickCount++
  if (clickResetTimer) clearTimeout(clickResetTimer)
  clickResetTimer = window.setTimeout(() => (clickCount = 0), 4000)
  if (clickCount >= 3) {
    triggerExcited()
    clickCount = 0
  }

  gsap.timeline()
    .to(catEl.value, { scale: 1.12, duration: 0.1, ease: 'power2.out' })
    .to(catEl.value, { scale: 0.95, duration: 0.12, ease: 'power2.in' })
    .to(catEl.value, {
      scale: 1.0,
      duration: 0.45,
      ease: 'elastic.out(1, 0.4)'
    })

  if (root.value) {
    const r = root.value.getBoundingClientRect()
    const burst = {
      id: ++burstId,
      x: e.clientX - r.left,
      y: e.clientY - r.top
    }
    clickBursts.value = { ...clickBursts.value, [burst.id]: burst }
    setTimeout(() => {
      const c = { ...clickBursts.value }
      delete c[burst.id]
      clickBursts.value = c
    }, 1100)
  }
}

let hoverVelocity = 0
let lastHoverX = 0
let lastHoverT = 0
let petTimer: number | null = null
function onCatMove(e: PointerEvent) {
  const now = performance.now()
  if (lastHoverT) {
    const dt = Math.max(1, now - lastHoverT)
    hoverVelocity = Math.abs(e.clientX - lastHoverX) / dt
  }
  lastHoverX = e.clientX
  lastHoverT = now

  if (hoverVelocity > 1.2) {
    if (petTimer) clearTimeout(petTimer)
    petTimer = window.setTimeout(triggerPetted, 60)
  }
}

function triggerPetted() {
  if (!catEl.value) return
  gsap.timeline()
    .to(catEl.value, { scale: 0.96, y: 6, duration: 0.15, ease: 'power2.out' })
    .to(catEl.value, {
      scale: 1.02,
      y: -4,
      duration: 0.2,
      ease: 'power2.out'
    })
    .to(catEl.value, {
      scale: 1.0,
      y: 0,
      duration: 0.5,
      ease: 'elastic.out(1, 0.3)'
    })
}

useIdleTrigger(
  () => {
    if (!catEl.value) return
    const dir = Math.random() > 0.5 ? 1 : -1
    gsap.timeline()
      .to(catEl.value, { rotation: dir * 2.5, duration: 0.06 })
      .to(catEl.value, { rotation: -dir * 1.5, duration: 0.08 })
      .to(catEl.value, { rotation: 0, duration: 0.4, ease: 'elastic.out(1, 0.3)' })
  },
  { minMs: 6000, maxMs: 10000, onlyWhenActive: () => !isHover.value }
)

useIdleTrigger(
  () => {
    if (!catEl.value) return
    gsap.timeline()
      .to(catEl.value, { y: -8, duration: 0.5, ease: 'power2.out' })
      .to(catEl.value, { y: 0, duration: 0.7, ease: 'elastic.out(1, 0.4)' })
  },
  { minMs: 12000, maxMs: 20000, onlyWhenActive: () => !isHover.value }
)

let excitedWatcher: ReturnType<typeof setInterval> | null = null
function startExcited() {
  if (excitedWatcher) return
  excitedWatcher = setInterval(() => {
    if (!catEl.value || !isExcited.value) return
    gsap.to(catEl.value, {
      rotation: Math.random() > 0.5 ? 6 : -6,
      duration: 0.18,
      ease: 'sine.inOut',
      yoyo: true,
      repeat: 1
    })
  }, 380)
}
function stopExcited() {
  if (excitedWatcher) {
    clearInterval(excitedWatcher)
    excitedWatcher = null
  }
}

const filterStyle = computed(() => {
  if (isHover.value) return 'saturate(1.15) contrast(1.05) brightness(1.03)'
  return 'saturate(1) contrast(1) brightness(1)'
})

onUnmounted(() => {
  if (excitedTimer) clearTimeout(excitedTimer)
  if (clickResetTimer) clearTimeout(clickResetTimer)
  if (petTimer) clearTimeout(petTimer)
  stopExcited()
  pointerUnwatch?.()
})

function onEnter() {
  isHover.value = true
}
function onLeave() {
  isHover.value = false
  if (catEl.value) {
    gsap.to(catEl.value, { y: 0, scale: 1, duration: 0.5, ease: 'power2.out' })
  }
}
</script>

<template>
  <div ref="root" class="interactive-cat" @click="onClick">
    <div ref="bgEl" class="cat-bg" aria-hidden="true">
      <img src="/cat.png" alt="" />
    </div>
    <CatShadow />
    <div
      ref="catEl"
      class="cat-wrap"
      :class="{ excited: isExcited, hover: isHover }"
      :style="{ filter: filterStyle }"
      @pointerenter="onEnter"
      @pointerleave="onLeave"
      @pointermove="onCatMove"
    >
      <img class="cat-img" src="/cat.png" alt="Cute cat — move your cursor, click to play" draggable="false" />
    </div>
    <ClickParticles v-for="b in Object.values(clickBursts)" :key="b.id" :x="b.x" :y="b.y" />
  </div>
</template>

<style scoped>
.interactive-cat {
  position: relative;
  width: min(520px, 90%);
  aspect-ratio: 1 / 1;
  display: flex;
  align-items: center;
  justify-content: center;
  perspective: 900px;
  cursor: pointer;
  user-select: none;
  -webkit-tap-highlight-color: transparent;
}
.cat-bg,
.cat-wrap {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  transform-style: preserve-3d;
  will-change: transform, filter;
}
.cat-bg {
  z-index: 0;
  filter: blur(18px) brightness(0.55) saturate(0.9);
  transform: scale(1.12);
  opacity: 0.7;
}
.cat-bg img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  mix-blend-mode: multiply;
}
.cat-wrap {
  z-index: 1;
  transition: filter 0.3s ease;
}
.cat-wrap.hover {
  filter: saturate(1.15) contrast(1.05) brightness(1.03);
}
.cat-img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  pointer-events: none;
  mix-blend-mode: multiply;
  filter: drop-shadow(0 18px 24px rgba(89, 70, 45, 0.18));
}
</style>
