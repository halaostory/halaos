<script setup lang="ts">
import { ref, onMounted, computed, h } from 'vue'
import {
  NCard, NDataTable, NButton, NModal, NForm, NFormItem, NInput,
  NSelect, NInputNumber, NSwitch, NSpace, NTag, useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { workflowAPI } from '../api/client'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const message = useMessage()

interface Trigger {
  id: number
  name: string
  description: string | null
  trigger_type: string
  entity_type: string
  trigger_config: Record<string, unknown>
  action_type: string
  action_config: Record<string, unknown>
  priority: number
  is_active: boolean
  created_at: string
}

const triggers = ref<Trigger[]>([])
const loading = ref(false)
const showModal = ref(false)
const editingId = ref<number | null>(null)

const form = ref({
  name: '',
  description: '',
  trigger_type: 'on_created',
  entity_type: 'leave_request',
  trigger_config: '{}',
  action_type: 'run_rules_then_agent',
  action_config: '{"agent_slug":"workflow","auto_threshold":0.90,"recommend_threshold":0.70}',
  priority: 100,
  is_active: true,
})

const triggerTypeOptions = computed(() => [
  { label: t('workflowTriggers.onCreated'), value: 'on_created' },
  { label: t('workflowTriggers.onStatusChanged'), value: 'on_status_changed' },
  { label: t('workflowTriggers.onSlaBreach'), value: 'on_sla_breach' },
])

const entityTypeOptions = computed(() => [
  { label: t('workflowTriggers.leaveRequest'), value: 'leave_request' },
  { label: t('workflowTriggers.overtimeRequest'), value: 'overtime_request' },
  { label: t('workflowTriggers.allEntities'), value: '*' },
])

const actionTypeOptions = computed(() => [
  { label: t('workflowTriggers.runRulesThenAgent'), value: 'run_rules_then_agent' },
  { label: t('workflowTriggers.autoApprove'), value: 'auto_approve' },
  { label: t('workflowTriggers.autoReject'), value: 'auto_reject' },
  { label: t('workflowTriggers.notify'), value: 'notify' },
])

type TagType = 'info' | 'success' | 'error' | 'warning' | 'default'
const actionTypeColor: Record<string, TagType> = {
  run_rules_then_agent: 'info',
  auto_approve: 'success',
  auto_reject: 'error',
  notify: 'warning',
}

const columns = computed<DataTableColumns<Trigger>>(() => [
  { title: t('workflowTriggers.name'), key: 'name', width: 180 },
  {
    title: t('workflowTriggers.triggerType'),
    key: 'trigger_type',
    width: 140,
    render: (row) => {
      const labels: Record<string, string> = {
        on_created: t('workflowTriggers.onCreated'),
        on_status_changed: t('workflowTriggers.onStatusChanged'),
        on_sla_breach: t('workflowTriggers.onSlaBreach'),
      }
      return labels[row.trigger_type] || row.trigger_type
    },
  },
  {
    title: t('workflowTriggers.entityType'),
    key: 'entity_type',
    width: 140,
    render: (row) => {
      const labels: Record<string, string> = {
        leave_request: t('workflowTriggers.leaveRequest'),
        overtime_request: t('workflowTriggers.overtimeRequest'),
        '*': t('workflowTriggers.allEntities'),
      }
      return labels[row.entity_type] || row.entity_type
    },
  },
  {
    title: t('workflowTriggers.actionType'),
    key: 'action_type',
    width: 160,
    render: (row) => {
      const labels: Record<string, string> = {
        run_rules_then_agent: t('workflowTriggers.runRulesThenAgent'),
        auto_approve: t('workflowTriggers.autoApprove'),
        auto_reject: t('workflowTriggers.autoReject'),
        notify: t('workflowTriggers.notify'),
      }
      return h(NTag, { size: 'small', type: actionTypeColor[row.action_type] || 'default' }, () => labels[row.action_type] || row.action_type)
    },
  },
  { title: t('workflowTriggers.priority'), key: 'priority', width: 90 },
  {
    title: t('workflowTriggers.active'),
    key: 'is_active',
    width: 80,
    render: (row) => h(NTag, { size: 'small', type: row.is_active ? 'success' : 'default' }, () => row.is_active ? 'Yes' : 'No'),
  },
  {
    title: t('common.actions'),
    key: 'actions',
    width: 150,
    render: (row) =>
      h(NSpace, { size: 'small' }, () => [
        h(NButton, { size: 'tiny', onClick: () => openEdit(row) }, () => t('common.edit')),
        h(
          NButton,
          { size: 'tiny', type: 'error', onClick: () => deactivate(row.id) },
          () => t('common.delete'),
        ),
      ]),
  },
])

async function loadTriggers() {
  loading.value = true
  try {
    const res = await workflowAPI.listTriggers() as any
    triggers.value = res?.data ?? res ?? []
  } catch {
    message.error('Failed to load triggers')
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingId.value = null
  form.value = {
    name: '',
    description: '',
    trigger_type: 'on_created',
    entity_type: 'leave_request',
    trigger_config: '{}',
    action_type: 'run_rules_then_agent',
    action_config: '{"agent_slug":"workflow","auto_threshold":0.90,"recommend_threshold":0.70}',
    priority: 100,
    is_active: true,
  }
  showModal.value = true
}

function openEdit(trigger: Trigger) {
  editingId.value = trigger.id
  form.value = {
    name: trigger.name,
    description: trigger.description || '',
    trigger_type: trigger.trigger_type,
    entity_type: trigger.entity_type,
    trigger_config: JSON.stringify(trigger.trigger_config, null, 2),
    action_type: trigger.action_type,
    action_config: JSON.stringify(trigger.action_config, null, 2),
    priority: trigger.priority,
    is_active: trigger.is_active,
  }
  showModal.value = true
}

async function saveTrigger() {
  let triggerConfig, actionConfig
  try {
    triggerConfig = JSON.parse(form.value.trigger_config)
    actionConfig = JSON.parse(form.value.action_config)
  } catch {
    message.error('Invalid JSON in config fields')
    return
  }

  const data = {
    name: form.value.name,
    description: form.value.description || undefined,
    trigger_type: form.value.trigger_type,
    entity_type: form.value.entity_type,
    trigger_config: triggerConfig,
    action_type: form.value.action_type,
    action_config: actionConfig,
    priority: form.value.priority,
    is_active: form.value.is_active,
  }

  try {
    if (editingId.value) {
      await workflowAPI.updateTrigger(editingId.value, data)
      message.success('Trigger updated')
    } else {
      await workflowAPI.createTrigger(data)
      message.success('Trigger created')
    }
    showModal.value = false
    loadTriggers()
  } catch {
    message.error('Failed to save trigger')
  }
}

async function deactivate(id: number) {
  try {
    await workflowAPI.deactivateTrigger(id)
    message.success('Trigger deactivated')
    loadTriggers()
  } catch {
    message.error('Failed to deactivate trigger')
  }
}

onMounted(loadTriggers)
</script>

<template>
  <div>
    <NCard :title="t('workflowTriggers.title')">
      <template #header-extra>
        <NButton type="primary" @click="openCreate">
          {{ t('workflowTriggers.create') }}
        </NButton>
      </template>

      <NDataTable
        :columns="columns"
        :data="triggers"
        :loading="loading"
        :bordered="false"
        :scroll-x="900"
      />
    </NCard>

    <NModal
      v-model:show="showModal"
      :title="editingId ? t('workflowTriggers.edit') : t('workflowTriggers.create')"
      preset="card"
      style="max-width: 600px"
    >
      <NForm label-placement="left" label-width="140">
        <NFormItem :label="t('workflowTriggers.name')">
          <NInput v-model:value="form.name" />
        </NFormItem>
        <NFormItem :label="t('workflowTriggers.description')">
          <NInput v-model:value="form.description" type="textarea" :rows="2" />
        </NFormItem>
        <NFormItem :label="t('workflowTriggers.triggerType')">
          <NSelect v-model:value="form.trigger_type" :options="triggerTypeOptions" />
        </NFormItem>
        <NFormItem :label="t('workflowTriggers.entityType')">
          <NSelect v-model:value="form.entity_type" :options="entityTypeOptions" />
        </NFormItem>
        <NFormItem :label="t('workflowTriggers.actionType')">
          <NSelect v-model:value="form.action_type" :options="actionTypeOptions" />
        </NFormItem>
        <NFormItem :label="t('workflowTriggers.triggerConfig')">
          <NInput v-model:value="form.trigger_config" type="textarea" :rows="3" font-family="monospace" />
        </NFormItem>
        <NFormItem :label="t('workflowTriggers.actionConfig')">
          <NInput v-model:value="form.action_config" type="textarea" :rows="3" font-family="monospace" />
        </NFormItem>
        <NFormItem :label="t('workflowTriggers.priority')">
          <NInputNumber v-model:value="form.priority" :min="1" :max="999" />
        </NFormItem>
        <NFormItem v-if="editingId" :label="t('workflowTriggers.active')">
          <NSwitch v-model:value="form.is_active" />
        </NFormItem>
        <NFormItem>
          <NSpace>
            <NButton type="primary" @click="saveTrigger">
              {{ t('common.save') }}
            </NButton>
            <NButton @click="showModal = false">
              {{ t('common.cancel') }}
            </NButton>
          </NSpace>
        </NFormItem>
      </NForm>
    </NModal>
  </div>
</template>
