<script setup lang="ts">
const posters = Array.from({ length: 9 }, (_, i) => `/posters/poster-${i + 1}.svg`)
const tiles = [...posters, ...posters.slice(0, 3)]
</script>

<template>
  <div class="poster-wall" aria-hidden="true">
    <div class="wall">
      <div v-for="(src, i) in tiles" :key="i" class="tile">
        <img :src="src" alt="" loading="eager" />
      </div>
    </div>
    <div class="shade"></div>
    <div class="vignette"></div>
  </div>
</template>

<style scoped>
.poster-wall {
  position: fixed;
  inset: 0;
  z-index: 0;
  overflow: hidden;
  pointer-events: none;
  background: #eafaf3;
}

.wall {
  position: absolute;
  inset: -12%;
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 10px;
  transform: rotate(-4deg) scale(1.08);
  will-change: transform;
  backface-visibility: hidden;
  animation: drift 70s ease-in-out infinite alternate;
}
@keyframes drift {
  from {
    transform: rotate(-4deg) scale(1.08) translate(0, 0);
  }
  to {
    transform: rotate(-3deg) scale(1.12) translate(-2%, -1.5%);
  }
}

.tile {
  overflow: hidden;
  border-radius: 6px;
}
.tile img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.shade {
  position: absolute;
  inset: 0;
  background: rgba(234, 250, 243, 0.82);
}

.vignette {
  position: absolute;
  inset: 0;
  background: radial-gradient(ellipse at 50% 42%, transparent 30%, rgba(209, 250, 229, 0.45) 78%, rgba(167, 233, 205, 0.6) 100%);
}

@media (prefers-reduced-motion: reduce) {
  .wall {
    animation: none;
  }
}
</style>
