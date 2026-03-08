<script setup lang="ts">
import { ref, h, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NTabs, NTabPane, NDataTable, NButton, NModal, NForm, NFormItem,
  NInput, NSelect, NSpace, NTag, NStatistic, NGrid, NGi,
  NSwitch, useMessage, type DataTableColumns,
} from 'naive-ui'
import { grievanceAPI } from '../api/client'
import { useAuthStore } from '../stores/auth'
import { format } from 'date-fns'

const { t } = useI18n()
const message = useMessage()
const auth = useAuthStore()

interface GrievanceCase {
  id: number
  case_number: string
  employee_id: number
  employee_no?: string
  first_name?: string
  last_name?: string
  category: string
  subject: string
  description: string
  severity: string
  status: string
  assigned_to: number | null
  assigned_to_name?: string
  resolution: string | null
  resolution_date: string | null
  is_anonymous: boolean
  created_at: string
}

interface Summary {
  total: number
  open_count: number
  under_review: number
  in_mediation: number
  resolved: number
  critical_open: number
}

interface Comment {
  id: number
  grievance_id: number
  user_id: number
  user_name: string
  comment: string
  is_internal: boolean
  created_at: string
}

const cases = ref<GrievanceCase[]>([])
const myCases = ref<GrievanceCase[]>([])
const caseTotal = ref(0)
const summary = ref<Summary>({ total: 0, open_count: 0, under_review: 0, in_mediation: 0, resolved: 0, critical_open: 0 })
const loading = ref(false)
const filterStatus = ref('')
const filterCategory = ref('')

// Create modal
const showCreateModal = ref(false)
const createForm = ref({
  category: 'workplace_safety',
  subject: '',
  description: '',
  severity: 'medium',
  is_anonymous: false,
})

// Resolve modal
const showResolveModal = ref(false)
const resolveId = ref(0)
const resolution = ref('')

// Assign modal
const showAssignModal = ref(false)
const assignId = ref(0)
const assignUserId = ref('')

// Comments modal
const showCommentsModal = ref(false)
const commentsGrievanceId = ref(0)
const comments = ref<Comment[]>([])
const newComment = ref('')
const isInternalComment = ref(false)

const isManager = computed(() => auth.isAdmin || auth.isManager)

const categoryOptions = [
  { label: t('grievance.workplaceSafety'), value: 'workplace_safety' },
  { label: t('grievance.harassment'), value: 'harassment' },
  { label: t('grievance.discrimination'), value: 'discrimination' },
  { label: t('grievance.policyViolation'), value: 'policy_violation' },
  { label: t('grievance.compensation'), value: 'compensation' },
  { label: t('grievance.workingConditions'), value: 'working_conditions' },
  { label: t('grievance.other'), value: 'other' },
]

const severityOptions = [
  { label: t('grievance.low'), value: 'low' },
  { label: t('grievance.medium'), value: 'medium' },
  { label: t('grievance.high'), value: 'high' },
  { label: t('grievance.critical'), value: 'critical' },
]

const statusOptions = [
  { label: t('common.all'), value: '' },
  { label: t('grievance.open'), value: 'open' },
  { label: t('grievance.underReview'), value: 'under_review' },
  { label: t('grievance.inMediation'), value: 'in_mediation' },
  { label: t('grievance.resolved'), value: 'resolved' },
  { label: t('grievance.closed'), value: 'closed' },
  { label: t('grievance.withdrawn'), value: 'withdrawn' },
]

const statusColor: Record<string, string> = {
  open: 'warning',
  under_review: 'info',
  in_mediation: 'warning',
  resolved: 'success',
  closed: 'default',
  withdrawn: 'default',
}

const severityColor: Record<string, string> = {
  low: 'default',
  medium: 'info',
  high: 'warning',
  critical: 'error',
}

const caseColumns = computed<DataTableColumns<GrievanceCase>>(() => [
  { title: t('grievance.caseNumber'), key: 'case_number', width: 110 },
  {
    title: t('grievance.employee'), key: 'employee',
    render: (row) => row.is_anonymous ? t('grievance.anonymous') : (row.first_name ? `${row.first_name} ${row.last_name}` : '-'),
  },
  { title: t('grievance.subject'), key: 'subject', ellipsis: { tooltip: true } },
  {
    title: t('grievance.category'), key: 'category',
    render: (row) => h(NTag, { size: 'small' }, () => t(`grievance.${row.category}`) || row.category),
  },
  {
    title: t('grievance.severity'), key: 'severity',
    render: (row) => h(NTag, { size: 'small', type: (severityColor[row.severity] || 'default') as any }, () => t(`grievance.${row.severity}`)),
  },
  {
    title: t('common.status'), key: 'status',
    render: (row) => h(NTag, { size: 'small', type: (statusColor[row.status] || 'default') as any }, () => t(`grievance.${row.status}`) || row.status),
  },
  { title: t('grievance.assignedTo'), key: 'assigned_to_name', render: (row) => row.assigned_to_name || '-' },
  { title: t('grievance.date'), key: 'created_at', render: (row) => format(new Date(row.created_at), 'yyyy-MM-dd') },
  {
    title: t('common.actions'), key: 'actions',
    render: (row) => {
      const btns: ReturnType<typeof h>[] = []
      btns.push(h(NButton, { size: 'small', onClick: () => openComments(row.id) }, () => t('grievance.comments')))
      if (row.status === 'open' && isManager.value) {
        btns.push(h(NButton, { size: 'small', type: 'info', onClick: () => { assignId.value = row.id; showAssignModal.value = true } }, () => t('grievance.assign')))
      }
      if (['open', 'under_review', 'in_mediation'].includes(row.status) && isManager.value) {
        btns.push(h(NButton, { size: 'small', type: 'success', onClick: () => { resolveId.value = row.id; showResolveModal.value = true } }, () => t('grievance.resolve')))
      }
      return h(NSpace, { size: 4 }, () => btns)
    },
  },
])

const myCaseColumns = computed<DataTableColumns<GrievanceCase>>(() => [
  { title: t('grievance.caseNumber'), key: 'case_number', width: 110 },
  { title: t('grievance.subject'), key: 'subject', ellipsis: { tooltip: true } },
  {
    title: t('grievance.category'), key: 'category',
    render: (row) => h(NTag, { size: 'small' }, () => t(`grievance.${row.category}`) || row.category),
  },
  {
    title: t('grievance.severity'), key: 'severity',
    render: (row) => h(NTag, { size: 'small', type: (severityColor[row.severity] || 'default') as any }, () => t(`grievance.${row.severity}`)),
  },
  {
    title: t('common.status'), key: 'status',
    render: (row) => h(NTag, { size: 'small', type: (statusColor[row.status] || 'default') as any }, () => t(`grievance.${row.status}`) || row.status),
  },
  { title: t('grievance.date'), key: 'created_at', render: (row) => format(new Date(row.created_at), 'yyyy-MM-dd') },
  {
    title: t('common.actions'), key: 'actions',
    render: (row) => {
      const btns: ReturnType<typeof h>[] = []
      if (row.status === 'open') {
        btns.push(h(NButton, { size: 'small', type: 'error', onClick: () => withdrawCase(row.id) }, () => t('grievance.withdraw')))
      }
      return h(NSpace, { size: 4 }, () => btns)
    },
  },
])

function extractData(res: unknown): any[] {
  const d = (res as any)?.data ?? res
  return Array.isArray(d) ? d : []
}

async function fetchAll() {
  loading.value = true
  try {
    const myRes = await grievanceAPI.my()
    myCases.value = extractData(myRes)

    if (isManager.value) {
      const params: Record<string, string> = {}
      if (filterStatus.value) params.status = filterStatus.value
      if (filterCategory.value) params.category = filterCategory.value
      const [casesRes, summaryRes] = await Promise.all([
        grievanceAPI.list(params),
        grievanceAPI.summary(),
      ])
      const caseData = (casesRes as any)?.data ?? casesRes
      cases.value = caseData?.items ?? []
      caseTotal.value = caseData?.total ?? 0
      summary.value = (summaryRes as any)?.data ?? summaryRes
    }
  } catch {
    message.error(t('grievance.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function submitGrievance() {
  if (!createForm.value.subject || !createForm.value.description) {
    message.warning(t('common.fillRequired'))
    return
  }
  try {
    await grievanceAPI.create(createForm.value)
    message.success(t('common.created'))
    showCreateModal.value = false
    fetchAll()
  } catch {
    message.error(t('common.error'))
  }
}

async function submitResolve() {
  if (!resolution.value) {
    message.warning(t('common.fillRequired'))
    return
  }
  try {
    await grievanceAPI.resolve(resolveId.value, resolution.value)
    message.success(t('grievance.caseResolved'))
    showResolveModal.value = false
    resolution.value = ''
    fetchAll()
  } catch {
    message.error(t('common.error'))
  }
}

async function submitAssign() {
  const uid = parseInt(assignUserId.value)
  if (!uid) {
    message.warning(t('common.fillRequired'))
    return
  }
  try {
    await grievanceAPI.assign(assignId.value, uid)
    message.success(t('grievance.caseAssigned'))
    showAssignModal.value = false
    fetchAll()
  } catch {
    message.error(t('common.error'))
  }
}

async function withdrawCase(id: number) {
  try {
    await grievanceAPI.withdraw(id)
    message.success(t('grievance.caseWithdrawn'))
    fetchAll()
  } catch {
    message.error(t('common.error'))
  }
}

async function openComments(id: number) {
  commentsGrievanceId.value = id
  newComment.value = ''
  isInternalComment.value = false
  showCommentsModal.value = true
  try {
    const res = await grievanceAPI.listComments(id)
    comments.value = extractData(res)
  } catch (e) { console.error('Failed to load comments', e); comments.value = [] }
}

async function submitComment() {
  if (!newComment.value) return
  try {
    await grievanceAPI.addComment(commentsGrievanceId.value, newComment.value, isInternalComment.value)
    newComment.value = ''
    const res = await grievanceAPI.listComments(commentsGrievanceId.value)
    comments.value = extractData(res)
    message.success(t('grievance.commentAdded'))
  } catch {
    message.error(t('common.error'))
  }
}

onMounted(fetchAll)
</script>

<template>
  <div>
    <h2 style="margin-bottom: 16px;">{{ t('grievance.title') }}</h2>

    <!-- Summary (Manager+) -->
    <NGrid v-if="isManager" :cols="6" :x-gap="16" :y-gap="16" responsive="screen" style="margin-bottom: 24px;">
      <NGi><NStatistic :label="t('grievance.total')" :value="summary.total" /></NGi>
      <NGi><NStatistic :label="t('grievance.open')" :value="summary.open_count" /></NGi>
      <NGi><NStatistic :label="t('grievance.underReview')" :value="summary.under_review" /></NGi>
      <NGi><NStatistic :label="t('grievance.inMediation')" :value="summary.in_mediation" /></NGi>
      <NGi><NStatistic :label="t('grievance.resolved')" :value="summary.resolved" /></NGi>
      <NGi><NStatistic :label="t('grievance.criticalOpen')" :value="summary.critical_open" /></NGi>
    </NGrid>

    <NTabs type="line">
      <!-- My Grievances -->
      <NTabPane :name="t('grievance.myCases')" :tab="t('grievance.myCases')">
        <NSpace style="margin-bottom: 12px;">
          <NButton type="primary" @click="showCreateModal = true">{{ t('grievance.fileGrievance') }}</NButton>
        </NSpace>
        <NDataTable :columns="myCaseColumns" :data="myCases" :loading="loading" :bordered="false" />
      </NTabPane>

      <!-- All Cases (Manager+) -->
      <NTabPane v-if="isManager" :name="t('grievance.allCases')" :tab="t('grievance.allCases')">
        <NSpace style="margin-bottom: 12px;" align="center">
          <NSelect v-model:value="filterStatus" :options="statusOptions" style="width: 160px;" @update:value="fetchAll" />
          <NSelect v-model:value="filterCategory" :options="[{ label: t('common.all'), value: '' }, ...categoryOptions]" style="width: 180px;" @update:value="fetchAll" />
        </NSpace>
        <NDataTable :columns="caseColumns" :data="cases" :loading="loading" :bordered="false" />
      </NTabPane>
    </NTabs>

    <!-- Create Modal -->
    <NModal v-model:show="showCreateModal" preset="card" :title="t('grievance.fileGrievance')" style="max-width: 600px; width: 95vw;">
      <NForm label-placement="left" label-width="120">
        <NFormItem :label="t('grievance.category')">
          <NSelect v-model:value="createForm.category" :options="categoryOptions" />
        </NFormItem>
        <NFormItem :label="t('grievance.severity')">
          <NSelect v-model:value="createForm.severity" :options="severityOptions" />
        </NFormItem>
        <NFormItem :label="t('grievance.subject')">
          <NInput v-model:value="createForm.subject" />
        </NFormItem>
        <NFormItem :label="t('grievance.description')">
          <NInput v-model:value="createForm.description" type="textarea" :rows="4" />
        </NFormItem>
        <NFormItem :label="t('grievance.anonymous')">
          <NSwitch v-model:value="createForm.is_anonymous" />
        </NFormItem>
        <NFormItem>
          <NButton type="primary" @click="submitGrievance">{{ t('grievance.submit') }}</NButton>
        </NFormItem>
      </NForm>
    </NModal>

    <!-- Resolve Modal -->
    <NModal v-model:show="showResolveModal" preset="card" :title="t('grievance.resolveCase')" style="max-width: 500px; width: 95vw;">
      <NForm label-placement="left" label-width="100">
        <NFormItem :label="t('grievance.resolution')">
          <NInput v-model:value="resolution" type="textarea" :rows="4" />
        </NFormItem>
        <NFormItem>
          <NButton type="success" @click="submitResolve">{{ t('grievance.resolve') }}</NButton>
        </NFormItem>
      </NForm>
    </NModal>

    <!-- Assign Modal -->
    <NModal v-model:show="showAssignModal" preset="card" :title="t('grievance.assignCase')" style="max-width: 400px; width: 95vw;">
      <NForm label-placement="left" label-width="100">
        <NFormItem :label="t('grievance.assignTo')">
          <NInput v-model:value="assignUserId" placeholder="User ID" />
        </NFormItem>
        <NFormItem>
          <NButton type="primary" @click="submitAssign">{{ t('grievance.assign') }}</NButton>
        </NFormItem>
      </NForm>
    </NModal>

    <!-- Comments Modal -->
    <NModal v-model:show="showCommentsModal" preset="card" :title="t('grievance.comments')" style="max-width: 600px; width: 95vw;">
      <div style="max-height: 400px; overflow-y: auto; margin-bottom: 16px;">
        <div v-if="comments.length === 0" style="color: var(--n-text-color-3); text-align: center; padding: 24px;">
          {{ t('grievance.noComments') }}
        </div>
        <div v-for="c in comments" :key="c.id" style="padding: 8px 0; border-bottom: 1px solid var(--n-border-color);">
          <div style="display: flex; justify-content: space-between; margin-bottom: 4px;">
            <strong>{{ c.user_name }}</strong>
            <NSpace :size="8" align="center">
              <NTag v-if="c.is_internal" size="tiny" type="warning">{{ t('grievance.internal') }}</NTag>
              <span style="font-size: 12px; color: var(--n-text-color-3);">{{ format(new Date(c.created_at), 'yyyy-MM-dd HH:mm') }}</span>
            </NSpace>
          </div>
          <div>{{ c.comment }}</div>
        </div>
      </div>
      <NSpace v-if="isManager" vertical>
        <NInput v-model:value="newComment" type="textarea" :rows="2" :placeholder="t('grievance.addComment')" />
        <NSpace>
          <NSwitch v-model:value="isInternalComment">
            <template #checked>{{ t('grievance.internal') }}</template>
            <template #unchecked>{{ t('grievance.public') }}</template>
          </NSwitch>
          <NButton type="primary" @click="submitComment" :disabled="!newComment">{{ t('grievance.send') }}</NButton>
        </NSpace>
      </NSpace>
    </NModal>
  </div>
</template>
