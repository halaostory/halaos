<script setup lang="ts">
import { ref, h, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NTabs, NTabPane, NDataTable, NButton, NModal, NForm, NFormItem,
  NInput, NSelect, NInputNumber, NSpace, NTag, NStatistic, NGrid, NGi, useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { taxFilingAPI } from '../api/client'
import { format } from 'date-fns'

const { t } = useI18n()
const message = useMessage()

interface Filing {
  id: number
  filing_type: string
  period_type: string
  period_year: number
  period_month: number | null
  period_quarter: number | null
  due_date: string
  filing_date: string | null
  status: string
  amount: number | string
  penalty_amount: number | string
  reference_no: string | null
  notes: string | null
}

interface Summary {
  total: number
  filed: number
  overdue: number
  upcoming: number
  total_amount: number | string
  total_penalties: number | string
}

const filings = ref<Filing[]>([])
const summary = ref<Summary | null>(null)
const overdueFilings = ref<Filing[]>([])
const upcomingFilings = ref<Filing[]>([])
const loading = ref(false)
const currentYear = ref(new Date().getFullYear())

// Update status modal
const showStatusModal = ref(false)
const statusTarget = ref<Filing | null>(null)
const statusForm = ref({ status: '', reference_no: '', notes: '' })

const statusColors: Record<string, 'warning' | 'success' | 'error' | 'info' | 'default'> = {
  pending: 'warning', generated: 'info', submitted: 'info', filed: 'success', overdue: 'error',
}

const filingTypeLabels: Record<string, string> = {
  bir_1601c: 'BIR 1601-C',
  bir_2316: 'BIR 2316',
  bir_0619e: 'BIR 0619-E',
  sss_r3: 'SSS R-3',
  philhealth_rf1: 'PhilHealth RF-1',
  pagibig_ml1: 'Pag-IBIG ML-1',
}

function fmtDate(d: unknown): string {
  if (!d) return '-'
  try { return format(new Date(d as string), 'yyyy-MM-dd') } catch { return String(d) }
}

function fmtPeriod(f: Filing): string {
  if (f.period_type === 'annual') return String(f.period_year)
  if (f.period_month) return `${f.period_year}-${String(f.period_month).padStart(2, '0')}`
  if (f.period_quarter) return `${f.period_year} Q${f.period_quarter}`
  return String(f.period_year)
}

const columns: DataTableColumns<Filing> = [
  { title: t('taxFiling.type'), key: 'filing_type', width: 130, render: (r) => filingTypeLabels[r.filing_type] || r.filing_type },
  { title: t('taxFiling.period'), key: 'period', width: 100, render: (r) => fmtPeriod(r) },
  { title: t('taxFiling.dueDate'), key: 'due_date', width: 110, render: (r) => fmtDate(r.due_date) },
  { title: t('taxFiling.filingDate'), key: 'filing_date', width: 110, render: (r) => fmtDate(r.filing_date) },
  { title: t('taxFiling.amount'), key: 'amount', width: 120, render: (r) => `₱${Number(r.amount || 0).toLocaleString('en', { minimumFractionDigits: 2 })}` },
  {
    title: t('common.status'), key: 'status', width: 100,
    render: (r) => h(NTag, { type: statusColors[r.status] || 'default', size: 'small' }, () => r.status)
  },
  { title: t('taxFiling.refNo'), key: 'reference_no', width: 120, render: (r) => r.reference_no || '-' },
  {
    title: t('common.actions'), key: 'actions', width: 100,
    render: (r) => {
      if (r.status === 'filed') return ''
      return h(NButton, { size: 'small', type: 'info', onClick: () => openStatusUpdate(r) }, () => t('common.edit'))
    }
  },
]

function openStatusUpdate(filing: Filing) {
  statusTarget.value = filing
  statusForm.value = { status: filing.status, reference_no: filing.reference_no || '', notes: filing.notes || '' }
  showStatusModal.value = true
}

async function loadData() {
  loading.value = true
  try {
    const [filRes, overdueRes, upcomingRes] = await Promise.all([
      taxFilingAPI.list({ year: String(currentYear.value) }),
      taxFilingAPI.overdue(),
      taxFilingAPI.upcoming(),
    ]) as any[]
    filings.value = filRes?.data?.data || []
    summary.value = filRes?.data?.summary || null
    overdueFilings.value = overdueRes?.data || []
    upcomingFilings.value = upcomingRes?.data || []
  } catch {
    message.error(t('common.failed'))
  } finally {
    loading.value = false
  }
}

async function generateAnnual() {
  try {
    await taxFilingAPI.generateAnnual(currentYear.value)
    message.success(t('taxFiling.generated'))
    loadData()
  } catch {
    message.error(t('common.failed'))
  }
}

async function submitStatusUpdate() {
  if (!statusTarget.value) return
  try {
    await taxFilingAPI.updateStatus(statusTarget.value.id, statusForm.value)
    message.success(t('taxFiling.updated'))
    showStatusModal.value = false
    statusTarget.value = null
    loadData()
  } catch {
    message.error(t('common.saveFailed'))
  }
}

onMounted(loadData)
</script>

<template>
  <div>
    <NSpace justify="space-between" style="margin-bottom: 16px;">
      <h2>{{ t('taxFiling.title') }}</h2>
      <NSpace>
        <NInputNumber v-model:value="currentYear" :min="2020" :max="2030" style="width: 100px;" @update:value="loadData" />
        <NButton type="primary" size="small" @click="generateAnnual">{{ t('taxFiling.generateCalendar') }}</NButton>
      </NSpace>
    </NSpace>

    <!-- Summary -->
    <NGrid v-if="summary" :cols="5" :x-gap="16" responsive="screen" style="margin-bottom: 20px;">
      <NGi><NStatistic :label="t('taxFiling.totalFilings')" :value="summary.total" /></NGi>
      <NGi><NStatistic :label="t('taxFiling.filed')"><template #prefix><NTag type="success" size="small">{{ summary.filed }}</NTag></template></NStatistic></NGi>
      <NGi><NStatistic :label="t('taxFiling.overdue')"><template #prefix><NTag type="error" size="small">{{ summary.overdue }}</NTag></template></NStatistic></NGi>
      <NGi><NStatistic :label="t('taxFiling.upcoming')"><template #prefix><NTag type="warning" size="small">{{ summary.upcoming }}</NTag></template></NStatistic></NGi>
      <NGi><NStatistic :label="t('taxFiling.totalAmount')" :value="`₱${Number(summary.total_amount || 0).toLocaleString()}`" /></NGi>
    </NGrid>

    <NTabs type="line">
      <NTabPane :name="t('taxFiling.allFilings')" :tab="t('taxFiling.allFilings')">
        <NDataTable :columns="columns" :data="filings" :loading="loading" :row-key="(r: Filing) => r.id" />
      </NTabPane>
      <NTabPane :name="t('taxFiling.overdue')" :tab="`${t('taxFiling.overdue')} (${overdueFilings.length})`">
        <NDataTable :columns="columns" :data="overdueFilings" :row-key="(r: Filing) => r.id" />
      </NTabPane>
      <NTabPane :name="t('taxFiling.upcoming')" :tab="`${t('taxFiling.upcoming')} (${upcomingFilings.length})`">
        <NDataTable :columns="columns" :data="upcomingFilings" :row-key="(r: Filing) => r.id" />
      </NTabPane>
    </NTabs>

    <!-- Update Status Modal -->
    <NModal v-model:show="showStatusModal" :title="t('taxFiling.updateStatus')" preset="card" style="max-width: 420px; width: 95vw;">
      <NForm @submit.prevent="submitStatusUpdate">
        <NFormItem :label="t('common.status')">
          <NSelect v-model:value="statusForm.status" :options="[
            { label: 'Pending', value: 'pending' },
            { label: 'Generated', value: 'generated' },
            { label: 'Submitted', value: 'submitted' },
            { label: 'Filed', value: 'filed' },
          ]" />
        </NFormItem>
        <NFormItem :label="t('taxFiling.refNo')">
          <NInput v-model:value="statusForm.reference_no" :placeholder="t('taxFiling.refNoPlaceholder')" />
        </NFormItem>
        <NFormItem :label="t('taxFiling.notes')">
          <NInput v-model:value="statusForm.notes" type="textarea" :rows="2" />
        </NFormItem>
        <NSpace>
          <NButton type="primary" attr-type="submit">{{ t('common.save') }}</NButton>
          <NButton @click="showStatusModal = false">{{ t('common.cancel') }}</NButton>
        </NSpace>
      </NForm>
    </NModal>
  </div>
</template>
