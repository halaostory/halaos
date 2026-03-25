<script setup lang="ts">
import { ref, onMounted, computed, h } from 'vue'
import {
  NCard, NDataTable, NButton, NSpace, NModal, NForm, NFormItem,
  NSelect, NInputNumber, NSwitch, NDatePicker, NEmpty, NPopconfirm,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { benefitDeductionAPI } from '../../api/client'

const props = defineProps<{
  employeeId: number
}>()

const { t } = useI18n()
const message = useMessage()

interface Deduction {
  id: number
  deduction_type: string
  amount_per_period: string
  annual_limit: string
  reduces_fica: boolean
  effective_date: string
  end_date: string | null
}

const deductions = ref<Deduction[]>([])
const loading = ref(false)
const showModal = ref(false)
const editingId = ref<number | null>(null)

const formData = ref({
  deduction_type: '',
  amount_per_period: 0,
  annual_limit: 0,
  reduces_fica: false,
  effective_date: null as number | null,
  end_date: null as number | null,
})

const deductionTypeOptions = [
  { label: t('payroll.benefitDeductions.types.401k'), value: '401k' },
  { label: t('payroll.benefitDeductions.types.health_insurance'), value: 'health_insurance' },
  { label: t('payroll.benefitDeductions.types.dental_vision'), value: 'dental_vision' },
  { label: t('payroll.benefitDeductions.types.hsa'), value: 'hsa' },
  { label: t('payroll.benefitDeductions.types.fsa_health'), value: 'fsa_health' },
  { label: t('payroll.benefitDeductions.types.fsa_dependent'), value: 'fsa_dependent' },
]

async function loadDeductions() {
  loading.value = true
  try {
    const res = await benefitDeductionAPI.list(props.employeeId)
    deductions.value = (res as any).data || []
  } catch {
    message.error('Failed to load benefit deductions')
  } finally {
    loading.value = false
  }
}

function openAdd() {
  editingId.value = null
  formData.value = {
    deduction_type: '',
    amount_per_period: 0,
    annual_limit: 0,
    reduces_fica: false,
    effective_date: null,
    end_date: null,
  }
  showModal.value = true
}

function openEdit(row: Deduction) {
  editingId.value = row.id
  formData.value = {
    deduction_type: row.deduction_type,
    amount_per_period: parseFloat(row.amount_per_period) || 0,
    annual_limit: parseFloat(row.annual_limit) || 0,
    reduces_fica: row.reduces_fica,
    effective_date: row.effective_date ? new Date(row.effective_date).getTime() : null,
    end_date: row.end_date ? new Date(row.end_date).getTime() : null,
  }
  showModal.value = true
}

function formatDate(ts: number | null): string {
  if (!ts) return ''
  const d = new Date(ts)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

async function handleSave() {
  if (!formData.value.deduction_type || !formData.value.effective_date) {
    message.warning('Please fill required fields')
    return
  }
  const payload: Record<string, unknown> = {
    employee_id: props.employeeId,
    deduction_type: formData.value.deduction_type,
    amount_per_period: formData.value.amount_per_period,
    annual_limit: formData.value.annual_limit,
    reduces_fica: formData.value.reduces_fica,
    effective_date: formatDate(formData.value.effective_date),
    end_date: formData.value.end_date ? formatDate(formData.value.end_date) : '',
  }
  try {
    if (editingId.value) {
      await benefitDeductionAPI.update(editingId.value, payload)
      message.success('Deduction updated')
    } else {
      await benefitDeductionAPI.create(payload)
      message.success('Deduction created')
    }
    showModal.value = false
    await loadDeductions()
  } catch {
    message.error('Failed to save deduction')
  }
}

async function handleDelete(id: number) {
  try {
    await benefitDeductionAPI.remove(id)
    message.success('Deduction deleted')
    await loadDeductions()
  } catch {
    message.error('Failed to delete deduction')
  }
}

function formatAmount(val: string | number): string {
  const n = typeof val === 'string' ? parseFloat(val) : val
  return isNaN(n) ? '-' : `$${n.toFixed(2)}`
}

const columns = computed<DataTableColumns<Deduction>>(() => [
  { title: t('payroll.benefitDeductions.type'), key: 'deduction_type', width: 180,
    render: (row) => {
      const opt = deductionTypeOptions.find(o => o.value === row.deduction_type)
      return opt?.label || row.deduction_type
    }
  },
  { title: t('payroll.benefitDeductions.amountPerPeriod'), key: 'amount_per_period', width: 130,
    render: (row) => formatAmount(row.amount_per_period)
  },
  { title: t('payroll.benefitDeductions.annualLimit'), key: 'annual_limit', width: 120,
    render: (row) => formatAmount(row.annual_limit)
  },
  { title: t('payroll.benefitDeductions.reducesFica'), key: 'reduces_fica', width: 100,
    render: (row) => row.reduces_fica ? 'Yes' : 'No'
  },
  { title: t('payroll.benefitDeductions.effectiveDate'), key: 'effective_date', width: 120,
    render: (row) => row.effective_date?.split('T')[0] || '-'
  },
  { title: t('payroll.benefitDeductions.endDate'), key: 'end_date', width: 120,
    render: (row) => row.end_date?.split('T')[0] || '-'
  },
  { title: '', key: 'actions', width: 120,
    render: (row) => h(NSpace, { size: 4 }, () => [
      h(NButton, { size: 'tiny', onClick: () => openEdit(row) }, () => t('common.edit')),
      h(NPopconfirm, { onPositiveClick: () => handleDelete(row.id) }, {
        trigger: () => h(NButton, { size: 'tiny', type: 'error' }, () => t('common.delete')),
        default: () => t('common.confirmDelete'),
      }),
    ])
  },
])

onMounted(loadDeductions)
</script>

<template>
  <NCard :title="t('payroll.benefitDeductions.title')">
    <template #header-extra>
      <NButton type="primary" size="small" @click="openAdd">
        {{ t('payroll.benefitDeductions.add') }}
      </NButton>
    </template>
    <NDataTable v-if="deductions.length" :columns="columns" :data="deductions" :loading="loading" size="small" />
    <NEmpty v-else :description="t('payroll.benefitDeductions.noDeductions')" />
  </NCard>

  <NModal v-model:show="showModal" preset="card" :title="editingId ? t('payroll.benefitDeductions.edit') : t('payroll.benefitDeductions.add')" style="max-width: 480px">
    <NForm label-placement="left" label-width="140">
      <NFormItem :label="t('payroll.benefitDeductions.type')">
        <NSelect v-model:value="formData.deduction_type" :options="deductionTypeOptions" />
      </NFormItem>
      <NFormItem :label="t('payroll.benefitDeductions.amountPerPeriod')">
        <NInputNumber v-model:value="formData.amount_per_period" :precision="2" :min="0" style="width: 100%" />
      </NFormItem>
      <NFormItem :label="t('payroll.benefitDeductions.annualLimit')">
        <NInputNumber v-model:value="formData.annual_limit" :precision="2" :min="0" style="width: 100%" />
      </NFormItem>
      <NFormItem :label="t('payroll.benefitDeductions.reducesFica')">
        <NSwitch v-model:value="formData.reduces_fica" />
      </NFormItem>
      <NFormItem :label="t('payroll.benefitDeductions.effectiveDate')">
        <NDatePicker v-model:value="formData.effective_date" type="date" style="width: 100%" />
      </NFormItem>
      <NFormItem :label="t('payroll.benefitDeductions.endDate')">
        <NDatePicker v-model:value="formData.end_date" type="date" clearable style="width: 100%" />
      </NFormItem>
    </NForm>
    <template #footer>
      <NSpace justify="end">
        <NButton @click="showModal = false">{{ t('common.cancel') }}</NButton>
        <NButton type="primary" @click="handleSave">{{ t('common.save') }}</NButton>
      </NSpace>
    </template>
  </NModal>
</template>
