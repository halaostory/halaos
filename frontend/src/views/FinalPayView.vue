<script setup lang="ts">
import { ref, h, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NDataTable, NButton, NModal, NForm, NFormItem,
  NSelect, NDatePicker, NInputNumber, NSpace, NTag, NCard,
  NDescriptions, NDescriptionsItem, useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { finalPayAPI, employeeAPI } from '../api/client'
import { format } from 'date-fns'

const { t } = useI18n()
const message = useMessage()

const records = ref<Record<string, unknown>[]>([])
const loading = ref(false)
const showModal = ref(false)
const submitting = ref(false)

const employeeOptions = ref<{ label: string; value: number }[]>([])

const form = ref({
  employee_id: null as number | null,
  separation_date: null as number | null,
  separation_reason: 'resignation',
  unpaid_salary: 0,
  prorated_13th: 0,
  unused_leave_conversion: 0,
  separation_pay: 0,
  tax_refund: 0,
  other_deductions: 0,
})

const reasonOptions = [
  { label: t('finalPay.resignation'), value: 'resignation' },
  { label: t('finalPay.termination'), value: 'termination' },
  { label: t('finalPay.retirement'), value: 'retirement' },
  { label: t('finalPay.endOfContract'), value: 'end_of_contract' },
  { label: t('finalPay.redundancy'), value: 'redundancy' },
  { label: t('finalPay.retrenchment'), value: 'retrenchment' },
]

function fmtDate(d: unknown): string {
  if (!d) return '-'
  try { return format(new Date(d as string), 'yyyy-MM-dd') } catch { return String(d) }
}

function php(v: unknown): string {
  return Number(v || 0).toLocaleString('en-PH', { style: 'currency', currency: 'PHP' })
}

const statusMap: Record<string, 'success' | 'warning' | 'error' | 'info' | 'default'> = {
  calculated: 'info', approved: 'warning', released: 'success', cancelled: 'error',
}

const columns: DataTableColumns = [
  { title: t('employee.employeeNo'), key: 'employee_no', width: 100 },
  { title: t('employee.name'), key: 'name', width: 150, render: (r) => `${r.first_name} ${r.last_name}` },
  { title: t('finalPay.separationDate'), key: 'separation_date', width: 110, render: (r) => fmtDate(r.separation_date) },
  { title: t('finalPay.reason'), key: 'separation_reason', width: 120 },
  { title: t('finalPay.totalFinalPay'), key: 'total_final_pay', width: 130, render: (r) => php(r.total_final_pay) },
  {
    title: t('common.status'), key: 'status', width: 100,
    render: (r) => h(NTag, { type: statusMap[r.status as string] || 'default', size: 'small' }, () => String(r.status))
  },
  {
    title: t('common.actions'), key: 'actions', width: 140,
    render: (r) => {
      const btns: ReturnType<typeof h>[] = []
      if (r.status === 'calculated') {
        btns.push(h(NButton, { size: 'small', type: 'success', onClick: () => updateStatus(r, 'approved') }, () => t('common.approve')))
      }
      if (r.status === 'approved') {
        btns.push(h(NButton, { size: 'small', type: 'info', onClick: () => updateStatus(r, 'released') }, () => t('finalPay.release')))
      }
      return h(NSpace, { size: 4 }, () => btns)
    }
  },
]

async function loadData() {
  loading.value = true
  try {
    const res = await finalPayAPI.list()
    records.value = ((res as { data: Record<string, unknown>[] }).data) || (Array.isArray(res) ? res : []) as Record<string, unknown>[]
  } finally {
    loading.value = false
  }
}

async function loadEmployees() {
  try {
    const res = await employeeAPI.list({ limit: '200', page: '1' })
    const data = ((res as { data: { id: number; first_name: string; last_name: string }[] }).data) || []
    employeeOptions.value = data.map(e => ({ label: `${e.first_name} ${e.last_name}`, value: e.id }))
  } catch { /* ok */ }
}

onMounted(() => { loadData(); loadEmployees() })

function computedTotal(): number {
  return form.value.unpaid_salary + form.value.prorated_13th +
    form.value.unused_leave_conversion + form.value.separation_pay +
    form.value.tax_refund - form.value.other_deductions
}

async function submitFinalPay() {
  if (!form.value.employee_id || !form.value.separation_date) {
    message.warning(t('common.fillAllFields'))
    return
  }
  submitting.value = true
  try {
    await finalPayAPI.create({
      employee_id: form.value.employee_id,
      separation_date: format(new Date(form.value.separation_date), 'yyyy-MM-dd'),
      separation_reason: form.value.separation_reason,
      unpaid_salary: form.value.unpaid_salary,
      prorated_13th: form.value.prorated_13th,
      unused_leave_conversion: form.value.unused_leave_conversion,
      separation_pay: form.value.separation_pay,
      tax_refund: form.value.tax_refund,
      other_deductions: form.value.other_deductions,
    })
    message.success(t('finalPay.created'))
    showModal.value = false
    form.value = { employee_id: null, separation_date: null, separation_reason: 'resignation', unpaid_salary: 0, prorated_13th: 0, unused_leave_conversion: 0, separation_pay: 0, tax_refund: 0, other_deductions: 0 }
    loadData()
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } }
    message.error(err.data?.error?.message || t('common.saveFailed'))
  } finally {
    submitting.value = false
  }
}

async function updateStatus(row: Record<string, unknown>, status: string) {
  try {
    await finalPayAPI.updateStatus(row.id as number, status)
    message.success(t('finalPay.statusUpdated'))
    loadData()
  } catch {
    message.error(t('common.saveFailed'))
  }
}
</script>

<template>
  <div>
    <NSpace justify="space-between" style="margin-bottom: 16px;">
      <h2>{{ t('finalPay.title') }}</h2>
      <NButton type="primary" @click="showModal = true">{{ t('finalPay.compute') }}</NButton>
    </NSpace>

    <NDataTable :columns="columns" :data="records" :loading="loading" />

    <NModal v-model:show="showModal" :title="t('finalPay.compute')" preset="card" style="width: 600px;">
      <NForm @submit.prevent="submitFinalPay" label-placement="left" label-width="180">
        <NFormItem :label="t('employee.name')" required>
          <NSelect v-model:value="form.employee_id" :options="employeeOptions" filterable :placeholder="t('training.selectEmployee')" />
        </NFormItem>
        <NFormItem :label="t('finalPay.separationDate')" required>
          <NDatePicker v-model:value="form.separation_date" type="date" style="width: 100%;" />
        </NFormItem>
        <NFormItem :label="t('finalPay.reason')" required>
          <NSelect v-model:value="form.separation_reason" :options="reasonOptions" />
        </NFormItem>

        <NCard :title="t('finalPay.breakdown')" size="small" style="margin-bottom: 16px;">
          <NFormItem :label="t('finalPay.unpaidSalary')">
            <NInputNumber v-model:value="form.unpaid_salary" :min="0" :precision="2" style="width: 100%;" />
          </NFormItem>
          <NFormItem :label="t('finalPay.prorated13th')">
            <NInputNumber v-model:value="form.prorated_13th" :min="0" :precision="2" style="width: 100%;" />
          </NFormItem>
          <NFormItem :label="t('finalPay.unusedLeave')">
            <NInputNumber v-model:value="form.unused_leave_conversion" :min="0" :precision="2" style="width: 100%;" />
          </NFormItem>
          <NFormItem :label="t('finalPay.separationPay')">
            <NInputNumber v-model:value="form.separation_pay" :min="0" :precision="2" style="width: 100%;" />
          </NFormItem>
          <NFormItem :label="t('finalPay.taxRefund')">
            <NInputNumber v-model:value="form.tax_refund" :min="0" :precision="2" style="width: 100%;" />
          </NFormItem>
          <NFormItem :label="t('finalPay.otherDeductions')">
            <NInputNumber v-model:value="form.other_deductions" :min="0" :precision="2" style="width: 100%;" />
          </NFormItem>
        </NCard>

        <NDescriptions bordered :column="1" style="margin-bottom: 16px;">
          <NDescriptionsItem :label="t('finalPay.totalFinalPay')">
            <strong style="font-size: 16px; color: #18a058;">{{ php(computedTotal()) }}</strong>
          </NDescriptionsItem>
        </NDescriptions>

        <NSpace>
          <NButton type="primary" :loading="submitting" attr-type="submit">{{ t('finalPay.compute') }}</NButton>
          <NButton @click="showModal = false">{{ t('common.cancel') }}</NButton>
        </NSpace>
      </NForm>
    </NModal>
  </div>
</template>
