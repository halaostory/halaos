<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NDataTable, NButton, NSpace, NModal, NForm, NFormItem,
  NInputNumber, NInput, NSelect, NTag,
  useMessage, type DataTableColumns,
} from 'naive-ui'
import { leaveEncashmentAPI } from '../api/client'
import { useAuthStore } from '../stores/auth'
import { useCurrency } from '../composables/useCurrency'
import { format } from 'date-fns'

const { t } = useI18n()
const message = useMessage()
const auth = useAuthStore()
const { formatCurrency } = useCurrency()

interface ConvertibleBalance {
  id: number
  leave_type_id: number
  leave_type_name: string
  code: string
  remaining: number
  earned: number
  used: number
  carried: number
  adjusted: number
}

interface EncashmentRow {
  id: number
  employee_name: string
  employee_no: string
  leave_type_name: string
  year: number
  days: number
  daily_rate: number
  total_amount: number
  status: string
  remarks: string | null
  created_at: string
}

const loading = ref(false)
const balances = ref<ConvertibleBalance[]>([])
const encashments = ref<EncashmentRow[]>([])

// Request form
const showRequestModal = ref(false)
const requestLoading = ref(false)
const requestForm = ref({
  leave_type_id: null as number | null,
  days: 1,
  remarks: '',
})

const leaveTypeOptions = computed(() =>
  balances.value.map(b => ({
    label: `${b.leave_type_name} (${b.remaining} ${t('leaveEncashment.remaining').toLowerCase()})`,
    value: b.leave_type_id,
  }))
)

const selectedBalance = computed(() =>
  balances.value.find(b => b.leave_type_id === requestForm.value.leave_type_id)
)

const maxDays = computed(() => selectedBalance.value?.remaining ?? 0)

function fmtMoney(v: unknown): string {
  const n = Number(v)
  if (isNaN(n)) return '-'
  return formatCurrency(n)
}

function fmtDate(d: unknown): string {
  if (!d) return '-'
  try { return format(new Date(d as string), 'yyyy-MM-dd') } catch { return String(d) }
}

function statusType(s: string): 'default' | 'warning' | 'success' | 'error' | 'info' {
  const map: Record<string, 'default' | 'warning' | 'success' | 'error' | 'info'> = {
    pending: 'warning', approved: 'info', rejected: 'error', paid: 'success',
  }
  return map[s] || 'default'
}

const encashmentColumns = computed<DataTableColumns>(() => {
  const cols: DataTableColumns = []
  if (auth.isManager) {
    cols.push(
      { title: t('leaveEncashment.employee'), key: 'employee_name', width: 150,
        render: (r: any) => `${r.employee_name} (${r.employee_no})` },
    )
  }
  cols.push(
    { title: t('leaveEncashment.leaveType'), key: 'leave_type_name', width: 120 },
    { title: t('leaveEncashment.year'), key: 'year', width: 80 },
    { title: t('leaveEncashment.days'), key: 'days', width: 80 },
    { title: t('leaveEncashment.dailyRate'), key: 'daily_rate', width: 120, render: (r: any) => fmtMoney(r.daily_rate) },
    { title: t('leaveEncashment.totalAmount'), key: 'total_amount', width: 120, render: (r: any) => fmtMoney(r.total_amount) },
    {
      title: t('leaveEncashment.status'), key: 'status', width: 100,
      render: (r: any) => h(NTag, { type: statusType(r.status), size: 'small' },
        () => t(`leaveEncashment.${r.status}`)),
    },
    { title: t('leaveEncashment.remarks'), key: 'remarks', width: 150, ellipsis: { tooltip: true } },
    { title: t('common.date'), key: 'created_at', width: 110, render: (r: any) => fmtDate(r.created_at) },
  )

  if (auth.isManager) {
    cols.push({
      title: t('leaveEncashment.actions'), key: 'actions', width: 200, fixed: 'right',
      render: (r: any) => {
        const btns: ReturnType<typeof h>[] = []
        if (r.status === 'pending') {
          btns.push(
            h(NButton, { size: 'small', type: 'success', onClick: () => handleAction(r.id, 'approve') },
              () => t('leaveEncashment.approve')),
            h(NButton, { size: 'small', type: 'error', onClick: () => handleAction(r.id, 'reject') },
              () => t('leaveEncashment.reject')),
          )
        }
        if (r.status === 'approved') {
          btns.push(
            h(NButton, { size: 'small', type: 'primary', onClick: () => handleAction(r.id, 'paid') },
              () => t('leaveEncashment.markPaid')),
          )
        }
        return h(NSpace, { size: 4 }, () => btns)
      },
    })
  }
  return cols
})

async function fetchBalances() {
  try {
    const res = await leaveEncashmentAPI.getConvertible({ year: String(new Date().getFullYear()) })
    const data = (res as any)?.data ?? res
    balances.value = Array.isArray(data) ? data : []
  } catch {
    balances.value = []
  }
}

async function fetchEncashments() {
  loading.value = true
  try {
    const res = await leaveEncashmentAPI.list()
    const data = (res as any)?.data ?? res
    encashments.value = Array.isArray(data) ? data : []
  } catch {
    encashments.value = []
  } finally {
    loading.value = false
  }
}

async function submitRequest() {
  if (!requestForm.value.leave_type_id || requestForm.value.days <= 0) return
  requestLoading.value = true
  try {
    await leaveEncashmentAPI.create({
      leave_type_id: requestForm.value.leave_type_id,
      year: new Date().getFullYear(),
      days: requestForm.value.days,
      remarks: requestForm.value.remarks || undefined,
    })
    message.success(t('leaveEncashment.requestSubmitted'))
    showRequestModal.value = false
    requestForm.value = { leave_type_id: null, days: 1, remarks: '' }
    await Promise.all([fetchBalances(), fetchEncashments()])
  } catch {
    message.error(t('common.failed'))
  } finally {
    requestLoading.value = false
  }
}

async function handleAction(id: number, action: 'approve' | 'reject' | 'paid') {
  try {
    if (action === 'approve') await leaveEncashmentAPI.approve(id)
    else if (action === 'reject') await leaveEncashmentAPI.reject(id)
    else await leaveEncashmentAPI.markPaid(id)
    message.success(t('leaveEncashment.statusUpdated'))
    await fetchEncashments()
  } catch {
    message.error(t('common.failed'))
  }
}

onMounted(() => {
  fetchBalances()
  fetchEncashments()
})
</script>

<script lang="ts">
import { h } from 'vue'
export default {}
</script>

<template>
  <div>
    <NSpace justify="space-between" align="center" style="margin-bottom: 16px;">
      <h2 style="margin: 0;">{{ t('leaveEncashment.title') }}</h2>
      <NButton type="primary" @click="showRequestModal = true">
        {{ t('leaveEncashment.requestEncashment') }}
      </NButton>
    </NSpace>

    <NDataTable
      :columns="encashmentColumns"
      :data="encashments"
      :loading="loading"
      size="small"
      :scroll-x="1000"
      :max-height="500"
    />

    <!-- Request Modal -->
    <NModal
      v-model:show="showRequestModal"
      :title="t('leaveEncashment.requestEncashment')"
      preset="card"
      style="width: 500px;"
    >
      <NForm label-placement="left" label-width="140">
        <NFormItem :label="t('leaveEncashment.leaveType')" required>
          <NSelect
            v-model:value="requestForm.leave_type_id"
            :options="leaveTypeOptions"
            :placeholder="t('leaveEncashment.leaveType')"
          />
        </NFormItem>
        <div v-if="leaveTypeOptions.length === 0" style="color: #999; margin-bottom: 12px; font-size: 13px;">
          {{ t('leaveEncashment.noConvertible') }}
        </div>
        <NFormItem v-if="selectedBalance" :label="t('leaveEncashment.daysToConvert')" required>
          <NInputNumber
            v-model:value="requestForm.days"
            :min="0.5"
            :max="maxDays"
            :step="0.5"
            style="width: 100%;"
          />
        </NFormItem>
        <NFormItem :label="t('leaveEncashment.remarks')">
          <NInput v-model:value="requestForm.remarks" type="textarea" :rows="2" />
        </NFormItem>
        <NSpace justify="end">
          <NButton @click="showRequestModal = false">{{ t('common.cancel') }}</NButton>
          <NButton
            type="primary"
            :loading="requestLoading"
            :disabled="!requestForm.leave_type_id || requestForm.days <= 0"
            @click="submitRequest"
          >
            {{ t('common.submit') }}
          </NButton>
        </NSpace>
      </NForm>
    </NModal>
  </div>
</template>
