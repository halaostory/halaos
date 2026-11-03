<script setup lang="ts">
import { ref, onMounted, h, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { NButton, NDataTable, NModal, NForm, NFormItem, NInput, NSwitch, NTag, NSpace, useMessage, type DataTableColumns } from 'naive-ui'
import { workplacesAPI } from '../api/client'

interface Workplace { id: number; name: string; address: string; city: string; region: string; country: string; is_remote: boolean; is_active: boolean; headcount: number }
const { t } = useI18n()
const message = useMessage()
const rows = ref<Workplace[]>([])
const loading = ref(false)
const show = ref(false)
const editing = ref<number | null>(null)
const form = ref({ name: '', address: '', city: '', region: '', country: 'PHL', is_remote: false, is_active: true })

const columns = computed<DataTableColumns<Workplace>>(() => [
  { title: t('workplace.name'), key: 'name', render: (r) => h('div', [h('div', { style: 'font-weight:500' }, r.name), h('div', { style: 'font-size:12px;opacity:.6' }, [r.address, r.city, r.region].filter(Boolean).join(', ') || '—')]) },
  { title: t('workplace.type'), key: 'is_remote', width: 120, render: (r) => h(NTag, { size: 'small', round: true, bordered: false, type: r.is_remote ? 'info' : 'default' }, () => (r.is_remote ? t('workplace.remote') : t('workplace.office'))) },
  { title: t('workplace.people'), key: 'headcount', width: 100 },
  { title: '', key: 'state', width: 100, render: (r) => (r.is_active ? '' : h(NTag, { size: 'small', round: true, bordered: false }, () => t('workplace.inactive'))) },
  { title: '', key: 'edit', width: 80, render: (r) => h(NButton, { size: 'small', quaternary: true, onClick: () => edit(r) }, () => t('common.edit')) },
])

async function load() {
  loading.value = true
  try { rows.value = ((await workplacesAPI.list()) as { data: Workplace[] }).data } finally { loading.value = false }
}
function add() {
  editing.value = null
  form.value = { name: '', address: '', city: '', region: '', country: 'PHL', is_remote: false, is_active: true }
  show.value = true
}
function edit(r: Workplace) {
  editing.value = r.id
  form.value = { name: r.name, address: r.address, city: r.city, region: r.region, country: r.country, is_remote: r.is_remote, is_active: r.is_active }
  show.value = true
}
async function save() {
  try {
    if (editing.value) await workplacesAPI.update(editing.value, form.value)
    else await workplacesAPI.create(form.value)
    show.value = false
    await load()
  } catch (e) {
    message.error((e as { data?: { error?: { message?: string } } }).data?.error?.message || t('common.saveFailed'))
  }
}
onMounted(load)
</script>

<template>
  <div>
    <div class="head">
      <div>
        <h1>{{ t('workplace.title') }}</h1>
        <p class="muted">{{ t('workplace.desc') }}</p>
      </div>
      <NButton type="primary" @click="add">{{ t('workplace.add') }}</NButton>
    </div>
    <NDataTable :columns="columns" :data="rows" :loading="loading" />
    <NModal v-model:show="show" preset="card" :title="editing ? t('common.edit') : t('workplace.add')" style="max-width: 520px">
      <NForm label-placement="top">
        <NFormItem :label="t('workplace.name')"><NInput v-model:value="form.name" :placeholder="t('workplace.namePh')" /></NFormItem>
        <NFormItem :label="t('workplace.address')"><NInput v-model:value="form.address" /></NFormItem>
        <NSpace>
          <NFormItem :label="t('workplace.city')"><NInput v-model:value="form.city" /></NFormItem>
          <NFormItem :label="t('workplace.region')"><NInput v-model:value="form.region" /></NFormItem>
          <NFormItem :label="t('workplace.country')"><NInput v-model:value="form.country" style="width: 90px" /></NFormItem>
        </NSpace>
        <NSpace>
          <NFormItem :label="t('workplace.remote')"><NSwitch v-model:value="form.is_remote" /></NFormItem>
          <NFormItem v-if="editing" :label="t('workplace.active')"><NSwitch v-model:value="form.is_active" /></NFormItem>
        </NSpace>
      </NForm>
      <template #footer>
        <NSpace justify="end">
          <NButton @click="show = false">{{ t('common.cancel') }}</NButton>
          <NButton type="primary" :disabled="!form.name" @click="save">{{ t('common.save') }}</NButton>
        </NSpace>
      </template>
    </NModal>
  </div>
</template>

<style scoped>
.head { display: flex; justify-content: space-between; align-items: flex-end; gap: 12px; flex-wrap: wrap; margin-bottom: 16px; }
.head h1 { font-size: 24px; font-weight: 650; margin: 0 0 4px; }
.muted { opacity: .7; font-size: 13px; margin: 0; max-width: 640px; }
</style>
