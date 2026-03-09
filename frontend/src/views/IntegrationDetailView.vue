<script setup lang="ts">
import { ref, onMounted, h } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import {
  NCard,
  NSpace,
  NButton,
  NTag,
  NDataTable,
  NDescriptions,
  NDescriptionsItem,
  NModal,
  NForm,
  NFormItem,
  NInput,
  NSelect,
  NSwitch,
  NSpin,
  NEmpty,
  useMessage,
  type DataTableColumns,
} from "naive-ui";
import { integrationAPI } from "../api/client";

const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const message = useMessage();
const loading = ref(true);

interface Connection {
  id: string;
  provider: string;
  display_name: string;
  status: string;
  auth_type: string;
  last_used_at: string | null;
  error_count: number;
  created_at: string;
}

interface Template {
  id: string;
  connection_id: string;
  event_trigger: string;
  action_type: string;
  department_filter: string;
  employment_type_filter: string;
  requires_approval: boolean;
  is_active: boolean;
}

interface Job {
  id: string;
  employee_name: string;
  action_type: string;
  status: string;
  scheduled_at: string;
  completed_at: string | null;
  error: string | null;
}

const connection = ref<Connection | null>(null);
const templates = ref<Template[]>([]);
const jobs = ref<Job[]>([]);

// Template form
const showTemplateModal = ref(false);
const templateLoading = ref(false);
const templateForm = ref({
  event_trigger: "employee_created",
  action_type: "create_account",
  department_filter: "",
  employment_type_filter: "",
  requires_approval: false,
  is_active: true,
});

const eventTriggerOptions = [
  { label: t("integration.triggerCreated"), value: "employee_created" },
  { label: t("integration.triggerActivated"), value: "employee_activated" },
  { label: t("integration.triggerDeactivated"), value: "employee_deactivated" },
  { label: t("integration.triggerDeptChanged"), value: "department_changed" },
];

const actionTypeOptions = [
  { label: t("integration.actionCreate"), value: "create_account" },
  { label: t("integration.actionSuspend"), value: "suspend_account" },
  { label: t("integration.actionDelete"), value: "delete_account" },
  { label: t("integration.actionUpdate"), value: "update_account" },
];

const statusMap: Record<string, "success" | "warning" | "error" | "info" | "default"> = {
  active: "success",
  inactive: "default",
  error: "error",
  pending: "default",
  running: "info",
  completed: "success",
  failed: "error",
  requires_approval: "warning",
};

const templateColumns: DataTableColumns<Template> = [
  {
    title: () => t("integration.eventTrigger"),
    key: "event_trigger",
    render: (row) => row.event_trigger.replace(/_/g, " "),
  },
  {
    title: () => t("integration.actionType"),
    key: "action_type",
    render: (row) => row.action_type.replace(/_/g, " "),
  },
  {
    title: () => t("integration.deptFilter"),
    key: "department_filter",
    render: (row) => row.department_filter || t("common.allEmployees"),
  },
  {
    title: () => t("integration.requiresApproval"),
    key: "requires_approval",
    width: 100,
    render: (row) => (row.requires_approval ? t("common.yes") : t("common.no")),
  },
  {
    title: () => t("common.active"),
    key: "is_active",
    width: 80,
    render: (row) =>
      h(NTag, { type: row.is_active ? "success" : "default", size: "small" }, { default: () => (row.is_active ? t("common.yes") : t("common.no")) }),
  },
  {
    title: () => t("common.actions"),
    key: "actions",
    width: 140,
    render: (row) =>
      h(NSpace, { size: 4 }, {
        default: () => [
          h(NButton, { size: "small", type: "error", onClick: () => deleteTemplate(row.id) }, { default: () => t("common.delete") }),
        ],
      }),
  },
];

const jobColumns: DataTableColumns<Job> = [
  { title: () => t("integration.employee"), key: "employee_name" },
  {
    title: () => t("integration.actionType"),
    key: "action_type",
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

async function loadData() {
  loading.value = true;
  const connId = route.params.id as string;
  try {
    const [connRes, tmplRes, jobsRes] = await Promise.allSettled([
      integrationAPI.getConnection(connId),
      integrationAPI.listTemplates({ connection_id: connId }),
      integrationAPI.listJobs({ connection_id: connId, limit: "20" }),
    ]);
    if (connRes.status === "fulfilled") {
      const res = connRes.value as { data?: Connection };
      connection.value = res.data || (res as unknown as Connection);
    }
    if (tmplRes.status === "fulfilled") {
      const res = tmplRes.value as { data?: Template[] };
      templates.value = res.data || (Array.isArray(res) ? (res as Template[]) : []);
    }
    if (jobsRes.status === "fulfilled") {
      const res = jobsRes.value as { data?: Job[] };
      jobs.value = res.data || (Array.isArray(res) ? (res as Job[]) : []);
    }
  } catch {
    message.error(t("common.loadFailed"));
  } finally {
    loading.value = false;
  }
}

async function testConnection() {
  const connId = route.params.id as string;
  try {
    await integrationAPI.testConnection(connId);
    message.success(t("integration.testSuccess"));
  } catch {
    message.error(t("integration.testFailed"));
  }
}

async function deleteConnectionAction() {
  const connId = route.params.id as string;
  try {
    await integrationAPI.deleteConnection(connId);
    message.success(t("integration.connectionDeleted"));
    router.push({ name: "integrations" });
  } catch {
    message.error(t("common.deleteFailed"));
  }
}

async function addTemplate() {
  templateLoading.value = true;
  const connId = route.params.id as string;
  try {
    await integrationAPI.createTemplate({
      connection_id: connId,
      ...templateForm.value,
    });
    message.success(t("integration.templateCreated"));
    showTemplateModal.value = false;
    loadData();
  } catch {
    message.error(t("common.failed"));
  } finally {
    templateLoading.value = false;
  }
}

async function deleteTemplate(id: string) {
  try {
    await integrationAPI.deleteTemplate(id);
    message.success(t("integration.templateDeleted"));
    loadData();
  } catch {
    message.error(t("common.deleteFailed"));
  }
}

async function retryJob(id: string) {
  try {
    await integrationAPI.retryJob(id);
    message.success(t("integration.jobRetried"));
    loadData();
  } catch {
    message.error(t("common.failed"));
  }
}

async function skipJob(id: string) {
  try {
    await integrationAPI.skipJob(id);
    message.success(t("integration.jobSkipped"));
    loadData();
  } catch {
    message.error(t("common.failed"));
  }
}

onMounted(loadData);
</script>

<template>
  <NSpin :show="loading">
    <NSpace vertical :size="16" v-if="connection">
      <NSpace justify="space-between" align="center">
        <h2>{{ connection.display_name }}</h2>
        <NButton @click="router.push({ name: 'integrations' })">{{ t("common.back") }}</NButton>
      </NSpace>

      <!-- Connection Info -->
      <NCard :title="t('integration.connectionInfo')">
        <NDescriptions label-placement="left" :column="2" bordered>
          <NDescriptionsItem :label="t('integration.provider')">
            {{ connection.provider }}
          </NDescriptionsItem>
          <NDescriptionsItem :label="t('common.status')">
            <NTag :type="statusMap[connection.status] || 'default'" size="small">{{ connection.status }}</NTag>
          </NDescriptionsItem>
          <NDescriptionsItem :label="t('integration.displayName')">{{ connection.display_name }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('integration.authType')">{{ connection.auth_type }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('integration.lastUsed')">
            {{ connection.last_used_at ? new Date(connection.last_used_at).toLocaleString() : "-" }}
          </NDescriptionsItem>
          <NDescriptionsItem :label="t('integration.errorCount')">{{ connection.error_count }}</NDescriptionsItem>
        </NDescriptions>
        <NSpace style="margin-top: 12px;">
          <NButton type="primary" @click="testConnection">{{ t("integration.test") }}</NButton>
          <NButton type="error" @click="deleteConnectionAction">{{ t("common.delete") }}</NButton>
        </NSpace>
      </NCard>

      <!-- Provisioning Templates -->
      <NCard :title="t('integration.templates')">
        <template #header-extra>
          <NButton size="small" type="primary" @click="showTemplateModal = true">{{ t("integration.addTemplate") }}</NButton>
        </template>
        <NDataTable v-if="templates.length" :columns="templateColumns" :data="templates" :row-key="(row: Template) => row.id" size="small" />
        <NEmpty v-else :description="t('integration.noTemplates')" />
      </NCard>

      <!-- Recent Jobs -->
      <NCard :title="t('integration.recentJobs')">
        <template #header-extra>
          <NButton size="small" text type="primary" @click="router.push({ name: 'provisioning-jobs' })">
            {{ t("integration.viewAllJobs") }}
          </NButton>
        </template>
        <NDataTable v-if="jobs.length" :columns="jobColumns" :data="jobs" :row-key="(row: Job) => row.id" size="small" />
        <NEmpty v-else :description="t('integration.noJobs')" />
      </NCard>
    </NSpace>

    <NCard v-else-if="!loading">
      <p>{{ t("common.notFound") }}</p>
      <NButton @click="router.push({ name: 'integrations' })">{{ t("common.back") }}</NButton>
    </NCard>

    <!-- Template Modal -->
    <NModal v-model:show="showTemplateModal" preset="card" :title="t('integration.addTemplate')" style="width: 520px; max-width: 95vw;">
      <NForm label-placement="left" label-width="160">
        <NFormItem :label="t('integration.eventTrigger')" required>
          <NSelect v-model:value="templateForm.event_trigger" :options="eventTriggerOptions" />
        </NFormItem>
        <NFormItem :label="t('integration.actionType')" required>
          <NSelect v-model:value="templateForm.action_type" :options="actionTypeOptions" />
        </NFormItem>
        <NFormItem :label="t('integration.deptFilter')">
          <NInput v-model:value="templateForm.department_filter" :placeholder="t('integration.deptFilterHint')" />
        </NFormItem>
        <NFormItem :label="t('integration.employmentFilter')">
          <NInput v-model:value="templateForm.employment_type_filter" :placeholder="t('integration.employmentFilterHint')" />
        </NFormItem>
        <NFormItem :label="t('integration.requiresApproval')">
          <NSwitch v-model:value="templateForm.requires_approval" />
        </NFormItem>
        <NFormItem :label="t('common.active')">
          <NSwitch v-model:value="templateForm.is_active" />
        </NFormItem>
        <NSpace>
          <NButton type="primary" :loading="templateLoading" @click="addTemplate">{{ t("common.save") }}</NButton>
          <NButton @click="showTemplateModal = false">{{ t("common.cancel") }}</NButton>
        </NSpace>
      </NForm>
    </NModal>
  </NSpin>
</template>
