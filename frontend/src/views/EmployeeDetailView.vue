<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { useAuthStore } from "../stores/auth";
import {
  NCard,
  NDescriptions,
  NDescriptionsItem,
  NSpin,
  NButton,
  NSpace,
  NTag,
  NModal,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NDatePicker,
  NSelect,
  NUpload,
  NDataTable,
  NTimeline,
  NTimelineItem,
  NEmpty,
  useMessage,
  type DataTableColumns,
  type UploadFileInfo,
} from "naive-ui";
import { h } from "vue";
import { employeeAPI, salaryAPI, companyAPI, integrationAPI, userAPI } from "../api/client";
import { format } from "date-fns";
import { useCurrency } from "../composables/useCurrency";

const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const message = useMessage();
const authStore = useAuthStore();
const { formatCurrency } = useCurrency();
const companyCountry = computed(() => authStore.user?.company_country || "PHL");
const isPHL = computed(() => companyCountry.value === "PHL");
const employee = ref<Record<string, unknown> | null>(null);
const profile = ref<Record<string, unknown> | null>(null);
const salary = ref<Record<string, unknown> | null>(null);
const loading = ref(true);
const error = ref("");
const departmentMap = ref(new Map<number, string>());
const positionMap = ref(new Map<number, string>());
const managerName = ref("");

function fmtDate(d: unknown): string {
  if (!d) return "-";
  try {
    return format(new Date(d as string), "yyyy-MM-dd");
  } catch {
    return String(d);
  }
}

const statusMap: Record<
  string,
  "success" | "warning" | "error" | "info" | "default"
> = {
  active: "success",
  probationary: "info",
  suspended: "warning",
  separated: "error",
};

// Salary assignment modal
const showSalaryModal = ref(false);
const salaryLoading = ref(false);
const salaryForm = ref({
  basic_salary: 0,
  structure_id: null as number | null,
  effective_from: Date.now(),
  effective_to: null as number | null,
  remarks: "",
});
const structureOptions = ref<{ label: string; value: number }[]>([]);

onMounted(async () => {
  try {
    const id = Number(route.params.id);
    const [emp, prof, sal, structs, depts, positions] = await Promise.allSettled([
      employeeAPI.get(id),
      employeeAPI.getProfile(id),
      employeeAPI.getSalary(id),
      salaryAPI.listStructures(),
      companyAPI.listDepartments(),
      companyAPI.listPositions(),
    ]);
    if (emp.status === "fulfilled") {
      const res = emp.value as {
        success: boolean;
        data: Record<string, unknown>;
      };
      employee.value = res.data || (res as unknown as Record<string, unknown>);
    } else {
      error.value = t("employee.notFound");
    }
    if (prof.status === "fulfilled") {
      const res = prof.value as {
        success: boolean;
        data: Record<string, unknown>;
      };
      profile.value = res.data || null;
    }
    if (sal.status === "fulfilled") {
      const res = sal.value as {
        success: boolean;
        data: Record<string, unknown> | null;
      };
      salary.value = res.data || null;
    }
    if (structs.status === "fulfilled") {
      const res = structs.value as {
        data?: { id: number; name: string }[];
      };
      const arr = res.data || (Array.isArray(res) ? res : []);
      structureOptions.value = (
        arr as { id: number; name: string }[]
      ).map((s) => ({ label: s.name, value: s.id }));
    }
    if (depts.status === "fulfilled") {
      const res = depts.value as {
        data?: { id: number; name: string }[];
      };
      const arr = res.data || (Array.isArray(res) ? res : []);
      const newMap = new Map<number, string>();
      (arr as { id: number; name: string }[]).forEach((d) => newMap.set(d.id, d.name));
      departmentMap.value = newMap;
    }
    if (positions.status === "fulfilled") {
      const res = positions.value as {
        data?: { id: number; title: string }[];
      };
      const arr = res.data || (Array.isArray(res) ? res : []);
      const newMap = new Map<number, string>();
      (arr as { id: number; title: string }[]).forEach((p) => newMap.set(p.id, p.title));
      positionMap.value = newMap;
    }
    // Resolve manager name if manager_id exists
    if (employee.value?.manager_id) {
      try {
        const mgrRes = await employeeAPI.get(employee.value.manager_id as number);
        const mgrData = (mgrRes as { data: Record<string, unknown> }).data ||
          (mgrRes as unknown as Record<string, unknown>);
        if (mgrData) {
          managerName.value = `${mgrData.first_name || ""} ${mgrData.last_name || ""}`.trim();
        }
      } catch {
        managerName.value = "";
      }
    }
    loadDocuments();
    loadTimeline();
    loadIntegrations();
  } catch {
    error.value = t("employee.loadFailed");
  } finally {
    loading.value = false;
  }
});

// Status Change
const showStatusModal = ref(false);
const statusForm = ref({ status: "", remarks: "" });
const statusLoading = ref(false);
const statusOptions = [
  { label: t("employee.active"), value: "active" },
  { label: t("employee.probationary"), value: "probationary" },
  { label: t("employee.suspended"), value: "suspended" },
  { label: t("employee.separated"), value: "separated" },
];

function openStatusChange() {
  statusForm.value = { status: (employee.value?.status as string) || "active", remarks: "" };
  showStatusModal.value = true;
}

async function submitStatusChange() {
  if (!statusForm.value.status) return;
  statusLoading.value = true;
  try {
    const id = Number(route.params.id);
    const res = (await employeeAPI.changeStatus(id, {
      status: statusForm.value.status,
      remarks: statusForm.value.remarks || undefined,
    })) as { data: Record<string, unknown> };
    employee.value = res.data || (res as unknown as Record<string, unknown>);
    showStatusModal.value = false;
    message.success(t("employee.statusChanged"));
    loadTimeline();
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } };
    message.error(err.data?.error?.message || t("common.saveFailed"));
  } finally {
    statusLoading.value = false;
  }
}

// Letter Generation
const showLetterModal = ref(false);
const letterLoading = ref(false);
const letterForm = ref({
  letter_type: "nte" as string,
  subject: "",
  body: "",
  violations: "",
  deadline: null as number | null,
});
const letterTypeOptions = [
  { label: t("employee.letterNTE"), value: "nte" },
  { label: t("employee.letterCOEC"), value: "coec" },
  { label: t("employee.letterClearance"), value: "clearance" },
  { label: t("employee.letterMemo"), value: "memo" },
];

async function generateLetter() {
  letterLoading.value = true;
  try {
    const id = Number(route.params.id);
    const token = localStorage.getItem("token");
    const url = employeeAPI.generateLetterUrl(id);
    const payload: Record<string, unknown> = {
      letter_type: letterForm.value.letter_type,
    };
    if (letterForm.value.subject) payload.subject = letterForm.value.subject;
    if (letterForm.value.body) payload.body = letterForm.value.body;
    if (letterForm.value.violations) payload.violations = letterForm.value.violations;
    if (letterForm.value.deadline) {
      payload.deadline = format(new Date(letterForm.value.deadline), "yyyy-MM-dd");
    }

    const res = await fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify(payload),
    });
    if (!res.ok) throw new Error("Failed");

    const blob = await res.blob();
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob);
    link.download = `${letterForm.value.letter_type.toUpperCase()}_${employee.value?.employee_no || id}.pdf`;
    link.click();
    URL.revokeObjectURL(link.href);
    showLetterModal.value = false;
  } catch {
    message.error(t("common.failed"));
  } finally {
    letterLoading.value = false;
  }
}

// Connected Accounts (Integration Identities)
interface IntegrationIdentity {
  id: string;
  provider: string;
  external_email: string | null;
  external_username: string | null;
  account_status: string;
  provisioned_at: string | null;
}
const integrationIdentities = ref<IntegrationIdentity[]>([]);

const identityColumns: DataTableColumns<IntegrationIdentity> = [
  {
    title: t("integration.provider"),
    key: "provider",
    width: 120,
    render: (row) => row.provider.charAt(0).toUpperCase() + row.provider.slice(1),
  },
  {
    title: t("integration.externalEmail"),
    key: "external_email",
    render: (row) => row.external_email || "-",
  },
  {
    title: t("integration.externalUsername"),
    key: "external_username",
    render: (row) => row.external_username || "-",
  },
  {
    title: t("common.status"),
    key: "account_status",
    width: 100,
    render: (row) => {
      const typeMap: Record<string, "success" | "warning" | "error" | "default"> = {
        active: "success",
        suspended: "warning",
        deleted: "error",
      };
      return h(NTag, { type: typeMap[row.account_status] || "default", size: "small" }, { default: () => row.account_status });
    },
  },
  {
    title: t("integration.provisionedAt"),
    key: "provisioned_at",
    width: 140,
    render: (row) => (row.provisioned_at ? fmtDate(row.provisioned_at) : "-"),
  },
];

async function loadIntegrations() {
  try {
    const id = Number(route.params.id);
    const res = await integrationAPI.getEmployeeIntegrations(id);
    const data = (res as { data?: IntegrationIdentity[] }).data ?? res;
    integrationIdentities.value = Array.isArray(data) ? data : [];
  } catch {
    integrationIdentities.value = [];
  }
}

// Timeline
interface TimelineEvent {
  id: number;
  action_type: string;
  effective_date: string;
  remarks: string | null;
  from_department: string;
  to_department: string;
  from_position: string;
  to_position: string;
  created_by_email: string;
  created_at: string;
}
const timeline = ref<TimelineEvent[]>([]);

const actionTypeMap: Record<string, 'success' | 'warning' | 'error' | 'info' | 'default'> = {
  hired: 'success', promoted: 'info', transferred: 'warning',
  regularized: 'success', separated: 'error', reinstated: 'success',
};

function timelineContent(ev: TimelineEvent): string {
  const parts: string[] = [];
  if (ev.from_position && ev.to_position && ev.from_position !== ev.to_position) {
    parts.push(`${ev.from_position} → ${ev.to_position}`);
  }
  if (ev.from_department && ev.to_department && ev.from_department !== ev.to_department) {
    parts.push(`${ev.from_department} → ${ev.to_department}`);
  }
  if (ev.remarks) parts.push(ev.remarks);
  return parts.join(' | ') || '-';
}

async function loadTimeline() {
  try {
    const id = Number(route.params.id);
    const res = await employeeAPI.getTimeline(id);
    const data = (res as { data: TimelineEvent[] }).data ?? res;
    timeline.value = Array.isArray(data) ? data : [];
  } catch (e) { console.error('Failed to load timeline', e); timeline.value = []; }
}

// Documents
interface EmployeeDoc {
  id: string;
  doc_type: string;
  file_name: string;
  file_size: number;
  mime_type: string | null;
  created_at: string;
}

const documents = ref<EmployeeDoc[]>([]);
const uploadDocType = ref("general");
const uploadExpiryDate = ref<number | null>(null);
const docTypeOptions = [
  { label: t("employee.contract"), value: "contract" },
  { label: t("employee.idPhoto"), value: "id_photo" },
  { label: t("employee.govId"), value: "gov_id" },
  { label: t("employee.certificate"), value: "certificate" },
  { label: t("employee.general"), value: "general" },
];

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return bytes + " B";
  if (bytes < 1048576) return (bytes / 1024).toFixed(1) + " KB";
  return (bytes / 1048576).toFixed(1) + " MB";
}

const docColumns: DataTableColumns<EmployeeDoc> = [
  { title: t("employee.fileName"), key: "file_name", ellipsis: { tooltip: true } },
  {
    title: t("employee.docType"),
    key: "doc_type",
    width: 120,
    render: (row) => {
      const key = `employee.${row.doc_type}` as const;
      return t(key) !== key ? t(key) : row.doc_type;
    },
  },
  {
    title: t("employee.fileSize"),
    key: "file_size",
    width: 100,
    render: (row) => formatFileSize(row.file_size),
  },
  {
    title: t("employee.uploadedAt"),
    key: "created_at",
    width: 120,
    render: (row) => fmtDate(row.created_at),
  },
  {
    title: t("common.actions"),
    key: "actions",
    width: 160,
    render: (row) => {
      const id = Number(route.params.id);
      const token = localStorage.getItem("token");
      const downloadUrl = employeeAPI.downloadDocumentUrl(id, row.id);
      return h(NSpace, { size: 4 }, {
        default: () => [
          h("a", {
            href: downloadUrl,
            target: "_blank",
            style: "text-decoration: none;",
            onClick: (e: MouseEvent) => {
              e.preventDefault();
              const a = document.createElement("a");
              a.href = downloadUrl + "?token=" + token;
              fetch(downloadUrl, { headers: { Authorization: `Bearer ${token}` } })
                .then(r => r.blob())
                .then(blob => {
                  const url = URL.createObjectURL(blob);
                  const link = document.createElement("a");
                  link.href = url;
                  link.download = row.file_name;
                  link.click();
                  URL.revokeObjectURL(url);
                });
            },
          }, [h(NButton, { size: "small" }, { default: () => t("employee.download") })]),
          h(NButton, { size: "small", type: "error", onClick: () => handleDeleteDoc(row.id) }, { default: () => t("common.delete") }),
        ],
      });
    },
  },
];

async function loadDocuments() {
  try {
    const id = Number(route.params.id);
    const res = await employeeAPI.listDocuments(id);
    const data = (res as any)?.data ?? res;
    documents.value = Array.isArray(data) ? data : [];
  } catch (e) { console.error('Failed to load documents', e); documents.value = []; }
}

async function handleUpload({ file }: { file: UploadFileInfo }) {
  if (!file.file) return;
  const id = Number(route.params.id);
  const formData = new FormData();
  formData.append("file", file.file);
  formData.append("doc_type", uploadDocType.value);
  if (uploadExpiryDate.value) {
    formData.append("expiry_date", new Date(uploadExpiryDate.value).toISOString().split("T")[0]);
  }
  try {
    await employeeAPI.uploadDocument(id, formData);
    message.success(t("employee.documentUploaded"));
    uploadExpiryDate.value = null;
    loadDocuments();
  } catch {
    message.error(t("common.failed"));
  }
}

async function handleDeleteDoc(docId: string) {
  const id = Number(route.params.id);
  try {
    await employeeAPI.deleteDocument(id, docId);
    message.success(t("employee.documentDeleted"));
    loadDocuments();
  } catch {
    message.error(t("common.failed"));
  }
}

function downloadCOE() {
  const id = Number(route.params.id);
  const url = employeeAPI.downloadCOEUrl(id);
  const token = localStorage.getItem("token");
  fetch(url, { headers: { Authorization: `Bearer ${token}` } })
    .then((r) => r.blob())
    .then((blob) => {
      const link = document.createElement("a");
      link.href = URL.createObjectURL(blob);
      link.download = `COE_${employee.value?.employee_no || id}.pdf`;
      link.click();
      URL.revokeObjectURL(link.href);
    })
    .catch(() => message.error(t("common.failed")));
}

// Create Employee Account modal
const showAccountModal = ref(false);
const accountLoading = ref(false);
const accountForm = ref({
  email: '',
  password: '',
  role: 'employee',
});
const roleOptions = [
  { label: 'Admin', value: 'admin' },
  { label: 'Manager', value: 'manager' },
  { label: 'Employee', value: 'employee' },
];

async function handleCreateAccount() {
  if (!accountForm.value.email || !accountForm.value.password) {
    message.warning(t('profile.fillAllFields'));
    return;
  }
  if (accountForm.value.password.length < 8) {
    message.warning(t('auth.passwordTooShort'));
    return;
  }
  accountLoading.value = true;
  try {
    const id = Number(route.params.id);
    await userAPI.createEmployeeAccount({
      employee_id: id,
      email: accountForm.value.email,
      password: accountForm.value.password,
      role: accountForm.value.role,
    });
    message.success(t('employee.accountCreated'));
    showAccountModal.value = false;
    // Refresh employee data to show user_id
    const res = await employeeAPI.get(id) as { success: boolean; data: Record<string, unknown> };
    employee.value = res.data || (res as unknown as Record<string, unknown>);
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } };
    message.error(err.data?.error?.message || t('common.saveFailed'));
  } finally {
    accountLoading.value = false;
  }
}

async function handleAssignSalary() {
  if (!salaryForm.value.basic_salary || !salaryForm.value.effective_from) {
    message.warning(t("common.fillAllFields"));
    return;
  }
  salaryLoading.value = true;
  try {
    const id = Number(route.params.id);
    const payload: Record<string, unknown> = {
      basic_salary: salaryForm.value.basic_salary,
      effective_from: format(
        new Date(salaryForm.value.effective_from),
        "yyyy-MM-dd"
      ),
    };
    if (salaryForm.value.structure_id)
      payload.structure_id = salaryForm.value.structure_id;
    if (salaryForm.value.effective_to)
      payload.effective_to = format(
        new Date(salaryForm.value.effective_to),
        "yyyy-MM-dd"
      );
    if (salaryForm.value.remarks) payload.remarks = salaryForm.value.remarks;

    const res = (await employeeAPI.assignSalary(id, payload)) as {
      success: boolean;
      data: Record<string, unknown>;
    };
    salary.value = res.data;
    showSalaryModal.value = false;
    message.success(t("employee.salaryAssigned"));
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } };
    message.error(err.data?.error?.message || t("common.saveFailed"));
  } finally {
    salaryLoading.value = false;
  }
}
</script>

<template>
  <NSpin :show="loading">
    <NSpace vertical :size="16" v-if="employee">
      <NSpace justify="space-between">
        <h2>{{ employee.first_name }} {{ employee.last_name }}</h2>
        <NSpace>
          <NButton v-if="authStore.isAdmin && !employee.user_id" type="info" @click="() => { accountForm.email = String(employee?.email || ''); showAccountModal = true; }">{{ t("employee.createAccount") }}</NButton>
          <NTag v-if="employee.user_id" type="success" size="small">{{ t("employee.hasLoginAccount") }}</NTag>
          <NButton v-if="authStore.isAdmin" type="warning" @click="openStatusChange">{{ t("employee.changeStatus") }}</NButton>
          <NButton v-if="authStore.isAdmin || authStore.isManager" @click="showLetterModal = true">{{ t("employee.generateLetter") }}</NButton>
          <NButton v-if="authStore.isAdmin || authStore.isManager" @click="downloadCOE">{{ t("employee.downloadCOE") }}</NButton>
          <NButton
            type="primary"
            @click="
              router.push({
                name: 'employee-edit',
                params: { id: route.params.id },
              })
            "
            >{{ t("common.edit") }}</NButton
          >
          <NButton @click="router.back()">{{ t("common.back") }}</NButton>
        </NSpace>
      </NSpace>

      <NCard :title="t('employee.basicInfo')">
        <NDescriptions label-placement="left" :column="2" bordered>
          <NDescriptionsItem :label="t('employee.employeeNo')">{{
            employee.employee_no
          }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('common.status')">
            <NTag
              :type="statusMap[employee.status as string] || 'default'"
              size="small"
              >{{ employee.status }}</NTag
            >
          </NDescriptionsItem>
          <NDescriptionsItem :label="t('common.email')">{{
            employee.email || "-"
          }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('common.phone')">{{
            employee.phone || "-"
          }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('employee.employmentType')">{{
            employee.employment_type
          }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('employee.department')">{{
            departmentMap.get(employee.department_id as number) || '-'
          }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('employee.position')">{{
            positionMap.get(employee.position_id as number) || '-'
          }}</NDescriptionsItem>
          <NDescriptionsItem v-if="employee.manager_id" :label="t('selfService.manager')">{{
            managerName || '-'
          }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('employee.hireDate')">{{
            fmtDate(employee.hire_date)
          }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('employee.gender')">{{
            employee.gender || "-"
          }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('employee.birthDate')">{{
            fmtDate(employee.birth_date)
          }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('employee.civilStatus')">{{
            employee.civil_status || "-"
          }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('employee.nationality')">{{
            employee.nationality || "-"
          }}</NDescriptionsItem>
        </NDescriptions>
      </NCard>

      <NCard :title="t('employee.govIds')" v-if="profile">
        <NDescriptions label-placement="left" :column="2" bordered>
          <NDescriptionsItem :label="isPHL ? t('employee.tin') : t('country.govId.LKA.tin')">{{
            profile.tin || "-"
          }}</NDescriptionsItem>
          <template v-if="isPHL">
            <NDescriptionsItem :label="t('employee.sssNo')">{{
              profile.sss_no || "-"
            }}</NDescriptionsItem>
            <NDescriptionsItem :label="t('employee.philhealthNo')">{{
              profile.philhealth_no || "-"
            }}</NDescriptionsItem>
            <NDescriptionsItem :label="t('employee.pagibigNo')">{{
              profile.pagibig_no || "-"
            }}</NDescriptionsItem>
          </template>
          <template v-else-if="companyCountry === 'LKA'">
            <NDescriptionsItem :label="t('country.govId.LKA.epf')">{{
              profile.sss_no || "-"
            }}</NDescriptionsItem>
            <NDescriptionsItem :label="t('country.govId.LKA.nic')">{{
              profile.philhealth_no || "-"
            }}</NDescriptionsItem>
          </template>
          <NDescriptionsItem :label="t('employee.bank')">{{
            profile.bank_name || "-"
          }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('employee.accountNo')">{{
            profile.bank_account_no || "-"
          }}</NDescriptionsItem>
        </NDescriptions>
      </NCard>

      <NCard
        :title="t('employee.emergencyContact')"
        v-if="profile && profile.emergency_name"
      >
        <NDescriptions label-placement="left" :column="2" bordered>
          <NDescriptionsItem :label="t('employee.emergencyName')">{{
            profile.emergency_name
          }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('employee.emergencyPhone')">{{
            profile.emergency_phone || "-"
          }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('employee.emergencyRelation')">{{
            profile.emergency_relation || "-"
          }}</NDescriptionsItem>
        </NDescriptions>
      </NCard>

      <!-- Salary Card -->
      <NCard :title="t('employee.salary')">
        <template v-if="salary">
          <NDescriptions label-placement="left" :column="2" bordered>
            <NDescriptionsItem :label="t('employee.basicSalary')">{{
              formatCurrency(salary.basic_salary)
            }}</NDescriptionsItem>
            <NDescriptionsItem :label="t('employee.effectiveFrom')">{{
              fmtDate(salary.effective_from)
            }}</NDescriptionsItem>
            <NDescriptionsItem :label="t('employee.effectiveTo')">{{
              salary.effective_to ? fmtDate(salary.effective_to) : "-"
            }}</NDescriptionsItem>
            <NDescriptionsItem :label="t('employee.remarks')">{{
              salary.remarks || "-"
            }}</NDescriptionsItem>
          </NDescriptions>
        </template>
        <p v-else>{{ t("employee.noSalary") }}</p>
        <NButton
          type="primary"
          size="small"
          style="margin-top: 12px"
          @click="showSalaryModal = true"
          >{{ t("employee.assignSalary") }}</NButton
        >
      </NCard>

      <!-- Documents Card -->
      <NCard :title="t('employee.documents')">
        <template #header-extra>
          <NSpace :size="8" align="center">
            <NSelect v-model:value="uploadDocType" :options="docTypeOptions" size="small" style="width: 140px;" />
            <NDatePicker v-model:value="uploadExpiryDate" type="date" size="small" :placeholder="t('dashboard.expiryDate')" clearable style="width: 150px;" />
            <NUpload :show-file-list="false" :custom-request="({ file }: any) => handleUpload({ file })">
              <NButton type="primary" size="small">{{ t('employee.uploadDocument') }}</NButton>
            </NUpload>
          </NSpace>
        </template>
        <NDataTable v-if="documents.length" :columns="docColumns" :data="documents" :row-key="(row: EmployeeDoc) => row.id" size="small" />
        <NEmpty v-else :description="t('employee.noDocuments')" />
      </NCard>

      <!-- Connected Accounts -->
      <NCard :title="t('integration.connectedAccounts')">
        <NDataTable v-if="integrationIdentities.length" :columns="identityColumns" :data="integrationIdentities" :row-key="(row: IntegrationIdentity) => row.id" size="small" />
        <NEmpty v-else :description="t('integration.noConnectedAccounts')" />
      </NCard>

      <!-- Employment Timeline -->
      <NCard :title="t('employee.timeline')">
        <NTimeline v-if="timeline.length">
          <NTimelineItem
            v-for="ev in timeline"
            :key="ev.id"
            :type="actionTypeMap[ev.action_type] || 'default'"
            :title="t(`employee.action_${ev.action_type}`, ev.action_type)"
            :time="fmtDate(ev.effective_date)"
            :content="timelineContent(ev)"
          />
        </NTimeline>
        <NEmpty v-else :description="t('employee.noTimeline')" />
      </NCard>

      <!-- Status Change Modal -->
      <NModal
        v-model:show="showStatusModal"
        preset="card"
        :title="t('employee.changeStatus')"
        style="width: 420px"
      >
        <NForm label-placement="left" label-width="100">
          <NFormItem :label="t('common.status')" required>
            <NSelect v-model:value="statusForm.status" :options="statusOptions" />
          </NFormItem>
          <NFormItem :label="t('employee.remarks')">
            <NInput v-model:value="statusForm.remarks" type="textarea" :rows="3" />
          </NFormItem>
          <NSpace>
            <NButton type="primary" :loading="statusLoading" @click="submitStatusChange">{{ t("common.save") }}</NButton>
            <NButton @click="showStatusModal = false">{{ t("common.cancel") }}</NButton>
          </NSpace>
        </NForm>
      </NModal>

      <!-- Letter Generation Modal -->
      <NModal
        v-model:show="showLetterModal"
        preset="card"
        :title="t('employee.generateLetter')"
        style="width: 520px"
      >
        <NForm label-placement="left" label-width="120">
          <NFormItem :label="t('common.type')" required>
            <NSelect v-model:value="letterForm.letter_type" :options="letterTypeOptions" />
          </NFormItem>
          <NFormItem v-if="letterForm.letter_type === 'nte' || letterForm.letter_type === 'memo'" :label="t('employee.letterSubject')">
            <NInput v-model:value="letterForm.subject" />
          </NFormItem>
          <NFormItem v-if="letterForm.letter_type === 'nte'" :label="t('employee.letterViolations')">
            <NInput v-model:value="letterForm.violations" type="textarea" :rows="3" />
          </NFormItem>
          <NFormItem v-if="letterForm.letter_type === 'nte'" :label="t('employee.letterDeadline')">
            <NDatePicker v-model:value="letterForm.deadline" type="date" style="width: 100%;" />
          </NFormItem>
          <NFormItem v-if="letterForm.letter_type === 'memo'" :label="t('employee.letterBody')">
            <NInput v-model:value="letterForm.body" type="textarea" :rows="5" />
          </NFormItem>
          <NSpace>
            <NButton type="primary" :loading="letterLoading" @click="generateLetter">{{ t("employee.generateLetter") }}</NButton>
            <NButton @click="showLetterModal = false">{{ t("common.cancel") }}</NButton>
          </NSpace>
        </NForm>
      </NModal>

      <!-- Create Account Modal -->
      <NModal
        v-model:show="showAccountModal"
        preset="card"
        :title="t('employee.createAccount')"
        style="width: 420px"
      >
        <NForm label-placement="left" label-width="100">
          <NFormItem :label="t('auth.email')" required>
            <NInput v-model:value="accountForm.email" placeholder="email@company.com" />
          </NFormItem>
          <NFormItem :label="t('auth.password')" required>
            <NInput v-model:value="accountForm.password" type="password" show-password-on="click" placeholder="Min 8 characters" />
          </NFormItem>
          <NFormItem :label="t('auth.role')">
            <NSelect v-model:value="accountForm.role" :options="roleOptions" />
          </NFormItem>
          <NSpace>
            <NButton type="primary" :loading="accountLoading" @click="handleCreateAccount">{{ t("common.save") }}</NButton>
            <NButton @click="showAccountModal = false">{{ t("common.cancel") }}</NButton>
          </NSpace>
        </NForm>
      </NModal>

      <!-- Salary Assignment Modal -->
      <NModal
        v-model:show="showSalaryModal"
        preset="card"
        :title="t('employee.assignSalary')"
        style="width: 480px"
      >
        <NForm label-placement="left" label-width="140">
          <NFormItem :label="t('employee.basicSalary')" required>
            <NInputNumber
              v-model:value="salaryForm.basic_salary"
              :min="0"
              :precision="2"
              style="width: 100%"
            />
          </NFormItem>
          <NFormItem :label="t('salary.structures')">
            <NSelect
              v-model:value="salaryForm.structure_id"
              :options="structureOptions"
              clearable
            />
          </NFormItem>
          <NFormItem :label="t('employee.effectiveFrom')" required>
            <NDatePicker
              v-model:value="salaryForm.effective_from"
              type="date"
              style="width: 100%"
            />
          </NFormItem>
          <NFormItem :label="t('employee.effectiveTo')">
            <NDatePicker
              v-model:value="salaryForm.effective_to"
              type="date"
              clearable
              style="width: 100%"
            />
          </NFormItem>
          <NFormItem :label="t('employee.remarks')">
            <NInput v-model:value="salaryForm.remarks" type="textarea" />
          </NFormItem>
          <NSpace>
            <NButton
              type="primary"
              :loading="salaryLoading"
              @click="handleAssignSalary"
              >{{ t("common.save") }}</NButton
            >
            <NButton @click="showSalaryModal = false">{{
              t("common.cancel")
            }}</NButton>
          </NSpace>
        </NForm>
      </NModal>
    </NSpace>

    <NCard v-else-if="!loading">
      <p>{{ error || t("employee.notFound") }}</p>
      <NButton @click="router.back()">{{ t("common.back") }}</NButton>
    </NCard>
  </NSpin>
</template>
