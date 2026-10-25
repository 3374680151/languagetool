<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { usePointer } from '../composables/usePointer'
import { gsap } from 'gsap'

const shadow = ref<HTMLDivElement | null>(null)
const pointer = usePointer()
let raf = 0
let lastX = 2

function tick() {
  const x = pointer.value.x
  if (shadow.value && Math.abs(x - lastX) > 0.002) {
    lastX = x
    const offsetX = x * 12
    const scaleX = 1 + Math.abs(x) * 0.08
    shadow.value.style.transform = `translate(${offsetX}px, 0) scaleX(${scaleX}) scaleY(${
      1 - Math.abs(x) * 0.04
    })`
  }
  raf = requestAnimationFrame(tick)
}

onMounted(() => {
  raf = requestAnimationFrame(tick)
  if (shadow.value) {
    gsap.to(shadow.value, { opacity: 0.45, duration: 1.5, ease: 'sine.inOut', yoyo: true, repeat: -1 })
  }
})
onUnmounted(() => cancelAnimationFrame(raf))
</script>

<template>
  <div ref="shadow" class="cat-shadow" aria-hidden="true" />
</template>

<style scoped>
.cat-shadow {
  position: absolute;
  bottom: 6%;
  left: 50%;
  width: 70%;
  height: 28px;
  border-radius: 50%;
  background: radial-gradient(ellipse at center, rgba(89, 70, 45, 0.55), rgba(89, 70, 45, 0) 70%);
  transform: translate(-50%, 0);
  filter: blur(8px);
  z-index: 0;
  pointer-events: none;
  opacity: 0.35;
}
</style>
