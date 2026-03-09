<script setup lang="ts">
import { ref, onMounted, h } from "vue";
import { useI18n } from "vue-i18n";
import {
  NCard,
  NSpace,
  NButton,
  NTag,
  NDataTable,
  NSelect,
  NDatePicker,
  NSpin,
  NPagination,
  useMessage,
  type DataTableColumns,
} from "naive-ui";
import { integrationAPI } from "../api/client";

const { t } = useI18n();
const message = useMessage();
const loading = ref(true);

interface Job {
  id: string;
  provider: string;
  employee_name: string;
  action_type: string;
  status: string;
  scheduled_at: string;
  completed_at: string | null;
  error: string | null;
}

const jobs = ref<Job[]>([]);
const page = ref(1);
const pageSize = ref(20);
const total = ref(0);

// Filters
const statusFilter = ref<string | null>(null);
const providerFilter = ref<string | null>(null);
const dateRange = ref<[number, number] | null>(null);

const statusOptions = [
  { label: t("integration.allStatuses"), value: "" },
  { label: t("common.pending"), value: "pending" },
  { label: t("integration.running"), value: "running" },
  { label: t("integration.completed"), value: "completed" },
  { label: t("integration.failed"), value: "failed" },
  { label: t("integration.requiresApprovalStatus"), value: "requires_approval" },
];

const providerOptions = [
  { label: t("integration.allProviders"), value: "" },
  { label: "Slack", value: "slack" },
  { label: "Google", value: "google" },
  { label: "GitHub", value: "github" },
  { label: "Telegram", value: "telegram" },
];

const statusMap: Record<string, "success" | "warning" | "error" | "info" | "default"> = {
  pending: "default",
  running: "info",
  completed: "success",
  failed: "error",
  requires_approval: "warning",
};

const columns: DataTableColumns<Job> = [
  {
    title: "ID",
    key: "id",
    width: 100,
    ellipsis: { tooltip: true },
    render: (row) => row.id.slice(0, 8) + "...",
  },
  {
    title: () => t("integration.provider"),
    key: "provider",
    width: 100,
    render: (row) => row.provider.charAt(0).toUpperCase() + row.provider.slice(1),
  },
  {
    title: () => t("integration.employee"),
    key: "employee_name",
    ellipsis: { tooltip: true },
  },
  {
    title: () => t("integration.actionType"),
    key: "action_type",
    width: 140,
    render: (row) => row.action_type.replace(/_/g, " "),
  },
  {
    title: () => t("common.status"),
    key: "status",
    width: 140,
    render: (row) =>
      h(NTag, { type: statusMap[row.status] || "default", size: "small" }, { default: () => row.status.replace(/_/g, " ") }),
  },
  {
    title: () => t("integration.scheduledAt"),
    key: "scheduled_at",
    width: 160,
    render: (row) => new Date(row.scheduled_at).toLocaleString(),
  },
  {
    title: () => t("integration.completedAt"),
    key: "completed_at",
    width: 160,
    render: (row) => (row.completed_at ? new Date(row.completed_at).toLocaleString() : "-"),
  },
  {
    title: () => t("integration.error"),
    key: "error",
    ellipsis: { tooltip: true },
    render: (row) => row.error || "-",
  },
  {
    title: () => t("common.actions"),
    key: "actions",
    width: 140,
    render: (row) =>
      h(NSpace, { size: 4 }, {
        default: () => [
          row.status === "failed"
            ? h(NButton, { size: "small", onClick: () => retryJob(row.id) }, { default: () => t("integration.retry") })
            : null,
          row.status === "pending" || row.status === "requires_approval"
            ? h(NButton, { size: "small", onClick: () => skipJob(row.id) }, { default: () => t("integration.skip") })
            : null,
        ].filter(Boolean),
      }),
  },
];

async function loadJobs() {
  loading.value = true;
  try {
    const params: Record<string, string> = {
      page: String(page.value),
      limit: String(pageSize.value),
    };
    if (statusFilter.value) params.status = statusFilter.value;
    if (providerFilter.value) params.provider = providerFilter.value;
    if (dateRange.value) {
      params.start = new Date(dateRange.value[0]).toISOString().split("T")[0];
      params.end = new Date(dateRange.value[1]).toISOString().split("T")[0];
    }
    const res = (await integrationAPI.listJobs(params)) as {
      data?: Job[];
      meta?: { total?: number };
    };
    jobs.value = res.data || (Array.isArray(res) ? (res as Job[]) : []);
    total.value = res.meta?.total || jobs.value.length;
  } catch {
    jobs.value = [];
  } finally {
    loading.value = false;
  }
}

async function retryJob(id: string) {
  try {
    await integrationAPI.retryJob(id);
    message.success(t("integration.jobRetried"));
    loadJobs();
  } catch {
    message.error(t("common.failed"));
  }
}

async function skipJob(id: string) {
  try {
    await integrationAPI.skipJob(id);
    message.success(t("integration.jobSkipped"));
    loadJobs();
  } catch {
    message.error(t("common.failed"));
  }
}

function handlePageChange(p: number) {
  page.value = p;
  loadJobs();
}

onMounted(loadJobs);
</script>

<template>
  <NSpin :show="loading">
    <NSpace vertical :size="16">
      <h2>{{ t("integration.jobsTitle") }}</h2>

      <!-- Filters -->
      <NCard>
        <NSpace :size="12" align="center">
          <NSelect
            v-model:value="statusFilter"
            :options="statusOptions"
            :placeholder="t('common.status')"
            style="width: 160px;"
            clearable
            @update:value="loadJobs"
          />
          <NSelect
            v-model:value="providerFilter"
            :options="providerOptions"
            :placeholder="t('integration.provider')"
            style="width: 160px;"
            clearable
            @update:value="loadJobs"
          />
          <NDatePicker
            v-model:value="dateRange"
            type="daterange"
            clearable
            @update:value="loadJobs"
          />
          <NButton @click="loadJobs">{{ t("common.filter") }}</NButton>
        </NSpace>
      </NCard>

      <!-- Jobs Table -->
      <NCard>
        <NDataTable :columns="columns" :data="jobs" :row-key="(row: Job) => row.id" size="small" />
        <div style="display: flex; justify-content: flex-end; margin-top: 12px;" v-if="total > pageSize">
          <NPagination :page="page" :page-size="pageSize" :item-count="total" @update:page="handlePageChange" />
        </div>
      </NCard>
    </NSpace>
  </NSpin>
</template>
