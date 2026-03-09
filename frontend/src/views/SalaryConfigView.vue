<script setup lang="ts">
import { ref, h, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NTabs, NTabPane, NDataTable, NButton, NModal, NForm, NFormItem,
  NInput, NSelect, NSwitch, NSpace, NTag, useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { salaryAPI } from '../api/client'

const { t } = useI18n()
const message = useMessage()

const structures = ref<Record<string, unknown>[]>([])
const components = ref<Record<string, unknown>[]>([])
const loading = ref(false)

// Structure modal
const showStructureModal = ref(false)
const structureForm = ref({ name: '', description: '' })

// Component modal
const showComponentModal = ref(false)
const componentForm = ref({
  code: '',
  name: '',
  component_type: 'allowance',
  is_taxable: true,
  is_statutory: false,
  is_fixed: true,
})

const componentTypes = [
  { label: t('salary.basic'), value: 'basic' },
  { label: t('salary.allowance'), value: 'allowance' },
  { label: t('salary.deduction'), value: 'deduction' },
  { label: t('salary.benefit'), value: 'benefit' },
  { label: t('salary.reimbursement'), value: 'reimbursement' },
]

const structureColumns: DataTableColumns = [
  { title: t('employee.name'), key: 'name' },
  { title: t('salary.description'), key: 'description' },
]

const componentColumns: DataTableColumns = [
  { title: t('common.code'), key: 'code', width: 120 },
  { title: t('common.name'), key: 'name' },
  { title: t('common.type'), key: 'component_type', width: 120 },
  {
    title: t('salary.taxable'), key: 'is_taxable', width: 80,
    render: (r) => h(NTag, { type: r.is_taxable ? 'warning' : 'default', size: 'small' }, () => r.is_taxable ? t('common.yes') : t('common.no'))
  },
  {
    title: t('salary.statutory'), key: 'is_statutory', width: 90,
    render: (r) => h(NTag, { type: r.is_statutory ? 'info' : 'default', size: 'small' }, () => r.is_statutory ? t('common.yes') : t('common.no'))
  },
  {
    title: t('salary.fixed'), key: 'is_fixed', width: 70,
    render: (r) => r.is_fixed ? t('salary.fixed') : t('salary.computed')
  },
]

function extract(res: unknown): Record<string, unknown>[] {
  const r = res as { data?: Record<string, unknown>[] }
  return r.data || (Array.isArray(r) ? r : []) as Record<string, unknown>[]
}

async function loadData() {
  loading.value = true
  try {
    const [s, c] = await Promise.allSettled([
      salaryAPI.listStructures(),
      salaryAPI.listComponents(),
    ])
    if (s.status === 'fulfilled') structures.value = extract(s.value)
    if (c.status === 'fulfilled') components.value = extract(c.value)
  } finally {
    loading.value = false
  }
}

onMounted(loadData)

async function createStructure() {
  if (!structureForm.value.name) {
    message.warning(t('profile.fillAllFields'))
    return
  }
  try {
    await salaryAPI.createStructure({
      name: structureForm.value.name,
      description: structureForm.value.description || undefined,
    })
    message.success(t('salary.structureCreated'))
    showStructureModal.value = false
    structureForm.value = { name: '', description: '' }
    loadData()
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } }
    message.error(err.data?.error?.message || t('common.saveFailed'))
  }
}

async function createComponent() {
  if (!componentForm.value.code || !componentForm.value.name) {
    message.warning(t('profile.fillAllFields'))
    return
  }
  try {
    await salaryAPI.createComponent({
      code: componentForm.value.code,
      name: componentForm.value.name,
      component_type: componentForm.value.component_type,
      is_taxable: componentForm.value.is_taxable,
      is_statutory: componentForm.value.is_statutory,
      is_fixed: componentForm.value.is_fixed,
    })
    message.success(t('salary.componentCreated'))
    showComponentModal.value = false
    componentForm.value = { code: '', name: '', component_type: 'allowance', is_taxable: true, is_statutory: false, is_fixed: true }
    loadData()
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } }
    message.error(err.data?.error?.message || t('common.saveFailed'))
  }
}
</script>

<template>
  <div>
    <h2 style="margin-bottom: 16px;">{{ t('salary.title') }}</h2>
    <NTabs type="line">
      <NTabPane :name="t('salary.structures')" :tab="t('salary.structures')">
        <NSpace justify="end" style="margin-bottom: 12px;">
          <NButton type="primary" size="small" @click="showStructureModal = true">{{ t('common.create') }}</NButton>
        </NSpace>
        <NDataTable :columns="structureColumns" :data="structures" :loading="loading" />
      </NTabPane>
      <NTabPane :name="t('salary.components')" :tab="t('salary.components')">
        <NSpace justify="end" style="margin-bottom: 12px;">
          <NButton type="primary" size="small" @click="showComponentModal = true">{{ t('common.create') }}</NButton>
        </NSpace>
        <NDataTable :columns="componentColumns" :data="components" :loading="loading" />
      </NTabPane>
    </NTabs>

    <!-- Structure Modal -->
    <NModal v-model:show="showStructureModal" :title="t('salary.createStructure')" preset="card" style="max-width: 420px; width: 95vw;">
      <NForm @submit.prevent="createStructure">
        <NFormItem :label="t('employee.name')" required>
          <NInput v-model:value="structureForm.name" :placeholder="t('salary.structurePlaceholder')" />
        </NFormItem>
        <NFormItem :label="t('salary.description')">
          <NInput v-model:value="structureForm.description" type="textarea" />
        </NFormItem>
        <NSpace>
          <NButton type="primary" attr-type="submit">{{ t('common.save') }}</NButton>
          <NButton @click="showStructureModal = false">{{ t('common.cancel') }}</NButton>
        </NSpace>
      </NForm>
    </NModal>

    <!-- Component Modal -->
    <NModal v-model:show="showComponentModal" :title="t('salary.createComponent')" preset="card" style="max-width: 480px; width: 95vw;">
      <NForm @submit.prevent="createComponent">
        <NFormItem :label="t('common.code')" required>
          <NInput v-model:value="componentForm.code" :placeholder="t('salary.codePlaceholder')" />
        </NFormItem>
        <NFormItem :label="t('employee.name')" required>
          <NInput v-model:value="componentForm.name" :placeholder="t('salary.componentPlaceholder')" />
        </NFormItem>
        <NFormItem :label="t('common.type')">
          <NSelect v-model:value="componentForm.component_type" :options="componentTypes" />
        </NFormItem>
        <NSpace :size="24">
          <NFormItem :label="t('salary.taxable')">
            <NSwitch v-model:value="componentForm.is_taxable" />
          </NFormItem>
          <NFormItem :label="t('salary.statutory')">
            <NSwitch v-model:value="componentForm.is_statutory" />
          </NFormItem>
          <NFormItem :label="t('salary.fixed')">
            <NSwitch v-model:value="componentForm.is_fixed" />
          </NFormItem>
        </NSpace>
        <NSpace>
          <NButton type="primary" attr-type="submit">{{ t('common.save') }}</NButton>
          <NButton @click="showComponentModal = false">{{ t('common.cancel') }}</NButton>
        </NSpace>
      </NForm>
    </NModal>
  </div>
</template>
