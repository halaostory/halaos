<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

const scrolled = ref(false)
const mobileOpen = ref(false)

function onScroll() {
  scrolled.value = window.scrollY > 10
}

onMounted(() => window.addEventListener('scroll', onScroll))
onUnmounted(() => window.removeEventListener('scroll', onScroll))
</script>

<template>
  <header class="pub-nav" :class="{ scrolled }">
    <div class="pub-nav-inner">
      <router-link to="/" class="pub-logo">
        <span class="logo-icon">H</span>
        <span class="logo-text">HalaOS</span>
      </router-link>

      <nav class="pub-links" :class="{ open: mobileOpen }">
        <router-link to="/features" @click="mobileOpen = false">Features</router-link>
        <router-link to="/pricing" @click="mobileOpen = false">Pricing</router-link>
        <router-link to="/tools" @click="mobileOpen = false">Free Tools</router-link>
        <router-link to="/blog" @click="mobileOpen = false">Blog</router-link>
        <router-link to="/contact" @click="mobileOpen = false">Contact</router-link>
      </nav>

      <div class="pub-actions">
        <router-link to="/login" class="pub-btn-text">Log In</router-link>
        <router-link to="/register" class="pub-btn-primary">Get Started</router-link>
      </div>

      <button class="hamburger" @click="mobileOpen = !mobileOpen" aria-label="Menu">
        <span></span><span></span><span></span>
      </button>
    </div>
  </header>
</template>

<style scoped>
.pub-nav {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  z-index: 1000;
  background: rgba(255, 255, 255, 0.85);
  backdrop-filter: blur(12px);
  border-bottom: 1px solid transparent;
  transition: all 0.3s;
}
.pub-nav.scrolled {
  border-bottom-color: #e2e8f0;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.06);
}
.pub-nav-inner {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 24px;
  height: 64px;
  display: flex;
  align-items: center;
  gap: 32px;
}
.pub-logo {
  display: flex;
  align-items: center;
  gap: 8px;
  text-decoration: none;
  font-weight: 700;
  font-size: 20px;
  color: #0f172a;
}
.logo-icon {
  width: 32px;
  height: 32px;
  background: linear-gradient(135deg, #4f46e5, #7c3aed);
  color: #fff;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
  font-weight: 800;
}
.logo-text { letter-spacing: -0.5px; }
.pub-links {
  display: flex;
  gap: 28px;
  flex: 1;
}
.pub-links a {
  text-decoration: none;
  color: #475569;
  font-size: 15px;
  font-weight: 500;
  transition: color 0.2s;
}
.pub-links a:hover, .pub-links a.router-link-active { color: #4f46e5; }
.pub-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}
.pub-btn-text {
  text-decoration: none;
  color: #475569;
  font-weight: 500;
  font-size: 15px;
  padding: 8px 16px;
  border-radius: 8px;
  transition: background 0.2s;
}
.pub-btn-text:hover { background: #f1f5f9; }
.pub-btn-primary {
  text-decoration: none;
  background: #4f46e5;
  color: #fff;
  font-weight: 600;
  font-size: 15px;
  padding: 8px 20px;
  border-radius: 8px;
  transition: background 0.2s;
}
.pub-btn-primary:hover { background: #4338ca; }
.hamburger {
  display: none;
  flex-direction: column;
  gap: 5px;
  background: none;
  border: none;
  cursor: pointer;
  padding: 4px;
}
.hamburger span {
  width: 22px;
  height: 2px;
  background: #334155;
  border-radius: 1px;
}

@media (max-width: 768px) {
  .pub-links {
    display: none;
    position: absolute;
    top: 64px;
    left: 0;
    right: 0;
    background: #fff;
    flex-direction: column;
    padding: 16px 24px;
    gap: 16px;
    border-bottom: 1px solid #e2e8f0;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
  }
  .pub-links.open { display: flex; }
  .pub-actions { display: none; }
  .hamburger { display: flex; }
}
</style>
