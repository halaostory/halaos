<script setup lang="ts">
import { ref, h, computed, onMounted } from "vue";
import { useI18n } from "vue-i18n";
import {
  NDataTable,
  NButton,
  NSpace,
  NTabs,
  NTabPane,
  NTag,
  NModal,
  NForm,
  NFormItem,
  NInput,
  NSelect,
  useMessage,
  type DataTableColumns,
} from "naive-ui";
import { onboardingAPI } from "../api/client";

const { t } = useI18n();
const message = useMessage();

const activeTab = ref("onboarding");
const loading = ref(false);
const tasks = ref<Record<string, unknown>[]>([]);

// Initiate modal
const showInitiateModal = ref(false);
const initiateForm = ref({
  employee_id: null as number | null,
  workflow_type: "onboarding",
});

// Template modal
const showTemplateModal = ref(false);
const templateForm = ref({
  workflow_type: "onboarding",
  title: "",
  description: "",
  sort_order: 0,
  is_required: true,
  assignee_role: "hr",
  due_days: 0,
});

const workflowOptions = computed(() => [
  { label: t("onboarding.onboarding"), value: "onboarding" },
  { label: t("onboarding.offboarding"), value: "offboarding" },
]);

const roleOptions = computed(() => [
  { label: "HR", value: "hr" },
  { label: t("onboarding.manager"), value: "manager" },
  { label: t("onboarding.employee"), value: "employee" },
  { label: "IT", value: "it" },
]);

const statusColorMap: Record<string, "default" | "info" | "success" | "warning"> = {
  pending: "default",
  in_progress: "info",
  completed: "success",
  skipped: "warning",
};

const columns: DataTableColumns = [
  { title: t("employee.name"), key: "employee_name", width: 150,
    render(row) { return `${row.first_name} ${row.last_name}`; }
  },
  { title: t("employee.employeeNo"), key: "employee_no", width: 100 },
  { title: t("onboarding.task"), key: "title" },
  { title: t("onboarding.assignee"), key: "assignee_role", width: 80 },
  {
    title: t("onboarding.dueDate"),
    key: "due_date",
    width: 110,
    render(row) {
      const d = row.due_date as string;
      return d ? d.substring(0, 10) : "-";
    },
  },
  {
    title: t("common.status"),
    key: "status",
    width: 110,
    render(row) {
      const s = row.status as string;
      return h(
        NTag,
        { type: statusColorMap[s] || "default", size: "small" },
        () => s
      );
    },
  },
  {
    title: t("common.actions"),
    key: "actions",
    width: 130,
    render(row) {
      if (row.status === "completed" || row.status === "skipped") {
        return "";
      }
      return h(NSpace, { size: "small" }, () => [
        h(
          NButton,
          {
            size: "small",
            type: "success",
            onClick: () => handleComplete(row),
          },
          () => t("onboarding.complete")
        ),
        h(
          NButton,
          {
            size: "small",
            onClick: () => handleSkip(row),
          },
          () => t("onboarding.skip")
        ),
      ]);
    },
  },
];

async function loadTasks() {
  loading.value = true;
  try {
    const res = (await onboardingAPI.listPendingTasks({
      type: activeTab.value,
    })) as { data?: Record<string, unknown>[] };
    tasks.value = (res.data ||
      (Array.isArray(res) ? res : [])) as Record<string, unknown>[];
  } catch {
    tasks.value = [];
  } finally {
    loading.value = false;
  }
}

onMounted(loadTasks);

function onTabChange(tab: string | number) {
  activeTab.value = String(tab);
  loadTasks();
}

async function handleComplete(row: Record<string, unknown>) {
  try {
    await onboardingAPI.updateTask(row.id as number, { status: "completed" });
    message.success(t("onboarding.taskCompleted"));
    await loadTasks();
  } catch {
    message.error(t("common.failed"));
  }
}

async function handleSkip(row: Record<string, unknown>) {
  try {
    await onboardingAPI.updateTask(row.id as number, { status: "skipped" });
    await loadTasks();
  } catch {
    message.error(t("common.failed"));
  }
}

async function handleInitiate() {
  if (!initiateForm.value.employee_id) {
    message.warning(t("common.fillAllFields"));
    return;
  }
  try {
    await onboardingAPI.initiate({
      employee_id: initiateForm.value.employee_id,
      workflow_type: initiateForm.value.workflow_type,
    });
    showInitiateModal.value = false;
    message.success(t("onboarding.workflowStarted"));
    await loadTasks();
  } catch {
    message.error(t("common.failed"));
  }
}

async function handleCreateTemplate() {
  if (!templateForm.value.title) {
    message.warning(t("common.fillAllFields"));
    return;
  }
  try {
    await onboardingAPI.createTemplate({
      ...templateForm.value,
    });
    showTemplateModal.value = false;
    templateForm.value = {
      workflow_type: "onboarding",
      title: "",
      description: "",
      sort_order: 0,
      is_required: true,
      assignee_role: "hr",
      due_days: 0,
    };
    message.success(t("onboarding.templateCreated"));
  } catch {
    message.error(t("common.failed"));
  }
}
</script>

<template>
  <div>
    <NSpace justify="space-between" style="margin-bottom: 16px">
      <h2>{{ t("onboarding.title") }}</h2>
      <NSpace>
        <NButton @click="showTemplateModal = true">{{
          t("onboarding.manageTemplates")
        }}</NButton>
        <NButton type="primary" @click="showInitiateModal = true">{{
          t("onboarding.startWorkflow")
        }}</NButton>
      </NSpace>
    </NSpace>

    <NTabs type="line" :value="activeTab" @update:value="onTabChange">
      <NTabPane name="onboarding" :tab="t('onboarding.onboarding')">
        <NDataTable
          :columns="columns"
          :data="tasks"
          :loading="loading"
          size="small"
        />
      </NTabPane>
      <NTabPane name="offboarding" :tab="t('onboarding.offboarding')">
        <NDataTable
          :columns="columns"
          :data="tasks"
          :loading="loading"
          size="small"
        />
      </NTabPane>
    </NTabs>

    <!-- Initiate Workflow Modal -->
    <NModal
      v-model:show="showInitiateModal"
      preset="card"
      :title="t('onboarding.startWorkflow')"
      style="width: 450px"
    >
      <NForm label-placement="left" label-width="120">
        <NFormItem :label="t('onboarding.employeeId')" required>
          <NInput
            :value="String(initiateForm.employee_id || '')"
            @update:value="(v: string) => (initiateForm.employee_id = v ? Number(v) : null)"
            placeholder="Employee ID"
          />
        </NFormItem>
        <NFormItem :label="t('common.type')">
          <NSelect
            v-model:value="initiateForm.workflow_type"
            :options="workflowOptions"
          />
        </NFormItem>
        <NFormItem>
          <NButton type="primary" @click="handleInitiate">{{
            t("onboarding.startWorkflow")
          }}</NButton>
        </NFormItem>
      </NForm>
    </NModal>

    <!-- Create Template Modal -->
    <NModal
      v-model:show="showTemplateModal"
      preset="card"
      :title="t('onboarding.createTemplate')"
      style="width: 500px"
    >
      <NForm label-placement="left" label-width="120">
        <NFormItem :label="t('common.type')">
          <NSelect
            v-model:value="templateForm.workflow_type"
            :options="workflowOptions"
          />
        </NFormItem>
        <NFormItem :label="t('onboarding.task')" required>
          <NInput v-model:value="templateForm.title" />
        </NFormItem>
        <NFormItem :label="t('onboarding.description')">
          <NInput v-model:value="templateForm.description" type="textarea" />
        </NFormItem>
        <NFormItem :label="t('onboarding.assignee')">
          <NSelect
            v-model:value="templateForm.assignee_role"
            :options="roleOptions"
          />
        </NFormItem>
        <NFormItem :label="t('onboarding.dueDays')">
          <NInput
            :value="String(templateForm.due_days)"
            @update:value="(v: string) => (templateForm.due_days = Number(v) || 0)"
          />
        </NFormItem>
        <NFormItem>
          <NButton type="primary" @click="handleCreateTemplate">{{
            t("common.create")
          }}</NButton>
        </NFormItem>
      </NForm>
    </NModal>
  </div>
</template>
