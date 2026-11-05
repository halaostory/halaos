<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useI18n } from "vue-i18n";
import {
  List,
  Cell,
  CellGroup,
  Popup,
  Button,
  Divider,
  showToast,
} from "vant";
import { payrollAPI, onboardingChecklistAPI } from "../api/client";
import AiQuickAsk from "../components/ai/AiQuickAsk.vue";
import EmptyState from "../components/EmptyState.vue";
import { format } from "date-fns";
import type { Payslip, ApiResponse } from "../types";

const { t } = useI18n();

const payslips = ref<Payslip[]>([]);

// The API keeps a payslip's figures in `payload` (with a `breakdown`);
// flatten them for display.
const EARN = ["basic_pay", "overtime_pay", "holiday_pay", "night_diff", "bonus_pay", "allowances"];
const DEDUCT = ["sss_ee", "philhealth_ee", "pagibig_ee", "withholding_tax", "late_deduction", "undertime_deduction", "leave_deduction", "federal_tax", "state_tax", "social_security_ee", "medicare_ee", "epf_ee", "cpf_ee", "loan_deduction", "other_deductions"];
function fromApi(raw: Payslip & { payload?: Record<string, unknown> }): Payslip {
  const pl = (raw.payload || {}) as Record<string, unknown>;
  const bd = (pl.breakdown || {}) as Record<string, unknown>;
  const num = (v: unknown) => (typeof v === "number" ? v : Number(v) || 0);
  const items: Payslip["items"] = [];
  for (const k of EARN) if (num(bd[k] ?? pl[k])) items.push({ component_name: t("payslips.c." + k), component_type: "earning", amount: num(bd[k] ?? pl[k]) });
  for (const k of DEDUCT) if (num(bd[k])) items.push({ component_name: t("payslips.c." + k), component_type: "deduction", amount: num(bd[k]) });
  return {
    ...raw,
    net_pay: num(raw.net_pay ?? pl.net_pay),
    gross_pay: num(raw.gross_pay ?? pl.gross_pay),
    basic_salary: num(raw.basic_salary ?? pl.basic_pay),
    total_deductions: num(raw.total_deductions ?? pl.deductions),
    items: raw.items?.length ? raw.items : items,
  };
}
const page = ref(1);
const finished = ref(false);
const loading = ref(false);

const showDetail = ref(false);
const selectedPayslip = ref<Payslip | null>(null);
const detailLoading = ref(false);

async function loadPayslips() {
  loading.value = true;
  try {
    const res = (await payrollAPI.listPayslips({
      page: String(page.value),
      limit: "20",
    })) as ApiResponse<Payslip[]>;
    const items = res.data ?? (res as unknown as Payslip[]);
    if (Array.isArray(items)) {
      payslips.value = [...payslips.value, ...items.map(fromApi)];
      if (items.length < 20) finished.value = true;
      else page.value++;
    } else {
      finished.value = true;
    }
  } catch {
    finished.value = true;
  } finally {
    loading.value = false;
  }
}

async function openDetail(ps: Payslip) {
  showDetail.value = true;
  detailLoading.value = true;
  try {
    const res = (await payrollAPI.getPayslip(ps.id)) as ApiResponse<Payslip>;
    selectedPayslip.value = fromApi(res.data ?? (res as unknown as Payslip));
  } catch {
    showToast({ message: t("common.loadFailed"), type: "fail" });
    selectedPayslip.value = ps;
  } finally {
    detailLoading.value = false;
  }
}

// The PDF needs the Authorization header (a token in the URL would end up
// in server and proxy logs), so fetch it and open the downloaded file.
async function downloadPdf(id: string) {
  const win = window.open("", "_blank"); // opened in the click, so it isn't blocked
  try {
    const res = await fetch(payrollAPI.payslipPdfUrl(id), {
      headers: { Authorization: `Bearer ${localStorage.getItem("token") || ""}` },
    });
    if (!res.ok) throw new Error(String(res.status));
    const url = URL.createObjectURL(await res.blob());
    if (win) win.location.href = url;
    else window.location.href = url;
    setTimeout(() => URL.revokeObjectURL(url), 60_000);
  } catch {
    win?.close();
    showToast({ message: t("common.loadFailed"), type: "fail" });
  }
}

function formatCurrency(n: number) {
  return new Intl.NumberFormat("en-PH", {
    style: "currency",
    currency: "PHP",
  }).format(n);
}

function formatDate(dt: string) {
  return format(new Date(dt), "MMM dd, yyyy");
}

function formatPeriod(start: string, end: string) {
  return `${format(new Date(start), "MMM dd")} - ${format(new Date(end), "MMM dd, yyyy")}`;
}

onMounted(() => {
  onboardingChecklistAPI.completeStep('view_payslip').catch(() => {});
});
</script>

<template>
  <div class="payslips-page">
    <AiQuickAsk :questions="[
      t('ask.payslips1'),
      t('ask.payslips2'),
      t('ask.payslips3'),
    ]" />

    <List
      v-model:loading="loading"
      :finished="finished"
      :finished-text="payslips.length > 0 ? '' : ''"
      @load="loadPayslips"
    >
      <EmptyState
        v-if="payslips.length === 0 && finished"
        icon="💰"
        :title="t('emptyState.payslips.title')"
        :description="t('emptyState.payslips.desc')"
      />
      <CellGroup inset v-if="payslips.length > 0">
        <Cell
          v-for="ps in payslips"
          :key="ps.id"
          is-link
          @click="openDetail(ps)"
        >
          <template #title>
            <div class="payslip-title">
              {{ formatPeriod(ps.period_start, ps.period_end) }}
            </div>
          </template>
          <template #label>
            <div class="payslip-label">
              {{ t("payslips.payDate") }}: {{ formatDate(ps.pay_date) }}
            </div>
          </template>
          <template #right-icon>
            <span class="payslip-amount">{{ formatCurrency(ps.net_pay) }}</span>
          </template>
        </Cell>
      </CellGroup>
    </List>

    <!-- Detail Popup -->
    <Popup
      v-model:show="showDetail"
      position="bottom"
      round
      :style="{ height: '80%' }"
      closeable
    >
      <div class="detail-popup" v-if="selectedPayslip">
        <h3 class="detail-title">{{ t("payslips.details") }}</h3>

        <div class="detail-net">
          <div class="detail-net-label">{{ t("payslips.netPay") }}</div>
          <div class="detail-net-amount">
            {{ formatCurrency(selectedPayslip.net_pay) }}
          </div>
        </div>

        <CellGroup inset>
          <Cell
            :title="t('payslips.basicSalary')"
            :value="formatCurrency(selectedPayslip.basic_salary)"
          />
          <Cell
            :title="t('payslips.grossPay')"
            :value="formatCurrency(selectedPayslip.gross_pay)"
          />
          <Cell
            :title="t('payslips.deductions')"
            :value="formatCurrency(selectedPayslip.total_deductions)"
          />
        </CellGroup>

        <!-- Itemized breakdown -->
        <template v-if="selectedPayslip.items?.length > 0">
          <Divider>{{ t("payslips.earnings") }}</Divider>
          <CellGroup inset>
            <Cell
              v-for="item in selectedPayslip.items.filter(
                (i) => i.component_type === 'earning',
              )"
              :key="item.component_name"
              :title="item.component_name"
              :value="formatCurrency(item.amount)"
            />
          </CellGroup>

          <Divider>{{ t("payslips.deductions") }}</Divider>
          <CellGroup inset>
            <Cell
              v-for="item in selectedPayslip.items.filter(
                (i) => i.component_type === 'deduction',
              )"
              :key="item.component_name"
              :title="item.component_name"
              :value="formatCurrency(item.amount)"
            />
          </CellGroup>
        </template>

        <div class="detail-actions">
          <Button
            round
            block
            type="primary"
            icon="down"
            @click="downloadPdf(selectedPayslip.id)"
          >
            {{ t("payslips.downloadPdf") }}
          </Button>
        </div>
      </div>
    </Popup>
  </div>
</template>

<style scoped>
.payslips-page {
  padding: 8px 0;
}

.payslip-title {
  font-weight: 500;
}

.payslip-label {
  font-size: 12px;
  color: var(--text-secondary);
  margin-top: 4px;
}

.payslip-amount {
  font-size: 15px;
  font-weight: 600;
  color: var(--brand-color);
  white-space: nowrap;
}

.detail-popup {
  padding: 16px;
  overflow-y: auto;
  height: 100%;
}

.detail-title {
  text-align: center;
  font-size: 16px;
  margin-bottom: 16px;
}

.detail-net {
  text-align: center;
  margin-bottom: 20px;
}

.detail-net-label {
  font-size: 13px;
  color: var(--text-secondary);
}

.detail-net-amount {
  font-size: 32px;
  font-weight: 700;
  color: var(--brand-color);
  margin-top: 4px;
}

.detail-actions {
  padding: 20px 0;
}
</style>
