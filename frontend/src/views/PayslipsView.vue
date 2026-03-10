<script setup lang="ts">
import { ref, h, onMounted } from "vue";
import { useI18n } from "vue-i18n";
import {
  NDataTable,
  NModal,
  NDescriptions,
  NDescriptionsItem,
  NButton,
  NSpace,
  type DataTableColumns,
} from "naive-ui";
import { payrollAPI } from "../api/client";
import { format } from "date-fns";
import { useCurrency } from "../composables/useCurrency";

const { t } = useI18n();
const { formatCurrency } = useCurrency();
const data = ref<Record<string, unknown>[]>([]);
const loading = ref(false);
const showDetail = ref(false);
const selectedPayslip = ref<Record<string, unknown> | null>(null);

function fmtDate(d: unknown): string {
  if (!d) return "-";
  try {
    return format(new Date(d as string), "yyyy-MM-dd");
  } catch {
    return String(d);
  }
}

const columns: DataTableColumns = [
  {
    title: t("payroll.period"),
    key: "period",
    render: (row) =>
      `${fmtDate(row.period_start)} ~ ${fmtDate(row.period_end)}`,
  },
  {
    title: t("payroll.payDate"),
    key: "pay_date",
    width: 120,
    render: (row) => fmtDate(row.pay_date),
  },
  {
    title: t("approval.created"),
    key: "created_at",
    width: 160,
    render: (row) => fmtDate(row.created_at),
  },
  {
    title: t("common.actions"),
    key: "actions",
    width: 100,
    render: (row) => {
      return h(NSpace, { size: 4 }, () => [
        h(
          NButton,
          { size: "small", onClick: () => viewDetail(row) },
          () => t("common.view")
        ),
        h(
          NButton,
          { size: "small", type: "primary", onClick: () => downloadPdf(row) },
          () => "PDF"
        ),
      ]);
    },
  },
];

function viewDetail(row: Record<string, unknown>) {
  selectedPayslip.value = row;
  showDetail.value = true;
}

function downloadPdf(row: Record<string, unknown>) {
  const url = payrollAPI.payslipPdfUrl(String(row.id));
  const token = localStorage.getItem("token");
  const a = document.createElement("a");
  // Use fetch to include auth header
  fetch(url, { headers: { Authorization: `Bearer ${token}` } })
    .then((res) => res.blob())
    .then((blob) => {
      a.href = URL.createObjectURL(blob);
      a.download = `payslip_${row.id}.pdf`;
      a.click();
      URL.revokeObjectURL(a.href);
    });
}

function getPayload(
  payslip: Record<string, unknown>
): Record<string, unknown> {
  if (!payslip.payload) return {};
  if (typeof payslip.payload === "string") {
    try {
      return JSON.parse(payslip.payload);
    } catch {
      return {};
    }
  }
  return payslip.payload as Record<string, unknown>;
}

onMounted(async () => {
  loading.value = true;
  try {
    const res = (await payrollAPI.listPayslips({
      page: "1",
      limit: "50",
    })) as { data: Record<string, unknown>[] };
    data.value = (res.data ||
      (Array.isArray(res) ? res : [])) as Record<string, unknown>[];
  } catch {
    data.value = [];
  } finally {
    loading.value = false;
  }
});
</script>

<template>
  <div>
    <h2 style="margin-bottom: 16px">{{ t("nav.payslips") }}</h2>
    <NDataTable :columns="columns" :data="data" :loading="loading" />

    <NModal
      v-model:show="showDetail"
      preset="card"
      :title="t('payroll.payslipDetail')"
      style="width: 600px"
    >
      <template v-if="selectedPayslip">
        <NDescriptions bordered :column="2">
          <NDescriptionsItem :label="t('payroll.period')">{{
            fmtDate(selectedPayslip.period_start)
          }}
            ~
            {{ fmtDate(selectedPayslip.period_end) }}</NDescriptionsItem
          >
          <NDescriptionsItem :label="t('payroll.payDate')">{{
            fmtDate(selectedPayslip.pay_date)
          }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('employee.name')">{{
            getPayload(selectedPayslip).employee_name || "-"
          }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('employee.employeeNo')">{{
            getPayload(selectedPayslip).employee_no || "-"
          }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('payroll.basicPay')">{{
            formatCurrency(getPayload(selectedPayslip).basic_pay)
          }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('payroll.grossPay')">{{
            formatCurrency(getPayload(selectedPayslip).gross_pay)
          }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('payroll.deductions')">{{
            formatCurrency(getPayload(selectedPayslip).deductions)
          }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('payroll.netPay')">
            <strong>{{
              formatCurrency(getPayload(selectedPayslip).net_pay)
            }}</strong>
          </NDescriptionsItem>
        </NDescriptions>
        <div style="margin-top: 16px; text-align: right;">
          <NButton type="primary" @click="downloadPdf(selectedPayslip!)">
            Download PDF
          </NButton>
        </div>
      </template>
    </NModal>
  </div>
</template>
