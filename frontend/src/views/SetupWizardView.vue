<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  NCard, NSteps, NStep, NButton, NSpace, NForm, NFormItem,
  NInput, NSelect, NGrid, NGi, NTag, NEmpty, useMessage,
} from 'naive-ui'
import { companyAPI, employeeAPI } from '../api/client'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const { t } = useI18n()
const message = useMessage()
const auth = useAuthStore()

const currentStep = ref(1)
const loading = ref(false)

const windowWidth = ref(window.innerWidth)
const isMobile = computed(() => windowWidth.value < 640)

function handleResize() {
  windowWidth.value = window.innerWidth
}

// Step 1: Company Info
const companyForm = ref({
  legal_name: '',
  tin: '',
  rdo_code: '',
  address: '',
  city: '',
  province: '',
  zip_code: '',
  sss_er_no: '',
  philhealth_er_no: '',
  pagibig_er_no: '',
})

// Step 2: Departments
interface Department { id: number; code: string; name: string }
const departments = ref<Department[]>([])
const newDept = ref({ code: '', name: '' })

// Step 3: Positions
interface Position { id: number; code: string; title: string; department_id?: number }
const positions = ref<Position[]>([])
const newPos = ref({ code: '', title: '', department_id: undefined as number | undefined })

// Step 4: First Employee
const employeeForm = ref({
  employee_no: 'EMP-001',
  first_name: '',
  last_name: '',
  email: '',
  department_id: undefined as number | undefined,
  position_id: undefined as number | undefined,
  employment_type: 'regular',
  hire_date: new Date().toISOString().split('T')[0],
})

const isPH = computed(() => auth.user?.company_country === 'PHL' || !auth.user?.company_country)

const deptOptions = computed(() =>
  departments.value.map(d => ({ label: d.name, value: d.id }))
)
const posOptions = computed(() =>
  positions.value.map(p => ({ label: p.title, value: p.id }))
)

const employmentTypeOptions = [
  { label: 'Regular', value: 'regular' },
  { label: 'Probationary', value: 'probationary' },
  { label: 'Contractual', value: 'contractual' },
  { label: 'Part-time', value: 'part_time' },
]

onMounted(async () => {
  window.addEventListener('resize', handleResize)
  // Load existing company info
  try {
    const res = await companyAPI.getSettings()
    const data = (res as any)?.data ?? res
    if (data) {
      companyForm.value.legal_name = data.legal_name || ''
      companyForm.value.tin = data.tin || ''
      companyForm.value.rdo_code = data.rdo_code || ''
      companyForm.value.address = data.address || ''
      companyForm.value.city = data.city || ''
      companyForm.value.province = data.province || ''
      companyForm.value.zip_code = data.zip_code || ''
      companyForm.value.sss_er_no = data.sss_er_no || ''
      companyForm.value.philhealth_er_no = data.philhealth_er_no || ''
      companyForm.value.pagibig_er_no = data.pagibig_er_no || ''
    }
  } catch { /* fresh company */ }

  // Load existing departments
  try {
    const res = await companyAPI.listDepartments()
    const data = (res as any)?.data ?? res
    departments.value = Array.isArray(data) ? data : []
  } catch { /* none yet */ }

  // Load existing positions
  try {
    const res = await companyAPI.listPositions()
    const data = (res as any)?.data ?? res
    positions.value = Array.isArray(data) ? data : []
  } catch { /* none yet */ }
})

onUnmounted(() => window.removeEventListener('resize', handleResize))

async function saveCompanyInfo() {
  loading.value = true
  try {
    await companyAPI.updateSettings(companyForm.value)
    message.success('Company info saved')
    currentStep.value = 2
  } catch {
    message.error('Failed to save company info')
  } finally {
    loading.value = false
  }
}

async function addDepartment() {
  if (!newDept.value.code || !newDept.value.name) {
    message.warning('Please enter department code and name')
    return
  }
  loading.value = true
  try {
    const res = await companyAPI.createDepartment({
      code: newDept.value.code,
      name: newDept.value.name,
    })
    const data = (res as any)?.data ?? res
    departments.value.push(data)
    newDept.value = { code: '', name: '' }
    message.success('Department added')
  } catch {
    message.error('Failed to add department')
  } finally {
    loading.value = false
  }
}

async function addPosition() {
  if (!newPos.value.code || !newPos.value.title) {
    message.warning('Please enter position code and title')
    return
  }
  loading.value = true
  try {
    const res = await companyAPI.createPosition({
      code: newPos.value.code,
      title: newPos.value.title,
      department_id: newPos.value.department_id,
    })
    const data = (res as any)?.data ?? res
    positions.value.push(data)
    newPos.value = { code: '', title: '', department_id: undefined }
    message.success('Position added')
  } catch {
    message.error('Failed to add position')
  } finally {
    loading.value = false
  }
}

async function addEmployee() {
  if (!employeeForm.value.first_name || !employeeForm.value.last_name) {
    message.warning('Please enter employee name')
    return
  }
  loading.value = true
  try {
    await employeeAPI.create(employeeForm.value)
    message.success('Employee added!')
    completeSetup()
  } catch {
    message.error('Failed to add employee')
  } finally {
    loading.value = false
  }
}

function completeSetup() {
  localStorage.setItem('halaos_setup_done', 'true')
  currentStep.value = 5
}

function skipSetup() {
  localStorage.setItem('halaos_setup_done', 'true')
  router.push('/dashboard')
}

function goToDashboard() {
  router.push('/dashboard')
}
</script>

<template>
  <div class="setup-wizard">
    <div class="wizard-header">
      <h1>Welcome to HalaOS</h1>
      <p>Let's set up your company in a few quick steps. You can always update these later in Settings.</p>
      <NButton text type="primary" @click="skipSetup" style="margin-top: 8px;">
        Skip setup and go to dashboard
      </NButton>
    </div>

    <NSteps v-if="!isMobile" :current="currentStep" style="margin-bottom: 32px;">
      <NStep title="Company Info" />
      <NStep title="Departments" />
      <NStep title="Positions" />
      <NStep title="First Employee" />
    </NSteps>
    <div v-else class="mobile-step-indicator">
      Step {{ currentStep }} of 4
    </div>

    <!-- Step 1: Company Info -->
    <NCard v-if="currentStep === 1" title="Company Information">
      <template #header-extra>
        <NTag type="info" size="small">Step 1 of 4</NTag>
      </template>
      <NForm label-placement="top">
        <NGrid cols="1 s:2" :x-gap="16" :y-gap="0" responsive="screen">
          <NGi>
            <NFormItem label="Legal Name">
              <NInput v-model:value="companyForm.legal_name" placeholder="e.g. ABC Corporation" />
            </NFormItem>
          </NGi>
          <NGi>
            <NFormItem label="TIN">
              <NInput v-model:value="companyForm.tin" placeholder="e.g. 123-456-789-000" />
            </NFormItem>
          </NGi>
          <NGi v-if="isPH">
            <NFormItem label="BIR RDO Code">
              <NInput v-model:value="companyForm.rdo_code" placeholder="e.g. 044" />
            </NFormItem>
          </NGi>
          <NGi>
            <NFormItem label="Address">
              <NInput v-model:value="companyForm.address" placeholder="Street address" />
            </NFormItem>
          </NGi>
          <NGi>
            <NFormItem label="City">
              <NInput v-model:value="companyForm.city" placeholder="City" />
            </NFormItem>
          </NGi>
          <NGi>
            <NFormItem label="Province / State">
              <NInput v-model:value="companyForm.province" placeholder="Province" />
            </NFormItem>
          </NGi>
          <NGi>
            <NFormItem label="Zip Code">
              <NInput v-model:value="companyForm.zip_code" placeholder="Zip code" />
            </NFormItem>
          </NGi>
        </NGrid>

        <template v-if="isPH">
          <h4 style="margin: 16px 0 12px; font-size: 14px; color: #666;">Government Registration Numbers</h4>
          <NGrid cols="1 s:2 m:3" :x-gap="16" :y-gap="0" responsive="screen">
            <NGi>
              <NFormItem label="SSS ER No.">
                <NInput v-model:value="companyForm.sss_er_no" placeholder="SSS employer number" />
              </NFormItem>
            </NGi>
            <NGi>
              <NFormItem label="PhilHealth ER No.">
                <NInput v-model:value="companyForm.philhealth_er_no" placeholder="PhilHealth employer number" />
              </NFormItem>
            </NGi>
            <NGi>
              <NFormItem label="Pag-IBIG ER No.">
                <NInput v-model:value="companyForm.pagibig_er_no" placeholder="Pag-IBIG employer number" />
              </NFormItem>
            </NGi>
          </NGrid>
        </template>
      </NForm>
      <template #action>
        <NSpace justify="end">
          <NButton @click="currentStep = 2">Skip</NButton>
          <NButton type="primary" :loading="loading" @click="saveCompanyInfo">
            Save & Continue
          </NButton>
        </NSpace>
      </template>
    </NCard>

    <!-- Step 2: Departments -->
    <NCard v-if="currentStep === 2" title="Create Departments">
      <template #header-extra>
        <NTag type="info" size="small">Step 2 of 4</NTag>
      </template>
      <p style="margin-bottom: 16px; color: #666;">
        Add the departments in your organization. Common examples: Engineering, Sales, HR, Finance, Operations.
      </p>

      <NGrid cols="1 s:2 m:3" :x-gap="12" responsive="screen" style="margin-bottom: 16px;">
        <NGi>
          <NInput v-model:value="newDept.code" placeholder="Code (e.g. ENG)" />
        </NGi>
        <NGi>
          <NInput v-model:value="newDept.name" placeholder="Name (e.g. Engineering)" @keyup.enter="addDepartment" />
        </NGi>
        <NGi>
          <NButton type="primary" :loading="loading" @click="addDepartment" block>
            Add Department
          </NButton>
        </NGi>
      </NGrid>

      <div v-if="departments.length > 0" style="display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 16px;">
        <NTag v-for="d in departments" :key="d.id" type="success" size="medium">
          {{ d.code }} - {{ d.name }}
        </NTag>
      </div>
      <NEmpty v-else description="No departments yet. Add at least one to organize your team." />

      <template #action>
        <NSpace justify="space-between">
          <NButton @click="currentStep = 1">Back</NButton>
          <NSpace>
            <NButton @click="currentStep = 3">Skip</NButton>
            <NButton type="primary" :disabled="departments.length === 0" @click="currentStep = 3">
              Continue
            </NButton>
          </NSpace>
        </NSpace>
      </template>
    </NCard>

    <!-- Step 3: Positions -->
    <NCard v-if="currentStep === 3" title="Create Positions">
      <template #header-extra>
        <NTag type="info" size="small">Step 3 of 4</NTag>
      </template>
      <p style="margin-bottom: 16px; color: #666;">
        Add job positions/titles. Examples: Software Engineer, Accountant, Sales Manager, HR Officer.
      </p>

      <NGrid cols="1 s:2 m:3 l:4" :x-gap="12" responsive="screen" style="margin-bottom: 16px;">
        <NGi>
          <NInput v-model:value="newPos.code" placeholder="Code (e.g. SE)" />
        </NGi>
        <NGi>
          <NInput v-model:value="newPos.title" placeholder="Title (e.g. Software Engineer)" @keyup.enter="addPosition" />
        </NGi>
        <NGi>
          <NSelect
            v-model:value="newPos.department_id"
            :options="deptOptions"
            placeholder="Department (optional)"
            clearable
          />
        </NGi>
        <NGi>
          <NButton type="primary" :loading="loading" @click="addPosition" block>
            Add Position
          </NButton>
        </NGi>
      </NGrid>

      <div v-if="positions.length > 0" style="display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 16px;">
        <NTag v-for="p in positions" :key="p.id" type="success" size="medium">
          {{ p.code }} - {{ p.title }}
        </NTag>
      </div>
      <NEmpty v-else description="No positions yet. Add at least one for employee assignment." />

      <template #action>
        <NSpace justify="space-between">
          <NButton @click="currentStep = 2">Back</NButton>
          <NSpace>
            <NButton @click="currentStep = 4">Skip</NButton>
            <NButton type="primary" :disabled="positions.length === 0" @click="currentStep = 4">
              Continue
            </NButton>
          </NSpace>
        </NSpace>
      </template>
    </NCard>

    <!-- Step 4: First Employee -->
    <NCard v-if="currentStep === 4" title="Add Your First Employee">
      <template #header-extra>
        <NTag type="info" size="small">Step 4 of 4</NTag>
      </template>
      <p style="margin-bottom: 16px; color: #666;">
        Add your first employee to get started. You can add more employees later from the People section.
      </p>

      <NForm label-placement="top">
        <NGrid cols="1 s:2" :x-gap="16" :y-gap="0" responsive="screen">
          <NGi>
            <NFormItem label="Employee No.">
              <NInput v-model:value="employeeForm.employee_no" placeholder="EMP-001" />
            </NFormItem>
          </NGi>
          <NGi>
            <NFormItem label="Employment Type">
              <NSelect v-model:value="employeeForm.employment_type" :options="employmentTypeOptions" />
            </NFormItem>
          </NGi>
          <NGi>
            <NFormItem label="First Name" required>
              <NInput v-model:value="employeeForm.first_name" placeholder="First name" />
            </NFormItem>
          </NGi>
          <NGi>
            <NFormItem label="Last Name" required>
              <NInput v-model:value="employeeForm.last_name" placeholder="Last name" />
            </NFormItem>
          </NGi>
          <NGi>
            <NFormItem label="Email">
              <NInput v-model:value="employeeForm.email" placeholder="employee@company.com" />
            </NFormItem>
          </NGi>
          <NGi>
            <NFormItem label="Hire Date">
              <NInput v-model:value="employeeForm.hire_date" type="date" />
            </NFormItem>
          </NGi>
          <NGi>
            <NFormItem label="Department">
              <NSelect v-model:value="employeeForm.department_id" :options="deptOptions" placeholder="Select" clearable />
            </NFormItem>
          </NGi>
          <NGi>
            <NFormItem label="Position">
              <NSelect v-model:value="employeeForm.position_id" :options="posOptions" placeholder="Select" clearable />
            </NFormItem>
          </NGi>
        </NGrid>
      </NForm>

      <template #action>
        <NSpace justify="space-between">
          <NButton @click="currentStep = 3">Back</NButton>
          <NSpace>
            <NButton @click="completeSetup">Skip</NButton>
            <NButton type="primary" :loading="loading" @click="addEmployee">
              Add Employee & Finish
            </NButton>
          </NSpace>
        </NSpace>
      </template>
    </NCard>

    <!-- Step 5: Done -->
    <NCard v-if="currentStep === 5" class="done-card">
      <div style="text-align: center; padding: 40px 20px;">
        <div style="font-size: 48px; margin-bottom: 16px;">&#127881;</div>
        <h2 style="margin-bottom: 8px;">You're All Set!</h2>
        <p style="color: #666; margin-bottom: 32px; max-width: 480px; margin-left: auto; margin-right: auto;">
          Your company is configured and ready to go. Explore the features below to get the most out of HalaOS.
        </p>

        <NGrid cols="1 s:2 m:3" :x-gap="16" :y-gap="16" responsive="screen" style="max-width: 600px; margin: 0 auto 32px;">
          <NGi>
            <div class="explore-card" @click="router.push('/dashboard/employees')">
              <div style="font-size: 24px; margin-bottom: 8px;">&#128101;</div>
              <div style="font-weight: 600;">Add Employees</div>
              <div style="font-size: 12px; color: #999;">Import or add your team</div>
            </div>
          </NGi>
          <NGi>
            <div class="explore-card" @click="router.push('/dashboard/payroll')">
              <div style="font-size: 24px; margin-bottom: 8px;">&#128176;</div>
              <div style="font-weight: 600;">Run Payroll</div>
              <div style="font-size: 12px; color: #999;">Create your first payroll cycle</div>
            </div>
          </NGi>
          <NGi>
            <div class="explore-card" @click="router.push('/dashboard/settings')">
              <div style="font-size: 24px; margin-bottom: 8px;">&#9881;&#65039;</div>
              <div style="font-weight: 600;">Settings</div>
              <div style="font-size: 12px; color: #999;">Fine-tune your company setup</div>
            </div>
          </NGi>
        </NGrid>

        <NButton type="primary" size="large" @click="goToDashboard">
          Go to Dashboard
        </NButton>
      </div>
    </NCard>
  </div>
</template>

<style scoped>
.setup-wizard {
  max-width: 800px;
  margin: 0 auto;
  padding: 24px;
}
.wizard-header {
  text-align: center;
  margin-bottom: 32px;
}
.wizard-header h1 {
  font-size: 28px;
  font-weight: 700;
  margin-bottom: 8px;
}
.wizard-header p {
  color: #666;
  font-size: 15px;
}
.explore-card {
  padding: 20px 12px;
  border-radius: 12px;
  border: 1px solid #e5e7eb;
  text-align: center;
  cursor: pointer;
  transition: all 0.2s;
}
.explore-card:hover {
  border-color: #4f46e5;
  background: #f5f3ff;
}
.done-card {
  border: 2px solid #4f46e5;
}

.mobile-step-indicator {
  text-align: center;
  font-size: 14px;
  font-weight: 600;
  color: #4f46e5;
  padding: 12px 0;
  margin-bottom: 16px;
}

@media (max-width: 768px) {
  .setup-wizard {
    padding: 16px;
  }
}
</style>
