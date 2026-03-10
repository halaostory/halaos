<script setup lang="ts">
import { ref, onMounted, computed, h } from 'vue'
import {
  NCard, NDataTable, NTag, NProgress, NSpace, NSelect,
  NSwitch, NModal, NButton, NForm, NFormItem, NInput,
  NStatistic, NGrid, NGi, useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { workflowAPI } from '../api/client'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'

const { t } = useI18n()
const message = useMessage()
const route = useRoute()

interface Decision {
  id: number
  company_id: number
  trigger_id: number | null
  entity_type: string
  entity_id: number
  decision: string
  confidence: string
  reasoning: string | null
  context_snapshot: string | null
  ai_agent_slug: string | null
  tokens_used: number
  executed: boolean
  executed_at: string | null
  overridden_by: number | null
  override_action: string | null
  override_reason: string | null
  overridden_at: string | null
  created_at: string
  trigger_name: string | null
}

interface Stats {
  total: number
  auto_executed: number
  recommended: number
  escalated: number
  overridden: number
  override_rate: string
  avg_confidence: string
}

const decisions = ref<Decision[]>([])
const stats = ref<Stats | null>(null)
const loading = ref(false)
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)

// Filters
const entityTypeFilter = ref(route.query.entity_type as string || '')
const onlyOverridden = ref(false)

// Override modal
const showOverrideModal = ref(false)
const overrideDecisionId = ref(0)
const overrideForm = ref({ action: 'approved', reason: '' })

const entityTypeOptions = [
  { label: 'All', value: '' },
  { label: 'Leave Request', value: 'leave_request' },
  { label: 'Overtime Request', value: 'overtime_request' },
]

type TagType = 'info' | 'success' | 'error' | 'warning' | 'default'
const decisionColors: Record<string, TagType> = {
  auto_approve: 'success',
  auto_reject: 'error',
  recommend_approve: 'info',
  recommend_reject: 'warning',
  escalate: 'default',
  request_info: 'default',
}

function confidencePercent(conf: string): number {
  return Math.round(parseFloat(conf) * 100)
}

function confidenceStatus(conf: string): 'success' | 'warning' | 'error' {
  const v = parseFloat(conf)
  if (v >= 0.9) return 'success'
  if (v >= 0.7) return 'warning'
  return 'error'
}

const columns = computed<DataTableColumns<Decision>>(() => [
  {
    title: t('workflowDecisions.entityType'),
    key: 'entity_type',
    width: 130,
    render: (row) => {
      const labels: Record<string, string> = {
        leave_request: 'Leave',
        overtime_request: 'OT',
      }
      return h(NTag, { size: 'small' }, () => labels[row.entity_type] || row.entity_type)
    },
  },
  { title: 'Entity ID', key: 'entity_id', width: 80 },
  {
    title: t('workflowDecisions.decision'),
    key: 'decision',
    width: 150,
    render: (row) => {
      const labels: Record<string, string> = {
        auto_approve: t('workflowDecisions.autoApprove'),
        auto_reject: t('workflowDecisions.autoReject'),
        recommend_approve: t('workflowDecisions.recommendApprove'),
        recommend_reject: t('workflowDecisions.recommendReject'),
        escalate: t('workflowDecisions.escalate'),
        request_info: t('workflowDecisions.requestInfo'),
      }
      return h(NTag, {
        size: 'small',
        type: decisionColors[row.decision] || 'default',
      }, () => labels[row.decision] || row.decision)
    },
  },
  {
    title: t('workflowDecisions.confidence'),
    key: 'confidence',
    width: 120,
    render: (row) =>
      h(NProgress, {
        type: 'line',
        percentage: confidencePercent(row.confidence),
        status: confidenceStatus(row.confidence),
        indicatorPlacement: 'inside',
        height: 20,
      }),
  },
  {
    title: t('workflowDecisions.reasoning'),
    key: 'reasoning',
    ellipsis: { tooltip: true },
    render: (row) => row.reasoning || '-',
  },
  {
    title: t('workflowDecisions.executed'),
    key: 'executed',
    width: 80,
    render: (row) =>
      h(NTag, { size: 'small', type: row.executed ? 'success' : 'default' }, () => row.executed ? 'Yes' : 'No'),
  },
  {
    title: t('workflowDecisions.override'),
    key: 'overridden_at',
    width: 120,
    render: (row) => {
      if (row.overridden_at) {
        return h(NTag, { size: 'small', type: 'warning' }, () => row.override_action || 'Overridden')
      }
      return h(NButton, {
        size: 'tiny',
        onClick: () => openOverrideModal(row.id),
      }, () => t('workflowDecisions.recordOverride'))
    },
  },
  {
    title: t('common.date'),
    key: 'created_at',
    width: 150,
    render: (row) => new Date(row.created_at).toLocaleString(),
  },
])

async function loadDecisions() {
  loading.value = true
  try {
    const params: Record<string, string> = {
      page: String(page.value),
      page_size: String(pageSize.value),
    }
    if (entityTypeFilter.value) params.entity_type = entityTypeFilter.value
    if (onlyOverridden.value) params.only_overridden = 'true'

    const res = await workflowAPI.listDecisions(params) as any
    decisions.value = res?.data ?? res ?? []
    total.value = res?.meta?.total ?? 0
  } catch {
    message.error('Failed to load decisions')
  } finally {
    loading.value = false
  }
}

async function loadStats() {
  try {
    const res = await workflowAPI.getAnalytics() as any
    if (res?.data?.agent_decisions) {
      stats.value = res.data.agent_decisions
    }
  } catch {
    // Stats are optional
  }
}

function openOverrideModal(decisionId: number) {
  overrideDecisionId.value = decisionId
  overrideForm.value = { action: 'approved', reason: '' }
  showOverrideModal.value = true
}

async function submitOverride() {
  try {
    await workflowAPI.recordOverride(overrideDecisionId.value, {
      action: overrideForm.value.action,
      reason: overrideForm.value.reason || undefined,
    })
    message.success('Override recorded')
    showOverrideModal.value = false
    loadDecisions()
    loadStats()
  } catch {
    message.error('Failed to record override')
  }
}

function handlePageChange(p: number) {
  page.value = p
  loadDecisions()
}

onMounted(() => {
  loadDecisions()
  loadStats()
})
</script>

<template>
  <div>
    <!-- KPI Cards -->
    <NGrid v-if="stats" :cols="6" :x-gap="12" :y-gap="12" responsive="screen" :item-responsive="true" style="margin-bottom: 16px">
      <NGi :span="24 / 6">
        <NCard size="small">
          <NStatistic :label="t('workflowDecisions.total')" :value="stats.total" />
        </NCard>
      </NGi>
      <NGi :span="24 / 6">
        <NCard size="small">
          <NStatistic :label="t('workflowDecisions.autoExecuted')" :value="stats.auto_executed" />
        </NCard>
      </NGi>
      <NGi :span="24 / 6">
        <NCard size="small">
          <NStatistic :label="t('workflowDecisions.recommended')" :value="stats.recommended" />
        </NCard>
      </NGi>
      <NGi :span="24 / 6">
        <NCard size="small">
          <NStatistic :label="t('workflowDecisions.escalated')" :value="stats.escalated" />
        </NCard>
      </NGi>
      <NGi :span="24 / 6">
        <NCard size="small">
          <NStatistic :label="t('workflowDecisions.overrideRate')" :value="stats.override_rate" />
        </NCard>
      </NGi>
      <NGi :span="24 / 6">
        <NCard size="small">
          <NStatistic :label="t('workflowDecisions.avgConfidence')" :value="stats.avg_confidence" />
        </NCard>
      </NGi>
    </NGrid>

    <!-- Main Table -->
    <NCard :title="t('workflowDecisions.title')">
      <template #header-extra>
        <NSpace>
          <NSelect
            v-model:value="entityTypeFilter"
            :options="entityTypeOptions"
            size="small"
            style="width: 160px"
            @update:value="loadDecisions"
          />
          <NSwitch
            v-model:value="onlyOverridden"
            @update:value="loadDecisions"
          >
            <template #checked>{{ t('workflowDecisions.onlyOverridden') }}</template>
            <template #unchecked>{{ t('workflowDecisions.onlyOverridden') }}</template>
          </NSwitch>
        </NSpace>
      </template>

      <NDataTable
        :columns="columns"
        :data="decisions"
        :loading="loading"
        :bordered="false"
        :scroll-x="1000"
        :pagination="{
          page: page,
          pageSize: pageSize,
          itemCount: total,
          onChange: handlePageChange,
        }"
      />
    </NCard>

    <!-- Override Modal -->
    <NModal
      v-model:show="showOverrideModal"
      :title="t('workflowDecisions.recordOverride')"
      preset="card"
      style="max-width: 450px"
    >
      <NForm label-placement="left" label-width="100">
        <NFormItem :label="t('workflowDecisions.overrideAction')">
          <NSelect
            v-model:value="overrideForm.action"
            :options="[
              { label: 'Approved', value: 'approved' },
              { label: 'Rejected', value: 'rejected' },
            ]"
          />
        </NFormItem>
        <NFormItem :label="t('workflowDecisions.overrideReason')">
          <NInput v-model:value="overrideForm.reason" type="textarea" :rows="3" />
        </NFormItem>
        <NFormItem>
          <NSpace>
            <NButton type="primary" @click="submitOverride">
              {{ t('common.save') }}
            </NButton>
            <NButton @click="showOverrideModal = false">
              {{ t('common.cancel') }}
            </NButton>
          </NSpace>
        </NFormItem>
      </NForm>
    </NModal>
  </div>
</template>
