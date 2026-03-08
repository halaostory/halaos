<script setup lang="ts">
import { ref, h, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NTabs, NTabPane, NDataTable, NButton, NModal, NForm, NFormItem,
  NInput, NSelect, NDatePicker, NInputNumber, NSpace, NTag, useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { disciplinaryAPI, employeeAPI } from '../api/client'
import { format } from 'date-fns'

const { t } = useI18n()
const message = useMessage()

const activeTab = ref('incidents')
const incidents = ref<Record<string, unknown>[]>([])
const actions = ref<Record<string, unknown>[]>([])
const loading = ref(false)
const employees = ref<{ label: string; value: number }[]>([])

// Incident form
const showIncidentModal = ref(false)
const incidentForm = ref({
  employee_id: null as number | null,
  incident_date: null as number | null,
  category: '',
  severity: 'minor',
  description: '',
  witnesses: '',
  evidence_notes: '',
})

// Action form
const showActionModal = ref(false)
const actionForm = ref({
  employee_id: null as number | null,
  incident_id: null as number | null,
  action_type: '',
  action_date: null as number | null,
  description: '',
  suspension_days: null as number | null,
  effective_date: null as number | null,
  end_date: null as number | null,
  notes: '',
})

// Appeal form
const showAppealModal = ref(false)
const appealTarget = ref<Record<string, unknown> | null>(null)
const appealReason = ref('')

const categoryOptions = [
  { label: t('disciplinary.tardiness'), value: 'tardiness' },
  { label: t('disciplinary.absence'), value: 'absence' },
  { label: t('disciplinary.misconduct'), value: 'misconduct' },
  { label: t('disciplinary.insubordination'), value: 'insubordination' },
  { label: t('disciplinary.policyViolation'), value: 'policy_violation' },
  { label: t('disciplinary.performance'), value: 'performance' },
  { label: t('disciplinary.safety'), value: 'safety' },
]

const severityOptions = [
  { label: t('disciplinary.minor'), value: 'minor' },
  { label: t('disciplinary.moderate'), value: 'moderate' },
  { label: t('disciplinary.major'), value: 'major' },
  { label: t('disciplinary.grave'), value: 'grave' },
]

const actionTypeOptions = [
  { label: t('disciplinary.verbalWarning'), value: 'verbal_warning' },
  { label: t('disciplinary.writtenWarning'), value: 'written_warning' },
  { label: t('disciplinary.finalWarning'), value: 'final_warning' },
  { label: t('disciplinary.suspension'), value: 'suspension' },
  { label: t('disciplinary.termination'), value: 'termination' },
]

const severityColors: Record<string, 'info' | 'warning' | 'error' | 'default'> = {
  minor: 'info', moderate: 'warning', major: 'error', grave: 'error',
}

const statusColors: Record<string, 'warning' | 'info' | 'success' | 'default'> = {
  open: 'warning', under_review: 'info', resolved: 'success', dismissed: 'default',
}

function fmtDate(d: unknown): string {
  if (!d) return '-'
  try { return format(new Date(d as string), 'yyyy-MM-dd') } catch { return String(d) }
}

const incidentColumns: DataTableColumns = [
  { title: t('disciplinary.employee'), key: 'employee_name', width: 160, render: (r) => `${r.first_name} ${r.last_name}` },
  { title: t('disciplinary.incidentDate'), key: 'incident_date', width: 110, render: (r) => fmtDate(r.incident_date) },
  { title: t('disciplinary.category'), key: 'category', width: 120, render: (r) => t(`disciplinary.${r.category}`) || String(r.category) },
  { title: t('disciplinary.severity'), key: 'severity', width: 90, render: (r) => h(NTag, { type: severityColors[r.severity as string] || 'default', size: 'small' }, () => t(`disciplinary.${r.severity}`)) },
  { title: t('common.status'), key: 'status', width: 100, render: (r) => h(NTag, { type: statusColors[r.status as string] || 'default', size: 'small' }, () => String(r.status)) },
  { title: t('disciplinary.description'), key: 'description', ellipsis: { tooltip: true } },
]

const actionColumns: DataTableColumns = [
  { title: t('disciplinary.employee'), key: 'employee_name', width: 160, render: (r) => `${r.first_name} ${r.last_name}` },
  { title: t('disciplinary.actionDate'), key: 'action_date', width: 110, render: (r) => fmtDate(r.action_date) },
  { title: t('disciplinary.actionType'), key: 'action_type', width: 140, render: (r) => {
    const colors: Record<string, 'info' | 'warning' | 'error'> = {
      verbal_warning: 'info', written_warning: 'warning', final_warning: 'error', suspension: 'error', termination: 'error',
    }
    return h(NTag, { type: colors[r.action_type as string] || 'default', size: 'small' }, () => t(`disciplinary.${r.action_type}`) || String(r.action_type))
  }},
  { title: t('disciplinary.description'), key: 'description', ellipsis: { tooltip: true } },
  { title: t('disciplinary.acknowledged'), key: 'employee_acknowledged', width: 100, render: (r) => h(NTag, { type: r.employee_acknowledged ? 'success' : 'default', size: 'small' }, () => r.employee_acknowledged ? t('common.yes') : t('common.no')) },
  { title: t('disciplinary.appeal'), key: 'appeal_status', width: 110, render: (r) => r.appeal_status ? h(NTag, { type: r.appeal_status === 'appealed' ? 'warning' : 'info', size: 'small' }, () => String(r.appeal_status)) : '-' },
]

async function loadEmployees() {
  try {
    const res = await employeeAPI.list({ page: '1', limit: '500' }) as any
    const emps = res?.data?.data || res?.data || []
    employees.value = emps.map((e: any) => ({ label: `${e.first_name} ${e.last_name} (${e.employee_no})`, value: e.id }))
  } catch { /* ok */ }
}

async function loadIncidents() {
  loading.value = true
  try {
    const res = await disciplinaryAPI.listIncidents({ page: '1', limit: '100' }) as any
    incidents.value = res?.data?.data || res?.data || []
  } catch { message.error(t('common.failed')) }
  finally { loading.value = false }
}

async function loadActions() {
  loading.value = true
  try {
    const res = await disciplinaryAPI.listActions({ page: '1', limit: '100' }) as any
    actions.value = res?.data?.data || res?.data || []
  } catch { message.error(t('common.failed')) }
  finally { loading.value = false }
}

async function submitIncident() {
  if (!incidentForm.value.employee_id || !incidentForm.value.incident_date || !incidentForm.value.category || !incidentForm.value.description) {
    message.warning(t('disciplinary.fillRequired'))
    return
  }
  try {
    await disciplinaryAPI.createIncident({
      employee_id: incidentForm.value.employee_id,
      incident_date: format(new Date(incidentForm.value.incident_date), 'yyyy-MM-dd'),
      category: incidentForm.value.category,
      severity: incidentForm.value.severity,
      description: incidentForm.value.description,
      witnesses: incidentForm.value.witnesses || undefined,
      evidence_notes: incidentForm.value.evidence_notes || undefined,
    })
    message.success(t('disciplinary.incidentCreated'))
    showIncidentModal.value = false
    incidentForm.value = { employee_id: null, incident_date: null, category: '', severity: 'minor', description: '', witnesses: '', evidence_notes: '' }
    loadIncidents()
  } catch { message.error(t('common.saveFailed')) }
}

async function submitAction() {
  if (!actionForm.value.employee_id || !actionForm.value.action_date || !actionForm.value.action_type || !actionForm.value.description) {
    message.warning(t('disciplinary.fillRequired'))
    return
  }
  try {
    await disciplinaryAPI.createAction({
      employee_id: actionForm.value.employee_id,
      incident_id: actionForm.value.incident_id || undefined,
      action_type: actionForm.value.action_type,
      action_date: format(new Date(actionForm.value.action_date), 'yyyy-MM-dd'),
      description: actionForm.value.description,
      suspension_days: actionForm.value.suspension_days || undefined,
      effective_date: actionForm.value.effective_date ? format(new Date(actionForm.value.effective_date), 'yyyy-MM-dd') : undefined,
      end_date: actionForm.value.end_date ? format(new Date(actionForm.value.end_date), 'yyyy-MM-dd') : undefined,
      notes: actionForm.value.notes || undefined,
    })
    message.success(t('disciplinary.actionCreated'))
    showActionModal.value = false
    actionForm.value = { employee_id: null, incident_id: null, action_type: '', action_date: null, description: '', suspension_days: null, effective_date: null, end_date: null, notes: '' }
    loadActions()
  } catch { message.error(t('common.saveFailed')) }
}

async function submitAppeal() {
  if (!appealTarget.value || !appealReason.value) return
  try {
    await disciplinaryAPI.appealAction(appealTarget.value.id as number, appealReason.value)
    message.success(t('disciplinary.appealSubmitted'))
    showAppealModal.value = false
    appealTarget.value = null
    appealReason.value = ''
    loadActions()
  } catch { message.error(t('common.failed')) }
}

onMounted(() => {
  loadEmployees()
  loadIncidents()
  loadActions()
})
</script>

<template>
  <div>
    <NSpace justify="space-between" style="margin-bottom: 16px;">
      <h2>{{ t('disciplinary.title') }}</h2>
    </NSpace>

    <NTabs v-model:value="activeTab" type="line">
      <NTabPane name="incidents" :tab="t('disciplinary.incidents')">
        <NSpace justify="end" style="margin-bottom: 12px;">
          <NButton type="primary" size="small" @click="showIncidentModal = true">{{ t('disciplinary.reportIncident') }}</NButton>
        </NSpace>
        <NDataTable :columns="incidentColumns" :data="incidents" :loading="loading" />
      </NTabPane>

      <NTabPane name="actions" :tab="t('disciplinary.actions')">
        <NSpace justify="end" style="margin-bottom: 12px;">
          <NButton type="primary" size="small" @click="showActionModal = true">{{ t('disciplinary.issueAction') }}</NButton>
        </NSpace>
        <NDataTable :columns="actionColumns" :data="actions" :loading="loading" />
      </NTabPane>
    </NTabs>

    <!-- Report Incident Modal -->
    <NModal v-model:show="showIncidentModal" :title="t('disciplinary.reportIncident')" preset="card" style="max-width: 520px; width: 95vw;">
      <NForm @submit.prevent="submitIncident">
        <NFormItem :label="t('disciplinary.employee')" required>
          <NSelect v-model:value="incidentForm.employee_id" :options="employees" filterable :placeholder="t('disciplinary.selectEmployee')" />
        </NFormItem>
        <NFormItem :label="t('disciplinary.incidentDate')" required>
          <NDatePicker v-model:value="incidentForm.incident_date" type="date" style="width: 100%;" />
        </NFormItem>
        <NSpace :size="16">
          <NFormItem :label="t('disciplinary.category')" required>
            <NSelect v-model:value="incidentForm.category" :options="categoryOptions" style="width: 200px;" />
          </NFormItem>
          <NFormItem :label="t('disciplinary.severity')" required>
            <NSelect v-model:value="incidentForm.severity" :options="severityOptions" style="width: 160px;" />
          </NFormItem>
        </NSpace>
        <NFormItem :label="t('disciplinary.description')" required>
          <NInput v-model:value="incidentForm.description" type="textarea" :rows="3" />
        </NFormItem>
        <NFormItem :label="t('disciplinary.witnesses')">
          <NInput v-model:value="incidentForm.witnesses" :placeholder="t('disciplinary.witnessesPlaceholder')" />
        </NFormItem>
        <NSpace>
          <NButton type="primary" attr-type="submit">{{ t('common.save') }}</NButton>
          <NButton @click="showIncidentModal = false">{{ t('common.cancel') }}</NButton>
        </NSpace>
      </NForm>
    </NModal>

    <!-- Issue Action Modal -->
    <NModal v-model:show="showActionModal" :title="t('disciplinary.issueAction')" preset="card" style="max-width: 520px; width: 95vw;">
      <NForm @submit.prevent="submitAction">
        <NFormItem :label="t('disciplinary.employee')" required>
          <NSelect v-model:value="actionForm.employee_id" :options="employees" filterable :placeholder="t('disciplinary.selectEmployee')" />
        </NFormItem>
        <NSpace :size="16">
          <NFormItem :label="t('disciplinary.actionType')" required>
            <NSelect v-model:value="actionForm.action_type" :options="actionTypeOptions" style="width: 200px;" />
          </NFormItem>
          <NFormItem :label="t('disciplinary.actionDate')" required>
            <NDatePicker v-model:value="actionForm.action_date" type="date" />
          </NFormItem>
        </NSpace>
        <NFormItem :label="t('disciplinary.description')" required>
          <NInput v-model:value="actionForm.description" type="textarea" :rows="3" />
        </NFormItem>
        <NFormItem v-if="actionForm.action_type === 'suspension'" :label="t('disciplinary.suspensionDays')">
          <NInputNumber v-model:value="actionForm.suspension_days" :min="1" />
        </NFormItem>
        <NSpace v-if="actionForm.action_type === 'suspension' || actionForm.action_type === 'termination'" :size="16">
          <NFormItem :label="t('disciplinary.effectiveDate')">
            <NDatePicker v-model:value="actionForm.effective_date" type="date" />
          </NFormItem>
          <NFormItem v-if="actionForm.action_type === 'suspension'" :label="t('disciplinary.endDate')">
            <NDatePicker v-model:value="actionForm.end_date" type="date" />
          </NFormItem>
        </NSpace>
        <NFormItem :label="t('disciplinary.notes')">
          <NInput v-model:value="actionForm.notes" type="textarea" :rows="2" />
        </NFormItem>
        <NSpace>
          <NButton type="primary" attr-type="submit">{{ t('common.save') }}</NButton>
          <NButton @click="showActionModal = false">{{ t('common.cancel') }}</NButton>
        </NSpace>
      </NForm>
    </NModal>

    <!-- Appeal Modal -->
    <NModal v-model:show="showAppealModal" :title="t('disciplinary.submitAppeal')" preset="card" style="max-width: 420px; width: 95vw;">
      <NForm @submit.prevent="submitAppeal">
        <NFormItem :label="t('disciplinary.appealReason')" required>
          <NInput v-model:value="appealReason" type="textarea" :rows="4" />
        </NFormItem>
        <NSpace>
          <NButton type="primary" attr-type="submit">{{ t('common.submit') }}</NButton>
          <NButton @click="showAppealModal = false">{{ t('common.cancel') }}</NButton>
        </NSpace>
      </NForm>
    </NModal>
  </div>
</template>
