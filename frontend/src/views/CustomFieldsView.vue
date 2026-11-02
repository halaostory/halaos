<script setup lang="ts">
import { ref, h, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NCard, NSpace, NButton, NDataTable, NTag, NModal, NForm, NFormItem, NInput, NSelect,
  NDynamicTags, NCheckbox, NInputNumber, useMessage, type DataTableColumns,
} from 'naive-ui'
import { lifecycleAPI } from '../api/client'

interface Field {
  id: number
  field_key: string
  label: string
  field_type: string
  options: string[]
  is_required: boolean
  is_sensitive: boolean
  sort_order: number
  is_active: boolean
}

const { t } = useI18n()
const message = useMessage()
const fields = ref<Field[]>([])
const loading = ref(false)
const showModal = ref(false)
const saving = ref(false)
const editingId = ref<number | null>(null)

const blank = () => ({
  label: '', key: '', field_type: 'text', options: [] as string[],
  is_required: false, is_sensitive: false, sort_order: 0, is_active: true,
})
const form = ref(blank())

const typeOptions = computed(() =>
  ['text', 'number', 'date', 'select', 'boolean'].map((v) => ({ label: t('lifecycle.type_' + v), value: v })),
)

const columns = computed<DataTableColumns<Field>>(() => [
  { title: t('lifecycle.fieldLabel'), key: 'label' },
  { title: t('lifecycle.fieldKey'), key: 'field_key', render: (r) => h('code', r.field_key) },
  { title: t('lifecycle.fieldType'), key: 'field_type', width: 120, render: (r) => t('lifecycle.type_' + r.field_type) },
  {
    title: t('lifecycle.fieldOptions'), key: 'options',
    render: (r) => h(NSpace, { size: 4 }, () => r.options.map((o) => h(NTag, { size: 'small' }, () => o))),
  },
  {
    title: '', key: 'flags', width: 220,
    render: (r) => h(NSpace, { size: 4 }, () => [
      r.is_required ? h(NTag, { size: 'small', type: 'warning' }, () => t('lifecycle.required')) : null,
      r.is_sensitive ? h(NTag, { size: 'small', type: 'error' }, () => t('lifecycle.sensitive')) : null,
      !r.is_active ? h(NTag, { size: 'small' }, () => t('common.cancelled')) : null,
    ]),
  },
  {
    title: '', key: 'actions', width: 90,
    render: (r) => h(NButton, { size: 'small', onClick: () => openEdit(r) }, () => t('common.edit')),
  },
])

async function load() {
  loading.value = true
  try {
    const res = (await lifecycleAPI.listCustomFields({ include_inactive: 'true' })) as { data: Field[] }
    fields.value = res.data || []
  } catch {
    message.error(t('common.loadFailed'))
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingId.value = null
  form.value = blank()
  showModal.value = true
}

function openEdit(f: Field) {
  editingId.value = f.id
  form.value = {
    label: f.label, key: f.field_key, field_type: f.field_type, options: [...f.options],
    is_required: f.is_required, is_sensitive: f.is_sensitive, sort_order: f.sort_order, is_active: f.is_active,
  }
  showModal.value = true
}

async function save() {
  saving.value = true
  try {
    const body: Record<string, unknown> = { ...form.value }
    if (editingId.value) await lifecycleAPI.updateCustomField(editingId.value, body)
    else await lifecycleAPI.createCustomField(body)
    message.success(t('common.saved'))
    showModal.value = false
    await load()
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } }
    message.error(err.data?.error?.message || t('common.saveFailed'))
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<template>
  <NSpace vertical :size="16">
    <NSpace justify="space-between" align="center">
      <div>
        <h2 style="margin: 0">{{ t('lifecycle.customFieldsTitle') }}</h2>
        <div style="opacity: 0.7">{{ t('lifecycle.customFieldsSubtitle') }}</div>
      </div>
      <NButton type="primary" @click="openCreate">{{ t('lifecycle.addField') }}</NButton>
    </NSpace>

    <NCard>
      <NDataTable :columns="columns" :data="fields" :loading="loading" :row-key="(r: Field) => r.id" />
    </NCard>

    <NModal v-model:show="showModal" preset="card" style="max-width: 520px" :title="editingId ? t('common.edit') : t('lifecycle.addField')">
      <NForm label-placement="top">
        <NFormItem :label="t('lifecycle.fieldLabel')" required>
          <NInput v-model:value="form.label" />
        </NFormItem>
        <NFormItem v-if="!editingId" :label="t('lifecycle.fieldKey')">
          <NInput v-model:value="form.key" placeholder="t_shirt_size" />
        </NFormItem>
        <NFormItem :label="t('lifecycle.fieldType')">
          <NSelect v-model:value="form.field_type" :options="typeOptions" :disabled="!!editingId" />
        </NFormItem>
        <NFormItem v-if="form.field_type === 'select'" :label="t('lifecycle.fieldOptions')" :feedback="t('lifecycle.fieldOptionsHint')">
          <NDynamicTags v-model:value="form.options" />
        </NFormItem>
        <NFormItem label="#">
          <NInputNumber v-model:value="form.sort_order" :min="0" />
        </NFormItem>
        <NSpace vertical>
          <NCheckbox v-model:checked="form.is_required">{{ t('lifecycle.required') }}</NCheckbox>
          <NCheckbox v-model:checked="form.is_sensitive">{{ t('lifecycle.sensitive') }}</NCheckbox>
          <NCheckbox v-if="editingId" v-model:checked="form.is_active">{{ t('lifecycle.active') }}</NCheckbox>
        </NSpace>
      </NForm>
      <NSpace justify="end" style="margin-top: 16px">
        <NButton @click="showModal = false">{{ t('common.cancel') }}</NButton>
        <NButton type="primary" :loading="saving" @click="save">{{ t('common.save') }}</NButton>
      </NSpace>
    </NModal>
  </NSpace>
</template>
