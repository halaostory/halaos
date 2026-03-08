<script setup lang="ts">
import { ref, h, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NTabs, NTabPane, NDataTable, NButton, NModal, NForm, NFormItem,
  NInput, NSelect, NInputNumber, NUpload, NSpace, NTag, NStatistic,
  NGrid, NGi, NDatePicker, NAlert, useMessage, type DataTableColumns, type UploadFileInfo,
} from 'naive-ui'
import { file201API } from '../api/client'
import { useAuthStore } from '../stores/auth'
import { format } from 'date-fns'

const { t } = useI18n()
const message = useMessage()
const auth = useAuthStore()

interface Category {
  id: number
  name: string
  slug: string
  description: string | null
  sort_order: number
  is_system: boolean
}

interface Document {
  id: string
  employee_id: number
  category_id: number | null
  category_name: string | null
  category_slug: string | null
  title: string | null
  doc_type: string
  file_name: string
  file_size: number
  status: string
  expiry_date: string | null
  version: number
  notes: string | null
  created_at: string
}

interface Stats {
  total_documents: number
  active_documents: number
  expired_documents: number
  expiring_soon: number
}

interface ExpiringDoc {
  id: string
  file_name: string
  category_name: string | null
  employee_no: string
  first_name: string
  last_name: string
  expiry_date: string
}

interface ComplianceItem {
  requirement_id: number
  document_name: string
  is_required: boolean
  category_name: string
  is_fulfilled: boolean
  document_id: string | null
  expiry_date: string | null
  document_status: string | null
}

interface Requirement {
  id: number
  category_id: number
  document_name: string
  is_required: boolean
  applies_to: string
  expiry_months: number | null
  category_name: string
}

// State
const categories = ref<Category[]>([])
const selectedEmployeeId = ref<number | null>(null)
const documents = ref<Document[]>([])
const stats = ref<Stats>({ total_documents: 0, active_documents: 0, expired_documents: 0, expiring_soon: 0 })
const expiringDocs = ref<ExpiringDoc[]>([])
const compliance = ref<ComplianceItem[]>([])
const requirements = ref<Requirement[]>([])
const loading = ref(false)
const selectedCategory = ref<number>(0)

// Upload modal
const showUploadModal = ref(false)
const uploadForm = ref({
  title: '',
  doc_type: 'general',
  category_id: null as number | null,
  expiry_date: null as number | null,
  notes: '',
})
const fileList = ref<UploadFileInfo[]>([])

// Requirement modal
const showReqModal = ref(false)
const reqForm = ref({
  category_id: null as number | null,
  document_name: '',
  is_required: 'yes' as string,
  applies_to: 'all',
  expiry_months: null as number | null,
})

const isAdmin = computed(() => auth.isAdmin)

const categoryOptions = computed(() => [
  { label: t('file201.allCategories'), value: 0 },
  ...categories.value.map(c => ({ label: c.name, value: c.id })),
])

const categorySelectOptions = computed(() =>
  categories.value.map(c => ({ label: c.name, value: c.id }))
)

const docTypeOptions = [
  { label: 'General', value: 'general' },
  { label: 'Contract', value: 'contract' },
  { label: 'Government ID', value: 'gov_id' },
  { label: 'Certificate', value: 'certificate' },
  { label: 'Medical', value: 'medical' },
  { label: 'Legal', value: 'legal' },
  { label: 'Other', value: 'other' },
]

const appliesToOptions = [
  { label: t('file201.allEmployees'), value: 'all' },
  { label: t('file201.regular'), value: 'regular' },
  { label: t('file201.contractual'), value: 'contractual' },
  { label: t('file201.probationary'), value: 'probationary' },
]

const statusColor: Record<string, string> = {
  active: 'success',
  expired: 'error',
  archived: 'default',
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
}

// Document table
const docColumns = computed<DataTableColumns<Document>>(() => [
  { title: t('file201.category'), key: 'category_name', render: (row) => row.category_name || '-' },
  { title: t('file201.documentTitle'), key: 'title', render: (row) => row.title || row.file_name },
  { title: t('file201.docType'), key: 'doc_type' },
  { title: t('file201.fileName'), key: 'file_name', ellipsis: { tooltip: true } },
  { title: t('file201.fileSize'), key: 'file_size', render: (row) => formatSize(row.file_size) },
  {
    title: t('common.status'), key: 'status',
    render: (row) => h(NTag, { size: 'small', type: (statusColor[row.status] || 'default') as any }, () => row.status),
  },
  { title: t('file201.expiryDate'), key: 'expiry_date', render: (row) => row.expiry_date ? format(new Date(row.expiry_date), 'yyyy-MM-dd') : '-' },
  { title: t('file201.version'), key: 'version' },
  {
    title: t('common.actions'), key: 'actions',
    render: (row) => h(NSpace, { size: 4 }, () => [
      h(NButton, { size: 'small', onClick: () => downloadDoc(row) }, () => t('file201.download')),
      ...(isAdmin.value ? [h(NButton, { size: 'small', type: 'error', onClick: () => deleteDoc(row) }, () => t('common.delete'))] : []),
    ]),
  },
])

// Expiring docs table
const expiringColumns = computed<DataTableColumns<ExpiringDoc>>(() => [
  { title: t('file201.employee'), key: 'employee', render: (row) => `${row.first_name} ${row.last_name} (${row.employee_no})` },
  { title: t('file201.category'), key: 'category_name', render: (row) => row.category_name || '-' },
  { title: t('file201.fileName'), key: 'file_name' },
  { title: t('file201.expiryDate'), key: 'expiry_date', render: (row) => format(new Date(row.expiry_date), 'yyyy-MM-dd') },
])

// Compliance table
const complianceColumns = computed<DataTableColumns<ComplianceItem>>(() => [
  { title: t('file201.category'), key: 'category_name' },
  { title: t('file201.documentName'), key: 'document_name' },
  {
    title: t('file201.required'), key: 'is_required',
    render: (row) => h(NTag, { size: 'small', type: row.is_required ? 'error' : 'default' }, () => row.is_required ? t('common.yes') : t('common.no')),
  },
  {
    title: t('common.status'), key: 'is_fulfilled',
    render: (row) => h(NTag, { size: 'small', type: row.is_fulfilled ? 'success' : 'warning' }, () => row.is_fulfilled ? t('file201.fulfilled') : t('file201.missing')),
  },
])

// Requirements table
const reqColumns = computed<DataTableColumns<Requirement>>(() => [
  { title: t('file201.category'), key: 'category_name' },
  { title: t('file201.documentName'), key: 'document_name' },
  {
    title: t('file201.required'), key: 'is_required',
    render: (row) => h(NTag, { size: 'small', type: row.is_required ? 'error' : 'default' }, () => row.is_required ? t('common.yes') : t('common.no')),
  },
  { title: t('file201.appliesTo'), key: 'applies_to' },
  { title: t('file201.expiryMonths'), key: 'expiry_months', render: (row) => row.expiry_months ? `${row.expiry_months} mo` : '-' },
  ...(isAdmin.value ? [{
    title: t('common.actions'), key: 'actions',
    render: (row: Requirement) => h(NButton, { size: 'small', type: 'error', onClick: () => deleteReq(row.id) }, () => t('common.delete')),
  }] : []),
])

async function fetchCategories() {
  try {
    const res = await file201API.listCategories()
    categories.value = extractData(res)
  } catch { /* ignore */ }
}

async function loadEmployeeDocs() {
  if (!selectedEmployeeId.value) return
  loading.value = true
  try {
    const params: Record<string, string> = {}
    if (selectedCategory.value) params.category_id = String(selectedCategory.value)
    const [docsRes, statsRes, compRes] = await Promise.all([
      file201API.listDocuments(selectedEmployeeId.value, params),
      file201API.getStats(selectedEmployeeId.value),
      file201API.compliance(selectedEmployeeId.value),
    ])
    documents.value = extractData(docsRes)
    stats.value = (statsRes as any)?.data ?? statsRes
    compliance.value = extractData(compRes)
  } catch {
    message.error(t('file201.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function loadExpiring() {
  try {
    const res = await file201API.expiring()
    expiringDocs.value = extractData(res)
  } catch { message.error(t('common.loadFailed')) }
}

async function loadRequirements() {
  try {
    const res = await file201API.listRequirements()
    requirements.value = extractData(res)
  } catch { message.error(t('common.loadFailed')) }
}

function extractData(res: unknown): any[] {
  const d = (res as any)?.data ?? res
  return Array.isArray(d) ? d : []
}

async function handleUpload() {
  if (!selectedEmployeeId.value || fileList.value.length === 0) {
    message.warning(t('common.fillRequired'))
    return
  }
  const fileInfo = fileList.value[0]
  if (!fileInfo.file) return

  const formData = new FormData()
  formData.append('file', fileInfo.file)
  formData.append('title', uploadForm.value.title)
  formData.append('doc_type', uploadForm.value.doc_type)
  if (uploadForm.value.category_id) {
    formData.append('category_id', String(uploadForm.value.category_id))
  }
  if (uploadForm.value.expiry_date) {
    formData.append('expiry_date', format(new Date(uploadForm.value.expiry_date), 'yyyy-MM-dd'))
  }
  if (uploadForm.value.notes) {
    formData.append('notes', uploadForm.value.notes)
  }

  try {
    await file201API.upload(selectedEmployeeId.value, formData)
    message.success(t('file201.uploaded'))
    showUploadModal.value = false
    fileList.value = []
    loadEmployeeDocs()
  } catch {
    message.error(t('common.error'))
  }
}

function downloadDoc(doc: Document) {
  const url = file201API.download(doc.id)
  window.open(url, '_blank')
}

async function deleteDoc(doc: Document) {
  try {
    await file201API.deleteDocument(doc.id)
    message.success(t('common.deleted'))
    loadEmployeeDocs()
  } catch {
    message.error(t('common.error'))
  }
}

async function submitReq() {
  if (!reqForm.value.category_id || !reqForm.value.document_name) {
    message.warning(t('common.fillRequired'))
    return
  }
  try {
    await file201API.createRequirement({
      ...reqForm.value,
      is_required: reqForm.value.is_required === 'yes',
    })
    message.success(t('common.created'))
    showReqModal.value = false
    loadRequirements()
  } catch {
    message.error(t('common.error'))
  }
}

async function deleteReq(id: number) {
  try {
    await file201API.deleteRequirement(id)
    message.success(t('common.deleted'))
    loadRequirements()
  } catch {
    message.error(t('common.error'))
  }
}

function openUpload() {
  uploadForm.value = { title: '', doc_type: 'general', category_id: null, expiry_date: null, notes: '' }
  fileList.value = []
  showUploadModal.value = true
}

onMounted(() => {
  fetchCategories()
  loadExpiring()
  if (isAdmin.value) loadRequirements()
})
</script>

<template>
  <div>
    <h2 style="margin-bottom: 16px;">{{ t('file201.title') }}</h2>

    <NTabs type="line">
      <!-- Employee 201 File Tab -->
      <NTabPane :name="t('file201.employee201')" :tab="t('file201.employee201')">
        <NSpace style="margin-bottom: 12px;" align="center">
          <NInputNumber
            v-model:value="selectedEmployeeId"
            :placeholder="t('file201.enterEmployeeId')"
            :min="1"
            style="width: 200px;"
          />
          <NSelect
            v-model:value="selectedCategory"
            :options="categoryOptions"
            style="width: 200px;"
            @update:value="loadEmployeeDocs"
          />
          <NButton type="primary" @click="loadEmployeeDocs" :disabled="!selectedEmployeeId">
            {{ t('file201.load') }}
          </NButton>
          <NButton v-if="selectedEmployeeId" @click="openUpload">{{ t('file201.upload') }}</NButton>
        </NSpace>

        <!-- Stats -->
        <NGrid v-if="selectedEmployeeId && stats.total_documents > 0" :cols="4" :x-gap="16" :y-gap="16" responsive="screen" style="margin-bottom: 16px;">
          <NGi><NStatistic :label="t('file201.totalDocs')" :value="stats.total_documents" /></NGi>
          <NGi><NStatistic :label="t('file201.active')" :value="stats.active_documents" /></NGi>
          <NGi><NStatistic :label="t('file201.expired')" :value="stats.expired_documents" /></NGi>
          <NGi><NStatistic :label="t('file201.expiringSoon')" :value="stats.expiring_soon" /></NGi>
        </NGrid>

        <NDataTable :columns="docColumns" :data="documents" :loading="loading" :bordered="false" />

        <!-- Compliance Checklist -->
        <div v-if="selectedEmployeeId && compliance.length > 0" style="margin-top: 24px;">
          <h3>{{ t('file201.complianceChecklist') }}</h3>
          <NDataTable :columns="complianceColumns" :data="compliance" :bordered="false" size="small" />
        </div>
      </NTabPane>

      <!-- Expiring Documents Tab -->
      <NTabPane :name="t('file201.expiringDocs')" :tab="t('file201.expiringDocs')">
        <NAlert v-if="expiringDocs.length > 0" type="warning" style="margin-bottom: 12px;">
          {{ t('file201.expiringAlert', { count: expiringDocs.length }) }}
        </NAlert>
        <NDataTable :columns="expiringColumns" :data="expiringDocs" :bordered="false" />
      </NTabPane>

      <!-- Requirements Tab (Admin) -->
      <NTabPane v-if="isAdmin" :name="t('file201.requirements')" :tab="t('file201.requirements')">
        <NSpace style="margin-bottom: 12px;">
          <NButton type="primary" @click="showReqModal = true">{{ t('file201.addRequirement') }}</NButton>
        </NSpace>
        <NDataTable :columns="reqColumns" :data="requirements" :bordered="false" />
      </NTabPane>

      <!-- Categories Tab (Admin) -->
      <NTabPane v-if="isAdmin" :name="t('file201.categories')" :tab="t('file201.categories')">
        <NDataTable
          :columns="[
            { title: t('file201.categoryName'), key: 'name' },
            { title: t('file201.slug'), key: 'slug' },
            { title: t('file201.description'), key: 'description', render: (row: any) => row.description || '-' },
            { title: t('file201.sortOrder'), key: 'sort_order' },
            { title: t('file201.system'), key: 'is_system', render: (row: any) => row.is_system ? 'Yes' : 'No' },
          ]"
          :data="categories"
          :bordered="false"
        />
      </NTabPane>
    </NTabs>

    <!-- Upload Modal -->
    <NModal v-model:show="showUploadModal" preset="card" :title="t('file201.uploadDocument')" style="max-width: 550px; width: 95vw;">
      <NForm label-placement="left" label-width="120">
        <NFormItem :label="t('file201.file')">
          <NUpload
            v-model:file-list="fileList"
            :max="1"
            :default-upload="false"
          >
            <NButton>{{ t('file201.selectFile') }}</NButton>
          </NUpload>
        </NFormItem>
        <NFormItem :label="t('file201.documentTitle')">
          <NInput v-model:value="uploadForm.title" />
        </NFormItem>
        <NFormItem :label="t('file201.category')">
          <NSelect v-model:value="uploadForm.category_id" :options="categorySelectOptions" clearable />
        </NFormItem>
        <NFormItem :label="t('file201.docType')">
          <NSelect v-model:value="uploadForm.doc_type" :options="docTypeOptions" />
        </NFormItem>
        <NFormItem :label="t('file201.expiryDate')">
          <NDatePicker v-model:value="uploadForm.expiry_date" type="date" clearable style="width: 100%;" />
        </NFormItem>
        <NFormItem :label="t('file201.notes')">
          <NInput v-model:value="uploadForm.notes" type="textarea" />
        </NFormItem>
        <NFormItem>
          <NButton type="primary" @click="handleUpload">{{ t('file201.upload') }}</NButton>
        </NFormItem>
      </NForm>
    </NModal>

    <!-- Requirement Modal -->
    <NModal v-model:show="showReqModal" preset="card" :title="t('file201.addRequirement')" style="max-width: 500px; width: 95vw;">
      <NForm label-placement="left" label-width="120">
        <NFormItem :label="t('file201.category')">
          <NSelect v-model:value="reqForm.category_id" :options="categorySelectOptions" />
        </NFormItem>
        <NFormItem :label="t('file201.documentName')">
          <NInput v-model:value="reqForm.document_name" />
        </NFormItem>
        <NFormItem :label="t('file201.required')">
          <NSelect v-model:value="reqForm.is_required" :options="[{ label: 'Yes', value: 'yes' }, { label: 'No', value: 'no' }]" />
        </NFormItem>
        <NFormItem :label="t('file201.appliesTo')">
          <NSelect v-model:value="reqForm.applies_to" :options="appliesToOptions" />
        </NFormItem>
        <NFormItem :label="t('file201.expiryMonths')">
          <NInputNumber v-model:value="reqForm.expiry_months" :min="0" clearable style="width: 100%;" />
        </NFormItem>
        <NFormItem>
          <NButton type="primary" @click="submitReq">{{ t('common.save') }}</NButton>
        </NFormItem>
      </NForm>
    </NModal>
  </div>
</template>
