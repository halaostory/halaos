<script setup lang="ts">
import { ref, h, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NTabs, NTabPane, NDataTable, NButton, NModal, NForm, NFormItem,
  NInput, NInputNumber, NSelect, NSwitch, NDatePicker, NSpace, NTag,
  NStatistic, NGrid, NGi, NCard,
  useMessage, type DataTableColumns,
} from 'naive-ui'
import { expenseAPI, formPrefillAPI } from '../api/client'
import { useAuthStore } from '../stores/auth'
import { format } from 'date-fns'

const { t } = useI18n()
const message = useMessage()
const auth = useAuthStore()

interface ExpenseCategory {
  id: number
  name: string
  description: string | null
  max_amount: number | null
  requires_receipt: boolean
  is_active: boolean
}

interface ExpenseClaim {
  id: number
  claim_number: string
  category_id: number
  category_name: string
  description: string
  amount: number
  currency: string
  expense_date: string
  receipt_path: string | null
  status: string
  submitted_at: string | null
  approved_at: string | null
  rejection_reason: string | null
  paid_at: string | null
  paid_reference: string | null
  notes: string | null
  first_name?: string
  last_name?: string
  employee_no?: string
  created_at: string
}

interface ExpenseSummary {
  draft_count: number
  submitted_count: number
  approved_count: number
  rejected_count: number
  paid_count: number
  pending_amount: number
  approved_amount: number
  paid_amount: number
}

const categories = ref<ExpenseCategory[]>([])
const myClaims = ref<ExpenseClaim[]>([])
const allClaims = ref<ExpenseClaim[]>([])
const summary = ref<ExpenseSummary | null>(null)
const loading = ref(false)

const isAdmin = computed(() => auth.isAdmin)
const isManager = computed(() => auth.isAdmin || auth.isManager)

// Claim modal
const showClaimModal = ref(false)
const claimForm = ref({
  category_id: null as number | null,
  description: '',
  amount: null as number | null,
  currency: 'PHP',
  expense_date: Date.now() as number | null,
  notes: '',
  submit: false,
})

// Category modal
const showCatModal = ref(false)
const editingCat = ref<ExpenseCategory | null>(null)
const catForm = ref({
  name: '',
  description: '',
  max_amount: null as number | null,
  requires_receipt: true,
  is_active: true,
})

// Reject modal
const showRejectModal = ref(false)
const rejectClaimId = ref(0)
const rejectReason = ref('')

// Pay modal
const showPayModal = ref(false)
const payClaimId = ref(0)
const payReference = ref('')

const statusColor: Record<string, string> = {
  draft: 'default',
  submitted: 'warning',
  approved: 'success',
  rejected: 'error',
  paid: 'info',
}

const categoryOptions = computed(() =>
  categories.value
    .filter(c => c.is_active)
    .map(c => ({ label: c.name, value: c.id }))
)

function extractData(res: unknown): any {
  return (res as any)?.data ?? res
}

async function fetchAll() {
  loading.value = true
  try {
    const [catRes, myRes] = await Promise.all([
      expenseAPI.listCategories(),
      expenseAPI.my(),
    ])
    categories.value = Array.isArray(extractData(catRes)) ? extractData(catRes) : []
    myClaims.value = Array.isArray(extractData(myRes)) ? extractData(myRes) : []

    if (isManager.value) {
      const [allRes, sumRes] = await Promise.all([
        expenseAPI.list(),
        expenseAPI.summary(),
      ])
      const allData = extractData(allRes)
      allClaims.value = Array.isArray(allData) ? allData : (allData?.data ?? [])
      summary.value = extractData(sumRes) as ExpenseSummary
    }
  } catch {
    message.error(t('expense.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function openCreateClaim() {
  claimForm.value = {
    category_id: null,
    description: '',
    amount: null,
    currency: 'PHP',
    expense_date: Date.now(),
    notes: '',
    submit: false,
  }
  showClaimModal.value = true
  try {
    const res = await formPrefillAPI.get('expense')
    const d = (res as any)?.data ?? res
    if (d) {
      if (d.category_id) claimForm.value.category_id = d.category_id
      if (d.suggested_amount) claimForm.value.amount = d.suggested_amount
    }
  } catch { /* prefill is best-effort */ }
}

async function saveClaim() {
  if (!claimForm.value.category_id || !claimForm.value.description || !claimForm.value.amount) {
    message.warning(t('common.fillRequired'))
    return
  }
  try {
    await expenseAPI.create({
      category_id: claimForm.value.category_id,
      description: claimForm.value.description,
      amount: claimForm.value.amount,
      currency: claimForm.value.currency,
      expense_date: claimForm.value.expense_date ? format(new Date(claimForm.value.expense_date), 'yyyy-MM-dd') : format(new Date(), 'yyyy-MM-dd'),
      notes: claimForm.value.notes || null,
      submit: claimForm.value.submit,
    })
    message.success(t('common.created'))
    showClaimModal.value = false
    fetchAll()
  } catch {
    message.error(t('common.error'))
  }
}

async function submitClaim(id: number) {
  try {
    await expenseAPI.submit(id)
    message.success(t('expense.submitted'))
    fetchAll()
  } catch {
    message.error(t('common.error'))
  }
}

async function approveClaim(id: number) {
  try {
    await expenseAPI.approve(id)
    message.success(t('expense.approved'))
    fetchAll()
  } catch {
    message.error(t('common.error'))
  }
}

async function rejectClaim() {
  try {
    await expenseAPI.reject(rejectClaimId.value, rejectReason.value)
    message.success(t('expense.rejected'))
    showRejectModal.value = false
    rejectReason.value = ''
    fetchAll()
  } catch {
    message.error(t('common.error'))
  }
}

async function markPaid() {
  try {
    await expenseAPI.markPaid(payClaimId.value, payReference.value)
    message.success(t('expense.markedPaid'))
    showPayModal.value = false
    payReference.value = ''
    fetchAll()
  } catch {
    message.error(t('common.error'))
  }
}

function openCatCreate() {
  editingCat.value = null
  catForm.value = { name: '', description: '', max_amount: null, requires_receipt: true, is_active: true }
  showCatModal.value = true
}

function openCatEdit(cat: ExpenseCategory) {
  editingCat.value = cat
  catForm.value = {
    name: cat.name,
    description: cat.description || '',
    max_amount: cat.max_amount,
    requires_receipt: cat.requires_receipt,
    is_active: cat.is_active,
  }
  showCatModal.value = true
}

async function saveCat() {
  if (!catForm.value.name) {
    message.warning(t('common.fillRequired'))
    return
  }
  try {
    if (editingCat.value) {
      await expenseAPI.updateCategory(editingCat.value.id, {
        ...catForm.value,
        description: catForm.value.description || null,
      })
      message.success(t('common.updated'))
    } else {
      await expenseAPI.createCategory({
        ...catForm.value,
        description: catForm.value.description || null,
      })
      message.success(t('common.created'))
    }
    showCatModal.value = false
    fetchAll()
  } catch {
    message.error(t('common.error'))
  }
}

const myColumns = computed<DataTableColumns<ExpenseClaim>>(() => [
  { title: t('expense.claimNumber'), key: 'claim_number', width: 120 },
  { title: t('expense.category'), key: 'category_name' },
  { title: t('expense.description'), key: 'description', ellipsis: { tooltip: true } },
  { title: t('expense.amount'), key: 'amount', width: 120, render: (row) => `${row.currency} ${Number(row.amount).toLocaleString('en-PH', { minimumFractionDigits: 2 })}` },
  { title: t('expense.expenseDate'), key: 'expense_date', width: 110, render: (row) => format(new Date(row.expense_date), 'yyyy-MM-dd') },
  {
    title: t('common.status'), key: 'status', width: 100,
    render: (row) => h(NTag, { size: 'small', type: (statusColor[row.status] || 'default') as any }, () => t(`expense.${row.status}`)),
  },
  {
    title: t('common.actions'), key: 'actions', width: 100,
    render: (row) => {
      if (row.status === 'draft') {
        return h(NButton, { size: 'small', type: 'primary', onClick: () => submitClaim(row.id) }, () => t('expense.submit'))
      }
      return ''
    },
  },
])

const allColumns = computed<DataTableColumns<ExpenseClaim>>(() => [
  { title: t('expense.claimNumber'), key: 'claim_number', width: 120 },
  { title: t('expense.employee'), key: 'employee', render: (row) => `${row.first_name} ${row.last_name}` },
  { title: t('expense.category'), key: 'category_name' },
  { title: t('expense.amount'), key: 'amount', width: 120, render: (row) => `${row.currency} ${Number(row.amount).toLocaleString('en-PH', { minimumFractionDigits: 2 })}` },
  { title: t('expense.expenseDate'), key: 'expense_date', width: 110, render: (row) => format(new Date(row.expense_date), 'yyyy-MM-dd') },
  {
    title: t('common.status'), key: 'status', width: 100,
    render: (row) => h(NTag, { size: 'small', type: (statusColor[row.status] || 'default') as any }, () => t(`expense.${row.status}`)),
  },
  {
    title: t('common.actions'), key: 'actions', width: 200,
    render: (row) => {
      const btns: ReturnType<typeof h>[] = []
      if (row.status === 'submitted') {
        btns.push(h(NButton, { size: 'small', type: 'success', onClick: () => approveClaim(row.id) }, () => t('expense.approve')))
        btns.push(h(NButton, { size: 'small', type: 'error', onClick: () => { rejectClaimId.value = row.id; showRejectModal.value = true } }, () => t('expense.reject')))
      }
      if (row.status === 'approved' && isAdmin.value) {
        btns.push(h(NButton, { size: 'small', type: 'info', onClick: () => { payClaimId.value = row.id; showPayModal.value = true } }, () => t('expense.markPaid')))
      }
      return btns.length ? h(NSpace, { size: 4 }, () => btns) : ''
    },
  },
])

const catColumns = computed<DataTableColumns<ExpenseCategory>>(() => [
  { title: t('expense.categoryName'), key: 'name' },
  { title: t('expense.description'), key: 'description', render: (row) => row.description || '-' },
  { title: t('expense.maxAmount'), key: 'max_amount', render: (row) => row.max_amount ? `PHP ${Number(row.max_amount).toLocaleString('en-PH', { minimumFractionDigits: 2 })}` : '-' },
  {
    title: t('expense.requiresReceipt'), key: 'requires_receipt', width: 120,
    render: (row) => h(NTag, { size: 'small', type: row.requires_receipt ? 'warning' : 'default' }, () => row.requires_receipt ? t('common.yes') : t('common.no')),
  },
  {
    title: t('common.status'), key: 'is_active', width: 80,
    render: (row) => h(NTag, { size: 'small', type: row.is_active ? 'success' : 'default' }, () => row.is_active ? t('common.active') : t('common.inactive')),
  },
  {
    title: t('common.actions'), key: 'actions', width: 80,
    render: (row) => h(NButton, { size: 'small', onClick: () => openCatEdit(row) }, () => t('common.edit')),
  },
])

onMounted(fetchAll)
</script>

<template>
  <div>
    <h2 style="margin-bottom: 16px;">{{ t('expense.title') }}</h2>

    <!-- Summary Stats -->
    <NGrid v-if="isManager && summary" :cols="4" :x-gap="12" responsive="screen" style="margin-bottom: 16px;">
      <NGi>
        <NCard size="small">
          <NStatistic :label="t('expense.pending')" :value="summary.submitted_count" />
        </NCard>
      </NGi>
      <NGi>
        <NCard size="small">
          <NStatistic :label="t('expense.pendingAmount')" :value="`PHP ${Number(summary.pending_amount).toLocaleString('en-PH', { minimumFractionDigits: 2 })}`" />
        </NCard>
      </NGi>
      <NGi>
        <NCard size="small">
          <NStatistic :label="t('expense.approvedAmount')" :value="`PHP ${Number(summary.approved_amount).toLocaleString('en-PH', { minimumFractionDigits: 2 })}`" />
        </NCard>
      </NGi>
      <NGi>
        <NCard size="small">
          <NStatistic :label="t('expense.paidAmount')" :value="`PHP ${Number(summary.paid_amount).toLocaleString('en-PH', { minimumFractionDigits: 2 })}`" />
        </NCard>
      </NGi>
    </NGrid>

    <NTabs type="line">
      <!-- My Expenses -->
      <NTabPane :name="t('expense.myExpenses')" :tab="t('expense.myExpenses')">
        <NSpace style="margin-bottom: 12px;">
          <NButton type="primary" @click="openCreateClaim">{{ t('expense.newClaim') }}</NButton>
        </NSpace>
        <NDataTable :columns="myColumns" :data="myClaims" :loading="loading" :bordered="false" />
      </NTabPane>

      <!-- All Expenses (Manager+) -->
      <NTabPane v-if="isManager" :name="t('expense.allExpenses')" :tab="t('expense.allExpenses')">
        <NDataTable :columns="allColumns" :data="allClaims" :loading="loading" :bordered="false" />
      </NTabPane>

      <!-- Categories (Admin) -->
      <NTabPane v-if="isAdmin" :name="t('expense.categories')" :tab="t('expense.categories')">
        <NSpace style="margin-bottom: 12px;">
          <NButton type="primary" @click="openCatCreate">{{ t('expense.addCategory') }}</NButton>
        </NSpace>
        <NDataTable :columns="catColumns" :data="categories" :bordered="false" />
      </NTabPane>
    </NTabs>

    <!-- New Claim Modal -->
    <NModal v-model:show="showClaimModal" preset="card" :title="t('expense.newClaim')" style="max-width: 600px; width: 95vw;">
      <NForm label-placement="left" label-width="140">
        <NFormItem :label="t('expense.category')">
          <NSelect v-model:value="claimForm.category_id" :options="categoryOptions" :placeholder="t('expense.selectCategory')" />
        </NFormItem>
        <NFormItem :label="t('expense.description')">
          <NInput v-model:value="claimForm.description" type="textarea" :rows="3" />
        </NFormItem>
        <NFormItem :label="t('expense.amount')">
          <NInputNumber v-model:value="claimForm.amount" :min="0" :precision="2" style="width: 100%;">
            <template #prefix>PHP</template>
          </NInputNumber>
        </NFormItem>
        <NFormItem :label="t('expense.expenseDate')">
          <NDatePicker v-model:value="claimForm.expense_date" type="date" style="width: 100%;" />
        </NFormItem>
        <NFormItem :label="t('expense.notes')">
          <NInput v-model:value="claimForm.notes" />
        </NFormItem>
        <NFormItem :label="t('expense.submitImmediately')">
          <NSwitch v-model:value="claimForm.submit" />
        </NFormItem>
        <NFormItem>
          <NButton type="primary" @click="saveClaim">{{ t('common.save') }}</NButton>
        </NFormItem>
      </NForm>
    </NModal>

    <!-- Category Modal -->
    <NModal v-model:show="showCatModal" preset="card" :title="editingCat ? t('expense.editCategory') : t('expense.addCategory')" style="max-width: 500px; width: 95vw;">
      <NForm label-placement="left" label-width="140">
        <NFormItem :label="t('expense.categoryName')">
          <NInput v-model:value="catForm.name" />
        </NFormItem>
        <NFormItem :label="t('expense.description')">
          <NInput v-model:value="catForm.description" />
        </NFormItem>
        <NFormItem :label="t('expense.maxAmount')">
          <NInputNumber v-model:value="catForm.max_amount" :min="0" :precision="2" style="width: 100%;">
            <template #prefix>PHP</template>
          </NInputNumber>
        </NFormItem>
        <NFormItem :label="t('expense.requiresReceipt')">
          <NSwitch v-model:value="catForm.requires_receipt" />
        </NFormItem>
        <NFormItem v-if="editingCat" :label="t('common.active')">
          <NSwitch v-model:value="catForm.is_active" />
        </NFormItem>
        <NFormItem>
          <NButton type="primary" @click="saveCat">{{ t('common.save') }}</NButton>
        </NFormItem>
      </NForm>
    </NModal>

    <!-- Reject Modal -->
    <NModal v-model:show="showRejectModal" preset="card" :title="t('expense.rejectClaim')" style="max-width: 400px; width: 95vw;">
      <NForm label-placement="left" label-width="100">
        <NFormItem :label="t('expense.reason')">
          <NInput v-model:value="rejectReason" type="textarea" :rows="3" />
        </NFormItem>
        <NFormItem>
          <NButton type="error" @click="rejectClaim">{{ t('expense.reject') }}</NButton>
        </NFormItem>
      </NForm>
    </NModal>

    <!-- Pay Modal -->
    <NModal v-model:show="showPayModal" preset="card" :title="t('expense.markPaid')" style="max-width: 400px; width: 95vw;">
      <NForm label-placement="left" label-width="100">
        <NFormItem :label="t('expense.payReference')">
          <NInput v-model:value="payReference" />
        </NFormItem>
        <NFormItem>
          <NButton type="primary" @click="markPaid">{{ t('expense.markPaid') }}</NButton>
        </NFormItem>
      </NForm>
    </NModal>
  </div>
</template>
