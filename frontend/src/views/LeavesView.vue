<script setup lang="ts">
import { ref, h, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NTabs, NTabPane, NDataTable, NButton, NModal, NForm, NFormItem,
  NInput, NSelect, NDatePicker, NInputNumber, NSpace, NTag, NSwitch, useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { leaveAPI } from '../api/client'
import { useAuthStore } from '../stores/auth'
import { format } from 'date-fns'

const { t } = useI18n()
const message = useMessage()
const auth = useAuthStore()

const activeTab = ref('')
const requests = ref<Record<string, unknown>[]>([])
const balances = ref<Record<string, unknown>[]>([])
const leaveTypesRaw = ref<Record<string, unknown>[]>([])
const leaveTypes = ref<{ label: string; value: number }[]>([])
const loading = ref(false)
const showModal = ref(false)
const submitting = ref(false)

// Admin: Leave Type Management
const showTypeModal = ref(false)
const typeForm = ref({ name: '', default_days: 0, is_paid: true, is_statutory: false, max_carryover: 5, accrual_type: 'annual' })

// Admin: All Balances & Adjustment
const allBalances = ref<Record<string, unknown>[]>([])
const allBalancesLoading = ref(false)
const showAdjustModal = ref(false)
const adjustTarget = ref<Record<string, unknown> | null>(null)
const adjustDays = ref(0)
const carryoverLoading = ref(false)

const form = ref({
  leave_type_id: null as number | null,
  start_date: null as number | null,
  end_date: null as number | null,
  days: 1,
  reason: '',
})

function resetForm() {
  form.value = { leave_type_id: null, start_date: null, end_date: null, days: 1, reason: '' }
}

function fmtDate(d: unknown): string {
  if (!d) return '-'
  try { return format(new Date(d as string), 'yyyy-MM-dd') } catch { return String(d) }
}

const statusMap: Record<string, 'success' | 'warning' | 'error' | 'info' | 'default'> = {
  pending: 'warning', approved: 'success', rejected: 'error', cancelled: 'default',
}

const columns: DataTableColumns = [
  { title: t('leave.type'), key: 'leave_type_name', width: 140 },
  { title: t('leave.startDate'), key: 'start_date', width: 120, render: (row) => fmtDate(row.start_date) },
  { title: t('leave.endDate'), key: 'end_date', width: 120, render: (row) => fmtDate(row.end_date) },
  { title: t('leave.days'), key: 'days', width: 80 },
  {
    title: t('common.status'), key: 'status', width: 110,
    render: (row) => h(NTag, { type: statusMap[row.status as string] || 'default', size: 'small' }, () => t(`common.${row.status}`) || String(row.status))
  },
  { title: t('leave.reason'), key: 'reason' },
  {
    title: t('common.actions'), key: 'actions', width: 100,
    render: (row) => {
      if (row.status === 'pending') {
        return h(NButton, { size: 'small', type: 'error', onClick: () => cancelRequest(row) }, () => t('common.cancel'))
      }
      return ''
    }
  },
]

const balanceColumns: DataTableColumns = [
  { title: t('leave.type'), key: 'leave_type_name' },
  { title: t('leave.earned'), key: 'earned', width: 100 },
  { title: t('leave.used'), key: 'used', width: 100 },
  { title: t('leave.carried'), key: 'carried', width: 100 },
  { title: t('leave.remaining'), key: 'remaining', width: 100, render: (row) => String(Number(row.earned || 0) + Number(row.carried || 0) - Number(row.used || 0)) },
]

async function loadData() {
  loading.value = true
  try {
    const [reqRes, balRes, typeRes] = await Promise.all([
      leaveAPI.listRequests({ page: '1', limit: '50' }),
      leaveAPI.getBalances(),
      leaveAPI.listTypes(),
    ])
    requests.value = ((reqRes as { data: Record<string, unknown>[] }).data) || (Array.isArray(reqRes) ? reqRes : []) as Record<string, unknown>[]
    balances.value = ((balRes as { data: Record<string, unknown>[] }).data) || (Array.isArray(balRes) ? balRes : []) as Record<string, unknown>[]
    const rawTypes = ((typeRes as { data: { id: number; name: string }[] }).data) || (Array.isArray(typeRes) ? typeRes : []) as { id: number; name: string }[]
    leaveTypesRaw.value = rawTypes as unknown as Record<string, unknown>[]
    leaveTypes.value = rawTypes.map(lt => ({ label: lt.name, value: lt.id }))
  } catch {
    // ok
  } finally {
    loading.value = false
  }
}

onMounted(loadData)

watch(activeTab, (tab) => {
  if (tab === t('leave.allBalances') && allBalances.value.length === 0) {
    loadAllBalances()
  }
})

async function submitRequest() {
  if (!form.value.leave_type_id || !form.value.start_date || !form.value.end_date) {
    message.warning(t('leave.fillRequired'))
    return
  }
  if (form.value.end_date < form.value.start_date) {
    message.warning(t('leave.endAfterStart'))
    return
  }
  submitting.value = true
  try {
    await leaveAPI.createRequest({
      leave_type_id: form.value.leave_type_id,
      start_date: new Date(form.value.start_date).toISOString().split('T')[0],
      end_date: new Date(form.value.end_date).toISOString().split('T')[0],
      days: String(form.value.days),
      reason: form.value.reason || undefined,
    })
    message.success(t('leave.submitted'))
    showModal.value = false
    resetForm()
    loadData()
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } }
    message.error(err.data?.error?.message || t('common.saveFailed'))
  } finally {
    submitting.value = false
  }
}

async function cancelRequest(row: Record<string, unknown>) {
  try {
    await leaveAPI.cancelRequest(row.id as number)
    message.success(t('leave.cancelled'))
    loadData()
  } catch {
    message.error(t('common.saveFailed'))
  }
}

const leaveTypeColumns: DataTableColumns = [
  { title: t('employee.name'), key: 'name' },
  { title: t('leave.defaultDays'), key: 'default_days', width: 100 },
  {
    title: t('leave.paid'), key: 'is_paid', width: 80,
    render: (r) => h(NTag, { type: r.is_paid ? 'success' : 'default', size: 'small' }, () => r.is_paid ? t('common.yes') : t('common.no'))
  },
  {
    title: t('leave.statutory'), key: 'is_statutory', width: 90,
    render: (r) => h(NTag, { type: r.is_statutory ? 'info' : 'default', size: 'small' }, () => r.is_statutory ? t('common.yes') : t('common.no'))
  },
  { title: t('leave.accrualType'), key: 'accrual_type', width: 100 },
  { title: t('leave.maxCarryover'), key: 'max_carryover', width: 110, render: (r) => r.max_carryover != null ? String(r.max_carryover) : '5' },
]

// Admin: All Balances columns
const allBalanceColumns: DataTableColumns = [
  { title: t('employee.employeeNo'), key: 'employee_no', width: 100 },
  { title: t('employee.name'), key: 'employee_name', width: 150, render: (row) => `${row.first_name} ${row.last_name}` },
  { title: t('leave.type'), key: 'leave_type_name', width: 140 },
  { title: t('leave.earned'), key: 'earned', width: 90 },
  { title: t('leave.used'), key: 'used', width: 90 },
  { title: t('leave.carried'), key: 'carried', width: 90 },
  { title: t('leave.adjusted'), key: 'adjusted', width: 90 },
  {
    title: t('leave.remaining'), key: 'remaining', width: 100,
    render: (row) => String(Number(row.earned || 0) + Number(row.carried || 0) + Number(row.adjusted || 0) - Number(row.used || 0))
  },
  {
    title: t('common.actions'), key: 'actions', width: 100,
    render: (row) => h(NButton, { size: 'small', type: 'info', onClick: () => openAdjust(row) }, () => t('leave.adjustBalance'))
  },
]

async function loadAllBalances() {
  allBalancesLoading.value = true
  try {
    const year = new Date().getFullYear()
    const res = await leaveAPI.listAllBalances({ year: String(year) })
    allBalances.value = ((res as { data: Record<string, unknown>[] }).data) || (Array.isArray(res) ? res : []) as Record<string, unknown>[]
  } catch {
    // ok
  } finally {
    allBalancesLoading.value = false
  }
}

function openAdjust(row: Record<string, unknown>) {
  adjustTarget.value = row
  adjustDays.value = Number(row.adjusted || 0)
  showAdjustModal.value = true
}

async function submitAdjust() {
  if (!adjustTarget.value) return
  try {
    await leaveAPI.adjustBalance({
      employee_id: adjustTarget.value.employee_id as number,
      leave_type_id: adjustTarget.value.leave_type_id as number,
      year: adjustTarget.value.year as number,
      adjusted: adjustDays.value,
    })
    message.success(t('leave.adjustSuccess'))
    showAdjustModal.value = false
    adjustTarget.value = null
    loadAllBalances()
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } }
    message.error(err.data?.error?.message || t('common.saveFailed'))
  }
}

async function createLeaveType() {
  if (!typeForm.value.name) {
    message.warning(t('profile.fillAllFields'))
    return
  }
  try {
    await leaveAPI.createType({
      name: typeForm.value.name,
      default_days: typeForm.value.default_days,
      is_paid: typeForm.value.is_paid,
      is_statutory: typeForm.value.is_statutory,
      max_carryover: typeForm.value.max_carryover,
      accrual_type: typeForm.value.accrual_type,
    })
    message.success(t('leave.typeCreated'))
    showTypeModal.value = false
    typeForm.value = { name: '', default_days: 0, is_paid: true, is_statutory: false, max_carryover: 5, accrual_type: 'annual' }
    loadData()
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } }
    message.error(err.data?.error?.message || t('common.saveFailed'))
  }
}

async function runCarryover() {
  const year = new Date().getFullYear()
  carryoverLoading.value = true
  try {
    const res = await leaveAPI.carryover(year - 1, year) as any
    const data = res?.data ?? res
    const carried = data?.carried ?? 0
    const processed = data?.processed ?? 0
    const forfeited = data?.total_forfeited ?? 0
    const msg = forfeited > 0
      ? `${t('leave.carryoverDone')}: ${carried}/${processed}, ${t('leave.forfeited')}: ${forfeited}`
      : `${t('leave.carryoverDone')}: ${carried}/${processed}`
    message.success(msg)
    loadAllBalances()
  } catch {
    message.error(t('common.failed'))
  } finally {
    carryoverLoading.value = false
  }
}
</script>

<template>
  <div>
    <NSpace justify="space-between" style="margin-bottom: 16px;">
      <h2>{{ t('leave.title') }}</h2>
      <NButton type="primary" @click="showModal = true">{{ t('leave.apply') }}</NButton>
    </NSpace>

    <NTabs v-model:value="activeTab" type="line">
      <NTabPane :name="t('leave.myLeaves')">
        <NDataTable :columns="columns" :data="requests" :loading="loading" />
      </NTabPane>
      <NTabPane :name="t('leave.balance')">
        <NDataTable :columns="balanceColumns" :data="balances" />
      </NTabPane>
      <NTabPane v-if="auth.isAdmin" :name="t('leave.allBalances')" :tab="t('leave.allBalances')">
        <NSpace justify="end" style="margin-bottom: 12px;">
          <NButton type="warning" size="small" :loading="carryoverLoading" @click="runCarryover">
            {{ t('leave.carryover') }}
          </NButton>
        </NSpace>
        <NDataTable :columns="allBalanceColumns" :data="allBalances" :loading="allBalancesLoading" />
      </NTabPane>
      <NTabPane v-if="auth.isAdmin" :name="t('leave.manageTypes')" :tab="t('leave.manageTypes')">
        <NSpace justify="end" style="margin-bottom: 12px;">
          <NButton type="primary" size="small" @click="showTypeModal = true">{{ t('common.create') }}</NButton>
        </NSpace>
        <NDataTable :columns="leaveTypeColumns" :data="leaveTypesRaw" />
      </NTabPane>
    </NTabs>

    <!-- Create Leave Type Modal -->
    <NModal v-model:show="showTypeModal" :title="t('leave.createType')" preset="card" style="width: 420px;">
      <NForm @submit.prevent="createLeaveType">
        <NFormItem :label="t('employee.name')" required>
          <NInput v-model:value="typeForm.name" placeholder="e.g. Sick Leave, Vacation Leave" />
        </NFormItem>
        <NFormItem :label="t('leave.defaultDays')">
          <NInputNumber v-model:value="typeForm.default_days" :min="0" :step="1" />
        </NFormItem>
        <NFormItem :label="t('leave.accrualType')">
          <NSelect v-model:value="typeForm.accrual_type" :options="[
            { label: t('leave.accrualAnnual'), value: 'annual' },
            { label: t('leave.accrualMonthly'), value: 'monthly' },
          ]" />
        </NFormItem>
        <NFormItem :label="t('leave.maxCarryover')">
          <NInputNumber v-model:value="typeForm.max_carryover" :min="0" :step="1" />
        </NFormItem>
        <NSpace :size="24">
          <NFormItem :label="t('leave.paid')">
            <NSwitch v-model:value="typeForm.is_paid" />
          </NFormItem>
          <NFormItem :label="t('leave.statutory')">
            <NSwitch v-model:value="typeForm.is_statutory" />
          </NFormItem>
        </NSpace>
        <NSpace>
          <NButton type="primary" attr-type="submit">{{ t('common.save') }}</NButton>
          <NButton @click="showTypeModal = false">{{ t('common.cancel') }}</NButton>
        </NSpace>
      </NForm>
    </NModal>

    <!-- Adjust Balance Modal -->
    <NModal v-model:show="showAdjustModal" :title="t('leave.adjustBalance')" preset="card" style="width: 420px;">
      <div v-if="adjustTarget" style="margin-bottom: 16px;">
        <p><strong>{{ adjustTarget.first_name }} {{ adjustTarget.last_name }}</strong> — {{ adjustTarget.leave_type_name }}</p>
        <p style="color: #666; font-size: 13px;">{{ t('leave.adjustHint') }}</p>
      </div>
      <NForm @submit.prevent="submitAdjust">
        <NFormItem :label="t('leave.adjustDays')">
          <NInputNumber v-model:value="adjustDays" :step="0.5" style="width: 100%;" />
        </NFormItem>
        <NSpace>
          <NButton type="primary" attr-type="submit">{{ t('common.save') }}</NButton>
          <NButton @click="showAdjustModal = false">{{ t('common.cancel') }}</NButton>
        </NSpace>
      </NForm>
    </NModal>

    <NModal v-model:show="showModal" :title="t('leave.apply')" preset="card" style="width: 480px;" @after-leave="resetForm">
      <NForm @submit.prevent="submitRequest">
        <NFormItem :label="t('leave.type')" required>
          <NSelect v-model:value="form.leave_type_id" :options="leaveTypes" :placeholder="t('leave.selectType')" />
        </NFormItem>
        <NFormItem :label="t('leave.startDate')" required>
          <NDatePicker v-model:value="form.start_date" type="date" style="width: 100%;" />
        </NFormItem>
        <NFormItem :label="t('leave.endDate')" required>
          <NDatePicker v-model:value="form.end_date" type="date" style="width: 100%;" />
        </NFormItem>
        <NFormItem :label="t('leave.days')">
          <NInputNumber v-model:value="form.days" :min="0.5" :step="0.5" />
        </NFormItem>
        <NFormItem :label="t('leave.reason')">
          <NInput v-model:value="form.reason" type="textarea" />
        </NFormItem>
        <NSpace>
          <NButton type="primary" :loading="submitting" attr-type="submit">{{ t('common.submit') }}</NButton>
          <NButton @click="showModal = false">{{ t('common.cancel') }}</NButton>
        </NSpace>
      </NForm>
    </NModal>
  </div>
</template>
