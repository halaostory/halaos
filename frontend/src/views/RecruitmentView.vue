<script setup lang="ts">
import { ref, h, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NTabs, NTabPane, NDataTable, NButton, NModal, NForm, NFormItem,
  NInput, NInputNumber, NSelect, NSpace, NTag, NCard, NStatistic,
  NGrid, NGi, NDrawer, NDrawerContent, NDescriptions, NDescriptionsItem,
  NProgress, NTimeline, NTimelineItem, NDatePicker,
  useMessage, type DataTableColumns,
} from 'naive-ui'
import { recruitmentAPI } from '../api/client'
import { useAuthStore } from '../stores/auth'
import { format } from 'date-fns'

const { t } = useI18n()
const message = useMessage()
const auth = useAuthStore()
const activeTab = ref('jobs')
const loading = ref(false)

// --- Stats ---
interface Stats { open_positions: number; total_applicants: number; in_pipeline: number; hired_this_month: number }
const stats = ref<Stats>({ open_positions: 0, total_applicants: 0, in_pipeline: 0, hired_this_month: 0 })

// --- Job Postings ---
const jobs = ref<Record<string, unknown>[]>([])
const jobStatusFilter = ref('')
const showJobModal = ref(false)
const editingJobId = ref<number | null>(null)

function createEmptyJobForm() {
  return { title: '', department: '', description: '', requirements: '',
    salary_min: null as number | null, salary_max: null as number | null,
    employment_type: 'regular', location: '' }
}
const jobForm = ref(createEmptyJobForm())

const jobStatusOptions = [
  { label: t('recruitment.all'), value: '' },
  { label: t('recruitment.draft'), value: 'draft' },
  { label: t('recruitment.open'), value: 'open' },
  { label: t('recruitment.closed'), value: 'closed' },
  { label: t('recruitment.onHold'), value: 'on_hold' },
]
const employmentTypeOptions = [
  { label: t('recruitment.regular'), value: 'regular' },
  { label: t('recruitment.contractual'), value: 'contractual' },
  { label: t('recruitment.probationary'), value: 'probationary' },
  { label: t('recruitment.partTime'), value: 'part_time' },
  { label: t('recruitment.intern'), value: 'intern' },
]
const jobStatusColor: Record<string, 'default' | 'success' | 'info' | 'warning'> = {
  draft: 'default', open: 'success', closed: 'info', on_hold: 'warning',
}

function statusBtn(label: string, type: string, onClick: () => void) {
  return h(NButton, { size: 'small', type: type as 'primary', onClick }, () => label)
}

const jobColumns = computed<DataTableColumns>(() => [
  { title: t('recruitment.jobTitle'), key: 'title', ellipsis: { tooltip: true } },
  { title: t('recruitment.department'), key: 'department', width: 130 },
  { title: t('common.type'), key: 'employment_type', width: 120,
    render: (r) => {
      const m: Record<string, string> = { regular: t('recruitment.regular'), contractual: t('recruitment.contractual'),
        probationary: t('recruitment.probationary'), part_time: t('recruitment.partTime'), intern: t('recruitment.intern') }
      return m[r.employment_type as string] || String(r.employment_type)
    } },
  { title: t('recruitment.location'), key: 'location', width: 120 },
  { title: t('common.status'), key: 'status', width: 100,
    render: (r) => h(NTag, { type: jobStatusColor[r.status as string] || 'default', size: 'small' }, () => String(r.status)) },
  { title: t('recruitment.applicants'), key: 'applicant_count', width: 90 },
  { title: t('common.actions'), key: 'actions', width: 200,
    render: (r) => {
      const btns = [statusBtn(t('common.edit'), 'default', () => openEditJob(r))]
      const id = r.id as number
      if (r.status === 'draft') btns.push(statusBtn(t('recruitment.publish'), 'primary', () => updateJobStatus(id, 'open')))
      if (r.status === 'open') {
        btns.push(statusBtn(t('recruitment.hold'), 'warning', () => updateJobStatus(id, 'on_hold')))
        btns.push(statusBtn(t('recruitment.close'), 'info', () => updateJobStatus(id, 'closed')))
      }
      if (r.status === 'on_hold') btns.push(statusBtn(t('recruitment.reopen'), 'primary', () => updateJobStatus(id, 'open')))
      return h(NSpace, { size: 4 }, () => btns)
    } },
])

// --- Applicants ---
const applicants = ref<Record<string, unknown>[]>([])
const applicantJobFilter = ref('')
const applicantStatusFilter = ref('')
const showApplicantDrawer = ref(false)
const selectedApplicant = ref<Record<string, unknown> | null>(null)
const applicantTimeline = ref<Record<string, unknown>[]>([])

const applicantStatusOptions = [
  { label: t('recruitment.all'), value: '' },
  { label: t('recruitment.new'), value: 'new' },
  { label: t('recruitment.screening'), value: 'screening' },
  { label: t('recruitment.interview'), value: 'interview' },
  { label: t('recruitment.offer'), value: 'offer' },
  { label: t('recruitment.hired'), value: 'hired' },
  { label: t('recruitment.rejected'), value: 'rejected' },
  { label: t('recruitment.withdrawn'), value: 'withdrawn' },
]
const applicantStatusColor: Record<string, 'default' | 'info' | 'warning' | 'success' | 'error'> = {
  new: 'default', screening: 'info', interview: 'warning', offer: 'success',
  hired: 'success', rejected: 'error', withdrawn: 'default',
}
const jobFilterOptions = computed(() => {
  const opts: { label: string; value: string }[] = [{ label: t('recruitment.allJobs'), value: '' }]
  for (const job of jobs.value) opts.push({ label: String(job.title), value: String(job.id) })
  return opts
})

function fmtDate(d: unknown): string {
  if (!d) return '-'
  try { return format(new Date(d as string), 'yyyy-MM-dd') } catch { return String(d) }
}

function scoreColor(score: number): string {
  return score >= 80 ? '#18a058' : score >= 60 ? '#f0a020' : '#d03050'
}

const applicantColumns = computed<DataTableColumns>(() => [
  { title: t('common.name'), key: 'name', width: 150,
    render: (r) => `${r.first_name || ''} ${r.last_name || ''}`.trim() || String(r.name || '-') },
  { title: t('common.email'), key: 'email', width: 180, ellipsis: { tooltip: true } },
  { title: t('recruitment.jobTitle'), key: 'job_title', width: 150, ellipsis: { tooltip: true } },
  { title: t('recruitment.aiScore'), key: 'ai_score', width: 120,
    render: (r) => {
      const s = Number(r.ai_score || 0)
      return s ? h(NProgress, { type: 'line', percentage: s, showIndicator: true, color: scoreColor(s) }) : '-'
    } },
  { title: t('common.status'), key: 'status', width: 110,
    render: (r) => h(NTag, { type: applicantStatusColor[r.status as string] || 'default', size: 'small' },
      () => t(`recruitment.${r.status}`) || String(r.status)) },
  { title: t('recruitment.source'), key: 'source', width: 100 },
  { title: t('recruitment.appliedDate'), key: 'applied_date', width: 110,
    render: (r) => fmtDate(r.applied_date || r.created_at) },
  { title: t('common.actions'), key: 'actions', width: 80,
    render: (r) => h(NButton, { size: 'small', onClick: () => viewApplicant(r) }, () => t('common.view')) },
])

// --- Schedule Interview ---
const showInterviewModal = ref(false)
const interviewForm = ref({ interview_date: null as number | null, interview_type: 'in_person', interviewer: '', notes: '' })
const interviewTypeOptions = [
  { label: t('recruitment.inPerson'), value: 'in_person' },
  { label: t('recruitment.phoneInterview'), value: 'phone' },
  { label: t('recruitment.video'), value: 'video' },
]

// --- Helpers ---
function extractData(res: unknown): unknown { return (res as Record<string, unknown>)?.data ?? res }
function extractArray(res: unknown): Record<string, unknown>[] {
  const d = extractData(res); return Array.isArray(d) ? d : []
}
function handleErr(e: unknown) { message.error(e instanceof Error ? e.message : String(e)) }
function fmtTs(ts: number | null): string {
  if (!ts) return ''
  const d = new Date(ts)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

// --- Data Loading ---
async function loadStats() {
  try { const res = await recruitmentAPI.getStats(); stats.value = { ...stats.value, ...(extractData(res) as Stats) } }
  catch { /* optional */ }
}
async function loadJobs() {
  loading.value = true
  try {
    const p: Record<string, string> = {}
    if (jobStatusFilter.value) p.status = jobStatusFilter.value
    jobs.value = extractArray(await recruitmentAPI.listJobs(p))
  } catch (e) { handleErr(e); jobs.value = [] }
  finally { loading.value = false }
}
async function loadApplicants() {
  loading.value = true
  try {
    const p: Record<string, string> = {}
    if (applicantJobFilter.value) p.job_id = applicantJobFilter.value
    if (applicantStatusFilter.value) p.status = applicantStatusFilter.value
    applicants.value = extractArray(await recruitmentAPI.listApplicants(p))
  } catch (e) { handleErr(e); applicants.value = [] }
  finally { loading.value = false }
}

// --- Job CRUD ---
function openNewJob() { editingJobId.value = null; jobForm.value = createEmptyJobForm(); showJobModal.value = true }
function openEditJob(row: Record<string, unknown>) {
  editingJobId.value = row.id as number
  jobForm.value = {
    title: String(row.title || ''), department: String(row.department || ''),
    description: String(row.description || ''), requirements: String(row.requirements || ''),
    salary_min: row.salary_min != null ? Number(row.salary_min) : null,
    salary_max: row.salary_max != null ? Number(row.salary_max) : null,
    employment_type: String(row.employment_type || 'regular'), location: String(row.location || ''),
  }
  showJobModal.value = true
}
async function saveJob() {
  if (!jobForm.value.title) { message.warning(t('common.fillAllFields')); return }
  try {
    const p: Record<string, unknown> = { title: jobForm.value.title, employment_type: jobForm.value.employment_type }
    if (jobForm.value.department) p.department = jobForm.value.department
    if (jobForm.value.description) p.description = jobForm.value.description
    if (jobForm.value.requirements) p.requirements = jobForm.value.requirements
    if (jobForm.value.salary_min != null) p.salary_min = jobForm.value.salary_min
    if (jobForm.value.salary_max != null) p.salary_max = jobForm.value.salary_max
    if (jobForm.value.location) p.location = jobForm.value.location
    if (editingJobId.value) { await recruitmentAPI.updateJob(editingJobId.value, p); message.success(t('recruitment.jobUpdated')) }
    else { await recruitmentAPI.createJob(p); message.success(t('recruitment.jobCreated')) }
    showJobModal.value = false; await loadJobs(); await loadStats()
  } catch (e) { handleErr(e) }
}
async function updateJobStatus(id: number, status: string) {
  try { await recruitmentAPI.updateJob(id, { status }); message.success(t('recruitment.statusUpdated')); await loadJobs(); await loadStats() }
  catch (e) { handleErr(e) }
}

// --- Applicant Detail ---
async function viewApplicant(row: Record<string, unknown>) {
  try {
    const d = extractData(await recruitmentAPI.getApplicant(row.id as number)) as Record<string, unknown>
    selectedApplicant.value = (d.applicant ?? d) as Record<string, unknown>
    applicantTimeline.value = Array.isArray(d.timeline) ? d.timeline as Record<string, unknown>[] : []
    showApplicantDrawer.value = true
  } catch (e) { handleErr(e) }
}
async function updateApplicantStatus(status: string) {
  if (!selectedApplicant.value) return
  try {
    await recruitmentAPI.updateApplicantStatus(selectedApplicant.value.id as number, status, '')
    message.success(t('recruitment.statusUpdated'))
    selectedApplicant.value = { ...selectedApplicant.value, status }
    await loadApplicants(); await loadStats()
  } catch (e) { handleErr(e) }
}
function openScheduleInterview() {
  interviewForm.value = { interview_date: null, interview_type: 'in_person', interviewer: '', notes: '' }
  showInterviewModal.value = true
}
async function scheduleInterview() {
  if (!selectedApplicant.value || !interviewForm.value.interview_date) { message.warning(t('common.fillAllFields')); return }
  try {
    await recruitmentAPI.scheduleInterview(selectedApplicant.value.id as number, {
      interview_date: fmtTs(interviewForm.value.interview_date),
      interview_type: interviewForm.value.interview_type,
      interviewer: interviewForm.value.interviewer, notes: interviewForm.value.notes,
    })
    message.success(t('recruitment.interviewScheduled'))
    showInterviewModal.value = false; await viewApplicant(selectedApplicant.value)
  } catch (e) { handleErr(e) }
}

// --- Tab Change ---
function onTabChange(tab: string | number) {
  activeTab.value = String(tab)
  if (tab === 'jobs') loadJobs()
  else if (tab === 'applicants') { loadJobs(); loadApplicants() }
}
onMounted(() => { loadStats(); loadJobs() })

const isManager = computed(() => auth.isAdmin || auth.isManager)
const canAdvance = (from: string) => selectedApplicant.value?.status === from
</script>

<template>
  <div>
    <NSpace justify="space-between" style="margin-bottom: 16px;">
      <h2>{{ t('recruitment.title') }}</h2>
    </NSpace>

    <!-- Stats Cards -->
    <NGrid cols="1 s:2 m:4" :x-gap="16" :y-gap="16" responsive="screen" :item-responsive="true" style="margin-bottom: 24px;">
      <NGi><NCard><NStatistic :label="t('recruitment.openPositions')" :value="stats.open_positions" /></NCard></NGi>
      <NGi><NCard><NStatistic :label="t('recruitment.totalApplicants')" :value="stats.total_applicants" /></NCard></NGi>
      <NGi><NCard><NStatistic :label="t('recruitment.inPipeline')" :value="stats.in_pipeline" /></NCard></NGi>
      <NGi><NCard><NStatistic :label="t('recruitment.hiredThisMonth')" :value="stats.hired_this_month" /></NCard></NGi>
    </NGrid>

    <!-- Tabs -->
    <NTabs type="line" :value="activeTab" @update:value="onTabChange">
      <NTabPane name="jobs" :tab="t('recruitment.jobPostings')">
        <NSpace style="margin-bottom: 12px;" justify="space-between">
          <NSelect v-model:value="jobStatusFilter" :options="jobStatusOptions" style="width: 180px;" @update:value="loadJobs" />
          <NButton v-if="isManager" type="primary" @click="openNewJob">{{ t('recruitment.newJobPosting') }}</NButton>
        </NSpace>
        <NDataTable :columns="jobColumns" :data="jobs" :loading="loading" size="small" :scroll-x="900" />
      </NTabPane>

      <NTabPane name="applicants" :tab="t('recruitment.applicantsTab')">
        <NSpace style="margin-bottom: 12px;">
          <NSelect v-model:value="applicantJobFilter" :options="jobFilterOptions" style="width: 200px;" @update:value="loadApplicants" />
          <NSelect v-model:value="applicantStatusFilter" :options="applicantStatusOptions" style="width: 180px;" @update:value="loadApplicants" />
        </NSpace>
        <NDataTable :columns="applicantColumns" :data="applicants" :loading="loading" size="small" :scroll-x="1000" />
      </NTabPane>
    </NTabs>

    <!-- Create/Edit Job Modal -->
    <NModal v-model:show="showJobModal" preset="card"
      :title="editingJobId ? t('recruitment.editJob') : t('recruitment.newJobPosting')"
      style="max-width: 600px; width: 95vw;">
      <NForm @submit.prevent="saveJob" label-placement="left" label-width="120">
        <NFormItem :label="t('recruitment.jobTitle')" required><NInput v-model:value="jobForm.title" /></NFormItem>
        <NFormItem :label="t('recruitment.department')"><NInput v-model:value="jobForm.department" /></NFormItem>
        <NFormItem :label="t('recruitment.description')"><NInput v-model:value="jobForm.description" type="textarea" :rows="3" /></NFormItem>
        <NFormItem :label="t('recruitment.requirements')"><NInput v-model:value="jobForm.requirements" type="textarea" :rows="3" /></NFormItem>
        <NFormItem :label="t('recruitment.salaryMin')"><NInputNumber v-model:value="jobForm.salary_min" :min="0" style="width: 100%;" /></NFormItem>
        <NFormItem :label="t('recruitment.salaryMax')"><NInputNumber v-model:value="jobForm.salary_max" :min="0" style="width: 100%;" /></NFormItem>
        <NFormItem :label="t('recruitment.employmentType')"><NSelect v-model:value="jobForm.employment_type" :options="employmentTypeOptions" /></NFormItem>
        <NFormItem :label="t('recruitment.location')"><NInput v-model:value="jobForm.location" /></NFormItem>
        <NSpace>
          <NButton type="primary" attr-type="submit">{{ t('common.save') }}</NButton>
          <NButton @click="showJobModal = false">{{ t('common.cancel') }}</NButton>
        </NSpace>
      </NForm>
    </NModal>

    <!-- Applicant Detail Drawer -->
    <NDrawer v-model:show="showApplicantDrawer" :width="500" placement="right">
      <NDrawerContent :title="t('recruitment.applicantDetail')" closable>
        <template v-if="selectedApplicant">
          <NDescriptions bordered :column="1" size="small" style="margin-bottom: 16px;">
            <NDescriptionsItem :label="t('common.name')">{{ selectedApplicant.first_name }} {{ selectedApplicant.last_name }}</NDescriptionsItem>
            <NDescriptionsItem :label="t('common.email')">{{ selectedApplicant.email }}</NDescriptionsItem>
            <NDescriptionsItem :label="t('common.phone')">{{ selectedApplicant.phone || '-' }}</NDescriptionsItem>
            <NDescriptionsItem :label="t('recruitment.jobTitle')">{{ selectedApplicant.job_title || '-' }}</NDescriptionsItem>
            <NDescriptionsItem :label="t('common.status')">
              <NTag :type="applicantStatusColor[(selectedApplicant.status as string)] || 'default'" size="small">
                {{ t(`recruitment.${selectedApplicant.status}`) || selectedApplicant.status }}
              </NTag>
            </NDescriptionsItem>
            <NDescriptionsItem :label="t('recruitment.aiScore')">
              <NProgress v-if="Number(selectedApplicant.ai_score)" type="line"
                :percentage="Number(selectedApplicant.ai_score)" :color="scoreColor(Number(selectedApplicant.ai_score))"
                style="width: 200px;" />
              <span v-else>-</span>
            </NDescriptionsItem>
            <NDescriptionsItem :label="t('recruitment.source')">{{ selectedApplicant.source || '-' }}</NDescriptionsItem>
            <NDescriptionsItem :label="t('recruitment.appliedDate')">{{ fmtDate(selectedApplicant.applied_date || selectedApplicant.created_at) }}</NDescriptionsItem>
            <NDescriptionsItem v-if="selectedApplicant.resume_url" :label="t('recruitment.resume')">
              <a :href="(selectedApplicant.resume_url as string)" target="_blank">{{ t('recruitment.viewResume') }}</a>
            </NDescriptionsItem>
            <NDescriptionsItem v-if="selectedApplicant.notes" :label="t('recruitment.notes')">{{ selectedApplicant.notes }}</NDescriptionsItem>
          </NDescriptions>

          <!-- Status Update Buttons -->
          <NSpace style="margin-bottom: 16px;" v-if="isManager">
            <NButton v-if="canAdvance('new')" size="small" type="info" @click="updateApplicantStatus('screening')">{{ t('recruitment.moveToScreening') }}</NButton>
            <NButton v-if="canAdvance('screening')" size="small" type="warning" @click="updateApplicantStatus('interview')">{{ t('recruitment.moveToInterview') }}</NButton>
            <NButton v-if="canAdvance('interview')" size="small" type="success" @click="updateApplicantStatus('offer')">{{ t('recruitment.moveToOffer') }}</NButton>
            <NButton v-if="canAdvance('offer')" size="small" type="success" @click="updateApplicantStatus('hired')">{{ t('recruitment.markHired') }}</NButton>
            <NButton v-if="['new','screening','interview','offer'].includes(selectedApplicant.status as string)"
              size="small" type="error" @click="updateApplicantStatus('rejected')">{{ t('recruitment.markRejected') }}</NButton>
            <NButton v-if="canAdvance('interview')" size="small" @click="openScheduleInterview">{{ t('recruitment.scheduleInterview') }}</NButton>
          </NSpace>

          <!-- Interview Timeline -->
          <NCard v-if="applicantTimeline.length" :title="t('recruitment.timeline')" size="small">
            <NTimeline>
              <NTimelineItem v-for="(item, idx) in applicantTimeline" :key="idx"
                :title="String(item.title || item.action || '')" :content="String(item.notes || item.description || '')"
                :time="fmtDate(item.date || item.created_at)"
                :type="(item.type as 'default' | 'info' | 'success' | 'warning' | 'error') || 'default'" />
            </NTimeline>
          </NCard>
        </template>
      </NDrawerContent>
    </NDrawer>

    <!-- Schedule Interview Modal -->
    <NModal v-model:show="showInterviewModal" preset="card" :title="t('recruitment.scheduleInterview')" style="max-width: 480px; width: 95vw;">
      <NForm @submit.prevent="scheduleInterview" label-placement="left" label-width="120">
        <NFormItem :label="t('recruitment.interviewDate')" required>
          <NDatePicker v-model:value="interviewForm.interview_date" type="datetime" style="width: 100%;" />
        </NFormItem>
        <NFormItem :label="t('recruitment.interviewType')"><NSelect v-model:value="interviewForm.interview_type" :options="interviewTypeOptions" /></NFormItem>
        <NFormItem :label="t('recruitment.interviewer')"><NInput v-model:value="interviewForm.interviewer" /></NFormItem>
        <NFormItem :label="t('recruitment.notes')"><NInput v-model:value="interviewForm.notes" type="textarea" :rows="3" /></NFormItem>
        <NSpace>
          <NButton type="primary" attr-type="submit">{{ t('common.save') }}</NButton>
          <NButton @click="showInterviewModal = false">{{ t('common.cancel') }}</NButton>
        </NSpace>
      </NForm>
    </NModal>
  </div>
</template>
