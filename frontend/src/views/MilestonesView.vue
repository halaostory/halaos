<script setup lang="ts">
import { ref, h, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NDataTable, NButton, NModal, NForm, NFormItem, NInput,
  NSpace, NTag, NSelect, useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { milestoneAPI } from '../api/client'
import { format } from 'date-fns'

const { t } = useI18n()
const message = useMessage()

interface Milestone {
  id: number
  employee_id: number
  employee_no: string
  first_name: string
  last_name: string
  employment_type: string
  department_name: string
  milestone_type: string
  milestone_date: string
  days_remaining: number
  status: string
  notes: string | null
  created_at: string
}

const milestones = ref<Milestone[]>([])
const loading = ref(false)
const statusFilter = ref('')
const typeFilter = ref('')
const showNotesModal = ref(false)
const notesText = ref('')
const actionTarget = ref<Milestone | null>(null)
const actionType = ref<'acknowledge' | 'action'>('acknowledge')

const typeColors: Record<string, 'warning' | 'error' | 'info' | 'success'> = {
  probation_ending: 'warning',
  contract_expiring: 'error',
  anniversary: 'info',
  regularization_due: 'success',
}

const statusColors: Record<string, 'warning' | 'success' | 'info' | 'default'> = {
  pending: 'warning',
  acknowledged: 'info',
  actioned: 'success',
}

function fmtDate(d: unknown): string {
  if (!d) return '-'
  try { return format(new Date(d as string), 'yyyy-MM-dd') } catch { return String(d) }
}

const columns: DataTableColumns<Milestone> = [
  {
    title: t('milestone.employee'), key: 'employee_name', width: 180,
    render: (row) => `${row.first_name} ${row.last_name} (${row.employee_no})`
  },
  { title: t('milestone.department'), key: 'department_name', width: 130 },
  {
    title: t('milestone.type'), key: 'milestone_type', width: 150,
    render: (row) => h(NTag, { type: typeColors[row.milestone_type] || 'default', size: 'small' },
      () => t(`milestone.${row.milestone_type}`))
  },
  {
    title: t('milestone.date'), key: 'milestone_date', width: 120,
    render: (row) => fmtDate(row.milestone_date)
  },
  {
    title: t('milestone.daysRemaining'), key: 'days_remaining', width: 100,
    render: (row) => {
      const d = row.days_remaining
      const type = d < 0 ? 'error' : d <= 7 ? 'warning' : 'default'
      return h(NTag, { type, size: 'small' }, () => d < 0 ? `${Math.abs(d)} ${t('milestone.overdue')}` : `${d} ${t('milestone.daysLeft')}`)
    }
  },
  {
    title: t('common.status'), key: 'status', width: 110,
    render: (row) => h(NTag, { type: statusColors[row.status] || 'default', size: 'small' }, () => t(`milestone.status_${row.status}`))
  },
  {
    title: t('common.actions'), key: 'actions', width: 200,
    render: (row) => {
      const buttons: ReturnType<typeof h>[] = []
      if (row.status === 'pending') {
        buttons.push(
          h(NButton, { size: 'small', type: 'info', onClick: () => openAction(row, 'acknowledge') }, () => t('milestone.acknowledge')),
          h(NButton, { size: 'small', type: 'success', onClick: () => openAction(row, 'action'), style: 'margin-left:8px' }, () => t('milestone.takeAction'))
        )
      } else if (row.status === 'acknowledged') {
        buttons.push(
          h(NButton, { size: 'small', type: 'success', onClick: () => openAction(row, 'action') }, () => t('milestone.takeAction'))
        )
      }
      return h(NSpace, { size: 4 }, () => buttons)
    }
  },
]

const statusOptions = [
  { label: t('common.all'), value: '' },
  { label: t('milestone.status_pending'), value: 'pending' },
  { label: t('milestone.status_acknowledged'), value: 'acknowledged' },
  { label: t('milestone.status_actioned'), value: 'actioned' },
]

const typeOptions = [
  { label: t('common.all'), value: '' },
  { label: t('milestone.probation_ending'), value: 'probation_ending' },
  { label: t('milestone.contract_expiring'), value: 'contract_expiring' },
  { label: t('milestone.anniversary'), value: 'anniversary' },
]

async function loadData() {
  loading.value = true
  try {
    const params: Record<string, string> = { page: '1', limit: '100' }
    if (statusFilter.value) params.status = statusFilter.value
    if (typeFilter.value) params.type = typeFilter.value
    const res = await milestoneAPI.list(params) as any
    milestones.value = res?.data?.data || res?.data || []
  } catch {
    message.error(t('common.failed'))
  } finally {
    loading.value = false
  }
}

function openAction(row: Milestone, type: 'acknowledge' | 'action') {
  actionTarget.value = row
  actionType.value = type
  notesText.value = ''
  showNotesModal.value = true
}

async function submitAction() {
  if (!actionTarget.value) return
  try {
    if (actionType.value === 'acknowledge') {
      await milestoneAPI.acknowledge(actionTarget.value.id, notesText.value || undefined)
    } else {
      await milestoneAPI.action(actionTarget.value.id, notesText.value || undefined)
    }
    message.success(t('milestone.actionSuccess'))
    showNotesModal.value = false
    actionTarget.value = null
    loadData()
  } catch {
    message.error(t('common.failed'))
  }
}

onMounted(loadData)
</script>

<template>
  <div>
    <NSpace justify="space-between" style="margin-bottom: 16px;">
      <h2>{{ t('milestone.title') }}</h2>
      <NSpace>
        <NSelect v-model:value="typeFilter" :options="typeOptions" style="width: 180px;" :placeholder="t('milestone.filterType')" @update:value="loadData" />
        <NSelect v-model:value="statusFilter" :options="statusOptions" style="width: 160px;" :placeholder="t('common.status')" @update:value="loadData" />
      </NSpace>
    </NSpace>

    <NDataTable :columns="columns" :data="milestones" :loading="loading" :row-key="(r: Milestone) => r.id" />

    <NModal v-model:show="showNotesModal" :title="actionType === 'acknowledge' ? t('milestone.acknowledge') : t('milestone.takeAction')" preset="card" style="max-width: 450px; width: 95vw;">
      <div v-if="actionTarget" style="margin-bottom: 12px;">
        <p><strong>{{ actionTarget.first_name }} {{ actionTarget.last_name }}</strong> ({{ actionTarget.employee_no }})</p>
        <p>{{ t(`milestone.${actionTarget.milestone_type}`) }} — {{ fmtDate(actionTarget.milestone_date) }}</p>
      </div>
      <NForm @submit.prevent="submitAction">
        <NFormItem :label="t('milestone.notes')">
          <NInput v-model:value="notesText" type="textarea" :placeholder="t('milestone.notesPlaceholder')" />
        </NFormItem>
        <NSpace>
          <NButton type="primary" attr-type="submit">{{ t('common.confirm') }}</NButton>
          <NButton @click="showNotesModal = false">{{ t('common.cancel') }}</NButton>
        </NSpace>
      </NForm>
    </NModal>
  </div>
</template>
