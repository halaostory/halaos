<script setup lang="ts">
import { ref, h, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import {
  NCard, NButton, NSpace, NTag, NInput, NDescriptions, NDescriptionsItem,
  NModal, NForm, NFormItem, NDatePicker, NDataTable, NEmpty, NAlert, useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { attendanceAPI, geofenceAPI } from '../api/client'
import { format } from 'date-fns'
import { useAuthStore } from '../stores/auth'

const { t } = useI18n()
const router = useRouter()
const message = useMessage()
const authStore = useAuthStore()
const clockedIn = ref(false)
const loading = ref(false)
const summary = ref<Record<string, unknown> | null>(null)
const geofenceEnabled = ref(false)
const locationStatus = ref<'idle' | 'acquiring' | 'acquired' | 'denied' | 'error'>('idle')
const currentLat = ref<string | undefined>(undefined)
const currentLng = ref<string | undefined>(undefined)

function getLocation(): Promise<{ lat: string; lng: string } | null> {
  if (!navigator.geolocation) return Promise.resolve(null)
  locationStatus.value = 'acquiring'
  return new Promise((resolve) => {
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        const lat = pos.coords.latitude.toFixed(7)
        const lng = pos.coords.longitude.toFixed(7)
        currentLat.value = lat
        currentLng.value = lng
        locationStatus.value = 'acquired'
        resolve({ lat, lng })
      },
      () => {
        locationStatus.value = 'denied'
        resolve(null)
      },
      { enableHighAccuracy: true, timeout: 10000, maximumAge: 60000 }
    )
  })
}

async function checkGeofenceEnabled() {
  try {
    const res = await geofenceAPI.getSettings()
    const data = (res as any)?.data ?? res
    geofenceEnabled.value = data?.geofence_enabled ?? false
  } catch { /* not admin or no settings */ }
}

function fmtTime(d: unknown): string {
  if (!d) return '-'
  try { return format(new Date(d as string), 'HH:mm') } catch { return String(d) }
}

function fmtDateTime(d: unknown): string {
  if (!d) return '-'
  try { return format(new Date(d as string), 'yyyy-MM-dd HH:mm') } catch { return String(d) }
}

onMounted(async () => {
  try {
    const res = await attendanceAPI.getSummary() as { data?: Record<string, unknown> }
    const data = res.data || res as unknown as Record<string, unknown>
    summary.value = data
    if (data.clock_in_at && !data.clock_out_at) {
      clockedIn.value = true
    }
  } catch {
    // no attendance today
  }
  checkGeofenceEnabled()
  loadMyCorrections()
  if (authStore.user?.role === 'admin' || authStore.user?.role === 'manager') {
    loadPendingCorrections()
  }
})

const note = ref('')

async function clockIn() {
  loading.value = true
  try {
    const loc = await getLocation()
    await attendanceAPI.clockIn({
      source: 'web',
      note: note.value || undefined,
      lat: loc?.lat,
      lng: loc?.lng,
    })
    clockedIn.value = true
    note.value = ''
    message.success(t('attendance.clockedInSuccess'))
    const res = await attendanceAPI.getSummary() as { data?: Record<string, unknown> }
    summary.value = res.data || res as unknown as Record<string, unknown>
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } }
    message.error(err.data?.error?.message || t('common.saveFailed'))
  } finally {
    loading.value = false
  }
}

async function clockOut() {
  loading.value = true
  try {
    const loc = await getLocation()
    await attendanceAPI.clockOut({
      source: 'web',
      note: note.value || undefined,
      lat: loc?.lat,
      lng: loc?.lng,
    })
    clockedIn.value = false
    note.value = ''
    message.success(t('attendance.clockedOutSuccess'))
    const res = await attendanceAPI.getSummary() as { data?: Record<string, unknown> }
    summary.value = res.data || res as unknown as Record<string, unknown>
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } }
    message.error(err.data?.error?.message || t('common.saveFailed'))
  } finally {
    loading.value = false
  }
}

// Attendance Corrections
interface Correction {
  id: number
  correction_date: string
  requested_clock_in: string | null
  requested_clock_out: string | null
  reason: string
  status: string
  review_note: string | null
  created_at: string
  first_name?: string
  last_name?: string
  employee_no?: string
}

const showCorrectionModal = ref(false)
const correctionForm = ref({
  correction_date: null as number | null,
  requested_clock_in: null as number | null,
  requested_clock_out: null as number | null,
  reason: '',
})

const myCorrections = ref<Correction[]>([])
const pendingCorrections = ref<Correction[]>([])

async function loadMyCorrections() {
  try {
    const res = await attendanceAPI.listMyCorrections()
    const data = (res as any)?.data ?? res
    myCorrections.value = Array.isArray(data) ? data : []
  } catch { /* ignore */ }
}

async function loadPendingCorrections() {
  try {
    const res = await attendanceAPI.listPendingCorrections()
    const data = (res as any)?.data ?? res
    pendingCorrections.value = Array.isArray(data) ? data : []
  } catch { /* ignore */ }
}

function openCorrectionModal() {
  correctionForm.value = {
    correction_date: null,
    requested_clock_in: null,
    requested_clock_out: null,
    reason: '',
  }
  showCorrectionModal.value = true
}

async function handleSubmitCorrection() {
  if (!correctionForm.value.correction_date || !correctionForm.value.reason.trim()) {
    message.warning(t('common.fillAllFields'))
    return
  }

  const payload: Record<string, unknown> = {
    correction_date: format(new Date(correctionForm.value.correction_date), 'yyyy-MM-dd'),
    reason: correctionForm.value.reason,
  }
  if (correctionForm.value.requested_clock_in) {
    payload.requested_clock_in = new Date(correctionForm.value.requested_clock_in).toISOString()
  }
  if (correctionForm.value.requested_clock_out) {
    payload.requested_clock_out = new Date(correctionForm.value.requested_clock_out).toISOString()
  }

  try {
    await attendanceAPI.createCorrection(payload as any)
    message.success(t('attendance.correctionSubmitted'))
    showCorrectionModal.value = false
    loadMyCorrections()
  } catch {
    message.error(t('common.failed'))
  }
}

async function handleApproveCorrection(id: number) {
  try {
    await attendanceAPI.approveCorrection(id)
    message.success(t('common.approved'))
    loadPendingCorrections()
  } catch {
    message.error(t('common.failed'))
  }
}

async function handleRejectCorrection(id: number) {
  try {
    await attendanceAPI.rejectCorrection(id)
    message.success(t('common.rejected'))
    loadPendingCorrections()
  } catch {
    message.error(t('common.failed'))
  }
}

const statusTagType: Record<string, 'default' | 'success' | 'error' | 'warning'> = {
  pending: 'warning',
  approved: 'success',
  rejected: 'error',
}

const myCorrectionColumns: DataTableColumns<Correction> = [
  {
    title: () => t('attendance.date'),
    key: 'correction_date',
    width: 110,
    render: (row) => row.correction_date?.split('T')[0] ?? '',
  },
  {
    title: () => t('attendance.correctionClockIn'),
    key: 'requested_clock_in',
    width: 140,
    render: (row) => fmtDateTime(row.requested_clock_in),
  },
  {
    title: () => t('attendance.correctionClockOut'),
    key: 'requested_clock_out',
    width: 140,
    render: (row) => fmtDateTime(row.requested_clock_out),
  },
  { title: () => t('attendance.correctionReason'), key: 'reason', ellipsis: { tooltip: true } },
  {
    title: () => t('common.status'),
    key: 'status',
    width: 100,
    render: (row) => h(NTag, { size: 'small', type: statusTagType[row.status] || 'default' }, { default: () => row.status }),
  },
]

const pendingCorrectionColumns: DataTableColumns<Correction> = [
  {
    title: () => t('employee.name'),
    key: 'employee',
    width: 150,
    render: (row) => `${row.last_name}, ${row.first_name} (${row.employee_no})`,
  },
  {
    title: () => t('attendance.date'),
    key: 'correction_date',
    width: 110,
    render: (row) => row.correction_date?.split('T')[0] ?? '',
  },
  {
    title: () => t('attendance.correctionClockIn'),
    key: 'requested_clock_in',
    width: 140,
    render: (row) => fmtDateTime(row.requested_clock_in),
  },
  {
    title: () => t('attendance.correctionClockOut'),
    key: 'requested_clock_out',
    width: 140,
    render: (row) => fmtDateTime(row.requested_clock_out),
  },
  { title: () => t('attendance.correctionReason'), key: 'reason', ellipsis: { tooltip: true } },
  {
    title: () => t('common.actions'),
    key: 'actions',
    width: 160,
    render: (row) => h(NSpace, { size: 'small' }, {
      default: () => [
        h(NButton, { size: 'small', type: 'success', onClick: () => handleApproveCorrection(row.id) },
          { default: () => t('common.approve') }),
        h(NButton, { size: 'small', type: 'error', onClick: () => handleRejectCorrection(row.id) },
          { default: () => t('common.reject') }),
      ],
    }),
  },
]
</script>

<template>
  <div>
    <NSpace justify="space-between" style="margin-bottom: 16px;">
      <h2>{{ t('attendance.title') }}</h2>
      <NSpace>
        <NButton @click="openCorrectionModal">{{ t('attendance.requestCorrection') }}</NButton>
        <NButton @click="router.push({ name: 'attendance-records' })">{{ t('attendance.records') }}</NButton>
      </NSpace>
    </NSpace>

    <NCard :title="t('attendance.todaySummary')" style="margin-bottom: 16px;" v-if="summary">
      <NDescriptions label-placement="left" :column="3" bordered>
        <NDescriptionsItem :label="t('attendance.timeIn')">{{ fmtTime(summary.clock_in_at) }}</NDescriptionsItem>
        <NDescriptionsItem :label="t('attendance.timeOut')">{{ fmtTime(summary.clock_out_at) }}</NDescriptionsItem>
        <NDescriptionsItem :label="t('attendance.workHours')">{{ summary.work_hours ? Number(summary.work_hours).toFixed(1) : '-' }}</NDescriptionsItem>
      </NDescriptions>
    </NCard>

    <NAlert v-if="geofenceEnabled" type="info" style="margin-bottom: 16px;">
      {{ t('attendance.geofenceActive') }}
      <template v-if="locationStatus === 'denied'"> — {{ t('attendance.locationDenied') }}</template>
      <template v-if="locationStatus === 'acquired' && currentLat"> — {{ currentLat }}, {{ currentLng }}</template>
    </NAlert>

    <NCard style="margin-bottom: 16px;">
      <NSpace vertical :size="16">
        <NInput v-model:value="note" :placeholder="t('attendance.note')" />
        <NSpace align="center">
          <NButton v-if="!clockedIn" type="primary" size="large" :loading="loading" @click="clockIn">
            {{ t('attendance.clockIn') }}
          </NButton>
          <template v-else>
            <NTag type="success" size="large">{{ t('dashboard.clockedIn') }}</NTag>
            <NButton type="warning" size="large" :loading="loading" @click="clockOut">
              {{ t('attendance.clockOut') }}
            </NButton>
          </template>
          <NTag v-if="locationStatus === 'acquiring'" size="small">{{ t('attendance.acquiringLocation') }}</NTag>
        </NSpace>
      </NSpace>
    </NCard>

    <!-- Pending Corrections (Manager/Admin) -->
    <NCard v-if="pendingCorrections.length > 0" :title="t('attendance.pendingCorrections')" style="margin-bottom: 16px;">
      <NDataTable
        :columns="pendingCorrectionColumns"
        :data="pendingCorrections"
        :row-key="(row: any) => row.id"
        size="small"
      />
    </NCard>

    <!-- My Corrections -->
    <NCard :title="t('attendance.myCorrections')">
      <NDataTable
        v-if="myCorrections.length"
        :columns="myCorrectionColumns"
        :data="myCorrections"
        :row-key="(row: any) => row.id"
        size="small"
      />
      <NEmpty v-else :description="t('attendance.noCorrections')" />
    </NCard>

    <!-- Correction Request Modal -->
    <NModal v-model:show="showCorrectionModal" preset="card"
      :title="t('attendance.requestCorrection')" style="width: 500px;">
      <NForm label-placement="top">
        <NFormItem :label="t('attendance.date')">
          <NDatePicker v-model:value="correctionForm.correction_date" type="date" style="width: 100%;" />
        </NFormItem>
        <NFormItem :label="t('attendance.correctionClockIn')">
          <NDatePicker v-model:value="correctionForm.requested_clock_in" type="datetime" style="width: 100%;" clearable />
        </NFormItem>
        <NFormItem :label="t('attendance.correctionClockOut')">
          <NDatePicker v-model:value="correctionForm.requested_clock_out" type="datetime" style="width: 100%;" clearable />
        </NFormItem>
        <NFormItem :label="t('attendance.correctionReason')">
          <NInput v-model:value="correctionForm.reason" type="textarea" :rows="3" />
        </NFormItem>
      </NForm>
      <template #footer>
        <NSpace justify="end">
          <NButton @click="showCorrectionModal = false">{{ t('common.cancel') }}</NButton>
          <NButton type="primary" @click="handleSubmitCorrection">{{ t('common.submit') }}</NButton>
        </NSpace>
      </template>
    </NModal>
  </div>
</template>
