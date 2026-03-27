<script setup lang="ts">
import { ref, computed, h } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NCard, NButton, NSpace, NUpload, NDatePicker, NInputNumber,
  NAlert, NResult, NDataTable, NTag, useMessage, NGrid, NGridItem,
  type UploadFileInfo, type DataTableColumns,
} from 'naive-ui'
import { importAPI, exportAPI } from '../api/client'

const { t } = useI18n()
const message = useMessage()

// Import state
const fileList = ref<UploadFileInfo[]>([])
const importing = ref(false)
const importResult = ref<{ imported: number; skipped: number; errors: string[] } | null>(null)

// Preview state
interface PreviewRow {
  line: number
  employee_no: string
  first_name: string
  last_name: string
  email: string
  hire_date: string
  employment_type: string
  department: string
  position: string
  valid: boolean
  errors: string[]
}
interface PreviewSummary {
  total: number
  valid: number
  invalid: number
}
const previewing = ref(false)
const previewRows = ref<PreviewRow[]>([])
const previewSummary = ref<PreviewSummary | null>(null)

const previewColumns = computed<DataTableColumns<PreviewRow>>(() => [
  { title: '#', key: 'line', width: 50 },
  { title: t('employee.employeeNo'), key: 'employee_no', width: 120 },
  { title: t('auth.firstName'), key: 'first_name', width: 120 },
  { title: t('auth.lastName'), key: 'last_name', width: 120 },
  { title: t('common.email'), key: 'email', width: 180 },
  { title: t('employee.hireDate'), key: 'hire_date', width: 120 },
  { title: t('employee.employmentType'), key: 'employment_type', width: 120 },
  { title: t('employee.department'), key: 'department', width: 150 },
  { title: t('employee.position'), key: 'position', width: 150 },
  {
    title: t('common.status'),
    key: 'valid',
    width: 100,
    render: (row) => h(NTag, { type: row.valid ? 'success' : 'error', size: 'small' }, {
      default: () => row.valid ? t('importExport.previewValid') : t('importExport.previewInvalid'),
    }),
  },
  {
    title: t('importExport.previewErrors'),
    key: 'errors',
    render: (row) => row.errors.length ? row.errors.join('; ') : '',
  },
])

// Export state
const attendanceDates = ref<[number, number] | null>(null)
const leaveYear = ref(new Date().getFullYear())

async function handlePreview() {
  if (fileList.value.length === 0 || !fileList.value[0].file) {
    message.warning(t('importExport.selectFile'))
    return
  }

  previewing.value = true
  previewRows.value = []
  previewSummary.value = null
  importResult.value = null

  const formData = new FormData()
  formData.append('file', fileList.value[0].file)

  try {
    const res = await importAPI.previewEmployeesCSV(formData)
    const data = (res as any)?.data ?? res
    previewRows.value = data?.rows ?? []
    previewSummary.value = data?.summary ?? null
  } catch {
    message.error(t('importExport.previewFailed'))
  }
  previewing.value = false
}

function clearPreview() {
  previewRows.value = []
  previewSummary.value = null
  importResult.value = null
}

async function handleImportEmployees() {
  if (fileList.value.length === 0 || !fileList.value[0].file) {
    message.warning(t('importExport.selectFile'))
    return
  }

  importing.value = true
  importResult.value = null

  const formData = new FormData()
  formData.append('file', fileList.value[0].file)

  try {
    const res = await importAPI.employeesCSV(formData)
    const data = (res as any)?.data ?? res
    importResult.value = {
      imported: data?.imported ?? 0,
      skipped: data?.skipped ?? 0,
      errors: data?.errors ?? [],
    }
    previewRows.value = []
    previewSummary.value = null
    if (importResult.value.skipped === 0) {
      message.success(t('importExport.importSuccess', { count: importResult.value.imported }))
    } else {
      message.warning(t('importExport.importPartial', {
        imported: importResult.value.imported,
        skipped: importResult.value.skipped,
      }))
    }
  } catch {
    message.error(t('importExport.importFailed'))
  }
  importing.value = false
}

function downloadWithAuth(url: string) {
  const token = localStorage.getItem('access_token')
  fetch(url, {
    headers: { Authorization: `Bearer ${token}` },
  })
    .then((res) => {
      if (!res.ok) throw new Error('Export failed')
      const disposition = res.headers.get('Content-Disposition')
      const match = disposition?.match(/filename=(.+)/)
      const filename = match ? match[1] : 'export.csv'
      return res.blob().then((blob) => ({ blob, filename }))
    })
    .then(({ blob, filename }) => {
      const a = document.createElement('a')
      a.href = URL.createObjectURL(blob)
      a.download = filename
      a.click()
      URL.revokeObjectURL(a.href)
    })
    .catch(() => {
      message.error(t('common.failed'))
    })
}

function exportEmployees() {
  downloadWithAuth(exportAPI.employeesCSV())
}

function exportAttendance() {
  if (!attendanceDates.value) {
    message.warning(t('common.fillAllFields'))
    return
  }
  const [startTs, endTs] = attendanceDates.value
  const start = new Date(startTs).toISOString().split('T')[0]
  const end = new Date(endTs).toISOString().split('T')[0]
  downloadWithAuth(exportAPI.attendanceCSV(start, end))
}

function exportLeaveBalances() {
  downloadWithAuth(exportAPI.leaveBalancesCSV(leaveYear.value))
}

function downloadTemplate() {
  const header = 'employee_no,first_name,last_name,middle_name,email,phone,gender,birth_date,hire_date,employment_type,department,position'
  const sample = 'EMP-001,Juan,Dela Cruz,,juan@example.com,09171234567,male,1990-01-15,2024-01-01,regular,Human Resources Department,HR Assistant'
  const blob = new Blob([header + '\n' + sample + '\n'], { type: 'text/csv' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = 'employee_import_template.csv'
  a.click()
  URL.revokeObjectURL(a.href)
}

const currentYear = computed(() => new Date().getFullYear())
</script>

<template>
  <NSpace vertical :size="24">
    <!-- Import Section -->
    <NCard :title="t('importExport.importEmployees')">
      <NSpace vertical :size="12">
        <NAlert type="info" :title="t('importExport.csvFormat')" />

        <NSpace align="center" :size="12">
          <NUpload
            v-model:file-list="fileList"
            accept=".csv"
            :max="1"
            :default-upload="false"
            @update:file-list="clearPreview"
            style="width: 300px;"
          >
            <NButton>{{ t('importExport.selectFile') }}</NButton>
          </NUpload>
          <NButton type="info" :loading="previewing" @click="handlePreview" :disabled="fileList.length === 0">
            {{ t('importExport.previewBtn') }}
          </NButton>
          <NButton @click="downloadTemplate">{{ t('importExport.downloadTemplate') }}</NButton>
        </NSpace>

        <!-- Preview Section -->
        <template v-if="previewSummary">
          <NAlert :type="previewSummary.invalid === 0 ? 'success' : 'warning'" :title="t('importExport.previewSummary')">
            {{ t('importExport.previewTotal') }}: {{ previewSummary.total }},
            {{ t('importExport.previewValid') }}: {{ previewSummary.valid }},
            {{ t('importExport.previewInvalid') }}: {{ previewSummary.invalid }}
          </NAlert>

          <NDataTable
            :columns="previewColumns"
            :data="previewRows"
            :max-height="400"
            :scroll-x="1000"
            size="small"
            :row-class-name="(row: PreviewRow) => row.valid ? '' : 'preview-row-invalid'"
          />

          <NSpace>
            <NButton
              type="primary"
              :loading="importing"
              :disabled="!previewSummary.valid"
              @click="handleImportEmployees"
            >
              {{ t('importExport.confirmImport', { count: previewSummary.valid }) }}
            </NButton>
            <NButton @click="clearPreview">{{ t('common.cancel') }}</NButton>
          </NSpace>
        </template>

        <NResult
          v-if="importResult"
          :status="importResult.skipped === 0 ? 'success' : 'warning'"
          :title="`${t('importExport.imported')}: ${importResult.imported}, ${t('importExport.skipped')}: ${importResult.skipped}`"
        >
          <template #footer v-if="importResult.errors.length">
            <div style="text-align: left; max-height: 200px; overflow-y: auto; font-size: 13px;">
              <div v-for="(err, i) in importResult.errors" :key="i" style="color: #d03050;">{{ err }}</div>
            </div>
          </template>
        </NResult>
      </NSpace>
    </NCard>

    <!-- Export Section -->
    <NCard :title="t('common.actions')">
      <NGrid :cols="2" :x-gap="24" :y-gap="24" responsive="screen">
        <NGridItem>
          <NCard size="small" :title="t('importExport.exportEmployees')">
            <NButton type="primary" @click="exportEmployees">{{ t('importExport.exportEmployees') }} (CSV)</NButton>
          </NCard>
        </NGridItem>

        <NGridItem>
          <NCard size="small" :title="t('importExport.exportAttendance')">
            <NSpace vertical :size="8">
              <NDatePicker v-model:value="attendanceDates" type="daterange" style="width: 100%;" />
              <NButton type="primary" @click="exportAttendance" :disabled="!attendanceDates">
                {{ t('importExport.exportAttendance') }} (CSV)
              </NButton>
            </NSpace>
          </NCard>
        </NGridItem>

        <NGridItem>
          <NCard size="small" :title="t('importExport.exportLeaveBalances')">
            <NSpace vertical :size="8">
              <NInputNumber v-model:value="leaveYear" :min="2020" :max="currentYear + 1" style="width: 100%;" />
              <NButton type="primary" @click="exportLeaveBalances">
                {{ t('importExport.exportLeaveBalances') }} (CSV)
              </NButton>
            </NSpace>
          </NCard>
        </NGridItem>
      </NGrid>
    </NCard>
  </NSpace>
</template>

<style scoped>
:deep(.preview-row-invalid td) {
  background-color: rgba(208, 48, 80, 0.06) !important;
}
</style>
