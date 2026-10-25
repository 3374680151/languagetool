<script setup lang="ts">
import { consoleUrl, loginUrl, registerUrl } from '../lib/links'
</script>

<template>
  <nav>
    <a class="brand" href="#"><i></i>api.aiinfinite.online</a>
    <div class="links">
      <a href="#">About</a>
      <a :href="consoleUrl()" target="_blank" rel="noopener">Docs</a>
      <a class="login" :href="loginUrl()" target="_top">登录</a>
      <a class="contact" :href="registerUrl()" target="_top">注册</a>
    </div>
  </nav>
</template>

<style scoped>
nav {
  position: relative;
  z-index: 2;
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 12px;
}
.brand {
  text-decoration: none;
  display: inline-flex;
  align-items: center;
  gap: 14px;
  font: italic 400 clamp(20px, 2vw, 28px) var(--font-serif);
  letter-spacing: 0.01em;
  color: var(--brand-darker);
}
.brand i {
  width: clamp(13px, 1.2vw, 17px);
  height: clamp(13px, 1.2vw, 17px);
  background: var(--brand-1);
  transform: rotate(45deg);
  flex: none;
}
.links {
  display: flex;
  gap: clamp(18px, 4vw, 55px);
  align-items: center;
  letter-spacing: 0.18em;
  font-weight: 600;
}
.links a {
  color: var(--brand-darker);
  text-decoration: none;
  transition: opacity 0.2s;
  text-transform: uppercase;
  font-size: 11px;
}
.links a:hover {
  opacity: 0.7;
}
.contact {
  border: 1px solid var(--brand-deep);
  color: var(--brand-deep);
  border-radius: 30px;
  padding: 13px 25px;
}
</style>
