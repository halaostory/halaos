<script setup lang="ts">
import { ref, h, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NDataTable, NButton, NModal, NForm, NFormItem, NInput, NSpace, NTag, useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { companyAPI } from '../api/client'

const { t } = useI18n()
const message = useMessage()
const data = ref<Record<string, unknown>[]>([])
const loading = ref(false)
const showModal = ref(false)
const editingId = ref<number | null>(null)
const form = ref({ code: '', name: '' })

const columns: DataTableColumns = [
  { title: t('common.code'), key: 'code', width: 120 },
  { title: t('employee.name'), key: 'name' },
  {
    title: t('common.status'), key: 'is_active', width: 100,
    render: (row) => h(NTag, { type: row.is_active !== false ? 'success' : 'default', size: 'small' }, () => row.is_active !== false ? t('common.active') : t('common.inactive'))
  },
  {
    title: t('common.actions'), key: 'actions', width: 80,
    render: (row) => h(NButton, { size: 'small', onClick: () => openEdit(row) }, () => t('common.edit'))
  },
]

onMounted(loadData)

async function loadData() {
  loading.value = true
  try {
    const res = await companyAPI.listDepartments() as { data?: Record<string, unknown>[] }
    data.value = (res.data || (Array.isArray(res) ? res : [])) as Record<string, unknown>[]
  } catch {
    data.value = []
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingId.value = null
  form.value = { code: '', name: '' }
  showModal.value = true
}

function openEdit(row: Record<string, unknown>) {
  editingId.value = row.id as number
  form.value = { code: String(row.code || ''), name: String(row.name || '') }
  showModal.value = true
}

async function handleSave() {
  if (!form.value.code || !form.value.name) {
    message.warning(t('profile.fillAllFields'))
    return
  }
  try {
    if (editingId.value) {
      await companyAPI.updateDepartment(editingId.value, { code: form.value.code, name: form.value.name })
      message.success(t('department.updated'))
    } else {
      await companyAPI.createDepartment({ code: form.value.code, name: form.value.name })
      message.success(t('department.created'))
    }
    showModal.value = false
    loadData()
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } }
    message.error(err.data?.error?.message || t('common.saveFailed'))
  }
}
</script>

<template>
  <div>
    <NSpace justify="space-between" style="margin-bottom: 16px;">
      <h2>{{ t('nav.departments') }}</h2>
      <NButton type="primary" @click="openCreate">{{ t('common.create') }}</NButton>
    </NSpace>
    <NDataTable :columns="columns" :data="data" :loading="loading" />

    <NModal v-model:show="showModal" :title="editingId ? t('department.edit') : t('department.create')" preset="card" style="max-width: 400px; width: 95vw;">
      <NForm @submit.prevent="handleSave">
        <NFormItem :label="t('common.code')" required>
          <NInput v-model:value="form.code" :placeholder="t('department.codePlaceholder')" :disabled="!!editingId" />
        </NFormItem>
        <NFormItem :label="t('employee.name')" required>
          <NInput v-model:value="form.name" :placeholder="t('department.namePlaceholder')" />
        </NFormItem>
        <NSpace>
          <NButton type="primary" attr-type="submit">{{ t('common.save') }}</NButton>
          <NButton @click="showModal = false">{{ t('common.cancel') }}</NButton>
        </NSpace>
      </NForm>
    </NModal>
  </div>
</template>
