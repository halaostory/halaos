<script setup lang="ts">
import { Tag } from "vant";

defineProps<{
  suggestions: string[];
}>();

const emit = defineEmits<{
  (e: "select", text: string): void;
}>();
</script>

<template>
  <div class="suggestion-chips" v-if="suggestions.length > 0">
    <Tag
      v-for="s in suggestions"
      :key="s"
      plain
      type="primary"
      size="medium"
      class="chip"
      @click="emit('select', s)"
    >
      {{ s }}
    </Tag>
  </div>
</template>

<style scoped>
.suggestion-chips {
  display: flex;
  gap: 8px;
  overflow-x: auto;
  padding: 8px 12px;
  -webkit-overflow-scrolling: touch;
}

.suggestion-chips::-webkit-scrollbar {
  display: none;
}

.chip {
  flex-shrink: 0;
  cursor: pointer;
  white-space: nowrap;
}
</style>
