<script setup lang="ts">
import { ref, computed } from "vue";
import AiConfirmCard from "./AiConfirmCard.vue";
import type { ChatMessage } from "../../types";
import { aiAPI } from "../../api/client";

const props = defineProps<{
  message: ChatMessage;
}>();

const emit = defineEmits<{
  (e: "confirm"): void;
  (e: "reject"): void;
}>();

const isUser = computed(() => props.message.role === "user");
const feedbackGiven = ref<"positive" | "negative" | null>(null);

// Simple markdown: bold, italic, code blocks, tool indicators
const rendered = computed(() => {
  let text = props.message.content;
  // Code blocks
  text = text.replace(
    /```(\w*)\n([\s\S]*?)```/g,
    '<pre class="code-block"><code>$2</code></pre>',
  );
  // Inline code
  text = text.replace(/`([^`]+)`/g, "<code>$1</code>");
  // Bold
  text = text.replace(/\*\*(.+?)\*\*/g, "<strong>$1</strong>");
  // Italic (but not tool indicators)
  text = text.replace(
    /(?<!\>)\*(?!\*)(.+?)(?<!\*)\*(?!\*)/g,
    "<em>$1</em>",
  );
  // Blockquote (tool indicators)
  text = text.replace(
    /^> (.+)$/gm,
    '<div class="tool-indicator">$1</div>',
  );
  // Line breaks
  text = text.replace(/\n/g, "<br>");
  return text;
});

const showFeedback = computed(
  () => !isUser.value && props.message.id && props.message.content,
);

async function giveFeedback(rating: "positive" | "negative") {
  if (!props.message.id || feedbackGiven.value) return;
  feedbackGiven.value = rating;
  try {
    await aiAPI.submitFeedback(props.message.id, rating);
  } catch {
    feedbackGiven.value = null;
  }
}
</script>

<template>
  <div class="message-bubble" :class="{ user: isUser, assistant: !isUser }">
    <div class="bubble-content" v-html="rendered" />
    <AiConfirmCard
      v-if="message.draft"
      :draft="message.draft"
      @confirm="emit('confirm')"
      @reject="emit('reject')"
    />
    <div v-if="showFeedback" class="feedback-row">
      <button
        class="feedback-btn"
        :class="{ active: feedbackGiven === 'positive' }"
        @click="giveFeedback('positive')"
      >
        <van-icon name="good-job-o" size="14" />
      </button>
      <button
        class="feedback-btn"
        :class="{ active: feedbackGiven === 'negative' }"
        @click="giveFeedback('negative')"
      >
        <van-icon name="good-job-o" size="14" style="transform: scaleY(-1)" />
      </button>
    </div>
  </div>
</template>

<style scoped>
.message-bubble {
  display: flex;
  flex-direction: column;
  margin: 8px 12px;
}

.message-bubble.user {
  align-items: flex-end;
}

.message-bubble.assistant {
  align-items: flex-start;
}

.bubble-content {
  max-width: 80%;
  padding: 10px 14px;
  border-radius: 12px;
  font-size: 14px;
  line-height: 1.5;
  word-break: break-word;
}

.user .bubble-content {
  background: var(--brand-color);
  color: #fff;
  border-bottom-right-radius: 4px;
}

.assistant .bubble-content {
  background: var(--card-bg, #fff);
  color: var(--text-primary);
  border-bottom-left-radius: 4px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.06);
}

.feedback-row {
  display: flex;
  gap: 4px;
  margin-top: 4px;
}

.feedback-btn {
  background: none;
  border: none;
  padding: 2px 6px;
  cursor: pointer;
  color: var(--text-secondary);
  opacity: 0.5;
  transition: opacity 0.2s, color 0.2s;
  -webkit-tap-highlight-color: transparent;
}

.feedback-btn:active,
.feedback-btn.active {
  opacity: 1;
  color: var(--brand-color);
}

:deep(.code-block) {
  background: #f5f5f5;
  border-radius: 6px;
  padding: 8px;
  overflow-x: auto;
  font-size: 12px;
  margin: 4px 0;
}

:deep(code) {
  background: #f0f0f0;
  padding: 1px 4px;
  border-radius: 3px;
  font-size: 13px;
}

:deep(.tool-indicator) {
  font-size: 12px;
  color: var(--text-secondary);
  padding: 4px 8px;
  border-left: 2px solid var(--brand-color);
  margin: 4px 0;
}
</style>
