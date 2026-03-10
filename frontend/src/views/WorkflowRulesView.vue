<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NCard, NButton, NSpace, NDataTable, NModal, NForm, NFormItem,
  NInput, NInputNumber, NSelect, NSwitch, NTag, NTabs, NTabPane,
  useMessage, type DataTableColumns,
} from 'naive-ui'
import { workflowAPI } from '../api/client'

const { t } = useI18n()
const message = useMessage()

interface WorkflowRule {
  id: number
  company_id: number
  name: string
  description: string | null
  entity_type: string
  rule_type: string
  conditions: Record<string, any>
  priority: number
  is_active: boolean
  created_at: string
  updated_at: string
}

interface RuleExecution {
  id: number
  rule_id: number
  entity_type: string
  entity_id: number
  action: string
  reason: string | null
  rule_name: string
  created_at: string
}

interface SLAConfig {
  id: number
  company_id: number
  entity_type: string
  reminder_after_hours: number
  second_reminder_hours: number
  escalate_after_hours: number
  auto_action_hours: number
  auto_action: string
  escalation_role: string
  is_active: boolean
}

const rules = ref<WorkflowRule[]>([])
const executions = ref<RuleExecution[]>([])
const slaConfigs = ref<SLAConfig[]>([])
const loading = ref(false)
const showModal = ref(false)
const showSLAModal = ref(false)
const editingRule = ref<WorkflowRule | null>(null)
const activeTab = ref('rules')

const slaForm = ref({
  entity_type: 'leave_request',
  reminder_after_hours: 12,
  second_reminder_hours: 24,
  escalate_after_hours: 48,
  auto_action_hours: 72,
  auto_action: 'approve',
  escalation_role: 'admin',
})

const form = ref({
  name: '',
  description: '',
  entity_type: 'leave_request',
  rule_type: 'auto_approve',
  priority: 100,
  is_active: true,
  // Leave conditions
  max_days: null as number | null,
  require_balance: true,
  require_no_conflict: false,
  allowed_leave_types: [] as string[],
  min_tenure_months: null as number | null,
  // OT conditions
  max_hours: null as number | null,
  allowed_ot_types: [] as string[],
})

const entityTypeOptions = [
  { label: t('workflowRules.leaveRequest'), value: 'leave_request' },
  { label: t('workflowRules.overtimeRequest'), value: 'overtime_request' },
]

const ruleTypeOptions = [
  { label: t('workflowRules.autoApprove'), value: 'auto_approve' },
  { label: t('workflowRules.autoReject'), value: 'auto_reject' },
]

const leaveTypeOptions = [
  { label: 'Sick Leave (SL)', value: 'SL' },
  { label: 'Vacation Leave (VL)', value: 'VL' },
  { label: 'Emergency Leave (EL)', value: 'EL' },
  { label: 'Birthday Leave (BL)', value: 'BL' },
  { label: 'Maternity Leave (ML)', value: 'ML' },
  { label: 'Paternity Leave (PL)', value: 'PL' },
]

const otTypeOptions = [
  { label: 'Regular', value: 'regular' },
  { label: 'Rest Day', value: 'rest_day' },
  { label: 'Holiday', value: 'holiday' },
  { label: 'Special Holiday', value: 'special_holiday' },
]

const isLeave = computed(() => form.value.entity_type === 'leave_request')

async function fetchRules() {
  loading.value = true
  try {
    const res = await workflowAPI.listRules() as any
    rules.value = res?.data ?? res ?? []
  } catch {
    message.error(t('common.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function fetchExecutions() {
  try {
    const res = await workflowAPI.listExecutions(undefined, { page_size: '100' }) as any
    const data = res?.data ?? res ?? []
    executions.value = Array.isArray(data) ? data : []
  } catch { /* ignore */ }
}

function openCreate() {
  editingRule.value = null
  form.value = {
    name: '', description: '', entity_type: 'leave_request', rule_type: 'auto_approve',
    priority: 100, is_active: true,
    max_days: null, require_balance: true, require_no_conflict: false,
    allowed_leave_types: [], min_tenure_months: null,
    max_hours: null, allowed_ot_types: [],
  }
  showModal.value = true
}

function openEdit(rule: WorkflowRule) {
  editingRule.value = rule
  const c = rule.conditions || {}
  form.value = {
    name: rule.name,
    description: rule.description || '',
    entity_type: rule.entity_type,
    rule_type: rule.rule_type,
    priority: rule.priority,
    is_active: rule.is_active,
    max_days: c.max_days ?? null,
    require_balance: c.require_balance ?? false,
    require_no_conflict: c.require_no_conflict ?? false,
    allowed_leave_types: c.allowed_leave_types ?? [],
    min_tenure_months: c.min_tenure_months ?? null,
    max_hours: c.max_hours ?? null,
    allowed_ot_types: c.allowed_ot_types ?? [],
  }
  showModal.value = true
}

function buildConditions(): Record<string, any> {
  const f = form.value
  if (f.entity_type === 'leave_request') {
    const c: Record<string, any> = {}
    if (f.max_days !== null) c.max_days = f.max_days
    if (f.require_balance) c.require_balance = true
    if (f.require_no_conflict) c.require_no_conflict = true
    if (f.allowed_leave_types.length) c.allowed_leave_types = f.allowed_leave_types
    if (f.min_tenure_months !== null) c.min_tenure_months = f.min_tenure_months
    return c
  }
  const c: Record<string, any> = {}
  if (f.max_hours !== null) c.max_hours = f.max_hours
  if (f.allowed_ot_types.length) c.allowed_ot_types = f.allowed_ot_types
  return c
}

async function handleSave() {
  const payload = {
    name: form.value.name,
    description: form.value.description || undefined,
    entity_type: form.value.entity_type,
    rule_type: form.value.rule_type,
    conditions: buildConditions(),
    priority: form.value.priority,
    is_active: form.value.is_active,
  }

  try {
    if (editingRule.value) {
      await workflowAPI.updateRule(editingRule.value.id, payload)
      message.success(t('workflowRules.ruleUpdated'))
    } else {
      await workflowAPI.createRule(payload)
      message.success(t('workflowRules.ruleCreated'))
    }
    showModal.value = false
    fetchRules()
  } catch {
    message.error(t('common.saveFailed'))
  }
}

async function handleDeactivate(id: number) {
  try {
    await workflowAPI.deactivateRule(id)
    message.success(t('workflowRules.ruleDeactivated'))
    fetchRules()
  } catch {
    message.error(t('common.failed'))
  }
}

function conditionsSummary(rule: WorkflowRule): string {
  const c = rule.conditions || {}
  const parts: string[] = []
  if (c.max_days) parts.push(`<= ${c.max_days} days`)
  if (c.max_hours) parts.push(`<= ${c.max_hours} hrs`)
  if (c.require_balance) parts.push('balance ok')
  if (c.require_no_conflict) parts.push('no conflicts')
  if (c.allowed_leave_types?.length) parts.push(c.allowed_leave_types.join(','))
  if (c.allowed_ot_types?.length) parts.push(c.allowed_ot_types.join(','))
  if (c.min_tenure_months) parts.push(`tenure >= ${c.min_tenure_months}mo`)
  return parts.join(' + ') || '-'
}

const ruleColumns = computed<DataTableColumns<WorkflowRule>>(() => [
  { title: t('workflowRules.ruleName'), key: 'name', width: 200 },
  {
    title: t('workflowRules.entityType'), key: 'entity_type', width: 140,
    render: (row) => h(NTag, { size: 'small', type: row.entity_type === 'leave_request' ? 'success' : 'warning' },
      { default: () => row.entity_type === 'leave_request' ? t('workflowRules.leaveRequest') : t('workflowRules.overtimeRequest') }),
  },
  {
    title: t('workflowRules.ruleType'), key: 'rule_type', width: 120,
    render: (row) => h(NTag, { size: 'small', type: row.rule_type === 'auto_approve' ? 'info' : 'error' },
      { default: () => row.rule_type === 'auto_approve' ? t('workflowRules.autoApprove') : t('workflowRules.autoReject') }),
  },
  { title: t('workflowRules.conditions'), key: 'conditions', render: (row) => conditionsSummary(row) },
  { title: t('workflowRules.priority'), key: 'priority', width: 90 },
  {
    title: t('common.status'), key: 'is_active', width: 90,
    render: (row) => h(NTag, { size: 'small', type: row.is_active ? 'success' : 'default' },
      { default: () => row.is_active ? t('common.active') : t('common.inactive') }),
  },
  {
    title: t('common.actions'), key: 'actions', width: 160,
    render: (row) => h(NSpace, { size: 'small' }, {
      default: () => [
        h(NButton, { size: 'small', onClick: () => openEdit(row) }, { default: () => t('common.edit') }),
        row.is_active
          ? h(NButton, { size: 'small', type: 'error', onClick: () => handleDeactivate(row.id) }, { default: () => t('common.delete') })
          : null,
      ],
    }),
  },
])

const executionColumns = computed<DataTableColumns<RuleExecution>>(() => [
  { title: 'ID', key: 'id', width: 60 },
  { title: t('workflowRules.ruleName'), key: 'rule_name', width: 200 },
  {
    title: t('workflowRules.entityType'), key: 'entity_type', width: 140,
    render: (row) => h(NTag, { size: 'small' }, { default: () => row.entity_type }),
  },
  { title: 'Entity ID', key: 'entity_id', width: 90 },
  {
    title: t('common.status'), key: 'action', width: 120,
    render: (row) => {
      const type = row.action === 'auto_approved' ? 'success' : row.action === 'auto_rejected' ? 'error' : 'default'
      return h(NTag, { size: 'small', type }, { default: () => row.action })
    },
  },
  { title: 'Reason', key: 'reason', ellipsis: { tooltip: true } },
  { title: 'Time', key: 'created_at', width: 160, render: (row) => new Date(row.created_at).toLocaleString() },
])

async function fetchSLAConfigs() {
  try {
    const res = await workflowAPI.listSLAConfigs() as any
    slaConfigs.value = res?.data ?? res ?? []
  } catch { /* ignore */ }
}

function openSLACreate() {
  slaForm.value = {
    entity_type: 'leave_request',
    reminder_after_hours: 12,
    second_reminder_hours: 24,
    escalate_after_hours: 48,
    auto_action_hours: 72,
    auto_action: 'approve',
    escalation_role: 'admin',
  }
  showSLAModal.value = true
}

function openSLAEdit(config: SLAConfig) {
  slaForm.value = {
    entity_type: config.entity_type,
    reminder_after_hours: config.reminder_after_hours,
    second_reminder_hours: config.second_reminder_hours,
    escalate_after_hours: config.escalate_after_hours,
    auto_action_hours: config.auto_action_hours,
    auto_action: config.auto_action,
    escalation_role: config.escalation_role,
  }
  showSLAModal.value = true
}

async function handleSLASave() {
  try {
    await workflowAPI.upsertSLAConfig(slaForm.value)
    message.success(t('common.saved'))
    showSLAModal.value = false
    fetchSLAConfigs()
  } catch {
    message.error(t('common.saveFailed'))
  }
}

const autoActionOptions = [
  { label: t('workflowRules.autoApprove'), value: 'approve' },
  { label: t('workflowRules.autoReject'), value: 'reject' },
  { label: 'None', value: 'none' },
]

const escalationRoleOptions = [
  { label: 'Admin', value: 'admin' },
  { label: 'HR Head', value: 'hr_head' },
  { label: 'Manager', value: 'manager' },
]

const slaColumns = computed<DataTableColumns<SLAConfig>>(() => [
  {
    title: t('workflowRules.entityType'), key: 'entity_type', width: 160,
    render: (row) => h(NTag, { size: 'small', type: row.entity_type === 'leave_request' ? 'success' : 'warning' },
      { default: () => row.entity_type === 'leave_request' ? t('workflowRules.leaveRequest') : t('workflowRules.overtimeRequest') }),
  },
  { title: t('approvalSla.reminderAfterHours'), key: 'reminder_after_hours', width: 140, render: (row) => `${row.reminder_after_hours}h` },
  { title: t('approvalSla.secondReminderHours'), key: 'second_reminder_hours', width: 150, render: (row) => `${row.second_reminder_hours}h` },
  { title: t('approvalSla.escalateAfterHours'), key: 'escalate_after_hours', width: 140, render: (row) => `${row.escalate_after_hours}h` },
  { title: t('approvalSla.autoActionHours'), key: 'auto_action_hours', width: 140, render: (row) => `${row.auto_action_hours}h` },
  {
    title: t('approvalSla.autoAction'), key: 'auto_action', width: 120,
    render: (row) => h(NTag, { size: 'small', type: row.auto_action === 'approve' ? 'success' : row.auto_action === 'reject' ? 'error' : 'default' },
      { default: () => row.auto_action }),
  },
  {
    title: t('common.actions'), key: 'actions', width: 100,
    render: (row) => h(NButton, { size: 'small', onClick: () => openSLAEdit(row) }, { default: () => t('common.edit') }),
  },
])

onMounted(() => {
  fetchRules()
  fetchExecutions()
  fetchSLAConfigs()
})
</script>

<template>
  <NCard :title="t('workflowRules.title')">
    <template #header-extra>
      <NButton type="primary" @click="openCreate">
        {{ t('workflowRules.createRule') }}
      </NButton>
    </template>

    <NTabs v-model:value="activeTab">
      <NTabPane name="rules" :tab="t('workflowRules.title')">
        <NDataTable
          :columns="ruleColumns"
          :data="rules"
          :loading="loading"
          :bordered="false"
          :row-key="(r: WorkflowRule) => r.id"
        />
      </NTabPane>

      <NTabPane name="executions" :tab="t('workflowRules.executionLog')">
        <NDataTable
          :columns="executionColumns"
          :data="executions"
          :bordered="false"
          :row-key="(r: RuleExecution) => r.id"
        />
      </NTabPane>

      <NTabPane name="sla" :tab="t('approvalSla.title')">
        <NSpace vertical>
          <NSpace justify="end">
            <NButton type="primary" @click="openSLACreate">
              {{ t('workflowRules.createRule') }}
            </NButton>
          </NSpace>
          <NDataTable
            :columns="slaColumns"
            :data="slaConfigs"
            :bordered="false"
            :row-key="(r: SLAConfig) => r.id"
          />
        </NSpace>
      </NTabPane>
    </NTabs>

    <NModal
      v-model:show="showModal"
      preset="card"
      :title="editingRule ? t('workflowRules.editRule') : t('workflowRules.createRule')"
      style="max-width: 600px"
    >
      <NForm label-placement="left" label-width="160">
        <NFormItem :label="t('workflowRules.ruleName')">
          <NInput v-model:value="form.name" />
        </NFormItem>
        <NFormItem :label="t('workflowRules.description')">
          <NInput v-model:value="form.description" type="textarea" :rows="2" />
        </NFormItem>
        <NFormItem :label="t('workflowRules.entityType')">
          <NSelect v-model:value="form.entity_type" :options="entityTypeOptions" />
        </NFormItem>
        <NFormItem :label="t('workflowRules.ruleType')">
          <NSelect v-model:value="form.rule_type" :options="ruleTypeOptions" />
        </NFormItem>
        <NFormItem :label="t('workflowRules.priority')">
          <NInputNumber v-model:value="form.priority" :min="1" :max="999" />
        </NFormItem>

        <template v-if="isLeave">
          <NFormItem :label="t('workflowRules.maxDays')">
            <NInputNumber v-model:value="form.max_days" :min="0.5" :max="30" :step="0.5" clearable />
          </NFormItem>
          <NFormItem :label="t('workflowRules.requireBalance')">
            <NSwitch v-model:value="form.require_balance" />
          </NFormItem>
          <NFormItem :label="t('workflowRules.requireNoConflict')">
            <NSwitch v-model:value="form.require_no_conflict" />
          </NFormItem>
          <NFormItem :label="t('workflowRules.allowedLeaveTypes')">
            <NSelect v-model:value="form.allowed_leave_types" :options="leaveTypeOptions" multiple />
          </NFormItem>
          <NFormItem :label="t('workflowRules.minTenureMonths')">
            <NInputNumber v-model:value="form.min_tenure_months" :min="0" :max="60" clearable />
          </NFormItem>
        </template>
        <template v-else>
          <NFormItem :label="t('workflowRules.maxHours')">
            <NInputNumber v-model:value="form.max_hours" :min="0.5" :max="24" :step="0.5" clearable />
          </NFormItem>
          <NFormItem :label="t('workflowRules.allowedOTTypes')">
            <NSelect v-model:value="form.allowed_ot_types" :options="otTypeOptions" multiple />
          </NFormItem>
        </template>

        <NFormItem v-if="editingRule" :label="t('common.active')">
          <NSwitch v-model:value="form.is_active" />
        </NFormItem>
      </NForm>

      <template #footer>
        <NSpace justify="end">
          <NButton @click="showModal = false">{{ t('common.cancel') }}</NButton>
          <NButton type="primary" @click="handleSave">{{ t('common.save') }}</NButton>
        </NSpace>
      </template>
    </NModal>

    <NModal
      v-model:show="showSLAModal"
      preset="card"
      :title="t('approvalSla.title')"
      style="max-width: 500px"
    >
      <NForm label-placement="left" label-width="200">
        <NFormItem :label="t('workflowRules.entityType')">
          <NSelect v-model:value="slaForm.entity_type" :options="entityTypeOptions" />
        </NFormItem>
        <NFormItem :label="t('approvalSla.reminderAfterHours')">
          <NInputNumber v-model:value="slaForm.reminder_after_hours" :min="1" :max="168" />
        </NFormItem>
        <NFormItem :label="t('approvalSla.secondReminderHours')">
          <NInputNumber v-model:value="slaForm.second_reminder_hours" :min="1" :max="168" />
        </NFormItem>
        <NFormItem :label="t('approvalSla.escalateAfterHours')">
          <NInputNumber v-model:value="slaForm.escalate_after_hours" :min="1" :max="336" />
        </NFormItem>
        <NFormItem :label="t('approvalSla.autoActionHours')">
          <NInputNumber v-model:value="slaForm.auto_action_hours" :min="1" :max="336" />
        </NFormItem>
        <NFormItem :label="t('approvalSla.autoAction')">
          <NSelect v-model:value="slaForm.auto_action" :options="autoActionOptions" />
        </NFormItem>
        <NFormItem :label="t('approvalSla.escalationRole')">
          <NSelect v-model:value="slaForm.escalation_role" :options="escalationRoleOptions" />
        </NFormItem>
      </NForm>

      <template #footer>
        <NSpace justify="end">
          <NButton @click="showSLAModal = false">{{ t('common.cancel') }}</NButton>
          <NButton type="primary" @click="handleSLASave">{{ t('common.save') }}</NButton>
        </NSpace>
      </template>
    </NModal>
  </NCard>
</template>
