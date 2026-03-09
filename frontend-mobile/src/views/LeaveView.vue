<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useI18n } from "vue-i18n";
import {
  Tab,
  Tabs,
  Cell,
  CellGroup,
  List,
  SwipeCell,
  Button,
  Form,
  Field,
  Popup,
  Picker,
  Calendar,
  Tag,
  showToast,
  showConfirmDialog,
} from "vant";
import { leaveAPI, formPrefillAPI } from "../api/client";
import AiFormAssist from "../components/ai/AiFormAssist.vue";
import { format } from "date-fns";
import type {
  LeaveBalance,
  LeaveType,
  LeaveRequest,
  ApiResponse,
} from "../types";

const { t } = useI18n();

const activeTab = ref(0);

// Balance
const balances = ref<LeaveBalance[]>([]);
const balanceLoading = ref(true);

// Apply form
const leaveTypes = ref<LeaveType[]>([]);
const selectedTypeId = ref<number | null>(null);
const selectedTypeName = ref("");
const showTypePicker = ref(false);
const showStartCal = ref(false);
const showEndCal = ref(false);
const startDate = ref("");
const endDate = ref("");
const reason = ref("");
const submitting = ref(false);
const prefillLoaded = ref(false);
const prefillHint = ref("");

// History
const requests = ref<LeaveRequest[]>([]);
const historyPage = ref(1);
const historyFinished = ref(false);
const historyLoading = ref(false);

async function loadBalances() {
  try {
    const res = (await leaveAPI.getBalances()) as ApiResponse<LeaveBalance[]>;
    balances.value = res.data ?? (res as unknown as LeaveBalance[]);
  } catch {
    showToast({ message: t("common.loadFailed"), type: "fail" });
  } finally {
    balanceLoading.value = false;
  }
}

async function loadTypes() {
  try {
    const res = (await leaveAPI.listTypes()) as ApiResponse<LeaveType[]>;
    leaveTypes.value = res.data ?? (res as unknown as LeaveType[]);
  } catch {
    // ignore
  }
}

async function loadPrefill() {
  if (prefillLoaded.value) return;
  try {
    const res = (await formPrefillAPI.get("leave")) as ApiResponse<{
      leave_type_id: number;
      leave_type: string;
      start_date: string;
      end_date: string;
      days: number;
      reason_hint: string;
    }>;
    const data = res.data ?? (res as unknown as typeof res.data);
    if (data && !selectedTypeId.value) {
      if (data.leave_type_id) {
        selectedTypeId.value = data.leave_type_id;
        selectedTypeName.value = data.leave_type || "";
      }
      if (data.start_date) startDate.value = data.start_date;
      if (data.end_date) endDate.value = data.end_date;
      if (data.reason_hint) prefillHint.value = data.reason_hint;
    }
    prefillLoaded.value = true;
  } catch {
    // non-critical
  }
}

function onTabChange(index: number) {
  if (index === 1) loadPrefill();
}

function onTypeConfirm({ selectedOptions }: { selectedOptions: Array<{ value: number; text: string }> }) {
  const opt = selectedOptions[0];
  if (opt) {
    selectedTypeId.value = opt.value;
    selectedTypeName.value = opt.text;
  }
  showTypePicker.value = false;
}

function onStartConfirm(date: Date) {
  startDate.value = format(date, "yyyy-MM-dd");
  showStartCal.value = false;
}

function onEndConfirm(date: Date) {
  endDate.value = format(date, "yyyy-MM-dd");
  showEndCal.value = false;
}

async function onSubmitLeave() {
  if (!selectedTypeId.value || !startDate.value || !endDate.value || !reason.value) {
    showToast(t("common.failed"));
    return;
  }
  submitting.value = true;
  try {
    await leaveAPI.createRequest({
      leave_type_id: selectedTypeId.value,
      start_date: startDate.value,
      end_date: endDate.value,
      reason: reason.value,
    });
    showToast({ message: t("leave.submitSuccess"), type: "success" });
    // Reset form
    selectedTypeId.value = null;
    selectedTypeName.value = "";
    startDate.value = "";
    endDate.value = "";
    reason.value = "";
    // Reload data
    loadBalances();
    requests.value = [];
    historyPage.value = 1;
    historyFinished.value = false;
  } catch {
    showToast({ message: t("leave.submitFailed"), type: "fail" });
  } finally {
    submitting.value = false;
  }
}

async function loadHistory() {
  historyLoading.value = true;
  try {
    const res = (await leaveAPI.listRequests({
      page: String(historyPage.value),
      limit: "20",
    })) as ApiResponse<LeaveRequest[]>;
    const items = res.data ?? (res as unknown as LeaveRequest[]);
    if (Array.isArray(items)) {
      requests.value = [...requests.value, ...items];
      if (items.length < 20) historyFinished.value = true;
      else historyPage.value++;
    } else {
      historyFinished.value = true;
    }
  } catch {
    historyFinished.value = true;
  } finally {
    historyLoading.value = false;
  }
}

async function cancelRequest(id: number) {
  try {
    await showConfirmDialog({ title: t("common.confirm"), message: t("leave.swipeCancel") + "?" });
    await leaveAPI.cancelRequest(id);
    showToast({ message: t("leave.cancelSuccess"), type: "success" });
    // Remove from list
    requests.value = requests.value.map((r) =>
      r.id === id ? { ...r, status: "cancelled" as const } : r,
    );
  } catch {
    // user cancelled dialog or API error
  }
}

function statusTag(status: string) {
  switch (status) {
    case "approved":
      return "success";
    case "rejected":
      return "danger";
    case "cancelled":
      return "default";
    default:
      return "warning";
  }
}

function onAiReasonSelect(text: string) {
  reason.value = text;
}

onMounted(() => {
  loadBalances();
  loadTypes();
});
</script>

<template>
  <div class="leave-page">
    <Tabs v-model:active="activeTab" sticky @change="onTabChange">
      <!-- Balance Tab -->
      <Tab :title="t('leave.balance')">
        <CellGroup inset>
          <Cell v-for="b in balances" :key="b.leave_type_id" :title="b.leave_type_name">
            <template #label>
              <div class="balance-detail">
                <span>{{ t("leave.total") }}: {{ b.total }}</span>
                <span>{{ t("leave.used") }}: {{ b.used }}</span>
                <span class="balance-remaining">{{ t("leave.remaining") }}: {{ b.remaining }}</span>
              </div>
            </template>
            <template #right-icon>
              <span class="balance-days">{{ t("leave.daysRemaining", { n: b.remaining }) }}</span>
            </template>
          </Cell>
          <Cell v-if="balances.length === 0 && !balanceLoading" :title="t('common.noData')" />
        </CellGroup>
      </Tab>

      <!-- Apply Tab -->
      <Tab :title="t('leave.apply')">
        <div v-if="prefillLoaded && selectedTypeId" class="prefill-banner">
          <van-icon name="bulb-o" size="16" />
          <span>{{ t("ai.prefillHint") }}</span>
        </div>
        <Form @submit="onSubmitLeave" class="leave-form">
          <CellGroup inset>
            <Field
              v-model="selectedTypeName"
              :label="t('leave.type')"
              :placeholder="t('leave.selectType')"
              readonly
              is-link
              @click="showTypePicker = true"
              :rules="[{ required: true }]"
            />
            <Field
              v-model="startDate"
              :label="t('leave.startDate')"
              :placeholder="t('leave.startDate')"
              readonly
              is-link
              @click="showStartCal = true"
              :rules="[{ required: true }]"
            />
            <Field
              v-model="endDate"
              :label="t('leave.endDate')"
              :placeholder="t('leave.endDate')"
              readonly
              is-link
              @click="showEndCal = true"
              :rules="[{ required: true }]"
            />
            <Field
              v-model="reason"
              :label="t('leave.reason')"
              :placeholder="prefillHint || t('leave.reasonPlaceholder')"
              type="textarea"
              rows="3"
              :rules="[{ required: true }]"
            >
              <template #button>
                <AiFormAssist
                  form-type="leave"
                  :leave-type="selectedTypeName"
                  :start-date="startDate"
                  :end-date="endDate"
                  @select="onAiReasonSelect"
                />
              </template>
            </Field>
          </CellGroup>

          <div class="form-actions">
            <Button round block type="primary" native-type="submit" :loading="submitting" size="large">
              {{ t("common.submit") }}
            </Button>
          </div>
        </Form>

        <!-- Type Picker -->
        <Popup v-model:show="showTypePicker" position="bottom" round>
          <Picker
            :columns="leaveTypes.map((lt) => ({ text: lt.name, value: lt.id }))"
            @confirm="onTypeConfirm"
            @cancel="showTypePicker = false"
          />
        </Popup>

        <!-- Start Date Calendar -->
        <Calendar v-model:show="showStartCal" @confirm="onStartConfirm" />
        <!-- End Date Calendar -->
        <Calendar v-model:show="showEndCal" @confirm="onEndConfirm" />
      </Tab>

      <!-- History Tab -->
      <Tab :title="t('leave.history')">
        <List
          v-model:loading="historyLoading"
          :finished="historyFinished"
          :finished-text="requests.length === 0 ? t('leave.noHistory') : ''"
          @load="loadHistory"
        >
          <SwipeCell v-for="r in requests" :key="r.id" :disabled="r.status !== 'pending'">
            <Cell :label="`${r.start_date} ~ ${r.end_date}`">
              <template #title>
                <div class="request-title">
                  <span>{{ r.leave_type_name || `Type #${r.leave_type_id}` }}</span>
                  <Tag :type="statusTag(r.status)" size="medium">
                    {{ t(`leave.${r.status}`) }}
                  </Tag>
                </div>
              </template>
              <template #right-icon>
                <span class="request-days">{{ r.days }}d</span>
              </template>
            </Cell>
            <template #right>
              <Button
                v-if="r.status === 'pending'"
                square
                type="danger"
                :text="t('leave.swipeCancel')"
                class="swipe-btn"
                @click="cancelRequest(r.id)"
              />
            </template>
          </SwipeCell>
        </List>
      </Tab>
    </Tabs>
  </div>
</template>

<style scoped>
.leave-page {
  min-height: 100%;
}

.balance-detail {
  display: flex;
  gap: 12px;
  font-size: 12px;
  color: var(--text-secondary);
  margin-top: 4px;
}

.balance-remaining {
  color: var(--brand-color);
  font-weight: 600;
}

.balance-days {
  font-size: 13px;
  color: var(--brand-color);
  font-weight: 600;
}

.leave-form {
  padding-top: 8px;
}

.form-actions {
  padding: 20px 16px;
}

.request-title {
  display: flex;
  align-items: center;
  gap: 8px;
}

.request-days {
  font-size: 13px;
  color: var(--text-secondary);
}

.swipe-btn {
  height: 100%;
}

.prefill-banner {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 8px 16px 0;
  padding: 8px 12px;
  background: #e8f4fd;
  border-radius: 8px;
  font-size: 12px;
  color: var(--brand-color);
}
</style>
