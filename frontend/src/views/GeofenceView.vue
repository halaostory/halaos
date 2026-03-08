<script setup lang="ts">
import { ref, h, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NDataTable, NButton, NModal, NForm, NFormItem,
  NInput, NInputNumber, NSwitch, NSpace, NTag,
  NAlert, useMessage, type DataTableColumns,
} from 'naive-ui'
import { geofenceAPI } from '../api/client'

const { t } = useI18n()
const message = useMessage()

interface Geofence {
  id: number
  name: string
  address: string | null
  latitude: number
  longitude: number
  radius_meters: number
  is_active: boolean
  enforce_on_clock_in: boolean
  enforce_on_clock_out: boolean
  created_at: string
}

const geofences = ref<Geofence[]>([])
const loading = ref(false)
const geofenceEnabled = ref(false)

// Modal
const showModal = ref(false)
const editingGeofence = ref<Geofence | null>(null)
const form = ref({
  name: '',
  address: '',
  latitude: 14.5995 as number | null,
  longitude: 120.9842 as number | null,
  radius_meters: 200 as number | null,
  is_active: true,
  enforce_on_clock_in: true,
  enforce_on_clock_out: false,
})

function extractData(res: unknown): any[] {
  const d = (res as any)?.data ?? res
  return Array.isArray(d) ? d : []
}

async function fetchAll() {
  loading.value = true
  try {
    const [geoRes, settingsRes] = await Promise.all([
      geofenceAPI.list(),
      geofenceAPI.getSettings(),
    ])
    geofences.value = extractData(geoRes)
    const settings = (settingsRes as any)?.data ?? settingsRes
    geofenceEnabled.value = settings?.geofence_enabled ?? false
  } catch {
    message.error(t('geofence.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function toggleGeofence(enabled: boolean) {
  try {
    await geofenceAPI.setSettings({ geofence_enabled: enabled })
    geofenceEnabled.value = enabled
    message.success(enabled ? t('geofence.enabled') : t('geofence.disabled'))
  } catch {
    message.error(t('common.error'))
  }
}

function openCreate() {
  editingGeofence.value = null
  form.value = {
    name: '',
    address: '',
    latitude: 14.5995,
    longitude: 120.9842,
    radius_meters: 200,
    is_active: true,
    enforce_on_clock_in: true,
    enforce_on_clock_out: false,
  }
  showModal.value = true
}

function openEdit(gf: Geofence) {
  editingGeofence.value = gf
  form.value = {
    name: gf.name,
    address: gf.address || '',
    latitude: gf.latitude,
    longitude: gf.longitude,
    radius_meters: gf.radius_meters,
    is_active: gf.is_active,
    enforce_on_clock_in: gf.enforce_on_clock_in,
    enforce_on_clock_out: gf.enforce_on_clock_out,
  }
  showModal.value = true
}

async function save() {
  if (!form.value.name || !form.value.latitude || !form.value.longitude) {
    message.warning(t('common.fillRequired'))
    return
  }
  const data = {
    name: form.value.name,
    address: form.value.address || null,
    latitude: form.value.latitude,
    longitude: form.value.longitude,
    radius_meters: form.value.radius_meters || 200,
    is_active: form.value.is_active,
    enforce_on_clock_in: form.value.enforce_on_clock_in,
    enforce_on_clock_out: form.value.enforce_on_clock_out,
  }
  try {
    if (editingGeofence.value) {
      await geofenceAPI.update(editingGeofence.value.id, data)
      message.success(t('common.updated'))
    } else {
      await geofenceAPI.create(data)
      message.success(t('common.created'))
    }
    showModal.value = false
    fetchAll()
  } catch {
    message.error(t('common.error'))
  }
}

async function deleteGeofence(id: number) {
  try {
    await geofenceAPI.delete(id)
    message.success(t('common.deleted'))
    fetchAll()
  } catch {
    message.error(t('common.error'))
  }
}

const columns = computed<DataTableColumns<Geofence>>(() => [
  { title: t('geofence.locationName'), key: 'name' },
  { title: t('geofence.address'), key: 'address', render: (row) => row.address || '-' },
  { title: t('geofence.latitude'), key: 'latitude', width: 110 },
  { title: t('geofence.longitude'), key: 'longitude', width: 110 },
  { title: t('geofence.radius'), key: 'radius_meters', width: 100, render: (row) => `${row.radius_meters}m` },
  {
    title: t('geofence.enforce'), key: 'enforce', width: 140,
    render: (row) => {
      const tags: ReturnType<typeof h>[] = []
      if (row.enforce_on_clock_in) tags.push(h(NTag, { size: 'small', type: 'info' }, () => t('geofence.clockIn')))
      if (row.enforce_on_clock_out) tags.push(h(NTag, { size: 'small', type: 'warning' }, () => t('geofence.clockOut')))
      return h(NSpace, { size: 4 }, () => tags)
    },
  },
  {
    title: t('common.status'), key: 'is_active', width: 80,
    render: (row) => h(NTag, { size: 'small', type: row.is_active ? 'success' : 'default' }, () => row.is_active ? t('common.active') : t('common.inactive')),
  },
  {
    title: t('common.actions'), key: 'actions', width: 150,
    render: (row) => h(NSpace, { size: 4 }, () => [
      h(NButton, { size: 'small', onClick: () => openEdit(row) }, () => t('common.edit')),
      h(NButton, { size: 'small', type: 'error', onClick: () => deleteGeofence(row.id) }, () => t('common.delete')),
    ]),
  },
])

onMounted(fetchAll)
</script>

<template>
  <div>
    <h2 style="margin-bottom: 16px;">{{ t('geofence.title') }}</h2>

    <NAlert type="info" style="margin-bottom: 16px;">
      {{ t('geofence.description') }}
    </NAlert>

    <NSpace style="margin-bottom: 16px;" align="center">
      <span style="font-weight: 600;">{{ t('geofence.enableGeofencing') }}:</span>
      <NSwitch :value="geofenceEnabled" @update:value="toggleGeofence" />
      <NTag :type="geofenceEnabled ? 'success' : 'default'">
        {{ geofenceEnabled ? t('geofence.enabled') : t('geofence.disabled') }}
      </NTag>
    </NSpace>

    <NSpace style="margin-bottom: 12px;">
      <NButton type="primary" @click="openCreate">{{ t('geofence.addLocation') }}</NButton>
    </NSpace>

    <NDataTable :columns="columns" :data="geofences" :loading="loading" :bordered="false" />

    <!-- Create/Edit Modal -->
    <NModal v-model:show="showModal" preset="card" :title="editingGeofence ? t('geofence.editLocation') : t('geofence.addLocation')" style="max-width: 600px; width: 95vw;">
      <NForm label-placement="left" label-width="160">
        <NFormItem :label="t('geofence.locationName')">
          <NInput v-model:value="form.name" />
        </NFormItem>
        <NFormItem :label="t('geofence.address')">
          <NInput v-model:value="form.address" />
        </NFormItem>
        <NFormItem :label="t('geofence.latitude')">
          <NInputNumber v-model:value="form.latitude" :precision="7" style="width: 100%;" />
        </NFormItem>
        <NFormItem :label="t('geofence.longitude')">
          <NInputNumber v-model:value="form.longitude" :precision="7" style="width: 100%;" />
        </NFormItem>
        <NFormItem :label="t('geofence.radius')">
          <NInputNumber v-model:value="form.radius_meters" :min="50" :max="5000" style="width: 100%;">
            <template #suffix>meters</template>
          </NInputNumber>
        </NFormItem>
        <NFormItem :label="t('geofence.enforceClockIn')">
          <NSwitch v-model:value="form.enforce_on_clock_in" />
        </NFormItem>
        <NFormItem :label="t('geofence.enforceClockOut')">
          <NSwitch v-model:value="form.enforce_on_clock_out" />
        </NFormItem>
        <NFormItem v-if="editingGeofence" :label="t('common.active')">
          <NSwitch v-model:value="form.is_active" />
        </NFormItem>
        <NFormItem>
          <NButton type="primary" @click="save">{{ t('common.save') }}</NButton>
        </NFormItem>
      </NForm>
    </NModal>
  </div>
</template>
