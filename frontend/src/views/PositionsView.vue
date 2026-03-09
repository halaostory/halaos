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
const form = ref({ code: '', title: '', grade: '' })

const columns: DataTableColumns = [
  { title: t('common.code'), key: 'code', width: 120 },
  { title: t('employee.position'), key: 'title' },
  { title: t('position.grade'), key: 'grade', width: 80 },
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
    const res = await companyAPI.listPositions() as { data?: Record<string, unknown>[] }
    data.value = (res.data || (Array.isArray(res) ? res : [])) as Record<string, unknown>[]
  } catch {
    data.value = []
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingId.value = null
  form.value = { code: '', title: '', grade: '' }
  showModal.value = true
}

function openEdit(row: Record<string, unknown>) {
  editingId.value = row.id as number
  form.value = { code: String(row.code || ''), title: String(row.title || ''), grade: String(row.grade || '') }
  showModal.value = true
}

async function handleSave() {
  if (!form.value.code || !form.value.title) {
    message.warning(t('profile.fillAllFields'))
    return
  }
  try {
    if (editingId.value) {
      await companyAPI.updatePosition(editingId.value, { code: form.value.code, title: form.value.title, grade: form.value.grade || undefined })
      message.success(t('position.updated'))
    } else {
      await companyAPI.createPosition({ code: form.value.code, title: form.value.title, grade: form.value.grade || undefined })
      message.success(t('position.created'))
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
      <h2>{{ t('nav.positions') }}</h2>
      <NButton type="primary" @click="openCreate">{{ t('common.create') }}</NButton>
    </NSpace>
    <NDataTable :columns="columns" :data="data" :loading="loading" />

    <NModal v-model:show="showModal" :title="editingId ? t('position.edit') : t('position.create')" preset="card" style="max-width: 400px; width: 95vw;">
      <NForm @submit.prevent="handleSave">
        <NFormItem :label="t('common.code')" required>
          <NInput v-model:value="form.code" :placeholder="t('position.codePlaceholder')" :disabled="!!editingId" />
        </NFormItem>
        <NFormItem :label="t('employee.position')" required>
          <NInput v-model:value="form.title" :placeholder="t('position.namePlaceholder')" />
        </NFormItem>
        <NFormItem :label="t('position.grade')">
          <NInput v-model:value="form.grade" :placeholder="t('position.gradePlaceholder')" />
        </NFormItem>
        <NSpace>
          <NButton type="primary" attr-type="submit">{{ t('common.save') }}</NButton>
          <NButton @click="showModal = false">{{ t('common.cancel') }}</NButton>
        </NSpace>
      </NForm>
    </NModal>
  </div>
</template>
