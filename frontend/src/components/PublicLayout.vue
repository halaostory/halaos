<script setup lang="ts">
import PublicNavbar from './PublicNavbar.vue'
import PublicFooter from './PublicFooter.vue'
</script>

<template>
  <div class="pub-layout">
    <PublicNavbar />
    <main class="pub-main">
      <router-view />
    </main>
    <PublicFooter />
  </div>
</template>

<style scoped>
.pub-layout {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}
.pub-main {
  flex: 1;
  padding-top: 64px; /* navbar height */
}
</style>
