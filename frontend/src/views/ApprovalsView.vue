<script setup lang="ts">
import { ref, h, onMounted } from "vue";
import { useI18n } from "vue-i18n";
import {
  NDataTable,
  NButton,
  NSpace,
  NTag,
  NEmpty,
  NModal,
  NInput,
  useMessage,
  useDialog,
  type DataTableColumns,
} from "naive-ui";
import { approvalAPI } from "../api/client";
import { format } from "date-fns";

const { t } = useI18n();
const message = useMessage();
const dialog = useDialog();
const data = ref<Record<string, unknown>[]>([]);
const loading = ref(false);
const showRejectModal = ref(false);
const rejectReason = ref("");
const rejectingId = ref<number | null>(null);

function fmtDate(d: unknown): string {
  if (!d) return "-";
  try {
    return format(new Date(d as string), "yyyy-MM-dd HH:mm");
  } catch {
    return String(d);
  }
}

const entityTypeMap: Record<string, string> = {
  leave_request: "approval.leaveRequest",
  overtime_request: "approval.overtimeRequest",
  payroll_cycle: "approval.payrollCycle",
};

const columns: DataTableColumns = [
  {
    title: t("approval.entityType"),
    key: "entity_type",
    width: 150,
    render: (row) => {
      const key = entityTypeMap[row.entity_type as string];
      return h(NTag, { size: "small" }, () =>
        key ? t(key) : row.entity_type
      );
    },
  },
  { title: t("approval.entityId"), key: "entity_id", width: 100 },
  { title: t("approval.step"), key: "step", width: 80 },
  {
    title: t("approval.created"),
    key: "created_at",
    width: 160,
    render: (row) => fmtDate(row.created_at),
  },
  {
    title: t("common.actions"),
    key: "actions",
    width: 200,
    render: (row) => {
      return h(NSpace, { size: "small" }, () => [
        h(
          NButton,
          {
            type: "primary",
            size: "small",
            onClick: () => handleApprove(row),
          },
          () => t("common.approve")
        ),
        h(
          NButton,
          { type: "error", size: "small", onClick: () => handleReject(row) },
          () => t("common.reject")
        ),
      ]);
    },
  },
];

onMounted(async () => {
  loading.value = true;
  try {
    const res = (await approvalAPI.listPending()) as {
      success: boolean;
      data: Record<string, unknown>[];
    };
    data.value = (res.data ||
      (Array.isArray(res) ? res : [])) as Record<string, unknown>[];
  } catch {
    data.value = [];
  } finally {
    loading.value = false;
  }
});

function handleApprove(row: Record<string, unknown>) {
  dialog.info({
    title: t("common.approve"),
    content: t("approval.approveConfirm"),
    positiveText: t("common.approve"),
    negativeText: t("common.cancel"),
    onPositiveClick: async () => {
      try {
        await approvalAPI.approve(row.id as number);
        message.success(t("approval.approvedSuccess"));
        data.value = data.value.filter((d) => d.id !== row.id);
      } catch {
        message.error(t("approval.approveFailed"));
      }
    },
  });
}

function handleReject(row: Record<string, unknown>) {
  rejectingId.value = row.id as number;
  rejectReason.value = "";
  showRejectModal.value = true;
}

async function confirmReject() {
  if (rejectingId.value === null) return;
  try {
    await approvalAPI.reject(rejectingId.value, {
      comments: rejectReason.value || undefined,
    });
    message.success(t("approval.rejectedSuccess"));
    data.value = data.value.filter((d) => d.id !== rejectingId.value);
    showRejectModal.value = false;
  } catch {
    message.error(t("approval.rejectFailed"));
  }
}
</script>

<template>
  <div>
    <h2 style="margin-bottom: 16px">{{ t("approval.title") }}</h2>
    <NDataTable :columns="columns" :data="data" :loading="loading" />
    <NEmpty
      v-if="!loading && data.length === 0"
      :description="t('approval.noPending')"
      style="margin-top: 24px"
    />
    <NModal v-model:show="showRejectModal" :title="t('common.reject')" preset="card" style="max-width: 420px; width: 95vw;">
      <p style="margin-bottom: 12px;">{{ t('approval.rejectConfirm') }}</p>
      <NInput v-model:value="rejectReason" type="textarea" :placeholder="t('approval.rejectReason')" :rows="3" />
      <NSpace style="margin-top: 16px;">
        <NButton type="error" @click="confirmReject">{{ t('common.reject') }}</NButton>
        <NButton @click="showRejectModal = false">{{ t('common.cancel') }}</NButton>
      </NSpace>
    </NModal>
  </div>
</template>
