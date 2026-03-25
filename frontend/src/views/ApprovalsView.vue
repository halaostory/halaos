<script setup lang="ts">
import { ref, h, onMounted } from "vue";
import { useI18n } from "vue-i18n";
import {
  NDataTable,
  NButton,
  NSpace,
  NTag,
  NModal,
  NInput,
  NDrawer,
  NDrawerContent,
  NDescriptions,
  NDescriptionsItem,
  NSpin,
  NAlert,
  NCollapse,
  NCollapseItem,
  useMessage,
  useDialog,
  type DataTableColumns,
} from "naive-ui";
import { approvalAPI } from "../api/client";
import EmptyState from '../components/EmptyState.vue'
import { format } from "date-fns";

const { t } = useI18n();
const message = useMessage();
const dialog = useDialog();
const data = ref<Record<string, unknown>[]>([]);
const loading = ref(false);
const showRejectModal = ref(false);
const rejectReason = ref("");
const rejectingId = ref<number | null>(null);

// Context drawer
const showContextDrawer = ref(false);
const contextLoading = ref(false);
const contextData = ref<Record<string, unknown> | null>(null);
const contextError = ref("");

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
    width: 280,
    render: (row) => {
      return h(NSpace, { size: "small" }, () => [
        h(
          NButton,
          {
            size: "small",
            onClick: () => handleShowContext(row),
          },
          () => t("approval.viewContext")
        ),
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

async function handleShowContext(row: Record<string, unknown>) {
  const entityType = row.entity_type as string;
  const entityId = row.entity_id as number;

  showContextDrawer.value = true;
  contextLoading.value = true;
  contextError.value = "";
  contextData.value = null;

  try {
    const res = (await approvalAPI.getContext(entityType, entityId)) as any;
    contextData.value = res?.data ?? res;
  } catch {
    contextError.value = t("common.loadFailed");
  } finally {
    contextLoading.value = false;
  }
}

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

function recommendationTag(rec: string | undefined) {
  if (!rec) return null;
  const upper = rec.toUpperCase();
  if (upper.startsWith("APPROVE")) return { type: "success" as const, label: "Approve" };
  if (upper.startsWith("CAUTION") || upper.startsWith("REVIEW")) return { type: "warning" as const, label: "Caution" };
  if (upper.startsWith("REJECT")) return { type: "error" as const, label: "Reject" };
  return { type: "info" as const, label: "Info" };
}
</script>

<template>
  <div>
    <h2 style="margin-bottom: 16px">{{ t("approval.title") }}</h2>
    <NDataTable :columns="columns" :data="data" :loading="loading" />
    <EmptyState
      v-if="!loading && data.length === 0"
      icon="✅"
      :title="t('emptyState.approvals.title')"
      :description="t('emptyState.approvals.desc')"
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

    <NDrawer v-model:show="showContextDrawer" width="480" placement="right">
      <NDrawerContent :title="t('approval.approvalContext')">
        <NSpin v-if="contextLoading" />
        <NAlert v-else-if="contextError" type="error" :title="contextError" />
        <template v-else-if="contextData">
          <!-- AI Recommendation -->
          <template v-if="contextData.recommendation">
            <NAlert
              :type="recommendationTag(contextData.recommendation as string)?.type || 'info'"
              style="margin-bottom: 16px"
            >
              <template #header>
                <NTag
                  :type="recommendationTag(contextData.recommendation as string)?.type || 'info'"
                  size="small"
                  style="margin-right: 8px"
                >
                  {{ recommendationTag(contextData.recommendation as string)?.label }}
                </NTag>
                AI Recommendation
              </template>
              {{ contextData.recommendation }}
            </NAlert>
          </template>

          <!-- Request Info -->
          <NCollapse :default-expanded-names="['request', 'employee', 'balance']">
            <NCollapseItem :title="t('approval.requestInfo')" name="request">
              <NDescriptions :column="1" label-placement="left" bordered size="small">
                <NDescriptionsItem :label="t('approval.entityType')">
                  {{ (contextData.request_info as any)?.entity_type }}
                </NDescriptionsItem>
                <NDescriptionsItem v-if="(contextData.request_info as any)?.leave_type_name" :label="t('approval.leaveType')">
                  {{ (contextData.request_info as any)?.leave_type_name }}
                </NDescriptionsItem>
                <NDescriptionsItem v-if="(contextData.request_info as any)?.start_date" :label="t('approval.dates')">
                  {{ (contextData.request_info as any)?.start_date }} ~ {{ (contextData.request_info as any)?.end_date }}
                </NDescriptionsItem>
                <NDescriptionsItem v-if="(contextData.request_info as any)?.days" :label="t('approval.days')">
                  {{ (contextData.request_info as any)?.days }}
                </NDescriptionsItem>
                <NDescriptionsItem v-if="(contextData.request_info as any)?.hours" :label="t('approval.hours')">
                  {{ (contextData.request_info as any)?.hours }}
                </NDescriptionsItem>
                <NDescriptionsItem v-if="(contextData.request_info as any)?.reason" :label="t('approval.reason')">
                  {{ (contextData.request_info as any)?.reason }}
                </NDescriptionsItem>
              </NDescriptions>
            </NCollapseItem>

            <NCollapseItem :title="t('approval.employeeInfo')" name="employee">
              <NDescriptions :column="1" label-placement="left" bordered size="small">
                <NDescriptionsItem :label="t('common.name')">
                  {{ (contextData.employee_info as any)?.name }}
                </NDescriptionsItem>
                <NDescriptionsItem v-if="(contextData.employee_info as any)?.department" :label="t('employee.department')">
                  {{ (contextData.employee_info as any)?.department }}
                </NDescriptionsItem>
                <NDescriptionsItem :label="t('approval.tenure')">
                  {{ (contextData.employee_info as any)?.tenure }}
                </NDescriptionsItem>
                <NDescriptionsItem :label="t('employee.hireDate')">
                  {{ (contextData.employee_info as any)?.hire_date }}
                </NDescriptionsItem>
              </NDescriptions>
            </NCollapseItem>

            <NCollapseItem v-if="(contextData.balance_impact as any[])?.length" :title="t('approval.balanceImpact')" name="balance">
              <NDataTable
                :data="contextData.balance_impact as any[]"
                :columns="[
                  { title: t('approval.leaveType'), key: 'leave_type' },
                  { title: t('approval.earned'), key: 'earned', width: 80 },
                  { title: t('approval.used'), key: 'used', width: 80 },
                  { title: t('approval.remaining'), key: 'remaining', width: 90 },
                ]"
                :bordered="true"
                size="small"
              />
            </NCollapseItem>

            <NCollapseItem v-if="(contextData.team_conflicts as any[])?.length" :title="t('approval.teamConflicts')">
              <NDataTable
                :data="contextData.team_conflicts as any[]"
                :columns="[
                  { title: t('common.name'), key: 'name' },
                  { title: t('approval.leaveType'), key: 'leave_type' },
                  { title: t('approval.dates'), key: 'dates' },
                ]"
                :bordered="true"
                size="small"
              />
            </NCollapseItem>

            <NCollapseItem v-if="(contextData.leave_history as any[])?.length" :title="t('approval.leaveHistory')">
              <NDataTable
                :data="contextData.leave_history as any[]"
                :columns="[
                  { title: t('approval.leaveType'), key: 'leave_type' },
                  { title: t('approval.dates'), key: 'start_date' },
                  { title: t('approval.days'), key: 'days', width: 60 },
                  { title: t('common.status'), key: 'status', width: 90 },
                ]"
                :bordered="true"
                size="small"
              />
            </NCollapseItem>
          </NCollapse>
        </template>
      </NDrawerContent>
    </NDrawer>
  </div>
</template>
