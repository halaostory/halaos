<script setup lang="ts">
import { ref, onMounted, h, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { NSpace, NButton, NDataTable, NTag, NModal, NForm, NFormItem, NInput, NSelect, NCheckbox, NAlert, useMessage, type DataTableColumns } from 'naive-ui'
import { payrollAPI, employeeAPI } from '../../api/client'

interface Schedule { id: number; name: string; frequency: string; is_default: boolean; assigned_employees: number }

const { t } = useI18n()
const message = useMessage()
const unwrap = <T,>(r: unknown): T => ((r as { data?: T }).data ?? r) as T
const errMsg = (e: unknown) => (e as { data?: { error?: { message?: string } } }).data?.error?.message || t('common.saveFailed')

const schedules = ref<Schedule[]>([])
const employees = ref<{ label: string; value: number }[]>([])
const freqOptions = computed(() => ['monthly', 'semi_monthly', 'bi_weekly', 'weekly'].map((f) => ({ label: t('paySchedules.freq_' + f), value: f })))

const show = ref(false)
const form = ref({ id: 0, name: '', frequency: 'semi_monthly', is_default: false })
const assignFor = ref<Schedule | null>(null)
const assignIds = ref<number[]>([])

async function load() {
  schedules.value = unwrap<Schedule[]>(await payrollAPI.listSchedules()) || []
}
async function save() {
  try {
    const body = { name: form.value.name, frequency: form.value.frequency, is_default: form.value.is_default }
    if (form.value.id) await payrollAPI.updateSchedule(form.value.id, body)
    else await payrollAPI.createSchedule(body)
    show.value = false
    message.success(t('common.saved'))
    await load()
  } catch (e) {
    message.error(errMsg(e))
  }
}
async function assign() {
  if (!assignFor.value) return
  try {
    await payrollAPI.assignSchedule(assignFor.value.id, assignIds.value)
    assignFor.value = null
    message.success(t('common.saved'))
    await load()
  } catch (e) {
    message.error(errMsg(e))
  }
}

const cols = computed<DataTableColumns<Schedule>>(() => [
  { title: t('paySchedules.name'), key: 'name', render: (r) => h(NSpace, { size: 6, align: 'center' }, () => [r.name, r.is_default ? h(NTag, { size: 'small', type: 'info' }, () => t('paySchedules.default')) : null]) },
  { title: t('paySchedules.frequency'), key: 'frequency', width: 160, render: (r) => t('paySchedules.freq_' + r.frequency) },
  { title: t('paySchedules.employees'), key: 'assigned_employees', width: 200, render: (r) => r.is_default ? `${r.assigned_employees} + ${t('paySchedules.unassigned')}` : String(r.assigned_employees) },
  { title: '', key: 'a', width: 280, render: (r) => h(NSpace, { size: 4, wrap: false }, () => [
    h(NButton, { size: 'small', onClick: () => { form.value = { ...r }; show.value = true } }, () => t('common.edit')),
    h(NButton, { size: 'small', onClick: () => { assignFor.value = r; assignIds.value = [] } }, () => t('paySchedules.assign')),
  ]) },
])

onMounted(async () => {
  await load()
  const e = unwrap<{ id: number; first_name: string; last_name: string }[]>(await employeeAPI.list({ status: 'active', limit: '500' })) || []
  employees.value = e.map((x) => ({ label: `${x.first_name} ${x.last_name}`, value: x.id }))
})
</script>

<template>
  <NSpace vertical :size="12">
    <NAlert type="info" :show-icon="false">{{ t('paySchedules.hint') }}</NAlert>
    <NButton type="primary" style="width: fit-content" @click="form = { id: 0, name: '', frequency: 'semi_monthly', is_default: !schedules.length }; show = true">{{ t('paySchedules.add') }}</NButton>
    <NDataTable :columns="cols" :data="schedules" :row-key="(r: Schedule) => r.id" />

    <NModal v-model:show="show" preset="card" style="max-width: 460px" :title="form.id ? t('common.edit') : t('paySchedules.add')">
      <NForm label-placement="top">
        <NFormItem :label="t('paySchedules.name')"><NInput v-model:value="form.name" /></NFormItem>
        <NFormItem :label="t('paySchedules.frequency')"><NSelect v-model:value="form.frequency" :options="freqOptions" /></NFormItem>
        <NCheckbox v-model:checked="form.is_default">{{ t('paySchedules.makeDefault') }}</NCheckbox>
      </NForm>
      <NSpace justify="end" style="margin-top: 12px"><NButton @click="show = false">{{ t('common.cancel') }}</NButton><NButton type="primary" :disabled="!form.name" @click="save">{{ t('common.save') }}</NButton></NSpace>
    </NModal>
    <NModal :show="!!assignFor" preset="card" style="max-width: 520px" :title="t('paySchedules.assignTo', { name: assignFor?.name || '' })" @update:show="(v: boolean) => { if (!v) assignFor = null }">
      <NSelect v-model:value="assignIds" multiple filterable :options="employees" />
      <NSpace justify="end" style="margin-top: 12px"><NButton type="primary" :disabled="!assignIds.length" @click="assign">{{ t('common.save') }}</NButton></NSpace>
    </NModal>
  </NSpace>
</template>
