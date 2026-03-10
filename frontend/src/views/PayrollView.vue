<script setup lang="ts">
import { ref, h, computed, onMounted } from "vue";
import { useI18n } from "vue-i18n";
import {
  NDataTable,
  NButton,
  NSpace,
  NModal,
  NForm,
  NFormItem,
  NInput,
  NDatePicker,
  NSelect,
  NTag,
  NTabs,
  NTabPane,
  NInputNumber,
  NSwitch,
  NCard,
  NEmpty,
  NTimeline,
  NTimelineItem,
  useMessage,
  useDialog,
  type DataTableColumns,
} from "naive-ui";
import { payrollAPI, exportAPI, thirteenthMonthAPI } from "../api/client";

interface AnomalyItem {
  type: string;
  severity: "critical" | "high" | "medium" | "low";
  employee_id: number;
  employee_name: string;
  employee_no: string;
  description: string;
  current_value: number;
  expected_value?: number;
  deviation_pct?: number;
}

interface AnomalyReport {
  run_id: number;
  cycle_id: number;
  total_items: number;
  anomalies: AnomalyItem[];
  summary: { critical: number; high: number; medium: number; low: number };
}

interface ThirteenthMonthRecord {
  employee_name: string;
  employee_no: string;
  months_worked: number;
  total_basic_salary: number;
  thirteenth_month_amount: number;
  tax_exempt: number;
  taxable: number;
  status: string;
}

import { format } from "date-fns";

const { t } = useI18n();
const message = useMessage();
const dialog = useDialog();

// Active tab
const activeTab = ref("cycles");

const data = ref<Record<string, unknown>[]>([]);
const loading = ref(false);

// Items Modal
const showItemsModal = ref(false);
const itemsLoading = ref(false);
const payrollItems = ref<Record<string, unknown>[]>([]);
const itemsTitle = ref("");

// Anomaly Modal
const showAnomalyModal = ref(false);
const anomalyLoading = ref(false);
const anomalyReport = ref<AnomalyReport | null>(null);
const anomalyTitle = ref("");

// Create Cycle Modal
const showCreateModal = ref(false);
const createLoading = ref(false);
const cycleForm = ref({
  name: "",
  period_start: null as number | null,
  period_end: null as number | null,
  pay_date: null as number | null,
  cycle_type: "regular",
});

const cycleTypeOptions = computed(() => [
  { label: t("payroll.typeRegular"), value: "regular" },
  { label: t("payroll.type13th"), value: "13th_month" },
  { label: t("payroll.typeFinal"), value: "final_pay" },
]);

const statusColorMap: Record<string, string> = {
  draft: "default",
  processing: "warning",
  computed: "info",
  approved: "success",
  paid: "success",
  void: "error",
};

const columns: DataTableColumns = [
  { title: t("payroll.cycleName"), key: "name" },
  {
    title: t("payroll.period"),
    key: "period",
    width: 200,
    render(row) {
      const start = row.period_start as string;
      const end = row.period_end as string;
      return `${formatDate(start)} ~ ${formatDate(end)}`;
    },
  },
  {
    title: t("payroll.payDate"),
    key: "pay_date",
    width: 120,
    render(row) {
      return formatDate(row.pay_date as string);
    },
  },
  { title: t("payroll.cycleType"), key: "cycle_type", width: 100 },
  {
    title: t("common.status"),
    key: "status",
    width: 110,
    render(row) {
      const status = row.status as string;
      return h(
        NTag,
        {
          type: (statusColorMap[status] || "default") as
            | "default"
            | "info"
            | "success"
            | "warning"
            | "error",
          size: "small",
        },
        () => status
      );
    },
  },
  {
    title: t("common.actions"),
    key: "actions",
    width: 320,
    render(row) {
      const btns: ReturnType<typeof h>[] = [];
      if (row.is_locked) {
        btns.push(
          h(NTag, { size: "small", type: "error" }, () => t("payroll.locked"))
        );
        btns.push(
          h(NButton, { size: "small", onClick: () => handleViewItems(row) }, () => t("payroll.viewItems"))
        );
        btns.push(
          h(NButton, { size: "small", onClick: () => handleUnlock(row) }, () => t("payroll.unlock"))
        );
        return h(NSpace, { size: "small" }, () => btns);
      }
      if (row.status === "draft") {
        btns.push(
          h(
            NButton,
            {
              size: "small",
              type: "primary",
              onClick: () => handleRunPayroll(row),
            },
            () => t("payroll.run")
          )
        );
      }
      if (
        row.status === "computed" ||
        row.status === "approved" ||
        row.status === "paid"
      ) {
        btns.push(
          h(
            NButton,
            { size: "small", onClick: () => handleViewItems(row) },
            () => t("payroll.viewItems")
          )
        );
        btns.push(
          h(
            NButton,
            { size: "small", onClick: () => handleExportCSV(row) },
            () => "CSV"
          )
        );
        btns.push(
          h(
            NButton,
            { size: "small", type: "info", onClick: () => handleExportBankFile(row) },
            () => t("payroll.bankFile")
          )
        );
        btns.push(
          h(
            NButton,
            {
              size: "small",
              type: "warning",
              onClick: () => handleScanAnomalies(row),
            },
            () => t("payroll.aiScan")
          )
        );
      }
      if (row.status === "draft" || row.status === "computed") {
        btns.push(
          h(
            NButton,
            {
              size: "small",
              type: "success",
              onClick: () => handleApprove(row),
            },
            () => t("common.approve")
          )
        );
      }
      if (row.status === "approved" || row.status === "paid") {
        btns.push(
          h(
            NButton,
            { size: "small", type: "error", onClick: () => handleLock(row) },
            () => t("payroll.lock")
          )
        );
      }
      return h(NSpace, { size: "small" }, () => btns);
    },
  },
];

function formatDate(d: string): string {
  if (!d) return "";
  try {
    return format(new Date(d), "yyyy-MM-dd");
  } catch {
    return d;
  }
}

async function fetchCycles() {
  loading.value = true;
  try {
    const resp = (await payrollAPI.listCycles({
      page: "1",
      limit: "50",
    })) as { success: boolean; data: Record<string, unknown>[] };
    data.value =
      resp.data || (resp as unknown as Record<string, unknown>[]);
  } catch {
    data.value = [];
  } finally {
    loading.value = false;
  }
}

onMounted(fetchCycles);

async function handleCreateCycle() {
  if (
    !cycleForm.value.name ||
    !cycleForm.value.period_start ||
    !cycleForm.value.period_end ||
    !cycleForm.value.pay_date
  ) {
    message.warning(t("common.fillAllFields"));
    return;
  }
  if (cycleForm.value.period_end < cycleForm.value.period_start) {
    message.warning(t("payroll.endAfterStart"));
    return;
  }
  if (cycleForm.value.pay_date < cycleForm.value.period_end) {
    message.warning(t("payroll.payAfterEnd"));
    return;
  }
  createLoading.value = true;
  try {
    await payrollAPI.createCycle({
      name: cycleForm.value.name,
      period_start: format(
        new Date(cycleForm.value.period_start),
        "yyyy-MM-dd"
      ),
      period_end: format(
        new Date(cycleForm.value.period_end),
        "yyyy-MM-dd"
      ),
      pay_date: format(new Date(cycleForm.value.pay_date), "yyyy-MM-dd"),
      cycle_type: cycleForm.value.cycle_type,
    });
    showCreateModal.value = false;
    cycleForm.value = {
      name: "",
      period_start: null,
      period_end: null,
      pay_date: null,
      cycle_type: "regular",
    };
    message.success(t("payroll.cycleCreated"));
    await fetchCycles();
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } };
    message.error(err.data?.error?.message || t("payroll.createFailed"));
  } finally {
    createLoading.value = false;
  }
}

async function handleRunPayroll(row: Record<string, unknown>) {
  dialog.warning({
    title: t("payroll.run"),
    content: t("payroll.runConfirm", { name: row.name }),
    positiveText: t("common.run"),
    negativeText: t("common.cancel"),
    onPositiveClick: async () => {
      try {
        await payrollAPI.runPayroll({ cycle_id: row.id as number });
        message.success(t("payroll.runStarted"));
        await fetchCycles();
      } catch (e: unknown) {
        const err = e as { data?: { error?: { message?: string } } };
        message.error(
          err.data?.error?.message || t("payroll.runFailed")
        );
      }
    },
  });
}

function php(v: unknown): string {
  return Number(v || 0).toLocaleString("en-PH", {
    style: "currency",
    currency: "PHP",
  });
}

async function handleViewItems(row: Record<string, unknown>) {
  itemsTitle.value = String(row.name || "");
  itemsLoading.value = true;
  showItemsModal.value = true;
  try {
    const res = (await payrollAPI.listCycleItems(
      row.id as number
    )) as { data?: Record<string, unknown>[] };
    payrollItems.value = (res.data ||
      (Array.isArray(res) ? res : [])) as Record<string, unknown>[];
  } catch {
    payrollItems.value = [];
  } finally {
    itemsLoading.value = false;
  }
}

// Bank file export
const showBankFileModal = ref(false);
const bankFileFormat = ref("generic");
const bankFileCycleId = ref(0);
const bankFormatOptions = computed(() => [
  { label: t("payroll.bankGeneric"), value: "generic" },
  { label: "UnionBank", value: "unionbank" },
  { label: "BDO", value: "bdo" },
  { label: "Landbank", value: "landbank" },
]);

function handleExportBankFile(row: Record<string, unknown>) {
  bankFileCycleId.value = row.id as number;
  bankFileFormat.value = "generic";
  showBankFileModal.value = true;
}

function downloadBankFile() {
  const url = exportAPI.payrollBankFile(bankFileCycleId.value, bankFileFormat.value);
  const token = localStorage.getItem("token");
  fetch(url, { headers: { Authorization: `Bearer ${token}` } })
    .then((res) => res.blob())
    .then((blob) => {
      const a = document.createElement("a");
      a.href = URL.createObjectURL(blob);
      a.download = `bank_file_${bankFileFormat.value}_${bankFileCycleId.value}.csv`;
      a.click();
      URL.revokeObjectURL(a.href);
    })
    .catch(() => message.error(t("common.failed")));
  showBankFileModal.value = false;
}

async function handleLock(row: Record<string, unknown>) {
  try {
    await payrollAPI.lockCycle(row.id as number);
    message.success(t("payroll.cycleLocked"));
    await fetchCycles();
  } catch {
    message.error(t("common.failed"));
  }
}

async function handleUnlock(row: Record<string, unknown>) {
  try {
    await payrollAPI.unlockCycle(row.id as number);
    message.success(t("payroll.cycleUnlocked"));
    await fetchCycles();
  } catch {
    message.error(t("common.failed"));
  }
}

function handleExportCSV(row: Record<string, unknown>) {
  const url = exportAPI.payrollCSV(row.id as number);
  const token = localStorage.getItem("token");
  // Use fetch to add auth header, then download
  fetch(url, { headers: { Authorization: `Bearer ${token}` } })
    .then((res) => res.blob())
    .then((blob) => {
      const a = document.createElement("a");
      a.href = URL.createObjectURL(blob);
      a.download = `payroll_${row.name || row.id}.csv`;
      a.click();
      URL.revokeObjectURL(a.href);
    })
    .catch(() => message.error(t("common.failed")));
}

async function handleScanAnomalies(row: Record<string, unknown>) {
  anomalyTitle.value = String(row.name || "");
  anomalyLoading.value = true;
  anomalyReport.value = null;
  showAnomalyModal.value = true;
  try {
    const res = (await payrollAPI.scanAnomalies(row.id as number)) as {
      data?: AnomalyReport;
    };
    anomalyReport.value = (res.data || res) as AnomalyReport;
  } catch {
    message.error(t("payroll.anomalyScanFailed"));
  } finally {
    anomalyLoading.value = false;
  }
}

const severityTagType: Record<
  string,
  "error" | "warning" | "info" | "default"
> = {
  critical: "error",
  high: "error",
  medium: "warning",
  low: "info",
};

async function handleApprove(row: Record<string, unknown>) {
  dialog.info({
    title: t("common.approve"),
    content: t("payroll.approveConfirm", { name: row.name }),
    positiveText: t("common.approve"),
    negativeText: t("common.cancel"),
    onPositiveClick: async () => {
      try {
        await payrollAPI.approveCycle(row.id as number);
        message.success(t("payroll.cycleApproved"));
        await fetchCycles();
      } catch (e: unknown) {
        const err = e as { data?: { error?: { message?: string } } };
        message.error(
          err.data?.error?.message || t("payroll.approveFailed")
        );
      }
    },
  });
}

// --- 13th Month Pay ---
const thirteenthMonthYear = ref(new Date().getFullYear());
const thirteenthMonthData = ref<ThirteenthMonthRecord[]>([]);
const thirteenthMonthLoading = ref(false);
const thirteenthMonthCalculating = ref(false);

const thirteenthMonthColumns: DataTableColumns = [
  { title: t("payroll.thirteenth.employeeName"), key: "employee_name" },
  { title: t("employee.employeeNo"), key: "employee_no", width: 120 },
  {
    title: t("payroll.thirteenth.monthsWorked"),
    key: "months_worked",
    width: 120,
  },
  {
    title: t("payroll.thirteenth.totalBasic"),
    key: "total_basic_salary",
    width: 150,
    render(row) {
      return php(row.total_basic_salary);
    },
  },
  {
    title: t("payroll.thirteenth.amount"),
    key: "thirteenth_month_amount",
    width: 160,
    render(row) {
      return php(row.thirteenth_month_amount);
    },
  },
  {
    title: t("payroll.thirteenth.taxExempt"),
    key: "tax_exempt",
    width: 130,
    render(row) {
      return php(row.tax_exempt);
    },
  },
  {
    title: t("payroll.thirteenth.taxable"),
    key: "taxable",
    width: 120,
    render(row) {
      return php(row.taxable);
    },
  },
  {
    title: t("payroll.thirteenth.status"),
    key: "status",
    width: 110,
    render(row) {
      const status = row.status as string;
      return h(
        NTag,
        {
          type: (statusColorMap[status] || "default") as
            | "default"
            | "info"
            | "success"
            | "warning"
            | "error",
          size: "small",
        },
        () => status
      );
    },
  },
];

async function fetchThirteenthMonth() {
  thirteenthMonthLoading.value = true;
  try {
    const res = (await thirteenthMonthAPI.list({
      year: String(thirteenthMonthYear.value),
    })) as { success: boolean; data: ThirteenthMonthRecord[] };
    thirteenthMonthData.value = res.data || (res as unknown as ThirteenthMonthRecord[]);
  } catch {
    thirteenthMonthData.value = [];
  } finally {
    thirteenthMonthLoading.value = false;
  }
}

async function handleCalculateThirteenthMonth() {
  thirteenthMonthCalculating.value = true;
  try {
    await thirteenthMonthAPI.calculate({ year: thirteenthMonthYear.value });
    message.success(t("payroll.thirteenth.calculated"));
    await fetchThirteenthMonth();
  } catch {
    message.error(t("payroll.thirteenth.calculateFailed"));
  } finally {
    thirteenthMonthCalculating.value = false;
  }
}

function handleTabChange(tabName: string) {
  if (tabName === "thirteenth" && thirteenthMonthData.value.length === 0) {
    fetchThirteenthMonth();
  }
  if (tabName === "auto" && !autoConfigLoaded.value) {
    fetchAutoConfig();
    fetchAutoLogs();
  }
}

// === Auto-Payroll Config ===
const autoConfigLoaded = ref(false);
const autoConfigSaving = ref(false);
const autoConfig = ref({
  auto_run_enabled: false,
  days_before_pay: 2,
  auto_approve_enabled: false,
  max_auto_approve_amount: 0,
  notify_on_auto: true,
});
const autoLogs = ref<any[]>([]);
const autoLogsLoading = ref(false);

async function fetchAutoConfig() {
  try {
    const res = await payrollAPI.getAutoConfig();
    const d = (res as any)?.data ?? res;
    autoConfig.value = {
      auto_run_enabled: d.auto_run_enabled ?? false,
      days_before_pay: d.days_before_pay ?? 2,
      auto_approve_enabled: d.auto_approve_enabled ?? false,
      max_auto_approve_amount: parseFloat(d.max_auto_approve_amount) || 0,
      notify_on_auto: d.notify_on_auto ?? true,
    };
    autoConfigLoaded.value = true;
  } catch {
    message.error("Failed to load auto-payroll config");
  }
}

async function saveAutoConfig() {
  autoConfigSaving.value = true;
  try {
    await payrollAPI.updateAutoConfig(autoConfig.value);
    message.success(t("payroll.auto.saved"));
  } catch {
    message.error("Failed to save auto-payroll config");
  } finally {
    autoConfigSaving.value = false;
  }
}

async function fetchAutoLogs() {
  autoLogsLoading.value = true;
  try {
    const res = await payrollAPI.listAutoLogs();
    const d = (res as any)?.data ?? res;
    autoLogs.value = Array.isArray(d) ? d : [];
  } catch { /* ignore */ }
  finally {
    autoLogsLoading.value = false;
  }
}

const actionColor: Record<string, string> = {
  auto_run: 'info',
  auto_approve: 'success',
  auto_skipped: 'warning',
}
</script>

<template>
  <div>
    <NTabs type="line" :value="activeTab" @update:value="(v: string) => { activeTab = v; handleTabChange(v); }">
      <!-- Tab 1: Payroll Cycles (existing content) -->
      <NTabPane name="cycles" :tab="t('payroll.title')">
        <NSpace justify="space-between" style="margin-bottom: 16px">
          <h2>{{ t("payroll.title") }}</h2>
          <NButton type="primary" @click="showCreateModal = true">{{
            t("payroll.createCycle")
          }}</NButton>
        </NSpace>
        <NDataTable :columns="columns" :data="data" :loading="loading" />
      </NTabPane>

      <!-- Tab 2: 13th Month Pay -->
      <NTabPane name="thirteenth" :tab="t('payroll.thirteenth.title')">
        <NSpace justify="space-between" style="margin-bottom: 16px">
          <h2>{{ t("payroll.thirteenth.title") }}</h2>
          <NSpace align="center">
            <span>{{ t("payroll.thirteenth.year") }}:</span>
            <NInputNumber
              v-model:value="thirteenthMonthYear"
              :min="2000"
              :max="2099"
              style="width: 120px"
            />
            <NButton
              type="primary"
              :loading="thirteenthMonthCalculating"
              @click="handleCalculateThirteenthMonth"
            >
              {{ t("payroll.thirteenth.calculate") }}
            </NButton>
          </NSpace>
        </NSpace>
        <NDataTable
          :columns="thirteenthMonthColumns"
          :data="thirteenthMonthData"
          :loading="thirteenthMonthLoading"
        />
      </NTabPane>

      <!-- Tab 3: Auto-Payroll Settings -->
      <NTabPane name="auto" :tab="t('payroll.auto.title')">
        <div style="max-width: 700px;">
          <h2 style="margin-bottom: 16px;">{{ t('payroll.auto.title') }}</h2>
          <p style="opacity: 0.6; margin-bottom: 24px;">{{ t('payroll.auto.subtitle') }}</p>

          <NCard :title="t('payroll.auto.autoRun')" style="margin-bottom: 16px;">
            <NForm label-placement="left" label-width="220">
              <NFormItem :label="t('payroll.auto.enableAutoRun')">
                <NSwitch v-model:value="autoConfig.auto_run_enabled" />
              </NFormItem>
              <NFormItem v-if="autoConfig.auto_run_enabled" :label="t('payroll.auto.daysBefore')">
                <NInputNumber v-model:value="autoConfig.days_before_pay" :min="1" :max="14" style="width: 120px;" />
              </NFormItem>
            </NForm>
          </NCard>

          <NCard :title="t('payroll.auto.autoApprove')" style="margin-bottom: 16px;">
            <NForm label-placement="left" label-width="220">
              <NFormItem :label="t('payroll.auto.enableAutoApprove')">
                <NSwitch v-model:value="autoConfig.auto_approve_enabled" />
              </NFormItem>
              <NFormItem v-if="autoConfig.auto_approve_enabled" :label="t('payroll.auto.maxAmount')">
                <NInputNumber v-model:value="autoConfig.max_auto_approve_amount" :min="0" :step="10000" style="width: 200px;">
                  <template #prefix>PHP</template>
                </NInputNumber>
              </NFormItem>
              <NFormItem v-if="autoConfig.auto_approve_enabled">
                <template #label>
                  <span style="font-size: 12px; opacity: 0.6;">{{ t('payroll.auto.maxAmountHint') }}</span>
                </template>
              </NFormItem>
            </NForm>
          </NCard>

          <NCard :title="t('payroll.auto.notifications')" style="margin-bottom: 16px;">
            <NForm label-placement="left" label-width="220">
              <NFormItem :label="t('payroll.auto.notifyAdmins')">
                <NSwitch v-model:value="autoConfig.notify_on_auto" />
              </NFormItem>
            </NForm>
          </NCard>

          <NButton type="primary" :loading="autoConfigSaving" @click="saveAutoConfig" style="margin-bottom: 32px;">
            {{ t('payroll.auto.save') }}
          </NButton>

          <!-- Auto-Payroll Activity Log -->
          <h3 style="margin-bottom: 12px;">{{ t('payroll.auto.activityLog') }}</h3>
          <NEmpty v-if="autoLogs.length === 0 && !autoLogsLoading" :description="t('payroll.auto.noLogs')" />
          <NTimeline v-else>
            <NTimelineItem
              v-for="log in autoLogs"
              :key="log.id"
              :type="(actionColor[log.action] || 'default') as 'default' | 'info' | 'success' | 'warning' | 'error'"
              :title="t('payroll.auto.action_' + log.action)"
              :content="log.detail || ''"
              :time="new Date(log.created_at).toLocaleString()"
            />
          </NTimeline>
        </div>
      </NTabPane>
    </NTabs>

    <NModal
      v-model:show="showCreateModal"
      preset="card"
      :title="t('payroll.createPayrollCycle')"
      style="width: 500px"
    >
      <NForm label-placement="left" label-width="120">
        <NFormItem :label="t('payroll.cycleName')" required>
          <NInput v-model:value="cycleForm.name" />
        </NFormItem>
        <NFormItem :label="t('payroll.periodStart')" required>
          <NDatePicker
            v-model:value="cycleForm.period_start"
            type="date"
            style="width: 100%"
          />
        </NFormItem>
        <NFormItem :label="t('payroll.periodEnd')" required>
          <NDatePicker
            v-model:value="cycleForm.period_end"
            type="date"
            style="width: 100%"
          />
        </NFormItem>
        <NFormItem :label="t('payroll.payDate')" required>
          <NDatePicker
            v-model:value="cycleForm.pay_date"
            type="date"
            style="width: 100%"
          />
        </NFormItem>
        <NFormItem :label="t('payroll.cycleType')">
          <NSelect
            v-model:value="cycleForm.cycle_type"
            :options="cycleTypeOptions"
          />
        </NFormItem>
        <NFormItem>
          <NSpace>
            <NButton
              type="primary"
              :loading="createLoading"
              @click="handleCreateCycle"
              >{{ t("common.create") }}</NButton
            >
            <NButton @click="showCreateModal = false">{{
              t("common.cancel")
            }}</NButton>
          </NSpace>
        </NFormItem>
      </NForm>
    </NModal>

    <!-- Anomaly Report Modal -->
    <NModal
      v-model:show="showAnomalyModal"
      preset="card"
      :title="t('payroll.anomalyReport') + ' - ' + anomalyTitle"
      style="width: 900px"
    >
      <template v-if="anomalyLoading">
        <div style="text-align: center; padding: 40px">
          {{ t("common.loading") }}
        </div>
      </template>
      <template v-else-if="anomalyReport">
        <NSpace
          style="margin-bottom: 16px"
          align="center"
        >
          <NTag v-if="anomalyReport.summary.critical > 0" type="error" size="small">
            {{ t("payroll.critical") }}: {{ anomalyReport.summary.critical }}
          </NTag>
          <NTag v-if="anomalyReport.summary.high > 0" type="error" size="small">
            {{ t("payroll.highSeverity") }}: {{ anomalyReport.summary.high }}
          </NTag>
          <NTag v-if="anomalyReport.summary.medium > 0" type="warning" size="small">
            {{ t("payroll.mediumSeverity") }}: {{ anomalyReport.summary.medium }}
          </NTag>
          <NTag v-if="anomalyReport.summary.low > 0" type="info" size="small">
            {{ t("payroll.lowSeverity") }}: {{ anomalyReport.summary.low }}
          </NTag>
          <NTag v-if="anomalyReport.anomalies.length === 0" type="success" size="small">
            {{ t("payroll.noAnomalies") }}
          </NTag>
          <span style="color: var(--text-color-3); font-size: 13px">
            {{ t("payroll.totalItems") }}: {{ anomalyReport.total_items }}
          </span>
        </NSpace>
        <NDataTable
          v-if="anomalyReport.anomalies.length > 0"
          :columns="[
            {
              title: t('common.status'),
              key: 'severity',
              width: 90,
              render: (r: Record<string, unknown>) =>
                h(NTag, { type: severityTagType[r.severity as string] || 'default', size: 'small' }, () => r.severity as string),
            },
            { title: t('employee.name'), key: 'employee_name', width: 140 },
            { title: t('employee.employeeNo'), key: 'employee_no', width: 90 },
            { title: t('common.type'), key: 'type', width: 160 },
            { title: t('payroll.anomalyDescription'), key: 'description' },
          ]"
          :data="anomalyReport.anomalies"
          size="small"
          max-height="400"
        />
      </template>
    </NModal>

    <!-- Bank File Export Modal -->
    <NModal
      v-model:show="showBankFileModal"
      preset="card"
      :title="t('payroll.bankFile')"
      style="width: 400px"
    >
      <NForm label-placement="top">
        <NFormItem :label="t('payroll.bankFormat')">
          <NSelect v-model:value="bankFileFormat" :options="bankFormatOptions" />
        </NFormItem>
      </NForm>
      <template #footer>
        <NSpace justify="end">
          <NButton @click="showBankFileModal = false">{{ t("common.cancel") }}</NButton>
          <NButton type="primary" @click="downloadBankFile">{{ t("common.confirm") }}</NButton>
        </NSpace>
      </template>
    </NModal>

    <!-- Payroll Items Modal -->
    <NModal
      v-model:show="showItemsModal"
      preset="card"
      :title="t('payroll.viewItems') + ' - ' + itemsTitle"
      style="width: 900px"
    >
      <NDataTable
        :columns="[
          { title: t('employee.name'), key: 'employee_name' },
          {
            title: t('employee.employeeNo'),
            key: 'employee_no',
            width: 100,
          },
          {
            title: t('payroll.basicPay'),
            key: 'basic_pay',
            width: 110,
            render: (r: Record<string, unknown>) => php(r.basic_pay),
          },
          {
            title: t('payroll.grossPay'),
            key: 'gross_pay',
            width: 110,
            render: (r: Record<string, unknown>) => php(r.gross_pay),
          },
          {
            title: t('payroll.nightDiff'),
            key: 'night_diff',
            width: 100,
            render: (r: Record<string, unknown>) => php(r.night_diff),
          },
          {
            title: t('payroll.holidayPay'),
            key: 'holiday_pay',
            width: 100,
            render: (r: Record<string, unknown>) => php(r.holiday_pay),
          },
          {
            title: t('compliance.sss'),
            key: 'sss_ee',
            width: 90,
            render: (r: Record<string, unknown>) => php(r.sss_ee),
          },
          {
            title: t('compliance.philhealth'),
            key: 'philhealth_ee',
            width: 90,
            render: (r: Record<string, unknown>) => php(r.philhealth_ee),
          },
          {
            title: t('compliance.pagibig'),
            key: 'pagibig_ee',
            width: 90,
            render: (r: Record<string, unknown>) => php(r.pagibig_ee),
          },
          {
            title: t('payroll.deductions'),
            key: 'withholding_tax',
            width: 90,
            render: (r: Record<string, unknown>) =>
              php(r.withholding_tax),
          },
          {
            title: t('payroll.netPay'),
            key: 'net_pay',
            width: 110,
            render: (r: Record<string, unknown>) => php(r.net_pay),
          },
        ]"
        :data="payrollItems"
        :loading="itemsLoading"
        size="small"
      />
    </NModal>
  </div>
</template>
