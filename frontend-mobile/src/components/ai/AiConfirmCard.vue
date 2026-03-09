<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { Button, Tag } from "vant";
import type { DraftConfirmation } from "../../types";

const props = defineProps<{
  draft: DraftConfirmation;
}>();

const emit = defineEmits<{
  (e: "confirm"): void;
  (e: "reject"): void;
}>();

const { t } = useI18n();

const isPending = computed(() => props.draft.status === "pending");
const isConfirmed = computed(() => props.draft.status === "confirmed");
const isRejected = computed(() => props.draft.status === "rejected");

const riskColor = computed(() => {
  const map: Record<string, string> = {
    low: "#52c41a",
    medium: "#faad14",
    high: "#ff4d4f",
  };
  return map[props.draft.risk_level] || "#999";
});

const riskType = computed(() => {
  const map: Record<string, "success" | "warning" | "danger"> = {
    low: "success",
    medium: "warning",
    high: "danger",
  };
  return map[props.draft.risk_level] || "warning";
});

const statusText = computed(() => {
  if (isConfirmed.value) return t("ai.confirmed");
  if (isRejected.value) return t("ai.rejected");
  return "";
});
</script>

<template>
  <div class="confirm-card" :style="{ borderLeftColor: riskColor }">
    <div class="card-header">
      <Tag :type="riskType" plain size="medium">
        {{ draft.risk_level.toUpperCase() }}
      </Tag>
      <span class="tool-name">{{ draft.tool_name }}</span>
    </div>

    <div class="card-description">{{ draft.description }}</div>

    <div v-if="isPending" class="card-actions">
      <Button
        type="danger"
        plain
        size="small"
        @click="emit('reject')"
      >
        {{ t("common.cancel") }}
      </Button>
      <Button
        type="primary"
        size="small"
        @click="emit('confirm')"
      >
        {{ t("common.confirm") }}
      </Button>
    </div>

    <div v-else class="card-status">
      <Tag :type="isConfirmed ? 'success' : 'default'" plain>
        {{ statusText }}
      </Tag>
    </div>
  </div>
</template>

<style scoped>
.confirm-card {
  background: var(--card-bg, #fff);
  border-left: 3px solid;
  border-radius: 8px;
  padding: 10px 12px;
  margin: 6px 0;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.08);
}

.card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}

.tool-name {
  font-size: 12px;
  color: var(--text-secondary);
}

.card-description {
  font-size: 13px;
  line-height: 1.5;
  color: var(--text-primary);
  margin-bottom: 10px;
}

.card-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}

.card-status {
  display: flex;
  justify-content: flex-end;
}
</style>
