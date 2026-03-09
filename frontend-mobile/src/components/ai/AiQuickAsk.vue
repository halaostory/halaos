<script setup lang="ts">
import { useRouter } from "vue-router";
import { Tag } from "vant";

const props = defineProps<{
  questions: string[];
}>();

const router = useRouter();

function askAi(question: string) {
  router.push({ name: "ai-chat", query: { q: question } });
}
</script>

<template>
  <div class="ai-quick-ask" v-if="questions.length > 0">
    <div class="ask-header">
      <van-icon name="chat-o" size="14" color="var(--brand-color)" />
      <span class="ask-label">AI</span>
    </div>
    <div class="ask-chips">
      <Tag
        v-for="q in questions"
        :key="q"
        plain
        round
        type="primary"
        size="medium"
        class="ask-chip"
        @click="askAi(q)"
      >
        {{ q }}
      </Tag>
    </div>
  </div>
</template>

<style scoped>
.ai-quick-ask {
  margin: 8px 16px;
  padding: 8px 12px;
  background: var(--card-bg, #fff);
  border-radius: 8px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.04);
}

.ask-header {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-bottom: 6px;
}

.ask-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--brand-color);
}

.ask-chips {
  display: flex;
  gap: 6px;
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
}

.ask-chips::-webkit-scrollbar {
  display: none;
}

.ask-chip {
  flex-shrink: 0;
  cursor: pointer;
  white-space: nowrap;
}
</style>
