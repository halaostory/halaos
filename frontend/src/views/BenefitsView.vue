<script setup lang="ts">
import { ref, h, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NTabs, NTabPane, NDataTable, NButton, NModal, NForm, NFormItem,
  NInput, NSelect, NInputNumber, NSpace, NTag, NStatistic, NGrid, NGi,
  NDatePicker, useMessage, type DataTableColumns,
} from 'naive-ui'
import { benefitAPI } from '../api/client'
import { useAuthStore } from '../stores/auth'
import { format } from 'date-fns'

const { t } = useI18n()
const message = useMessage()
const auth = useAuthStore()

interface Plan {
  id: number
  name: string
  category: string
  description: string | null
  provider: string | null
  employer_share: number | string
  employee_share: number | string
  coverage_amount: number | string | null
  eligibility_type: string
  eligibility_months: number
  is_active: boolean
}

interface Enrollment {
  id: number
  employee_id: number
  plan_id: number
  plan_name: string
  plan_category: string
  provider: string | null
  status: string
  enrollment_date: string
  effective_date: string
  end_date: string | null
  employer_share: number | string
  employee_share: number | string
  coverage_amount?: number | string | null
  employee_no?: string
  first_name?: string
  last_name?: string
  notes: string | null
}

interface Claim {
  id: number
  employee_id: number
  enrollment_id: number
  plan_name: string
  plan_category: string
  claim_date: string
  amount: number | string
  description: string
  status: string
  employee_no?: string
  first_name?: string
  last_name?: string
  rejection_reason?: string | null
}

interface Summary {
  total_plans: number
  enrolled_employees: number
  total_employer_cost: number | string
  total_employee_cost: number | string
}

// Data
const plans = ref<Plan[]>([])
const enrollments = ref<Enrollment[]>([])
const myEnrollments = ref<Enrollment[]>([])
const claims = ref<Claim[]>([])
const claimTotal = ref(0)
const summary = ref<Summary>({ total_plans: 0, enrolled_employees: 0, total_employer_cost: 0, total_employee_cost: 0 })
const loading = ref(false)

// Plan modal
const showPlanModal = ref(false)
const editingPlan = ref<Plan | null>(null)
const planForm = ref({
  name: '',
  category: 'medical',
  description: '',
  provider: '',
  employer_share: 0,
  employee_share: 0,
  coverage_amount: 0,
  eligibility_type: 'all',
  eligibility_months: 0,
  is_active: true,
})

// Enrollment modal
const showEnrollModal = ref(false)
const enrollForm = ref({
  employee_id: null as number | null,
  plan_id: null as number | null,
  effective_date: null as number | null,
  employer_share: 0,
  employee_share: 0,
  notes: '',
})

// Claim modal
const showClaimModal = ref(false)
const claimForm = ref({
  enrollment_id: null as number | null,
  claim_date: Date.now(),
  amount: 0,
  description: '',
})

// Reject modal
const showRejectModal = ref(false)
const rejectClaimId = ref(0)
const rejectReason = ref('')

const isAdmin = computed(() => auth.isAdmin)
const isManager = computed(() => auth.isAdmin || auth.isManager)

const categoryOptions = [
  { label: t('benefit.medical'), value: 'medical' },
  { label: t('benefit.dental'), value: 'dental' },
  { label: t('benefit.lifeInsurance'), value: 'life_insurance' },
  { label: t('benefit.retirement'), value: 'retirement' },
  { label: t('benefit.allowance'), value: 'allowance' },
  { label: t('benefit.other'), value: 'other' },
]

const eligibilityOptions = [
  { label: t('benefit.eligAll'), value: 'all' },
  { label: t('benefit.eligRegular'), value: 'regular' },
  { label: t('benefit.eligAfterProbation'), value: 'after_probation' },
  { label: t('benefit.eligByGrade'), value: 'by_grade' },
]

const categoryColor: Record<string, string> = {
  medical: 'success',
  dental: 'info',
  life_insurance: 'warning',
  retirement: 'default',
  allowance: 'success',
  other: 'default',
}

const statusColor: Record<string, string> = {
  active: 'success',
  pending: 'warning',
  cancelled: 'error',
  expired: 'default',
  approved: 'success',
  rejected: 'error',
  paid: 'info',
}

function num(v: number | string | null | undefined): number {
  if (v == null) return 0
  return typeof v === 'number' ? v : parseFloat(v) || 0
}

// Plans table
const planColumns = computed<DataTableColumns<Plan>>(() => [
  { title: t('benefit.planName'), key: 'name' },
  {
    title: t('benefit.category'), key: 'category',
    render: (row) => h(NTag, { size: 'small', type: (categoryColor[row.category] || 'default') as any }, () => t(`benefit.${row.category}`) || row.category),
  },
  { title: t('benefit.provider'), key: 'provider', render: (row) => row.provider || '-' },
  { title: t('benefit.employerShare'), key: 'employer_share', render: (row) => `₱${num(row.employer_share).toLocaleString()}` },
  { title: t('benefit.employeeShare'), key: 'employee_share', render: (row) => `₱${num(row.employee_share).toLocaleString()}` },
  { title: t('benefit.coverageAmount'), key: 'coverage_amount', render: (row) => row.coverage_amount ? `₱${num(row.coverage_amount).toLocaleString()}` : '-' },
  { title: t('benefit.eligibility'), key: 'eligibility_type', render: (row) => t(`benefit.elig${row.eligibility_type.charAt(0).toUpperCase() + row.eligibility_type.slice(1).replace(/_([a-z])/g, (_, c) => c.toUpperCase())}`) || row.eligibility_type },
  ...(isAdmin.value ? [{
    title: t('common.actions'), key: 'actions',
    render: (row: Plan) => h(NButton, { size: 'small', onClick: () => openEditPlan(row) }, () => t('common.edit')),
  }] : []),
])

// Enrollments table
const enrollmentColumns = computed<DataTableColumns<Enrollment>>(() => [
  { title: t('benefit.employee'), key: 'employee', render: (row) => row.employee_no ? `${row.first_name} ${row.last_name} (${row.employee_no})` : '-' },
  { title: t('benefit.planName'), key: 'plan_name' },
  {
    title: t('benefit.category'), key: 'plan_category',
    render: (row) => h(NTag, { size: 'small', type: (categoryColor[row.plan_category] || 'default') as any }, () => t(`benefit.${row.plan_category}`) || row.plan_category),
  },
  {
    title: t('common.status'), key: 'status',
    render: (row) => h(NTag, { size: 'small', type: (statusColor[row.status] || 'default') as any }, () => row.status),
  },
  { title: t('benefit.effectiveDate'), key: 'effective_date', render: (row) => format(new Date(row.effective_date), 'yyyy-MM-dd') },
  { title: t('benefit.employerShare'), key: 'employer_share', render: (row) => `₱${num(row.employer_share).toLocaleString()}` },
  { title: t('benefit.employeeShare'), key: 'employee_share', render: (row) => `₱${num(row.employee_share).toLocaleString()}` },
  ...(isManager.value ? [{
    title: t('common.actions'), key: 'actions',
    render: (row: Enrollment) => {
      const btns: ReturnType<typeof h>[] = []
      if (row.status === 'active') {
        btns.push(h(NButton, { size: 'small', type: 'error', onClick: () => cancelEnrollment(row.id) }, () => t('benefit.cancel')))
      }
      return h(NSpace, { size: 4 }, () => btns)
    },
  }] : []),
])

// My enrollments table
const myEnrollmentColumns = computed<DataTableColumns<Enrollment>>(() => [
  { title: t('benefit.planName'), key: 'plan_name' },
  {
    title: t('benefit.category'), key: 'plan_category',
    render: (row) => h(NTag, { size: 'small', type: (categoryColor[row.plan_category] || 'default') as any }, () => t(`benefit.${row.plan_category}`) || row.plan_category),
  },
  { title: t('benefit.provider'), key: 'provider', render: (row) => row.provider || '-' },
  {
    title: t('common.status'), key: 'status',
    render: (row) => h(NTag, { size: 'small', type: (statusColor[row.status] || 'default') as any }, () => row.status),
  },
  { title: t('benefit.effectiveDate'), key: 'effective_date', render: (row) => format(new Date(row.effective_date), 'yyyy-MM-dd') },
  { title: t('benefit.employerShare'), key: 'employer_share', render: (row) => `₱${num(row.employer_share).toLocaleString()}` },
  { title: t('benefit.employeeShare'), key: 'employee_share', render: (row) => `₱${num(row.employee_share).toLocaleString()}` },
  { title: t('benefit.coverageAmount'), key: 'coverage_amount', render: (row) => row.coverage_amount ? `₱${num(row.coverage_amount).toLocaleString()}` : '-' },
])

// Claims table
const claimColumns = computed<DataTableColumns<Claim>>(() => [
  { title: t('benefit.employee'), key: 'employee', render: (row) => row.employee_no ? `${row.first_name} ${row.last_name}` : '-' },
  { title: t('benefit.planName'), key: 'plan_name' },
  { title: t('benefit.claimDate'), key: 'claim_date', render: (row) => format(new Date(row.claim_date), 'yyyy-MM-dd') },
  { title: t('benefit.amount'), key: 'amount', render: (row) => `₱${num(row.amount).toLocaleString()}` },
  { title: t('benefit.description'), key: 'description', ellipsis: { tooltip: true } },
  {
    title: t('common.status'), key: 'status',
    render: (row) => h(NTag, { size: 'small', type: (statusColor[row.status] || 'default') as any }, () => row.status),
  },
  ...(isManager.value ? [{
    title: t('common.actions'), key: 'actions',
    render: (row: Claim) => {
      if (row.status !== 'pending') return '-'
      return h(NSpace, { size: 4 }, () => [
        h(NButton, { size: 'small', type: 'success', onClick: () => approveClaim(row.id) }, () => t('benefit.approve')),
        h(NButton, { size: 'small', type: 'error', onClick: () => openRejectClaim(row.id) }, () => t('benefit.reject')),
      ])
    },
  }] : []),
])

// Plan enrollment options for dropdown
const planOptions = computed(() => plans.value.map(p => ({ label: `${p.name} (${p.category})`, value: p.id })))
const enrollmentOptions = computed(() => myEnrollments.value.filter(e => e.status === 'active').map(e => ({ label: `${e.plan_name} (${e.plan_category})`, value: e.id })))

async function fetchAll() {
  loading.value = true
  try {
    const [plansRes, myRes] = await Promise.all([
      benefitAPI.listPlans(),
      benefitAPI.myEnrollments(),
    ])
    plans.value = extractData(plansRes)
    myEnrollments.value = extractData(myRes)

    if (isManager.value) {
      const [enrollRes, claimsRes, summaryRes] = await Promise.all([
        benefitAPI.listEnrollments(),
        benefitAPI.listClaims(),
        benefitAPI.summary(),
      ])
      enrollments.value = extractData(enrollRes)
      const claimData = (claimsRes as any)?.data ?? claimsRes
      claims.value = claimData?.items ?? []
      claimTotal.value = claimData?.total ?? 0
      summary.value = (summaryRes as any)?.data ?? summaryRes
    }
  } catch {
    message.error(t('benefit.loadFailed'))
  } finally {
    loading.value = false
  }
}

function extractData(res: unknown): any[] {
  const d = (res as any)?.data ?? res
  return Array.isArray(d) ? d : []
}

function openCreatePlan() {
  editingPlan.value = null
  planForm.value = { name: '', category: 'medical', description: '', provider: '', employer_share: 0, employee_share: 0, coverage_amount: 0, eligibility_type: 'all', eligibility_months: 0, is_active: true }
  showPlanModal.value = true
}

function openEditPlan(plan: Plan) {
  editingPlan.value = plan
  planForm.value = {
    name: plan.name,
    category: plan.category,
    description: plan.description || '',
    provider: plan.provider || '',
    employer_share: num(plan.employer_share),
    employee_share: num(plan.employee_share),
    coverage_amount: num(plan.coverage_amount),
    eligibility_type: plan.eligibility_type,
    eligibility_months: plan.eligibility_months,
    is_active: plan.is_active,
  }
  showPlanModal.value = true
}

async function savePlan() {
  try {
    if (editingPlan.value) {
      await benefitAPI.updatePlan(editingPlan.value.id, planForm.value)
      message.success(t('common.updated'))
    } else {
      await benefitAPI.createPlan(planForm.value)
      message.success(t('common.created'))
    }
    showPlanModal.value = false
    fetchAll()
  } catch {
    message.error(t('common.error'))
  }
}

async function cancelEnrollment(id: number) {
  try {
    await benefitAPI.cancelEnrollment(id)
    message.success(t('benefit.enrollmentCancelled'))
    fetchAll()
  } catch {
    message.error(t('common.error'))
  }
}

async function submitEnrollment() {
  if (!enrollForm.value.employee_id || !enrollForm.value.plan_id || !enrollForm.value.effective_date) {
    message.warning(t('common.fillRequired'))
    return
  }
  try {
    await benefitAPI.createEnrollment({
      employee_id: enrollForm.value.employee_id,
      plan_id: enrollForm.value.plan_id,
      effective_date: format(new Date(enrollForm.value.effective_date), 'yyyy-MM-dd'),
      employer_share: enrollForm.value.employer_share,
      employee_share: enrollForm.value.employee_share,
      notes: enrollForm.value.notes,
    })
    message.success(t('common.created'))
    showEnrollModal.value = false
    fetchAll()
  } catch {
    message.error(t('common.error'))
  }
}

async function submitClaim() {
  if (!claimForm.value.enrollment_id || !claimForm.value.amount) {
    message.warning(t('common.fillRequired'))
    return
  }
  try {
    await benefitAPI.createClaim({
      enrollment_id: claimForm.value.enrollment_id,
      claim_date: format(new Date(claimForm.value.claim_date), 'yyyy-MM-dd'),
      amount: claimForm.value.amount,
      description: claimForm.value.description,
    })
    message.success(t('common.created'))
    showClaimModal.value = false
    fetchAll()
  } catch {
    message.error(t('common.error'))
  }
}

async function approveClaim(id: number) {
  try {
    await benefitAPI.approveClaim(id)
    message.success(t('benefit.claimApproved'))
    fetchAll()
  } catch {
    message.error(t('common.error'))
  }
}

function openRejectClaim(id: number) {
  rejectClaimId.value = id
  rejectReason.value = ''
  showRejectModal.value = true
}

async function submitRejectClaim() {
  try {
    await benefitAPI.rejectClaim(rejectClaimId.value, rejectReason.value)
    message.success(t('benefit.claimRejected'))
    showRejectModal.value = false
    fetchAll()
  } catch {
    message.error(t('common.error'))
  }
}

onMounted(fetchAll)
</script>

<template>
  <div>
    <h2 style="margin-bottom: 16px;">{{ t('benefit.title') }}</h2>

    <!-- Summary Stats (Manager+) -->
    <NGrid v-if="isManager" :cols="4" :x-gap="16" :y-gap="16" responsive="screen" style="margin-bottom: 24px;">
      <NGi>
        <NStatistic :label="t('benefit.activePlans')" :value="summary.total_plans" />
      </NGi>
      <NGi>
        <NStatistic :label="t('benefit.enrolledEmployees')" :value="summary.enrolled_employees" />
      </NGi>
      <NGi>
        <NStatistic :label="t('benefit.totalEmployerCost')" :value="`₱${num(summary.total_employer_cost).toLocaleString()}`" />
      </NGi>
      <NGi>
        <NStatistic :label="t('benefit.totalEmployeeCost')" :value="`₱${num(summary.total_employee_cost).toLocaleString()}`" />
      </NGi>
    </NGrid>

    <NTabs type="line">
      <!-- My Benefits Tab -->
      <NTabPane :name="t('benefit.myBenefits')" :tab="t('benefit.myBenefits')">
        <NSpace style="margin-bottom: 12px;">
          <NButton type="primary" @click="showClaimModal = true">{{ t('benefit.fileClaim') }}</NButton>
        </NSpace>
        <NDataTable :columns="myEnrollmentColumns" :data="myEnrollments" :loading="loading" :bordered="false" />
      </NTabPane>

      <!-- Plans Tab -->
      <NTabPane :name="t('benefit.plans')" :tab="t('benefit.plans')">
        <NSpace v-if="isAdmin" style="margin-bottom: 12px;">
          <NButton type="primary" @click="openCreatePlan">{{ t('benefit.createPlan') }}</NButton>
        </NSpace>
        <NDataTable :columns="planColumns" :data="plans" :loading="loading" :bordered="false" />
      </NTabPane>

      <!-- Enrollments Tab (Manager+) -->
      <NTabPane v-if="isManager" :name="t('benefit.enrollments')" :tab="t('benefit.enrollments')">
        <NSpace style="margin-bottom: 12px;">
          <NButton type="primary" @click="showEnrollModal = true">{{ t('benefit.enrollEmployee') }}</NButton>
        </NSpace>
        <NDataTable :columns="enrollmentColumns" :data="enrollments" :loading="loading" :bordered="false" />
      </NTabPane>

      <!-- Claims Tab (Manager+) -->
      <NTabPane v-if="isManager" :name="t('benefit.claims')" :tab="t('benefit.claims')">
        <NDataTable :columns="claimColumns" :data="claims" :loading="loading" :bordered="false" />
      </NTabPane>
    </NTabs>

    <!-- Plan Modal -->
    <NModal v-model:show="showPlanModal" preset="card" :title="editingPlan ? t('benefit.editPlan') : t('benefit.createPlan')" style="max-width: 600px; width: 95vw;">
      <NForm label-placement="left" label-width="140">
        <NFormItem :label="t('benefit.planName')">
          <NInput v-model:value="planForm.name" />
        </NFormItem>
        <NFormItem :label="t('benefit.category')">
          <NSelect v-model:value="planForm.category" :options="categoryOptions" />
        </NFormItem>
        <NFormItem :label="t('benefit.description')">
          <NInput v-model:value="planForm.description" type="textarea" />
        </NFormItem>
        <NFormItem :label="t('benefit.provider')">
          <NInput v-model:value="planForm.provider" />
        </NFormItem>
        <NFormItem :label="t('benefit.employerShare')">
          <NInputNumber v-model:value="planForm.employer_share" :min="0" :precision="2" style="width: 100%;" />
        </NFormItem>
        <NFormItem :label="t('benefit.employeeShare')">
          <NInputNumber v-model:value="planForm.employee_share" :min="0" :precision="2" style="width: 100%;" />
        </NFormItem>
        <NFormItem :label="t('benefit.coverageAmount')">
          <NInputNumber v-model:value="planForm.coverage_amount" :min="0" :precision="2" style="width: 100%;" />
        </NFormItem>
        <NFormItem :label="t('benefit.eligibility')">
          <NSelect v-model:value="planForm.eligibility_type" :options="eligibilityOptions" />
        </NFormItem>
        <NFormItem :label="t('benefit.eligibilityMonths')">
          <NInputNumber v-model:value="planForm.eligibility_months" :min="0" style="width: 100%;" />
        </NFormItem>
        <NFormItem>
          <NButton type="primary" @click="savePlan">{{ t('common.save') }}</NButton>
        </NFormItem>
      </NForm>
    </NModal>

    <!-- Enrollment Modal -->
    <NModal v-model:show="showEnrollModal" preset="card" :title="t('benefit.enrollEmployee')" style="max-width: 500px; width: 95vw;">
      <NForm label-placement="left" label-width="140">
        <NFormItem :label="t('benefit.employee')">
          <NInputNumber v-model:value="enrollForm.employee_id" :min="1" placeholder="Employee ID" style="width: 100%;" />
        </NFormItem>
        <NFormItem :label="t('benefit.plan')">
          <NSelect v-model:value="enrollForm.plan_id" :options="planOptions" />
        </NFormItem>
        <NFormItem :label="t('benefit.effectiveDate')">
          <NDatePicker v-model:value="enrollForm.effective_date" type="date" style="width: 100%;" />
        </NFormItem>
        <NFormItem :label="t('benefit.employerShare')">
          <NInputNumber v-model:value="enrollForm.employer_share" :min="0" :precision="2" style="width: 100%;" />
        </NFormItem>
        <NFormItem :label="t('benefit.employeeShare')">
          <NInputNumber v-model:value="enrollForm.employee_share" :min="0" :precision="2" style="width: 100%;" />
        </NFormItem>
        <NFormItem :label="t('benefit.notes')">
          <NInput v-model:value="enrollForm.notes" type="textarea" />
        </NFormItem>
        <NFormItem>
          <NButton type="primary" @click="submitEnrollment">{{ t('benefit.enroll') }}</NButton>
        </NFormItem>
      </NForm>
    </NModal>

    <!-- Claim Modal -->
    <NModal v-model:show="showClaimModal" preset="card" :title="t('benefit.fileClaim')" style="max-width: 500px; width: 95vw;">
      <NForm label-placement="left" label-width="140">
        <NFormItem :label="t('benefit.plan')">
          <NSelect v-model:value="claimForm.enrollment_id" :options="enrollmentOptions" />
        </NFormItem>
        <NFormItem :label="t('benefit.claimDate')">
          <NDatePicker v-model:value="claimForm.claim_date" type="date" style="width: 100%;" />
        </NFormItem>
        <NFormItem :label="t('benefit.amount')">
          <NInputNumber v-model:value="claimForm.amount" :min="0" :precision="2" style="width: 100%;" />
        </NFormItem>
        <NFormItem :label="t('benefit.description')">
          <NInput v-model:value="claimForm.description" type="textarea" />
        </NFormItem>
        <NFormItem>
          <NButton type="primary" @click="submitClaim">{{ t('benefit.submit') }}</NButton>
        </NFormItem>
      </NForm>
    </NModal>

    <!-- Reject Claim Modal -->
    <NModal v-model:show="showRejectModal" preset="card" :title="t('benefit.rejectClaim')" style="max-width: 400px; width: 95vw;">
      <NForm label-placement="left" label-width="100">
        <NFormItem :label="t('benefit.reason')">
          <NInput v-model:value="rejectReason" type="textarea" />
        </NFormItem>
        <NFormItem>
          <NButton type="error" @click="submitRejectClaim">{{ t('benefit.reject') }}</NButton>
        </NFormItem>
      </NForm>
    </NModal>
  </div>
</template>
