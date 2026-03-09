<script setup lang="ts">
import { ref } from "vue";
import { useI18n } from "vue-i18n";
import { ActionSheet, Tag, showToast } from "vant";
import { formPrefillAPI } from "../../api/client";
import type { ApiResponse } from "../../types";

const props = defineProps<{
  formType: string;
  leaveType?: string;
  startDate?: string;
  endDate?: string;
}>();

const emit = defineEmits<{
  (e: "select", reason: string): void;
}>();

const { t } = useI18n();

const showSheet = ref(false);
const suggestions = ref<string[]>([]);
const loading = ref(false);

async function loadSuggestions() {
  if (suggestions.value.length > 0) {
    showSheet.value = true;
    return;
  }
  loading.value = true;
  try {
    const res = (await formPrefillAPI.get(props.formType)) as ApiResponse<{
      reason_suggestions?: string[];
      reason_hint?: string;
    }>;
    const data = res.data ?? (res as unknown as typeof res.data);
    if (data?.reason_suggestions && data.reason_suggestions.length > 0) {
      suggestions.value = data.reason_suggestions;
    } else if (data?.reason_hint) {
      suggestions.value = [data.reason_hint];
    }
    if (suggestions.value.length > 0) {
      showSheet.value = true;
    } else {
      showToast(t("common.noData"));
    }
  } catch {
    showToast(t("common.failed"));
  } finally {
    loading.value = false;
  }
}

function onSelect(item: { name: string }) {
  emit("select", item.name);
  showSheet.value = false;
}

const actions = ref<Array<{ name: string }>>([]);

function openSheet() {
  actions.value = suggestions.value.map((s) => ({ name: s }));
  loadSuggestions();
}
</script>

<template>
  <div class="form-assist">
    <Tag
      plain
      round
      type="primary"
      size="medium"
      class="assist-tag"
      @click="openSheet"
    >
      <van-icon name="bulb-o" size="12" />
      <span>{{ t("ai.suggestReason") }}</span>
    </Tag>

    <ActionSheet
      v-model:show="showSheet"
      :actions="actions"
      :title="t('ai.reasonSuggestions')"
      :cancel-text="t('common.cancel')"
      :loading="loading"
      @select="onSelect"
      close-on-click-action
    />
  </div>
</template>

<style scoped>
.form-assist {
  display: inline-flex;
}

.assist-tag {
  cursor: pointer;
  white-space: nowrap;
}
</style>
