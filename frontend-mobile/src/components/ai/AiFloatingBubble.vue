<script setup lang="ts">
import { useRouter } from "vue-router";

const router = useRouter();

function openChat() {
  router.push({ name: "ai-chat" });
}
</script>

<template>
  <Teleport to="body">
    <div class="ai-fab" @click="openChat">
      <van-icon name="chat-o" size="24" color="#fff" />
    </div>
  </Teleport>
</template>

<style scoped>
.ai-fab {
  position: fixed;
  right: 16px;
  bottom: 80px;
  width: 52px;
  height: 52px;
  border-radius: 50%;
  background: var(--brand-color, #1989fa);
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 4px 12px rgba(25, 137, 250, 0.4);
  z-index: 999;
  cursor: pointer;
  -webkit-tap-highlight-color: transparent;
  transition: transform 0.2s;
}

.ai-fab:active {
  transform: scale(0.92);
}
</style>
