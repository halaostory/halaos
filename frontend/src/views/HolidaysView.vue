<script setup lang="ts">
import { ref, h, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { NDataTable, NButton, NSpace, NTag, NSelect, NModal, NForm, NFormItem, NInput, NDatePicker, useMessage, type DataTableColumns } from 'naive-ui'
import { holidayAPI } from '../api/client'
import { format } from 'date-fns'

const { t } = useI18n()
const message = useMessage()
const data = ref<Record<string, unknown>[]>([])
const loading = ref(false)
const year = ref(new Date().getFullYear())
const showModal = ref(false)

const form = ref({
  name: '',
  holiday_date: null as number | null,
  holiday_type: 'regular',
})

const yearOptions = Array.from({ length: 5 }, (_, i) => {
  const y = new Date().getFullYear() - 1 + i
  return { label: String(y), value: y }
})

const typeOptions = [
  { label: t('holiday.regular'), value: 'regular' },
  { label: t('holiday.specialNonWorking'), value: 'special_non_working' },
  { label: t('holiday.specialWorking'), value: 'special_working' },
]

const typeColorMap: Record<string, 'error' | 'warning' | 'info'> = {
  regular: 'error',
  special_non_working: 'warning',
  special_working: 'info',
}

const columns: DataTableColumns = [
  {
    title: t('holiday.date'),
    key: 'holiday_date',
    width: 130,
    render: (row) => {
      try { return format(new Date(row.holiday_date as string), 'yyyy-MM-dd') } catch { return '' }
    }
  },
  { title: t('holiday.name'), key: 'name' },
  {
    title: t('holiday.type'),
    key: 'holiday_type',
    width: 180,
    render: (row) => {
      const type = row.holiday_type as string
      return h(NTag, { type: typeColorMap[type] || 'info', size: 'small' }, () => type.replace(/_/g, ' '))
    }
  },
  {
    title: t('common.actions'),
    key: 'actions',
    width: 80,
    render: (row) => h(NButton, {
      size: 'small',
      type: 'error',
      quaternary: true,
      onClick: () => handleDelete(row.id as number),
    }, () => t('common.delete'))
  },
]

onMounted(() => loadData())

async function loadData() {
  loading.value = true
  try {
    const res = await holidayAPI.list({ year: String(year.value) }) as { data: Record<string, unknown>[] }
    data.value = res.data || []
  } catch {
    data.value = []
  } finally {
    loading.value = false
  }
}

function handleYearChange(y: number) {
  year.value = y
  loadData()
}

async function handleCreate() {
  if (!form.value.name || !form.value.holiday_date) {
    message.warning(t('profile.fillAllFields'))
    return
  }
  try {
    const dateStr = format(new Date(form.value.holiday_date), 'yyyy-MM-dd')
    await holidayAPI.create({
      name: form.value.name,
      holiday_date: dateStr,
      holiday_type: form.value.holiday_type,
    })
    message.success(t('holiday.created'))
    showModal.value = false
    form.value = { name: '', holiday_date: null, holiday_type: 'regular' }
    loadData()
  } catch {
    message.error(t('common.saveFailed'))
  }
}

async function handleDelete(id: number) {
  try {
    await holidayAPI.delete(id)
    message.success(t('holiday.deleted'))
    loadData()
  } catch {
    message.error(t('common.failed'))
  }
}
</script>

<template>
  <div>
    <NSpace justify="space-between" style="margin-bottom: 16px;">
      <h2>{{ t('holiday.title') }}</h2>
      <NSpace>
        <NSelect :value="year" :options="yearOptions" @update:value="handleYearChange" style="width: 100px;" />
        <NButton type="primary" @click="showModal = true">{{ t('holiday.addHoliday') }}</NButton>
      </NSpace>
    </NSpace>

    <NDataTable :columns="columns" :data="data" :loading="loading" />

    <NModal v-model:show="showModal" :title="t('holiday.addHoliday')" preset="card" style="max-width: 480px; width: 95vw;">
      <NForm label-placement="left" label-width="120">
        <NFormItem :label="t('holiday.name')" required>
          <NInput v-model:value="form.name" />
        </NFormItem>
        <NFormItem :label="t('holiday.date')" required>
          <NDatePicker v-model:value="form.holiday_date" type="date" style="width: 100%;" />
        </NFormItem>
        <NFormItem :label="t('holiday.type')">
          <NSelect v-model:value="form.holiday_type" :options="typeOptions" />
        </NFormItem>
        <NSpace justify="end">
          <NButton @click="showModal = false">{{ t('common.cancel') }}</NButton>
          <NButton type="primary" @click="handleCreate">{{ t('common.save') }}</NButton>
        </NSpace>
      </NForm>
    </NModal>
  </div>
</template>
