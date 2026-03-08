<script setup lang="ts">
import { ref, h, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NTabs, NTabPane, NDataTable, NButton, NModal, NForm, NFormItem,
  NInput, NInputNumber, NSelect, NSwitch, NDatePicker, NSpace, NTag,
  NCard, NAlert, useMessage, type DataTableColumns,
} from 'naive-ui'
import { policyAPI } from '../api/client'
import { useAuthStore } from '../stores/auth'
import { format } from 'date-fns'

const { t } = useI18n()
const message = useMessage()
const auth = useAuthStore()

interface Policy {
  id: number
  title: string
  content: string
  category: string
  version: number
  effective_date: string
  requires_acknowledgment: boolean
  is_active: boolean
  ack_count?: number
  created_at: string
}

interface Acknowledgment {
  id: number
  policy_id: number
  employee_id: number
  employee_no: string
  first_name: string
  last_name: string
  acknowledged_at: string
}

const policies = ref<Policy[]>([])
const pendingPolicies = ref<Policy[]>([])
const loading = ref(false)

// Create/Edit modal
const showPolicyModal = ref(false)
const editingPolicy = ref<Policy | null>(null)
const policyForm = ref({
  title: '',
  content: '',
  category: 'general',
  effective_date: Date.now() as number | null,
  requires_acknowledgment: true,
  is_active: true,
  version: 1,
})

// View policy modal
const showViewModal = ref(false)
const viewPolicy = ref<Policy | null>(null)

// Acknowledgments modal
const showAckModal = ref(false)
const ackPolicyId = ref(0)
const ackPolicyTitle = ref('')
const acknowledgments = ref<Acknowledgment[]>([])
const ackStats = ref({ total_employees: 0, acknowledged_count: 0 })

const isAdmin = computed(() => auth.isAdmin)
const isManager = computed(() => auth.isAdmin || auth.isManager)

const categoryOptions = [
  { label: t('policy.general'), value: 'general' },
  { label: t('policy.codeOfConduct'), value: 'code_of_conduct' },
  { label: t('policy.safety'), value: 'safety' },
  { label: t('policy.benefits'), value: 'benefits' },
  { label: t('policy.leave'), value: 'leave' },
  { label: t('policy.dataPrivacy'), value: 'data_privacy' },
  { label: t('policy.antiHarassment'), value: 'anti_harassment' },
]

const categoryColor: Record<string, string> = {
  general: 'default',
  code_of_conduct: 'info',
  safety: 'warning',
  benefits: 'success',
  leave: 'success',
  data_privacy: 'error',
  anti_harassment: 'error',
}

const policyColumns = computed<DataTableColumns<Policy>>(() => [
  { title: t('policy.policyTitle'), key: 'title' },
  {
    title: t('policy.category'), key: 'category',
    render: (row) => h(NTag, { size: 'small', type: (categoryColor[row.category] || 'default') as any }, () => t(`policy.${row.category}`) || row.category),
  },
  { title: t('policy.version'), key: 'version', width: 80 },
  { title: t('policy.effectiveDate'), key: 'effective_date', render: (row) => format(new Date(row.effective_date), 'yyyy-MM-dd') },
  {
    title: t('policy.requiresAck'), key: 'requires_acknowledgment',
    render: (row) => h(NTag, { size: 'small', type: row.requires_acknowledgment ? 'warning' : 'default' }, () => row.requires_acknowledgment ? t('common.yes') : t('common.no')),
  },
  { title: t('policy.acknowledged'), key: 'ack_count', render: (row) => String(row.ack_count ?? 0) },
  {
    title: t('common.actions'), key: 'actions',
    render: (row) => {
      const btns: ReturnType<typeof h>[] = [
        h(NButton, { size: 'small', onClick: () => { viewPolicy.value = row; showViewModal.value = true } }, () => t('policy.view')),
      ]
      if (isManager.value) {
        btns.push(h(NButton, { size: 'small', type: 'info', onClick: () => openAcknowledgments(row) }, () => t('policy.ackList')))
      }
      if (isAdmin.value) {
        btns.push(h(NButton, { size: 'small', onClick: () => openEditPolicy(row) }, () => t('common.edit')))
      }
      return h(NSpace, { size: 4 }, () => btns)
    },
  },
])

const ackColumns = computed<DataTableColumns<Acknowledgment>>(() => [
  { title: t('policy.employee'), key: 'employee', render: (row) => `${row.first_name} ${row.last_name} (${row.employee_no})` },
  { title: t('policy.acknowledgedAt'), key: 'acknowledged_at', render: (row) => format(new Date(row.acknowledged_at), 'yyyy-MM-dd HH:mm') },
])

function extractData(res: unknown): any[] {
  const d = (res as any)?.data ?? res
  return Array.isArray(d) ? d : []
}

async function fetchAll() {
  loading.value = true
  try {
    const [policiesRes, pendingRes] = await Promise.all([
      policyAPI.list(),
      policyAPI.pending(),
    ])
    policies.value = extractData(policiesRes)
    pendingPolicies.value = extractData(pendingRes)
  } catch {
    message.error(t('policy.loadFailed'))
  } finally {
    loading.value = false
  }
}

function openCreatePolicy() {
  editingPolicy.value = null
  policyForm.value = { title: '', content: '', category: 'general', effective_date: Date.now(), requires_acknowledgment: true, is_active: true, version: 1 }
  showPolicyModal.value = true
}

function openEditPolicy(policy: Policy) {
  editingPolicy.value = policy
  policyForm.value = {
    title: policy.title,
    content: policy.content,
    category: policy.category,
    effective_date: new Date(policy.effective_date).getTime(),
    requires_acknowledgment: policy.requires_acknowledgment,
    is_active: policy.is_active,
    version: policy.version,
  }
  showPolicyModal.value = true
}

async function savePolicy() {
  if (!policyForm.value.title || !policyForm.value.content) {
    message.warning(t('common.fillRequired'))
    return
  }
  const data = {
    ...policyForm.value,
    effective_date: policyForm.value.effective_date ? format(new Date(policyForm.value.effective_date), 'yyyy-MM-dd') : format(new Date(), 'yyyy-MM-dd'),
  }
  try {
    if (editingPolicy.value) {
      await policyAPI.update(editingPolicy.value.id, data)
      message.success(t('common.updated'))
    } else {
      await policyAPI.create(data)
      message.success(t('common.created'))
    }
    showPolicyModal.value = false
    fetchAll()
  } catch {
    message.error(t('common.error'))
  }
}

async function acknowledgePolicy(id: number) {
  try {
    await policyAPI.acknowledge(id)
    message.success(t('policy.acknowledged'))
    fetchAll()
  } catch {
    message.error(t('common.error'))
  }
}

async function openAcknowledgments(policy: Policy) {
  ackPolicyId.value = policy.id
  ackPolicyTitle.value = policy.title
  showAckModal.value = true
  try {
    const [acksRes, statsRes] = await Promise.all([
      policyAPI.listAcknowledgments(policy.id),
      policyAPI.stats(policy.id),
    ])
    acknowledgments.value = extractData(acksRes)
    ackStats.value = (statsRes as any)?.data ?? statsRes
  } catch { message.error(t('common.loadFailed')) }
}

onMounted(fetchAll)
</script>

<template>
  <div>
    <h2 style="margin-bottom: 16px;">{{ t('policy.title') }}</h2>

    <!-- Pending Acknowledgments Alert -->
    <NAlert v-if="pendingPolicies.length > 0" type="warning" style="margin-bottom: 16px;">
      <strong>{{ t('policy.pendingAlert', { count: pendingPolicies.length }) }}</strong>
    </NAlert>

    <NTabs type="line">
      <!-- Pending Acknowledgments -->
      <NTabPane v-if="pendingPolicies.length > 0" :name="t('policy.pendingAck')" :tab="t('policy.pendingAck')">
        <NSpace vertical :size="16">
          <NCard v-for="p in pendingPolicies" :key="p.id" :title="p.title" size="small">
            <template #header-extra>
              <NSpace :size="8">
                <NTag size="small" :type="(categoryColor[p.category] || 'default') as any">{{ t(`policy.${p.category}`) || p.category }}</NTag>
                <span style="font-size: 12px; color: var(--n-text-color-3);">v{{ p.version }} | {{ format(new Date(p.effective_date), 'yyyy-MM-dd') }}</span>
              </NSpace>
            </template>
            <div style="white-space: pre-wrap; max-height: 200px; overflow-y: auto;">{{ p.content }}</div>
            <template #action>
              <NButton type="primary" @click="acknowledgePolicy(p.id)">{{ t('policy.acknowledge') }}</NButton>
            </template>
          </NCard>
        </NSpace>
      </NTabPane>

      <!-- All Policies -->
      <NTabPane :name="t('policy.allPolicies')" :tab="t('policy.allPolicies')">
        <NSpace v-if="isAdmin" style="margin-bottom: 12px;">
          <NButton type="primary" @click="openCreatePolicy">{{ t('policy.createPolicy') }}</NButton>
        </NSpace>
        <NDataTable :columns="policyColumns" :data="policies" :loading="loading" :bordered="false" />
      </NTabPane>
    </NTabs>

    <!-- Create/Edit Policy Modal -->
    <NModal v-model:show="showPolicyModal" preset="card" :title="editingPolicy ? t('policy.editPolicy') : t('policy.createPolicy')" style="max-width: 700px; width: 95vw;">
      <NForm label-placement="left" label-width="140">
        <NFormItem :label="t('policy.policyTitle')">
          <NInput v-model:value="policyForm.title" />
        </NFormItem>
        <NFormItem :label="t('policy.category')">
          <NSelect v-model:value="policyForm.category" :options="categoryOptions" />
        </NFormItem>
        <NFormItem :label="t('policy.content')">
          <NInput v-model:value="policyForm.content" type="textarea" :rows="10" />
        </NFormItem>
        <NFormItem :label="t('policy.effectiveDate')">
          <NDatePicker v-model:value="policyForm.effective_date" type="date" style="width: 100%;" />
        </NFormItem>
        <NFormItem :label="t('policy.version')">
          <NInputNumber v-model:value="policyForm.version" :min="1" />
        </NFormItem>
        <NFormItem :label="t('policy.requiresAck')">
          <NSwitch v-model:value="policyForm.requires_acknowledgment" />
        </NFormItem>
        <NFormItem>
          <NButton type="primary" @click="savePolicy">{{ t('common.save') }}</NButton>
        </NFormItem>
      </NForm>
    </NModal>

    <!-- View Policy Modal -->
    <NModal v-model:show="showViewModal" preset="card" :title="viewPolicy?.title" style="max-width: 700px; width: 95vw;">
      <div v-if="viewPolicy">
        <NSpace :size="8" style="margin-bottom: 12px;">
          <NTag :type="(categoryColor[viewPolicy.category] || 'default') as any">{{ t(`policy.${viewPolicy.category}`) || viewPolicy.category }}</NTag>
          <NTag>v{{ viewPolicy.version }}</NTag>
          <span>{{ t('policy.effectiveDate') }}: {{ format(new Date(viewPolicy.effective_date), 'yyyy-MM-dd') }}</span>
        </NSpace>
        <div style="white-space: pre-wrap; max-height: 500px; overflow-y: auto; padding: 12px; background: var(--n-color-modal); border-radius: 4px;">
          {{ viewPolicy.content }}
        </div>
      </div>
    </NModal>

    <!-- Acknowledgments Modal -->
    <NModal v-model:show="showAckModal" preset="card" :title="`${t('policy.ackList')}: ${ackPolicyTitle}`" style="max-width: 600px; width: 95vw;">
      <div style="margin-bottom: 12px;">
        <strong>{{ ackStats.acknowledged_count }}</strong> / {{ ackStats.total_employees }} {{ t('policy.employeesAcknowledged') }}
      </div>
      <NDataTable :columns="ackColumns" :data="acknowledgments" :bordered="false" size="small" />
    </NModal>
  </div>
</template>
