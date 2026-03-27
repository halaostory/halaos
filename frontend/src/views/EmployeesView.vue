<script setup lang="ts">
import { ref, computed, h, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  NDataTable, NButton, NSpace, NInput, NTag, NPagination, NModal, NForm, NFormItem,
  NInputNumber, NSelect, NDatePicker, NAlert, useMessage, type DataTableColumns,
} from 'naive-ui'
import { employeeAPI, exportAPI, companyAPI } from '../api/client'
import { useAuthStore } from '../stores/auth'
import { format } from 'date-fns'

const router = useRouter()
const { t } = useI18n()
const message = useMessage()
const authStore = useAuthStore()
const data = ref<Record<string, unknown>[]>([])
const loading = ref(false)
const search = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)

// Lookup maps for departments and positions
const departmentMap = ref<Map<number, string>>(new Map())
const positionMap = ref<Map<number, string>>(new Map())
const departments = ref<{ id: number; name: string }[]>([])

// Filter refs
const selectedDepartment = ref<number | null>(null)
const selectedStatus = ref<string | null>(null)

const statusColorMap: Record<string, 'success' | 'warning' | 'error' | 'info' | 'default'> = {
  active: 'success',
  probationary: 'info',
  suspended: 'warning',
  separated: 'error',
}

function formatDate(d: string): string {
  if (!d) return ''
  try { return format(new Date(d), 'yyyy-MM-dd') } catch { return d }
}

const columns = computed<DataTableColumns>(() => [
  { title: t('employee.employeeNo'), key: 'employee_no', width: 120 },
  { title: t('employee.name'), key: 'name', render: (row) => `${row.first_name} ${row.last_name}` },
  {
    title: t('employee.department'),
    key: 'department_id',
    width: 150,
    render: (row) => {
      const id = row.department_id as number
      return departmentMap.value.get(id) ?? String(id ?? '')
    },
  },
  {
    title: t('employee.position'),
    key: 'position_id',
    width: 150,
    render: (row) => {
      const id = row.position_id as number
      return positionMap.value.get(id) ?? String(id ?? '')
    },
  },
  { title: t('employee.employmentType'), key: 'employment_type', width: 130 },
  {
    title: t('common.status'),
    key: 'status',
    width: 110,
    render: (row) => {
      const s = row.status as string
      return h(NTag, { type: statusColorMap[s] || 'default', size: 'small' }, () => s)
    }
  },
  {
    title: t('employee.hireDate'),
    key: 'hire_date',
    width: 120,
    render: (row) => formatDate(row.hire_date as string)
  },
])

// Department filter options
const departmentOptions = computed(() =>
  departments.value.map((d) => ({ label: d.name, value: d.id }))
)

// Status filter options
const statusOptions = computed(() => [
  { label: t('employee.active'), value: 'active' },
  { label: t('employee.probationary'), value: 'probationary' },
  { label: t('employee.suspended'), value: 'suspended' },
  { label: t('employee.separated'), value: 'separated' },
])

// Reset page to 1 when any filter changes
watch([search, selectedDepartment, selectedStatus], () => {
  page.value = 1
})

// Filter data client-side by search, department, and status
const filteredData = computed(() => {
  let result = data.value

  if (selectedDepartment.value !== null) {
    result = result.filter((row) => row.department_id === selectedDepartment.value)
  }

  if (selectedStatus.value !== null) {
    result = result.filter((row) => row.status === selectedStatus.value)
  }

  if (search.value.trim()) {
    const q = search.value.toLowerCase()
    result = result.filter((row) => {
      const name = `${row.first_name} ${row.last_name}`.toLowerCase()
      const no = (row.employee_no as string || '').toLowerCase()
      const email = (row.email as string || '').toLowerCase()
      return name.includes(q) || no.includes(q) || email.includes(q)
    })
  }

  return result
})

const pagedData = computed(() => {
  const start = (page.value - 1) * pageSize.value
  return filteredData.value.slice(start, start + pageSize.value)
})

onMounted(async () => {
  loading.value = true
  try {
    const [employeeRes, deptRes, posRes] = await Promise.all([
      employeeAPI.list({ page: '1', limit: '200' }) as Promise<{ data: Record<string, unknown>[]; meta?: { total: number } }>,
      companyAPI.listDepartments() as Promise<{ data: { id: number; name: string }[] }>,
      companyAPI.listPositions() as Promise<{ data: { id: number; title: string }[] }>,
    ])

    data.value = employeeRes.data || []
    total.value = employeeRes.meta?.total || data.value.length

    const deptList = deptRes.data || []
    departments.value = deptList
    const dMap = new Map<number, string>()
    for (const d of deptList) {
      dMap.set(d.id, d.name)
    }
    departmentMap.value = dMap

    const posList = posRes.data || []
    const pMap = new Map<number, string>()
    for (const p of posList) {
      pMap.set(p.id, p.title)
    }
    positionMap.value = pMap
  } catch {
    data.value = []
  } finally {
    loading.value = false
  }
})

function handleRowClick(row: Record<string, unknown>) {
  router.push({ name: 'employee-detail', params: { id: String(row.id) } })
}

function handleExportCSV() {
  const url = exportAPI.employeesCSV()
  const token = localStorage.getItem('token')
  fetch(url, { headers: { Authorization: `Bearer ${token}` } })
    .then((res) => {
      if (!res.ok) throw new Error('Export failed')
      const disposition = res.headers.get('Content-Disposition')
      const match = disposition?.match(/filename=(.+)/)
      const filename = match ? match[1] : 'employees.csv'
      return res.blob().then((blob) => ({ blob, filename }))
    })
    .then(({ blob, filename }) => {
      const a = document.createElement('a')
      a.href = URL.createObjectURL(blob)
      a.download = filename
      a.click()
      URL.revokeObjectURL(a.href)
    })
    .catch(() => message.error(t('common.failed')))
}

// Bulk Salary Update
const showBulkSalary = ref(false)
const bulkSalaryLoading = ref(false)
const bulkSalaryForm = ref({
  updateType: 'percentage' as 'percentage' | 'fixed',
  value: 5,
  effectiveFrom: null as number | null,
  remarks: '',
})
const bulkSalaryResult = ref<{ updated: number; failed: number; results: { employee_id: number; old_salary: number; new_salary: number }[] } | null>(null)

const updateTypeOptions = [
  { label: t('employee.bulkPercentage'), value: 'percentage' },
  { label: t('employee.bulkFixed'), value: 'fixed' },
]

const activeEmployeeIds = computed(() =>
  data.value.filter(e => e.status === 'active').map(e => e.id as number)
)

async function handleBulkSalaryUpdate() {
  if (!bulkSalaryForm.value.effectiveFrom) {
    message.warning(t('common.fillAllFields'))
    return
  }
  bulkSalaryLoading.value = true
  bulkSalaryResult.value = null
  try {
    const effDate = new Date(bulkSalaryForm.value.effectiveFrom).toISOString().split('T')[0]
    const res = await employeeAPI.bulkSalaryUpdate({
      employee_ids: activeEmployeeIds.value,
      update_type: bulkSalaryForm.value.updateType,
      value: bulkSalaryForm.value.value,
      effective_from: effDate,
      remarks: bulkSalaryForm.value.remarks || undefined,
    })
    const d = (res as any)?.data ?? res
    bulkSalaryResult.value = d
    message.success(t('employee.bulkSalaryDone', { count: d.updated }))
  } catch {
    message.error(t('common.failed'))
  }
  bulkSalaryLoading.value = false
}

</script>

<template>
  <div>
    <NSpace justify="space-between" style="margin-bottom: 16px;">
      <h2>{{ t('employee.title') }}</h2>
      <NSpace>
        <NButton @click="handleExportCSV">CSV</NButton>
        <NButton v-if="authStore.isAdmin" @click="showBulkSalary = true">
          {{ t('employee.bulkSalaryUpdate') }}
        </NButton>
        <NButton type="primary" @click="router.push({ name: 'employee-new' })">
          {{ t('employee.addNew') }}
        </NButton>
      </NSpace>
    </NSpace>
    <NSpace style="margin-bottom: 16px;" align="center">
      <NSelect
        v-model:value="selectedDepartment"
        :options="departmentOptions"
        :placeholder="t('employee.department')"
        clearable
        filterable
        style="width: 200px;"
      />
      <NSelect
        v-model:value="selectedStatus"
        :options="statusOptions"
        :placeholder="t('common.status')"
        clearable
        style="width: 200px;"
      />
      <NInput v-model:value="search" :placeholder="t('common.search')" clearable style="width: 240px;" />
    </NSpace>
    <NDataTable
      :columns="columns"
      :data="pagedData"
      :loading="loading"
      :row-props="(row: Record<string, unknown>) => ({ style: 'cursor: pointer', onClick: () => handleRowClick(row) })"
    />
    <div style="display: flex; justify-content: flex-end; margin-top: 16px;" v-if="filteredData.length > pageSize">
      <NPagination v-model:page="page" :page-size="pageSize" :item-count="filteredData.length" />
    </div>

    <!-- Bulk Salary Update Modal -->
    <NModal v-model:show="showBulkSalary" preset="card" :title="t('employee.bulkSalaryUpdate')" style="max-width: 500px;">
      <NForm label-placement="left" label-width="140">
        <NFormItem :label="t('employee.bulkUpdateType')">
          <NSelect v-model:value="bulkSalaryForm.updateType" :options="updateTypeOptions" />
        </NFormItem>
        <NFormItem :label="t('employee.bulkValue')">
          <NInputNumber v-model:value="bulkSalaryForm.value" :min="0" style="width: 100%;">
            <template #suffix>{{ bulkSalaryForm.updateType === 'percentage' ? '%' : 'PHP' }}</template>
          </NInputNumber>
        </NFormItem>
        <NFormItem :label="t('employee.effectiveFrom')">
          <NDatePicker v-model:value="bulkSalaryForm.effectiveFrom" type="date" style="width: 100%;" />
        </NFormItem>
        <NFormItem :label="t('employee.remarks')">
          <NInput v-model:value="bulkSalaryForm.remarks" />
        </NFormItem>
      </NForm>
      <NAlert type="info" style="margin-bottom: 12px;">
        {{ t('employee.bulkSalaryInfo', { count: activeEmployeeIds.length }) }}
      </NAlert>
      <NAlert v-if="bulkSalaryResult" :type="bulkSalaryResult.failed === 0 ? 'success' : 'warning'" style="margin-bottom: 12px;">
        {{ t('employee.bulkSalaryDone', { count: bulkSalaryResult.updated }) }}
        <template v-if="bulkSalaryResult.failed > 0"> ({{ bulkSalaryResult.failed }} failed)</template>
      </NAlert>
      <NSpace justify="end">
        <NButton @click="showBulkSalary = false">{{ t('common.cancel') }}</NButton>
        <NButton type="primary" :loading="bulkSalaryLoading" :disabled="!activeEmployeeIds.length" @click="handleBulkSalaryUpdate">
          {{ t('common.confirm') }}
        </NButton>
      </NSpace>
    </NModal>
  </div>
</template>
