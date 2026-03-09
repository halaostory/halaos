<script setup lang="ts">
import { ref } from "vue";
import { Field, Button } from "vant";

defineProps<{
  disabled?: boolean;
}>();

const emit = defineEmits<{
  (e: "send", text: string): void;
}>();

const input = ref("");

function onSend() {
  const text = input.value.trim();
  if (!text) return;
  emit("send", text);
  input.value = "";
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === "Enter" && !e.shiftKey) {
    e.preventDefault();
    onSend();
  }
}
</script>

<template>
  <div class="chat-input-bar">
    <Field
      v-model="input"
      type="textarea"
      rows="1"
      autosize
      :placeholder="disabled ? 'Processing...' : 'Ask AI anything...'"
      :disabled="disabled"
      class="chat-input-field"
      @keydown="onKeydown"
    />
    <Button
      type="primary"
      size="small"
      icon="guide-o"
      round
      :disabled="!input.trim() || disabled"
      @click="onSend"
    />
  </div>
</template>

<style scoped>
.chat-input-bar {
  display: flex;
  align-items: flex-end;
  gap: 8px;
  padding: 8px 12px;
  padding-bottom: calc(8px + env(safe-area-inset-bottom));
  border-top: 1px solid #ebedf0;
  background: var(--card-bg, #fff);
}

.chat-input-field {
  flex: 1;
  padding: 0;
}

:deep(.van-field__body) {
  max-height: 100px;
}
</style>
