<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

const props = defineProps<{ x: number; y: number }>()

const items = Array.from({ length: 6 }, (_, i) => ({
  id: i,
  angle: (i / 6) * Math.PI * 2 + Math.random() * 0.6,
  distance: 60 + Math.random() * 50,
  emoji: ['💛', '✨', '🐾', '💛', '✨', '🐾'][i],
  size: 14 + Math.random() * 10
}))

const root = ref<HTMLDivElement | null>(null)

onMounted(() => {
  // Force reflow then animate via CSS
  if (root.value) {
    root.value.classList.add('go')
  }
})
</script>

<template>
  <div ref="root" class="click-particles" :style="{ left: `${x}px`, top: `${y}px` }">
    <span
      v-for="it in items"
      :key="it.id"
      :style="{
        '--tx': `${Math.cos(it.angle) * it.distance}px`,
        '--ty': `${Math.sin(it.angle) * it.distance}px`,
        fontSize: `${it.size}px`
      }"
      class="particle"
    >{{ it.emoji }}</span>
  </div>
</template>

<style scoped>
.click-particles {
  position: absolute;
  pointer-events: none;
  z-index: 5;
  transform: translate(-50%, -50%);
}
.particle {
  position: absolute;
  left: 0;
  top: 0;
  display: inline-block;
  opacity: 0;
  will-change: transform, opacity;
}
.click-particles.go .particle {
  animation: fly 1.1s cubic-bezier(0.2, 0.7, 0.3, 1) forwards;
}
@keyframes fly {
  0% {
    opacity: 0;
    transform: translate(0, 0) scale(0.4) rotate(0);
  }
  20% {
    opacity: 1;
    transform: translate(calc(var(--tx) * 0.4), calc(var(--ty) * 0.4)) scale(1.2) rotate(20deg);
  }
  100% {
    opacity: 0;
    transform: translate(var(--tx), var(--ty)) scale(0.6) rotate(80deg);
  }
}
</style>
