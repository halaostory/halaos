<script setup lang="ts">
import { ref, h, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NTabs, NTabPane, NDataTable, NButton, NModal, NForm, NFormItem,
  NInput, NSelect, NDatePicker, NInputNumber, NSpace, NTag, useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { trainingAPI, certificationAPI, employeeAPI } from '../api/client'
import { useAuthStore } from '../stores/auth'
import { format } from 'date-fns'

const { t } = useI18n()
const message = useMessage()
const auth = useAuthStore()

const trainings = ref<Record<string, unknown>[]>([])
const certifications = ref<Record<string, unknown>[]>([])
const expiring = ref<Record<string, unknown>[]>([])
const loading = ref(false)

// Training Modal
const showTrainingModal = ref(false)
const trainingForm = ref({
  title: '',
  description: '',
  trainer: '',
  training_type: 'internal',
  start_date: null as number | null,
  end_date: null as number | null,
  max_participants: null as number | null,
})

// Certification Modal
const showCertModal = ref(false)
const certForm = ref({
  employee_id: null as number | null,
  name: '',
  issuing_body: '',
  credential_id: '',
  issue_date: null as number | null,
  expiry_date: null as number | null,
})

const employeeOptions = ref<{ label: string; value: number }[]>([])

function fmtDate(d: unknown): string {
  if (!d) return '-'
  try { return format(new Date(d as string), 'yyyy-MM-dd') } catch { return String(d) }
}

const trainingTypeOptions = [
  { label: t('training.internal'), value: 'internal' },
  { label: t('training.external'), value: 'external' },
  { label: t('training.online'), value: 'online' },
]

const statusMap: Record<string, 'success' | 'warning' | 'error' | 'info' | 'default'> = {
  scheduled: 'info', in_progress: 'warning', completed: 'success', cancelled: 'error',
  active: 'success', expired: 'error', revoked: 'default',
}

const trainingColumns: DataTableColumns = [
  { title: t('training.title'), key: 'title', ellipsis: { tooltip: true } },
  { title: t('training.trainer'), key: 'trainer', width: 140 },
  { title: t('training.type'), key: 'training_type', width: 100, render: (r) => t(`training.${r.training_type}`) || String(r.training_type) },
  { title: t('training.startDate'), key: 'start_date', width: 110, render: (r) => fmtDate(r.start_date) },
  { title: t('training.endDate'), key: 'end_date', width: 110, render: (r) => fmtDate(r.end_date) },
  { title: t('training.participants'), key: 'participant_count', width: 90 },
  {
    title: t('common.status'), key: 'status', width: 110,
    render: (r) => h(NTag, { type: statusMap[r.status as string] || 'default', size: 'small' }, () => String(r.status))
  },
]

const certColumns: DataTableColumns = [
  { title: t('employee.name'), key: 'employee_name', width: 150, render: (r) => `${r.first_name} ${r.last_name}` },
  { title: t('training.certName'), key: 'name', ellipsis: { tooltip: true } },
  { title: t('training.issuingBody'), key: 'issuing_body', width: 150 },
  { title: t('training.issueDate'), key: 'issue_date', width: 110, render: (r) => fmtDate(r.issue_date) },
  { title: t('training.expiryDate'), key: 'expiry_date', width: 110, render: (r) => fmtDate(r.expiry_date) },
  {
    title: t('common.status'), key: 'status', width: 100,
    render: (r) => h(NTag, { type: statusMap[r.status as string] || 'default', size: 'small' }, () => String(r.status))
  },
  {
    title: t('common.actions'), key: 'actions', width: 80,
    render: (r) => auth.isAdmin ? h(NButton, { size: 'small', type: 'error', onClick: () => deleteCert(r) }, () => t('common.delete')) : ''
  },
]

const expiringColumns: DataTableColumns = [
  { title: t('employee.name'), key: 'employee_name', width: 150, render: (r) => `${r.first_name} ${r.last_name}` },
  { title: t('training.certName'), key: 'name', ellipsis: { tooltip: true } },
  { title: t('training.expiryDate'), key: 'expiry_date', width: 110, render: (r) => fmtDate(r.expiry_date) },
]

async function loadData() {
  loading.value = true
  try {
    const [tRes, cRes, eRes] = await Promise.allSettled([
      trainingAPI.list(),
      certificationAPI.list(),
      certificationAPI.expiring(),
    ])
    if (tRes.status === 'fulfilled') {
      trainings.value = ((tRes.value as { data: Record<string, unknown>[] }).data) || (Array.isArray(tRes.value) ? tRes.value : []) as Record<string, unknown>[]
    }
    if (cRes.status === 'fulfilled') {
      certifications.value = ((cRes.value as { data: Record<string, unknown>[] }).data) || (Array.isArray(cRes.value) ? cRes.value : []) as Record<string, unknown>[]
    }
    if (eRes.status === 'fulfilled') {
      expiring.value = ((eRes.value as { data: Record<string, unknown>[] }).data) || (Array.isArray(eRes.value) ? eRes.value : []) as Record<string, unknown>[]
    }
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

async function createTraining() {
  if (!trainingForm.value.title || !trainingForm.value.start_date) {
    message.warning(t('common.fillAllFields'))
    return
  }
  try {
    const payload: Record<string, unknown> = {
      title: trainingForm.value.title,
      training_type: trainingForm.value.training_type,
      start_date: format(new Date(trainingForm.value.start_date), 'yyyy-MM-dd'),
    }
    if (trainingForm.value.description) payload.description = trainingForm.value.description
    if (trainingForm.value.trainer) payload.trainer = trainingForm.value.trainer
    if (trainingForm.value.end_date) payload.end_date = format(new Date(trainingForm.value.end_date), 'yyyy-MM-dd')
    if (trainingForm.value.max_participants) payload.max_participants = trainingForm.value.max_participants
    await trainingAPI.create(payload)
    message.success(t('training.created'))
    showTrainingModal.value = false
    trainingForm.value = { title: '', description: '', trainer: '', training_type: 'internal', start_date: null, end_date: null, max_participants: null }
    loadData()
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } }
    message.error(err.data?.error?.message || t('common.saveFailed'))
  }
}

async function createCert() {
  if (!certForm.value.employee_id || !certForm.value.name || !certForm.value.issue_date) {
    message.warning(t('common.fillAllFields'))
    return
  }
  try {
    const payload: Record<string, unknown> = {
      employee_id: certForm.value.employee_id,
      name: certForm.value.name,
      issue_date: format(new Date(certForm.value.issue_date), 'yyyy-MM-dd'),
    }
    if (certForm.value.issuing_body) payload.issuing_body = certForm.value.issuing_body
    if (certForm.value.credential_id) payload.credential_id = certForm.value.credential_id
    if (certForm.value.expiry_date) payload.expiry_date = format(new Date(certForm.value.expiry_date), 'yyyy-MM-dd')
    await certificationAPI.create(payload)
    message.success(t('training.certCreated'))
    showCertModal.value = false
    certForm.value = { employee_id: null, name: '', issuing_body: '', credential_id: '', issue_date: null, expiry_date: null }
    loadData()
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } }
    message.error(err.data?.error?.message || t('common.saveFailed'))
  }
}

async function deleteCert(row: Record<string, unknown>) {
  try {
    await certificationAPI.delete(row.id as number)
    message.success(t('common.delete'))
    loadData()
  } catch {
    message.error(t('common.saveFailed'))
  }
}
</script>

<template>
  <div>
    <NSpace justify="space-between" style="margin-bottom: 16px;">
      <h2>{{ t('training.title') }}</h2>
    </NSpace>

    <NTabs type="line">
      <NTabPane :name="t('training.trainings')">
        <NSpace justify="end" style="margin-bottom: 12px;" v-if="auth.isAdmin">
          <NButton type="primary" size="small" @click="showTrainingModal = true">{{ t('training.addTraining') }}</NButton>
        </NSpace>
        <NDataTable :columns="trainingColumns" :data="trainings" :loading="loading" />
      </NTabPane>
      <NTabPane :name="t('training.certifications')">
        <NSpace justify="end" style="margin-bottom: 12px;" v-if="auth.isAdmin">
          <NButton type="primary" size="small" @click="showCertModal = true">{{ t('training.addCert') }}</NButton>
        </NSpace>
        <NDataTable :columns="certColumns" :data="certifications" :loading="loading" />
      </NTabPane>
      <NTabPane v-if="auth.isAdmin || auth.isManager" :name="t('training.expiring')" :tab="t('training.expiring')">
        <NDataTable :columns="expiringColumns" :data="expiring" :loading="loading" />
      </NTabPane>
    </NTabs>

    <!-- Create Training Modal -->
    <NModal v-model:show="showTrainingModal" :title="t('training.addTraining')" preset="card" style="width: 520px;">
      <NForm @submit.prevent="createTraining">
        <NFormItem :label="t('training.trainingTitle')" required>
          <NInput v-model:value="trainingForm.title" />
        </NFormItem>
        <NFormItem :label="t('training.description')">
          <NInput v-model:value="trainingForm.description" type="textarea" />
        </NFormItem>
        <NFormItem :label="t('training.trainer')">
          <NInput v-model:value="trainingForm.trainer" />
        </NFormItem>
        <NFormItem :label="t('training.type')">
          <NSelect v-model:value="trainingForm.training_type" :options="trainingTypeOptions" />
        </NFormItem>
        <NFormItem :label="t('training.startDate')" required>
          <NDatePicker v-model:value="trainingForm.start_date" type="date" style="width: 100%;" />
        </NFormItem>
        <NFormItem :label="t('training.endDate')">
          <NDatePicker v-model:value="trainingForm.end_date" type="date" style="width: 100%;" />
        </NFormItem>
        <NFormItem :label="t('training.maxParticipants')">
          <NInputNumber v-model:value="trainingForm.max_participants" :min="1" style="width: 100%;" />
        </NFormItem>
        <NSpace>
          <NButton type="primary" attr-type="submit">{{ t('common.save') }}</NButton>
          <NButton @click="showTrainingModal = false">{{ t('common.cancel') }}</NButton>
        </NSpace>
      </NForm>
    </NModal>

    <!-- Create Certification Modal -->
    <NModal v-model:show="showCertModal" :title="t('training.addCert')" preset="card" style="width: 520px;">
      <NForm @submit.prevent="createCert">
        <NFormItem :label="t('employee.name')" required>
          <NSelect v-model:value="certForm.employee_id" :options="employeeOptions" filterable :placeholder="t('training.selectEmployee')" />
        </NFormItem>
        <NFormItem :label="t('training.certName')" required>
          <NInput v-model:value="certForm.name" />
        </NFormItem>
        <NFormItem :label="t('training.issuingBody')">
          <NInput v-model:value="certForm.issuing_body" />
        </NFormItem>
        <NFormItem :label="t('training.credentialId')">
          <NInput v-model:value="certForm.credential_id" />
        </NFormItem>
        <NFormItem :label="t('training.issueDate')" required>
          <NDatePicker v-model:value="certForm.issue_date" type="date" style="width: 100%;" />
        </NFormItem>
        <NFormItem :label="t('training.expiryDate')">
          <NDatePicker v-model:value="certForm.expiry_date" type="date" style="width: 100%;" />
        </NFormItem>
        <NSpace>
          <NButton type="primary" attr-type="submit">{{ t('common.save') }}</NButton>
          <NButton @click="showCertModal = false">{{ t('common.cancel') }}</NButton>
        </NSpace>
      </NForm>
    </NModal>
  </div>
</template>
