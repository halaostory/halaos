<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NCard, NButton, NDataTable, NTag, NModal, NForm, NFormItem,
  NInput, NSelect, NSpace, NEmpty, NGrid, NGi, NStatistic,
  useMessage, NTabs, NTabPane,
} from 'naive-ui'
import { useAuthStore } from '../stores/auth'
import { hrRequestAPI } from '../api/client'

const { t } = useI18n()
const message = useMessage()
const auth = useAuthStore()

const loading = ref(false)
const myRequests = ref<any[]>([])
const allRequests = ref<any[]>([])
const stats = ref<any>(null)
const showCreateModal = ref(false)
const showDetailModal = ref(false)
const selectedRequest = ref<any>(null)
const activeTab = ref('my')

const form = ref({
  request_type: 'coe',
  subject: '',
  description: '',
  priority: 'normal',
})

const requestTypeOptions = [
  { label: t('hrRequest.coe'), value: 'coe' },
  { label: t('hrRequest.salaryCert'), value: 'salary_cert' },
  { label: t('hrRequest.idReplacement'), value: 'id_replacement' },
  { label: t('hrRequest.equipment'), value: 'equipment' },
  { label: t('hrRequest.scheduleChange'), value: 'schedule_change' },
  { label: t('hrRequest.general'), value: 'general' },
]

const priorityOptions = [
  { label: t('hrRequest.low'), value: 'low' },
  { label: t('hrRequest.normal'), value: 'normal' },
  { label: t('hrRequest.high'), value: 'high' },
  { label: t('hrRequest.urgent'), value: 'urgent' },
]

const statusOptions = [
  { label: t('hrRequest.open'), value: 'open' },
  { label: t('hrRequest.inProgress'), value: 'in_progress' },
  { label: t('hrRequest.resolved'), value: 'resolved' },
  { label: t('hrRequest.closed'), value: 'closed' },
]

type TagType = 'default' | 'info' | 'success' | 'warning' | 'error' | 'primary'

const statusType: Record<string, TagType> = {
  open: 'warning',
  in_progress: 'info',
  resolved: 'success',
  closed: 'default',
  cancelled: 'error',
}

const priorityType: Record<string, TagType> = {
  low: 'default',
  normal: 'info',
  high: 'warning',
  urgent: 'error',
}

const myColumns = [
  { title: t('hrRequest.type'), key: 'request_type', width: 120,
    render: (row: any) => t('hrRequest.' + row.request_type) || row.request_type },
  { title: t('hrRequest.subject'), key: 'subject', ellipsis: { tooltip: true } },
  { title: t('hrRequest.priority'), key: 'priority', width: 90,
    render: (row: any) => h(NTag, { type: priorityType[row.priority] || 'default', size: 'small', bordered: false },
      () => t('hrRequest.' + row.priority) || row.priority)
  },
  { title: t('hrRequest.status'), key: 'status', width: 110,
    render: (row: any) => h(NTag, { type: statusType[row.status] || 'default', size: 'small', bordered: false },
      () => t('hrRequest.' + row.status) || row.status)
  },
  { title: t('hrRequest.date'), key: 'created_at', width: 100,
    render: (row: any) => new Date(row.created_at).toLocaleDateString() },
]

const adminColumns = [
  { title: t('hrRequest.employee'), key: 'first_name', width: 150,
    render: (row: any) => `${row.first_name} ${row.last_name}` },
  { title: t('hrRequest.department'), key: 'department', width: 120 },
  ...myColumns,
]

// Filters
const filterStatus = ref('')
const filterType = ref('')

async function loadData() {
  loading.value = true
  try {
    const results = await Promise.allSettled([
      hrRequestAPI.listMy(),
      auth.isAdmin || auth.isManager ? hrRequestAPI.list({ status: filterStatus.value, request_type: filterType.value }) : Promise.resolve(null),
      auth.isAdmin || auth.isManager ? hrRequestAPI.getStats() : Promise.resolve(null),
    ])

    if (results[0].status === 'fulfilled') {
      const data = (results[0].value as any)?.data ?? results[0].value
      myRequests.value = Array.isArray(data) ? data : []
    }
    if (results[1].status === 'fulfilled' && results[1].value) {
      const data = (results[1].value as any)?.data ?? results[1].value
      allRequests.value = Array.isArray(data) ? data : []
    }
    if (results[2].status === 'fulfilled' && results[2].value) {
      stats.value = (results[2].value as any)?.data ?? results[2].value
    }
  } catch {
    message.error('Failed to load requests')
  } finally {
    loading.value = false
  }
}

function openCreate() {
  form.value = { request_type: 'coe', subject: '', description: '', priority: 'normal' }
  showCreateModal.value = true
}

async function submitRequest() {
  if (!form.value.subject.trim()) {
    message.warning(t('hrRequest.enterSubject'))
    return
  }
  try {
    await hrRequestAPI.create(form.value)
    message.success(t('hrRequest.created'))
    showCreateModal.value = false
    loadData()
  } catch {
    message.error('Failed to create request')
  }
}

// Status update
const updateForm = ref({ status: '', resolution_note: '' })

function openDetail(row: any) {
  selectedRequest.value = row
  updateForm.value = { status: row.status, resolution_note: '' }
  showDetailModal.value = true
}

async function updateStatus() {
  if (!selectedRequest.value) return
  try {
    await hrRequestAPI.updateStatus(selectedRequest.value.id, updateForm.value)
    message.success(t('hrRequest.updated'))
    showDetailModal.value = false
    loadData()
  } catch {
    message.error('Failed to update request')
  }
}

const openCount = computed(() => stats.value?.open_count ?? 0)

onMounted(loadData)
</script>

<template>
  <div style="max-width: 1200px; margin: 0 auto;">
    <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 24px;">
      <div>
        <h2 style="margin: 0;">{{ t('hrRequest.title') }}</h2>
        <p style="margin: 4px 0 0; opacity: 0.7;">{{ t('hrRequest.subtitle') }}</p>
      </div>
      <NButton type="primary" @click="openCreate">
        {{ t('hrRequest.newRequest') }}
      </NButton>
    </div>

    <!-- Stats (admin only) -->
    <NGrid v-if="auth.isAdmin || auth.isManager" :cols="4" :x-gap="16" :y-gap="16" responsive="screen" :item-responsive="true" style="margin-bottom: 24px;">
      <NGi span="4 m:2 l:1">
        <NCard>
          <NStatistic :label="t('hrRequest.openRequests')" :value="openCount" />
        </NCard>
      </NGi>
      <NGi v-for="s in (stats?.by_status || [])" :key="s.status" span="4 m:2 l:1">
        <NCard>
          <NStatistic :label="t('hrRequest.' + s.status) || s.status" :value="s.count" />
        </NCard>
      </NGi>
    </NGrid>

    <NTabs v-model:value="activeTab" type="line">
      <!-- My Requests -->
      <NTabPane name="my" :tab="t('hrRequest.myRequests')">
        <NEmpty v-if="myRequests.length === 0 && !loading" :description="t('hrRequest.noRequests')" style="padding: 40px 0;" />
        <NDataTable
          v-else
          :columns="myColumns"
          :data="myRequests"
          :loading="loading"
          :bordered="false"
          :single-line="false"
          :row-props="(row: any) => ({ style: 'cursor: pointer;', onClick: () => openDetail(row) })"
        />
      </NTabPane>

      <!-- All Requests (admin) -->
      <NTabPane v-if="auth.isAdmin || auth.isManager" name="all" :tab="t('hrRequest.allRequests')">
        <div style="display: flex; gap: 12px; margin-bottom: 12px; padding-top: 12px;">
          <NSelect
            v-model:value="filterStatus"
            :options="[{ label: t('hrRequest.allStatuses'), value: '' }, ...statusOptions]"
            style="width: 160px;"
            @update:value="loadData"
          />
          <NSelect
            v-model:value="filterType"
            :options="[{ label: t('hrRequest.allTypes'), value: '' }, ...requestTypeOptions]"
            style="width: 180px;"
            @update:value="loadData"
          />
        </div>

        <NEmpty v-if="allRequests.length === 0 && !loading" :description="t('hrRequest.noRequests')" style="padding: 40px 0;" />
        <NDataTable
          v-else
          :columns="adminColumns"
          :data="allRequests"
          :loading="loading"
          :bordered="false"
          :single-line="false"
          :row-props="(row: any) => ({ style: 'cursor: pointer;', onClick: () => openDetail(row) })"
        />
      </NTabPane>
    </NTabs>

    <!-- Create Modal -->
    <NModal v-model:show="showCreateModal" preset="card" :title="t('hrRequest.newRequest')" style="max-width: 550px;">
      <NForm label-placement="left" label-width="100">
        <NFormItem :label="t('hrRequest.type')">
          <NSelect v-model:value="form.request_type" :options="requestTypeOptions" />
        </NFormItem>
        <NFormItem :label="t('hrRequest.subject')">
          <NInput v-model:value="form.subject" :placeholder="t('hrRequest.subjectPlaceholder')" />
        </NFormItem>
        <NFormItem :label="t('hrRequest.description')">
          <NInput v-model:value="form.description" type="textarea" :rows="3" />
        </NFormItem>
        <NFormItem :label="t('hrRequest.priority')">
          <NSelect v-model:value="form.priority" :options="priorityOptions" />
        </NFormItem>
      </NForm>
      <template #action>
        <NSpace justify="end">
          <NButton @click="showCreateModal = false">{{ t('hrRequest.cancel') }}</NButton>
          <NButton type="primary" @click="submitRequest">{{ t('hrRequest.submit') }}</NButton>
        </NSpace>
      </template>
    </NModal>

    <!-- Detail / Update Modal -->
    <NModal v-model:show="showDetailModal" preset="card" :title="t('hrRequest.requestDetail')" style="max-width: 600px;">
      <div v-if="selectedRequest">
        <div style="display: grid; grid-template-columns: 100px 1fr; gap: 8px 16px; margin-bottom: 16px;">
          <span style="opacity: 0.6;">{{ t('hrRequest.type') }}</span>
          <span>{{ t('hrRequest.' + selectedRequest.request_type) || selectedRequest.request_type }}</span>
          <span style="opacity: 0.6;">{{ t('hrRequest.subject') }}</span>
          <strong>{{ selectedRequest.subject }}</strong>
          <span style="opacity: 0.6;">{{ t('hrRequest.description') }}</span>
          <span>{{ selectedRequest.description || '-' }}</span>
          <span style="opacity: 0.6;">{{ t('hrRequest.priority') }}</span>
          <NTag :type="priorityType[selectedRequest.priority]" size="small" :bordered="false">
            {{ t('hrRequest.' + selectedRequest.priority) }}
          </NTag>
          <span style="opacity: 0.6;">{{ t('hrRequest.status') }}</span>
          <NTag :type="statusType[selectedRequest.status]" size="small" :bordered="false">
            {{ t('hrRequest.' + selectedRequest.status) }}
          </NTag>
          <span style="opacity: 0.6;">{{ t('hrRequest.date') }}</span>
          <span>{{ new Date(selectedRequest.created_at).toLocaleString() }}</span>
          <template v-if="selectedRequest.resolution_note">
            <span style="opacity: 0.6;">{{ t('hrRequest.resolution') }}</span>
            <span>{{ selectedRequest.resolution_note }}</span>
          </template>
        </div>

        <!-- Admin update section -->
        <div v-if="auth.isAdmin || auth.isManager" style="border-top: 1px solid var(--n-border-color); padding-top: 16px;">
          <NForm label-placement="left" label-width="100">
            <NFormItem :label="t('hrRequest.status')">
              <NSelect v-model:value="updateForm.status" :options="statusOptions" />
            </NFormItem>
            <NFormItem :label="t('hrRequest.resolution')">
              <NInput v-model:value="updateForm.resolution_note" type="textarea" :rows="2" :placeholder="t('hrRequest.resolutionPlaceholder')" />
            </NFormItem>
          </NForm>
          <NSpace justify="end">
            <NButton type="primary" @click="updateStatus">{{ t('hrRequest.update') }}</NButton>
          </NSpace>
        </div>
      </div>
    </NModal>
  </div>
</template>
