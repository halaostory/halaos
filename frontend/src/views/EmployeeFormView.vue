<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { NCard, NForm, NFormItem, NInput, NButton, NSelect, NDatePicker, NSpace, NSpin, useMessage } from 'naive-ui'
import { employeeAPI, companyAPI } from '../api/client'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const message = useMessage()
const loading = ref(false)
const pageLoading = ref(false)

const editId = computed(() => {
  const id = route.params.id
  return id ? Number(id) : null
})
const isEdit = computed(() => !!editId.value)

const form = ref({
  employee_no: '',
  first_name: '',
  last_name: '',
  middle_name: '',
  email: '',
  phone: '',
  birth_date: null as number | null,
  gender: null as string | null,
  civil_status: null as string | null,
  nationality: '',
  hire_date: Date.now(),
  employment_type: 'regular',
  department_id: null as number | null,
  position_id: null as number | null,
})

const departmentOptions = ref<{ label: string; value: number }[]>([])
const positionOptions = ref<{ label: string; value: number }[]>([])

const employmentTypes = computed(() => [
  { label: t('employee.regular'), value: 'regular' },
  { label: t('employee.probationary'), value: 'probationary' },
  { label: t('employee.contractual'), value: 'contractual' },
  { label: t('employee.partTime'), value: 'part_time' },
])

const genderOptions = computed(() => [
  { label: t('employee.male'), value: 'male' },
  { label: t('employee.female'), value: 'female' },
])

const civilStatusOptions = computed(() => [
  { label: t('employee.single'), value: 'single' },
  { label: t('employee.married'), value: 'married' },
  { label: t('employee.widowed'), value: 'widowed' },
  { label: t('employee.separated'), value: 'separated' },
])

onMounted(async () => {
  pageLoading.value = true
  try {
    const [depts, positions] = await Promise.all([
      companyAPI.listDepartments() as Promise<{ success: boolean; data: { id: number; name: string; code: string }[] }>,
      companyAPI.listPositions() as Promise<{ success: boolean; data: { id: number; title: string; code: string }[] }>,
    ])
    const deptsData = depts.data || depts as unknown as { id: number; name: string; code: string }[]
    const posData = positions.data || positions as unknown as { id: number; title: string; code: string }[]
    departmentOptions.value = (Array.isArray(deptsData) ? deptsData : []).map((d) => ({ label: `${d.code} - ${d.name}`, value: d.id }))
    positionOptions.value = (Array.isArray(posData) ? posData : []).map((p) => ({ label: `${p.code} - ${p.title}`, value: p.id }))

    if (isEdit.value) {
      const res = await employeeAPI.get(editId.value!) as { success: boolean; data: Record<string, unknown> }
      const emp = res.data || res as unknown as Record<string, unknown>
      form.value.employee_no = String(emp.employee_no || '')
      form.value.first_name = String(emp.first_name || '')
      form.value.last_name = String(emp.last_name || '')
      form.value.middle_name = String(emp.middle_name || '')
      form.value.email = String(emp.email || '')
      form.value.phone = String(emp.phone || '')
      form.value.gender = emp.gender ? String(emp.gender) : null
      form.value.civil_status = emp.civil_status ? String(emp.civil_status) : null
      form.value.nationality = String(emp.nationality || '')
      form.value.employment_type = String(emp.employment_type || 'regular')
      form.value.department_id = emp.department_id ? Number(emp.department_id) : null
      form.value.position_id = emp.position_id ? Number(emp.position_id) : null
      if (emp.birth_date) form.value.birth_date = new Date(emp.birth_date as string).getTime()
      if (emp.hire_date) form.value.hire_date = new Date(emp.hire_date as string).getTime()
    }
  } catch {
    if (isEdit.value) message.error(t('employee.loadFailed'))
  } finally {
    pageLoading.value = false
  }
})

async function handleSubmit() {
  if (!form.value.employee_no || !form.value.first_name || !form.value.last_name || !form.value.hire_date) {
    message.warning(t('profile.fillAllFields'))
    return
  }
  if (form.value.email && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(form.value.email)) {
    message.warning(t('auth.invalidEmail'))
    return
  }
  if (form.value.birth_date && form.value.birth_date > Date.now()) {
    message.warning(t('employee.invalidBirthDate'))
    return
  }
  if (form.value.birth_date && form.value.birth_date > form.value.hire_date) {
    message.warning(t('employee.birthBeforeHire'))
    return
  }
  loading.value = true
  try {
    const payload: Record<string, unknown> = {
      employee_no: form.value.employee_no,
      first_name: form.value.first_name,
      last_name: form.value.last_name,
      hire_date: new Date(form.value.hire_date).toISOString().split('T')[0],
      employment_type: form.value.employment_type,
    }
    if (form.value.middle_name) payload.middle_name = form.value.middle_name
    if (form.value.email) payload.email = form.value.email
    if (form.value.phone) payload.phone = form.value.phone
    if (form.value.gender) payload.gender = form.value.gender
    if (form.value.civil_status) payload.civil_status = form.value.civil_status
    if (form.value.nationality) payload.nationality = form.value.nationality
    if (form.value.birth_date) payload.birth_date = new Date(form.value.birth_date).toISOString().split('T')[0]
    if (form.value.department_id) payload.department_id = form.value.department_id
    if (form.value.position_id) payload.position_id = form.value.position_id

    if (isEdit.value) {
      await employeeAPI.update(editId.value!, payload)
      message.success(t('employee.updated'))
      router.push({ name: 'employee-detail', params: { id: editId.value! } })
    } else {
      await employeeAPI.create(payload)
      message.success(t('employee.created'))
      router.push({ name: 'employees' })
    }
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } }
    message.error(err.data?.error?.message || t('common.saveFailed'))
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <NSpin :show="pageLoading">
    <NCard :title="isEdit ? t('employee.editEmployee') : t('employee.addNew')">
      <NForm @submit.prevent="handleSubmit" label-placement="left" label-width="140">
        <NFormItem :label="t('employee.employeeNo')" required>
          <NInput v-model:value="form.employee_no" :disabled="isEdit" />
        </NFormItem>
        <NSpace :size="12" style="width: 100%;">
          <NFormItem :label="t('auth.firstName')" required style="flex: 1;">
            <NInput v-model:value="form.first_name" />
          </NFormItem>
          <NFormItem :label="t('auth.lastName')" required style="flex: 1;">
            <NInput v-model:value="form.last_name" />
          </NFormItem>
        </NSpace>
        <NFormItem :label="t('employee.middleName')">
          <NInput v-model:value="form.middle_name" />
        </NFormItem>
        <NFormItem :label="t('auth.email')">
          <NInput v-model:value="form.email" />
        </NFormItem>
        <NFormItem :label="t('employee.phone')">
          <NInput v-model:value="form.phone" />
        </NFormItem>
        <NSpace :size="12" style="width: 100%;">
          <NFormItem :label="t('employee.gender')" style="flex: 1;">
            <NSelect v-model:value="form.gender" :options="genderOptions" clearable />
          </NFormItem>
          <NFormItem :label="t('employee.civilStatus')" style="flex: 1;">
            <NSelect v-model:value="form.civil_status" :options="civilStatusOptions" clearable />
          </NFormItem>
        </NSpace>
        <NFormItem :label="t('employee.nationality')">
          <NInput v-model:value="form.nationality" />
        </NFormItem>
        <NFormItem :label="t('employee.birthDate')">
          <NDatePicker v-model:value="form.birth_date" type="date" style="width: 100%;" />
        </NFormItem>
        <NFormItem :label="t('employee.hireDate')" required>
          <NDatePicker v-model:value="form.hire_date" type="date" style="width: 100%;" />
        </NFormItem>
        <NFormItem :label="t('employee.department')">
          <NSelect v-model:value="form.department_id" :options="departmentOptions" clearable :placeholder="t('employee.selectDepartment')" />
        </NFormItem>
        <NFormItem :label="t('employee.position')">
          <NSelect v-model:value="form.position_id" :options="positionOptions" clearable :placeholder="t('employee.selectPosition')" />
        </NFormItem>
        <NFormItem :label="t('employee.employmentType')">
          <NSelect v-model:value="form.employment_type" :options="employmentTypes" />
        </NFormItem>
        <NSpace>
          <NButton type="primary" :loading="loading" attr-type="submit">{{ t('common.save') }}</NButton>
          <NButton @click="router.back()">{{ t('common.cancel') }}</NButton>
        </NSpace>
      </NForm>
    </NCard>
  </NSpin>
</template>
