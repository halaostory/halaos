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
  NInputNumber,
  NSelect,
  NDatePicker,
  NDescriptions,
  NDescriptionsItem,
  useMessage,
  useDialog,
  type DataTableColumns,
} from "naive-ui";
import { loanAPI } from "../api/client";
import { useAuthStore } from "../stores/auth";

const { t } = useI18n();
const message = useMessage();
const dialog = useDialog();
const auth = useAuthStore();

const activeTab = ref("my-loans");
const loading = ref(false);

// Data
const loanTypes = ref<Record<string, unknown>[]>([]);
const allLoans = ref<Record<string, unknown>[]>([]);
const myLoans = ref<Record<string, unknown>[]>([]);

// Modals
const showApplyModal = ref(false);
const showTypeModal = ref(false);
const showDetailModal = ref(false);
const showPaymentModal = ref(false);

const loanDetail = ref<Record<string, unknown> | null>(null);
const loanPayments = ref<Record<string, unknown>[]>([]);

function php(v: unknown): string {
  return Number(v || 0).toLocaleString("en-PH", {
    style: "currency",
    currency: "PHP",
  });
}

function fmtDate(d: unknown): string {
  if (!d) return "-";
  const s = String(d);
  return s.length >= 10 ? s.substring(0, 10) : s;
}

const statusColorMap: Record<string, "default" | "info" | "success" | "warning" | "error"> = {
  pending: "default",
  approved: "info",
  active: "success",
  completed: "success",
  cancelled: "error",
};

// Apply form
const applyForm = ref({
  loan_type_id: null as number | null,
  amount: 0,
  term_months: 12,
  reference_no: "",
  start_date: null as number | null,
  remarks: "",
});

// Type form
const typeForm = ref({
  name: "",
  code: "",
  provider: "company",
  max_term_months: 24,
  interest_rate: 1,
  max_amount: null as number | null,
});

// Payment form
const paymentForm = ref({
  amount: 0,
  payment_date: null as number | null,
  remarks: "",
});

const loanTypeOptions = computed(() =>
  loanTypes.value.map((lt) => ({
    label: lt.name as string,
    value: lt.id as number,
  })),
);

const providerOptions = computed(() => [
  { label: t("loan.government"), value: "government" },
  { label: t("loan.company"), value: "company" },
]);

function formatDate(ts: number | null): string {
  if (!ts) return "";
  const d = new Date(ts);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

// Columns
const loanColumns: DataTableColumns = [
  {
    title: t("employee.name"),
    key: "employee_name",
    render(row) {
      return row.first_name ? `${row.first_name} ${row.last_name}` : "-";
    },
  },
  { title: t("loan.typeName"), key: "loan_type_name", width: 150 },
  {
    title: t("loan.principalAmount"),
    key: "principal_amount",
    width: 120,
    render(row) { return php(row.principal_amount); },
  },
  {
    title: t("loan.monthlyAmort"),
    key: "monthly_amortization",
    width: 120,
    render(row) { return php(row.monthly_amortization); },
  },
  {
    title: t("loan.remainingBalance"),
    key: "remaining_balance",
    width: 120,
    render(row) { return php(row.remaining_balance); },
  },
  {
    title: t("common.status"),
    key: "status",
    width: 100,
    render(row) {
      const s = row.status as string;
      return h(NTag, { type: statusColorMap[s] || "default", size: "small" }, () => s);
    },
  },
  {
    title: t("common.actions"),
    key: "actions",
    width: 200,
    render(row) {
      const btns = [
        h(NButton, { size: "small", onClick: () => viewDetail(row) }, () => t("common.view")),
      ];
      if (row.status === "pending" && auth.isAdmin) {
        btns.push(
          h(NButton, { size: "small", type: "primary", onClick: () => handleApprove(row) }, () => t("loan.approve")),
        );
      }
      if ((row.status === "pending" || row.status === "approved") as boolean) {
        btns.push(
          h(NButton, { size: "small", type: "error", onClick: () => handleCancel(row) }, () => t("loan.cancel")),
        );
      }
      return h(NSpace, { size: "small" }, () => btns);
    },
  },
];

const myLoanColumns: DataTableColumns = [
  { title: t("loan.typeName"), key: "loan_type_name", width: 150 },
  {
    title: t("loan.principalAmount"),
    key: "principal_amount",
    width: 120,
    render(row) { return php(row.principal_amount); },
  },
  {
    title: t("loan.monthlyAmort"),
    key: "monthly_amortization",
    width: 120,
    render(row) { return php(row.monthly_amortization); },
  },
  {
    title: t("loan.remainingBalance"),
    key: "remaining_balance",
    width: 120,
    render(row) { return php(row.remaining_balance); },
  },
  {
    title: t("loan.startDate"),
    key: "start_date",
    width: 110,
    render(row) { return fmtDate(row.start_date); },
  },
  {
    title: t("common.status"),
    key: "status",
    width: 100,
    render(row) {
      const s = row.status as string;
      return h(NTag, { type: statusColorMap[s] || "default", size: "small" }, () => s);
    },
  },
  {
    title: t("common.actions"),
    key: "actions",
    width: 80,
    render(row) {
      return h(NButton, { size: "small", onClick: () => viewDetail(row) }, () => t("common.view"));
    },
  },
];

const paymentColumns: DataTableColumns = [
  {
    title: t("loan.paymentDate"),
    key: "payment_date",
    width: 110,
    render(row) { return fmtDate(row.payment_date); },
  },
  {
    title: t("loan.paymentAmount"),
    key: "amount",
    width: 120,
    render(row) { return php(row.amount); },
  },
  { title: t("loan.paymentType"), key: "payment_type", width: 100 },
  { title: t("employee.remarks"), key: "remarks" },
];

// Load
async function loadLoanTypes() {
  try {
    const res = (await loanAPI.listTypes()) as { data?: Record<string, unknown>[] };
    loanTypes.value = (res.data || (Array.isArray(res) ? res : [])) as Record<string, unknown>[];
  } catch {
    loanTypes.value = [];
  }
}

async function loadAllLoans() {
  loading.value = true;
  try {
    const res = (await loanAPI.list()) as { data?: Record<string, unknown>[] };
    allLoans.value = (res.data || (Array.isArray(res) ? res : [])) as Record<string, unknown>[];
  } catch {
    allLoans.value = [];
  } finally {
    loading.value = false;
  }
}

async function loadMyLoans() {
  loading.value = true;
  try {
    const res = (await loanAPI.listMy()) as { data?: Record<string, unknown>[] };
    myLoans.value = (res.data || (Array.isArray(res) ? res : [])) as Record<string, unknown>[];
  } catch {
    myLoans.value = [];
  } finally {
    loading.value = false;
  }
}

async function viewDetail(row: Record<string, unknown>) {
  try {
    const res = (await loanAPI.get(row.id as number)) as {
      data?: { loan: Record<string, unknown>; payments: Record<string, unknown>[] };
    };
    const data = res.data || (res as unknown as { loan: Record<string, unknown>; payments: Record<string, unknown>[] });
    loanDetail.value = data.loan || null;
    loanPayments.value = data.payments || [];
    showDetailModal.value = true;
  } catch {
    message.error(t("common.failed"));
  }
}

// Actions
async function handleApply() {
  if (!applyForm.value.loan_type_id || !applyForm.value.amount || !applyForm.value.start_date) {
    message.warning(t("common.fillAllFields"));
    return;
  }
  try {
    await loanAPI.apply({
      loan_type_id: applyForm.value.loan_type_id,
      amount: applyForm.value.amount,
      term_months: applyForm.value.term_months,
      reference_no: applyForm.value.reference_no || undefined,
      start_date: formatDate(applyForm.value.start_date),
      remarks: applyForm.value.remarks || undefined,
    });
    showApplyModal.value = false;
    applyForm.value = { loan_type_id: null, amount: 0, term_months: 12, reference_no: "", start_date: null, remarks: "" };
    message.success(t("loan.applied"));
    await loadMyLoans();
  } catch {
    message.error(t("common.failed"));
  }
}

async function handleCreateType() {
  if (!typeForm.value.name || !typeForm.value.code) {
    message.warning(t("common.fillAllFields"));
    return;
  }
  try {
    await loanAPI.createType({
      name: typeForm.value.name,
      code: typeForm.value.code,
      provider: typeForm.value.provider,
      max_term_months: typeForm.value.max_term_months,
      interest_rate: typeForm.value.interest_rate / 100,
      max_amount: typeForm.value.max_amount || undefined,
    });
    showTypeModal.value = false;
    typeForm.value = { name: "", code: "", provider: "company", max_term_months: 24, interest_rate: 1, max_amount: null };
    message.success(t("loan.typeCreated"));
    await loadLoanTypes();
  } catch {
    message.error(t("common.failed"));
  }
}

function handleApprove(row: Record<string, unknown>) {
  dialog.info({
    title: t("loan.approve"),
    content: `Approve loan for ${row.first_name} ${row.last_name}?`,
    positiveText: t("common.confirm"),
    negativeText: t("common.cancel"),
    onPositiveClick: async () => {
      try {
        await loanAPI.approve(row.id as number);
        message.success(t("loan.loanApproved"));
        await loadAllLoans();
      } catch {
        message.error(t("common.failed"));
      }
    },
  });
}

function handleCancel(row: Record<string, unknown>) {
  dialog.warning({
    title: t("loan.cancel"),
    content: `Cancel this loan?`,
    positiveText: t("common.confirm"),
    negativeText: t("common.cancel"),
    onPositiveClick: async () => {
      try {
        await loanAPI.cancel(row.id as number);
        message.success(t("loan.loanCancelled"));
        if (activeTab.value === "all-loans") await loadAllLoans();
        else await loadMyLoans();
      } catch {
        message.error(t("common.failed"));
      }
    },
  });
}

async function handleRecordPayment() {
  if (!loanDetail.value || !paymentForm.value.amount || !paymentForm.value.payment_date) {
    message.warning(t("common.fillAllFields"));
    return;
  }
  try {
    await loanAPI.recordPayment(loanDetail.value.id as number, {
      amount: paymentForm.value.amount,
      payment_date: formatDate(paymentForm.value.payment_date),
      remarks: paymentForm.value.remarks || undefined,
    });
    showPaymentModal.value = false;
    paymentForm.value = { amount: 0, payment_date: null, remarks: "" };
    message.success(t("loan.paymentRecorded"));
    await viewDetail(loanDetail.value);
  } catch {
    message.error(t("common.failed"));
  }
}

function onTabChange(tab: string | number) {
  activeTab.value = String(tab);
  if (tab === "my-loans") loadMyLoans();
  else if (tab === "all-loans") loadAllLoans();
}

onMounted(async () => {
  await loadLoanTypes();
  await loadMyLoans();
});
</script>

<template>
  <div>
    <NSpace justify="space-between" style="margin-bottom: 16px">
      <h2>{{ t("loan.title") }}</h2>
      <NSpace>
        <NButton v-if="auth.isAdmin" @click="showTypeModal = true">
          {{ t("loan.createType") }}
        </NButton>
        <NButton type="primary" @click="showApplyModal = true">
          {{ t("loan.apply") }}
        </NButton>
      </NSpace>
    </NSpace>

    <NTabs type="line" :value="activeTab" @update:value="onTabChange">
      <NTabPane name="my-loans" :tab="t('loan.myLoans')">
        <NDataTable :columns="myLoanColumns" :data="myLoans" :loading="loading" size="small" />
      </NTabPane>
      <NTabPane v-if="auth.isAdmin" name="all-loans" :tab="t('loan.allLoans')">
        <NDataTable :columns="loanColumns" :data="allLoans" :loading="loading" size="small" />
      </NTabPane>
    </NTabs>

    <!-- Apply Loan Modal -->
    <NModal v-model:show="showApplyModal" preset="card" :title="t('loan.apply')" style="max-width: 500px; width: 95vw">
      <NForm label-placement="left" label-width="130">
        <NFormItem :label="t('loan.typeName')" required>
          <NSelect v-model:value="applyForm.loan_type_id" :options="loanTypeOptions" />
        </NFormItem>
        <NFormItem :label="t('loan.principalAmount')" required>
          <NInputNumber v-model:value="applyForm.amount" :min="0" :precision="2" style="width: 100%" />
        </NFormItem>
        <NFormItem :label="t('loan.termMonths')" required>
          <NInputNumber v-model:value="applyForm.term_months" :min="1" :max="60" style="width: 100%" />
        </NFormItem>
        <NFormItem :label="t('loan.referenceNo')">
          <NInput v-model:value="applyForm.reference_no" placeholder="SSS/Pag-IBIG loan number" />
        </NFormItem>
        <NFormItem :label="t('loan.startDate')" required>
          <NDatePicker v-model:value="applyForm.start_date" type="date" style="width: 100%" />
        </NFormItem>
        <NFormItem :label="t('employee.remarks')">
          <NInput v-model:value="applyForm.remarks" type="textarea" />
        </NFormItem>
        <NFormItem>
          <NButton type="primary" @click="handleApply">{{ t("common.submit") }}</NButton>
        </NFormItem>
      </NForm>
    </NModal>

    <!-- Create Loan Type Modal -->
    <NModal v-model:show="showTypeModal" preset="card" :title="t('loan.createType')" style="max-width: 500px; width: 95vw">
      <NForm label-placement="left" label-width="130">
        <NFormItem :label="t('common.name')" required>
          <NInput v-model:value="typeForm.name" />
        </NFormItem>
        <NFormItem :label="t('loan.code')" required>
          <NInput v-model:value="typeForm.code" placeholder="e.g. sss_salary, pagibig_mpl" />
        </NFormItem>
        <NFormItem :label="t('loan.provider')">
          <NSelect v-model:value="typeForm.provider" :options="providerOptions" />
        </NFormItem>
        <NFormItem :label="t('loan.maxTerm')">
          <NInputNumber v-model:value="typeForm.max_term_months" :min="1" style="width: 100%" />
        </NFormItem>
        <NFormItem :label="t('loan.interestRate')">
          <NInputNumber v-model:value="typeForm.interest_rate" :min="0" :precision="2" style="width: 100%" />
        </NFormItem>
        <NFormItem>
          <NButton type="primary" @click="handleCreateType">{{ t("common.create") }}</NButton>
        </NFormItem>
      </NForm>
    </NModal>

    <!-- Loan Detail Modal -->
    <NModal v-model:show="showDetailModal" preset="card" :title="t('loan.typeName')" style="max-width: 700px; width: 95vw">
      <template v-if="loanDetail">
        <NDescriptions bordered :column="2" size="small" style="margin-bottom: 16px">
          <NDescriptionsItem v-if="loanDetail.first_name" :label="t('employee.name')">
            {{ loanDetail.first_name }} {{ loanDetail.last_name }}
          </NDescriptionsItem>
          <NDescriptionsItem :label="t('loan.typeName')">{{ loanDetail.loan_type_name }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('loan.principalAmount')">{{ php(loanDetail.principal_amount) }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('loan.totalAmount')">{{ php(loanDetail.total_amount) }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('loan.monthlyAmort')">{{ php(loanDetail.monthly_amortization) }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('loan.termMonths')">{{ loanDetail.term_months }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('loan.totalPaid')">{{ php(loanDetail.total_paid) }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('loan.remainingBalance')">
            <strong>{{ php(loanDetail.remaining_balance) }}</strong>
          </NDescriptionsItem>
          <NDescriptionsItem :label="t('loan.startDate')">{{ fmtDate(loanDetail.start_date) }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('loan.endDate')">{{ fmtDate(loanDetail.end_date) }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('common.status')">
            <NTag :type="statusColorMap[(loanDetail.status as string)] || 'default'" size="small">
              {{ loanDetail.status }}
            </NTag>
          </NDescriptionsItem>
          <NDescriptionsItem v-if="loanDetail.reference_no" :label="t('loan.referenceNo')">
            {{ loanDetail.reference_no }}
          </NDescriptionsItem>
        </NDescriptions>

        <NSpace style="margin-bottom: 12px">
          <NButton v-if="loanDetail.status === 'active' && auth.isAdmin" type="primary" size="small" @click="showPaymentModal = true">
            {{ t("loan.recordPayment") }}
          </NButton>
        </NSpace>

        <h4 style="margin-bottom: 8px">{{ t("loan.payments") }}</h4>
        <NDataTable :columns="paymentColumns" :data="loanPayments" size="small" :pagination="false" />
      </template>
    </NModal>

    <!-- Record Payment Modal -->
    <NModal v-model:show="showPaymentModal" preset="card" :title="t('loan.recordPayment')" style="max-width: 400px; width: 95vw">
      <NForm label-placement="left" label-width="100">
        <NFormItem :label="t('loan.paymentAmount')" required>
          <NInputNumber v-model:value="paymentForm.amount" :min="0" :precision="2" style="width: 100%" />
        </NFormItem>
        <NFormItem :label="t('loan.paymentDate')" required>
          <NDatePicker v-model:value="paymentForm.payment_date" type="date" style="width: 100%" />
        </NFormItem>
        <NFormItem :label="t('employee.remarks')">
          <NInput v-model:value="paymentForm.remarks" />
        </NFormItem>
        <NFormItem>
          <NButton type="primary" @click="handleRecordPayment">{{ t("common.submit") }}</NButton>
        </NFormItem>
      </NForm>
    </NModal>
  </div>
</template>
