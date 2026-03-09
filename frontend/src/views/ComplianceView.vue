<script setup lang="ts">
import { ref, h, onMounted } from "vue";
import { useI18n } from "vue-i18n";
import {
  NTabs, NTabPane, NDataTable, NTag, NEmpty, NButton, NSpace, NSelect,
  NModal, NForm, NFormItem, useMessage, type DataTableColumns,
} from "naive-ui";
import { complianceAPI, reportsAPI } from "../api/client";
import { format } from "date-fns";

const { t } = useI18n();
const message = useMessage();

const sssData = ref<Record<string, unknown>[]>([]);
const philhealthData = ref<Record<string, unknown>[]>([]);
const pagibigData = ref<Record<string, unknown>[]>([]);
const birData = ref<Record<string, unknown>[]>([]);
const govFormsData = ref<Record<string, unknown>[]>([]);
const loading = ref(false);

// Form generation
const showGenModal = ref(false);
const genLoading = ref(false);
const genForm = ref({
  form_type: "BIR_2316",
  tax_year: new Date().getFullYear(),
  month: new Date().getMonth() + 1,
});

const formTypeOptions = [
  { label: "BIR 2316 (Annual Tax Certificate)", value: "BIR_2316" },
  { label: "SSS R-3 (Monthly Contribution)", value: "SSS_R3" },
  { label: "PhilHealth RF1 (Monthly Remittance)", value: "PHILHEALTH_RF1" },
  { label: "BIR 1601-C (Monthly Withholding Tax)", value: "BIR_1601C" },
];

const yearOptions = Array.from({ length: 5 }, (_, i) => {
  const y = new Date().getFullYear() - 2 + i;
  return { label: String(y), value: y };
});

const monthOptions = [
  { label: "January", value: 1 }, { label: "February", value: 2 },
  { label: "March", value: 3 }, { label: "April", value: 4 },
  { label: "May", value: 5 }, { label: "June", value: 6 },
  { label: "July", value: 7 }, { label: "August", value: 8 },
  { label: "September", value: 9 }, { label: "October", value: 10 },
  { label: "November", value: 11 }, { label: "December", value: 12 },
];

const isMonthlyForm = (type: string) => type !== "BIR_2316";

async function handleGenerate() {
  genLoading.value = true;
  try {
    const payload: Record<string, unknown> = {
      form_type: genForm.value.form_type,
      tax_year: genForm.value.tax_year,
    };
    if (isMonthlyForm(genForm.value.form_type)) {
      payload.month = genForm.value.month;
    }
    await complianceAPI.generateForm(payload);
    message.success(t("compliance.formGenerated"));
    showGenModal.value = false;
    await loadGovForms();
  } catch {
    message.error(t("common.failed"));
  } finally {
    genLoading.value = false;
  }
}

function fmt(v: unknown): string {
  if (v === null || v === undefined) return "-";
  const n = Number(v);
  return isNaN(n)
    ? String(v)
    : n.toLocaleString("en-PH", {
        minimumFractionDigits: 2,
        maximumFractionDigits: 2,
      });
}

function pct(v: unknown): string {
  if (v === null || v === undefined) return "-";
  return `${(Number(v) * 100).toFixed(1)}%`;
}

const sssColumns: DataTableColumns = [
  { title: t("compliance.msc"), key: "msc", width: 100, render: (r) => fmt(r.msc) },
  { title: t("compliance.salaryMin"), key: "msc_min", width: 120, render: (r) => fmt(r.msc_min) },
  { title: t("compliance.salaryMax"), key: "msc_max", width: 120, render: (r) => fmt(r.msc_max) },
  { title: t("compliance.eeShare"), key: "ee_share", width: 100, render: (r) => fmt(r.ee_share) },
  { title: t("compliance.erShare"), key: "er_share", width: 100, render: (r) => fmt(r.er_share) },
  { title: t("compliance.ec"), key: "ec", width: 60, render: (r) => fmt(r.ec) },
  { title: t("compliance.total"), key: "total", width: 100, render: (r) => fmt(r.total) },
];

const philhealthColumns: DataTableColumns = [
  { title: t("compliance.salaryMin"), key: "salary_min", render: (r) => fmt(r.salary_min) },
  { title: t("compliance.salaryMax"), key: "salary_max", render: (r) => fmt(r.salary_max) },
  { title: t("compliance.premiumRate"), key: "premium_rate", render: (r) => pct(r.premium_rate) },
  { title: t("compliance.eeShare"), key: "ee_share_rate", render: (r) => pct(r.ee_share_rate) },
  { title: t("compliance.erShare"), key: "er_share_rate", render: (r) => pct(r.er_share_rate) },
  { title: t("compliance.floor"), key: "floor_premium", render: (r) => fmt(r.floor_premium) },
  { title: t("compliance.ceiling"), key: "ceiling_premium", render: (r) => fmt(r.ceiling_premium) },
];

const pagibigColumns: DataTableColumns = [
  { title: t("compliance.salaryMin"), key: "salary_min", render: (r) => fmt(r.salary_min) },
  { title: t("compliance.salaryMax"), key: "salary_max", render: (r) => fmt(r.salary_max) },
  { title: t("compliance.eeRate"), key: "ee_rate", render: (r) => pct(r.ee_rate) },
  { title: t("compliance.erRate"), key: "er_rate", render: (r) => pct(r.er_rate) },
  { title: t("compliance.maxEE"), key: "max_ee", render: (r) => fmt(r.max_ee) },
  { title: t("compliance.maxER"), key: "max_er", render: (r) => fmt(r.max_er) },
];

const birColumns: DataTableColumns = [
  { title: t("compliance.frequency"), key: "frequency", width: 120 },
  { title: t("compliance.bracketMin"), key: "bracket_min", render: (r) => fmt(r.bracket_min) },
  { title: t("compliance.bracketMax"), key: "bracket_max", render: (r) => fmt(r.bracket_max) },
  { title: t("compliance.fixedTax"), key: "fixed_tax", render: (r) => fmt(r.fixed_tax) },
  { title: t("compliance.rate"), key: "rate", render: (r) => pct(r.rate) },
  { title: t("compliance.excessOver"), key: "excess_over", render: (r) => fmt(r.excess_over) },
];

function fmtDate(d: unknown): string {
  if (!d) return "-";
  try { return format(new Date(d as string), "yyyy-MM-dd"); } catch { return String(d); }
}

function formTypeLabel(type: string): string {
  const map: Record<string, string> = {
    BIR_2316: "BIR 2316",
    SSS_R3: "SSS R-3",
    PHILHEALTH_RF1: "PhilHealth RF1",
    BIR_1601C: "BIR 1601-C",
  };
  return map[type] || type;
}

const govFormColumns: DataTableColumns = [
  {
    title: t("common.type"), key: "form_type", width: 140,
    render: (r) => h(NTag, { size: "small", type: "info" }, () => formTypeLabel(r.form_type as string)),
  },
  { title: "Tax Year", key: "tax_year", width: 100 },
  { title: t("payroll.period"), key: "period", width: 120 },
  {
    title: t("common.status"), key: "status", width: 100,
    render: (r) => h(NTag, { type: r.status === "submitted" ? "success" : "default", size: "small" }, () => String(r.status)),
  },
  { title: t("approval.created"), key: "created_at", width: 160, render: (r) => fmtDate(r.created_at) },
  {
    title: t("common.actions"), key: "actions", width: 80,
    render: (r) => h(NButton, {
      size: "small", quaternary: true,
      onClick: () => viewFormPayload(r),
    }, () => t("common.view")),
  },
];

function viewFormPayload(row: Record<string, unknown>) {
  const payload = row.payload;
  if (payload) {
    const data = typeof payload === "string" ? JSON.parse(payload) : payload;
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `${row.form_type}_${row.tax_year}${row.period ? "_" + row.period : ""}.json`;
    a.click();
    URL.revokeObjectURL(url);
  }
}

function extract(res: unknown): Record<string, unknown>[] {
  const r = res as { data?: Record<string, unknown>[] };
  return (r.data || (Array.isArray(r) ? r : [])) as Record<string, unknown>[];
}

const doleLoading = ref(false);

async function downloadDOLERegister() {
  doleLoading.value = true;
  try {
    const token = localStorage.getItem("token");
    const res = await fetch(reportsAPI.doleRegisterUrl(), {
      headers: { Authorization: `Bearer ${token}` },
    });
    if (!res.ok) throw new Error("Download failed");
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `DOLE_Register_${new Date().toISOString().slice(0, 10)}.pdf`;
    a.click();
    URL.revokeObjectURL(url);
    message.success(t("compliance.downloaded"));
  } catch {
    message.error(t("common.failed"));
  } finally {
    doleLoading.value = false;
  }
}

async function loadGovForms() {
  try {
    const gf = await complianceAPI.listGovernmentForms();
    govFormsData.value = extract(gf);
  } catch (e) { console.error('Failed to load government forms', e) }
}

onMounted(async () => {
  loading.value = true;
  try {
    const [sss, ph, pi, bir] = await Promise.allSettled([
      complianceAPI.listSSS(),
      complianceAPI.listPhilHealth(),
      complianceAPI.listPagIBIG(),
      complianceAPI.listBIRTax(),
    ]);
    if (sss.status === "fulfilled") sssData.value = extract(sss.value);
    if (ph.status === "fulfilled") philhealthData.value = extract(ph.value);
    if (pi.status === "fulfilled") pagibigData.value = extract(pi.value);
    if (bir.status === "fulfilled") birData.value = extract(bir.value);
    await loadGovForms();
  } finally {
    loading.value = false;
  }
});
</script>

<template>
  <div>
    <h2 style="margin-bottom: 16px">{{ t("compliance.title") }}</h2>
    <NTabs type="line">
      <NTabPane name="SSS" :tab="t('compliance.sss')">
        <NDataTable :columns="sssColumns" :data="sssData" :loading="loading" size="small" :max-height="500" />
      </NTabPane>
      <NTabPane name="PhilHealth" :tab="t('compliance.philhealth')">
        <NDataTable :columns="philhealthColumns" :data="philhealthData" :loading="loading" size="small" />
      </NTabPane>
      <NTabPane name="PagIBIG" :tab="t('compliance.pagibig')">
        <NDataTable :columns="pagibigColumns" :data="pagibigData" :loading="loading" size="small" />
      </NTabPane>
      <NTabPane name="BIR" :tab="t('compliance.birTax')">
        <NDataTable :columns="birColumns" :data="birData" :loading="loading" size="small" />
      </NTabPane>
      <NTabPane name="GovForms" :tab="t('compliance.govForms')">
        <NSpace justify="end" style="margin-bottom: 12px;">
          <NButton type="primary" @click="showGenModal = true">{{ t('compliance.generateForm') }}</NButton>
        </NSpace>
        <NDataTable :columns="govFormColumns" :data="govFormsData" :loading="loading" size="small" />
        <NEmpty v-if="!loading && govFormsData.length === 0" :description="t('common.noPending')" style="margin-top: 24px;" />
      </NTabPane>
      <NTabPane name="DOLERegister" :tab="t('compliance.doleRegister')">
        <NSpace vertical align="center" style="padding: 40px 0;">
          <p style="color: #666; margin-bottom: 16px;">{{ t('compliance.downloadDOLERegister') }}</p>
          <NButton type="primary" :loading="doleLoading" @click="downloadDOLERegister">
            {{ t('compliance.downloadDOLERegister') }}
          </NButton>
        </NSpace>
      </NTabPane>
    </NTabs>

    <NModal v-model:show="showGenModal" :title="t('compliance.generateForm')" preset="card" style="max-width: 500px; width: 95vw;">
      <NForm label-placement="left" label-width="120">
        <NFormItem :label="t('common.type')" required>
          <NSelect v-model:value="genForm.form_type" :options="formTypeOptions" />
        </NFormItem>
        <NFormItem label="Tax Year" required>
          <NSelect v-model:value="genForm.tax_year" :options="yearOptions" />
        </NFormItem>
        <NFormItem v-if="isMonthlyForm(genForm.form_type)" label="Month" required>
          <NSelect v-model:value="genForm.month" :options="monthOptions" />
        </NFormItem>
        <NSpace justify="end">
          <NButton @click="showGenModal = false">{{ t('common.cancel') }}</NButton>
          <NButton type="primary" :loading="genLoading" @click="handleGenerate">{{ t('compliance.generateForm') }}</NButton>
        </NSpace>
      </NForm>
    </NModal>
  </div>
</template>
