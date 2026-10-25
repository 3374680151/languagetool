<script setup lang="ts">
import NavBar from './components/NavBar.vue'
import HeroCard from './components/HeroCard.vue'
import PosterWall from './components/PosterWall.vue'
import BackgroundMarquee from './components/BackgroundMarquee.vue'
</script>

<template>
  <div class="shell">
    <PosterWall />
    <BackgroundMarquee />
    <NavBar />
    <HeroCard />
  </div>
</template>

<style scoped>
.shell {
  position: relative;
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  padding: 28px clamp(20px, 5vw, 76px) 48px;
  background: linear-gradient(135deg, #eafaf3, #dcf5ea 60%, #cfeede);
  overflow: hidden;
}
</style>
