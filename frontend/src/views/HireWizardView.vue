<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  NCard, NSteps, NStep, NForm, NFormItem, NInput, NInputNumber, NSelect, NDatePicker,
  NButton, NSpace, NCheckbox, NResult, NList, NListItem, NTag, NEmpty, useMessage,
} from 'naive-ui'
import { format } from 'date-fns'
import { lifecycleAPI, companyAPI, employeeAPI, documentsAPI } from '../api/client'

interface CustomField {
  id: number
  field_key: string
  label: string
  field_type: string
  options: string[]
  is_required: boolean
}
interface Step { key: string; status: string; detail?: string }

const { t } = useI18n()
const router = useRouter()
const message = useMessage()

const current = ref(1)
const submitting = ref(false)
const result = ref<{ employee: { id: number; first_name: string; last_name: string }; steps: Step[]; invite_url?: string; signature_url?: string } | null>(null)
const offerTemplates = ref<{ label: string; value: number }[]>([])
function copy(s: string) {
  navigator.clipboard?.writeText(s)
  message.success(t('docs.copied'))
}

const form = ref({
  first_name: '',
  last_name: '',
  email: '',
  phone: '',
  employee_no: '',
  hire_date: Date.now() as number | null,
  employment_type: 'regular',
  department_id: null as number | null,
  position_id: null as number | null,
  manager_id: null as number | null,
  basic_salary: null as number | null,
  skip_onboarding: false,
  send_invite: true,
  offer_template_id: null as number | null,
  email_links: false,
})
const custom = ref<Record<string, string>>({})
const customFields = ref<CustomField[]>([])
const departments = ref<{ label: string; value: number }[]>([])
const positions = ref<{ label: string; value: number }[]>([])
const managers = ref<{ label: string; value: number }[]>([])

const employmentTypes = computed(() => [
  { label: t('employee.regular'), value: 'regular' },
  { label: t('employee.probationary'), value: 'probationary' },
  { label: t('employee.contractual'), value: 'contractual' },
  { label: t('employee.partTime'), value: 'part_time' },
])

const plannedSteps = computed(() => {
  const s = ['employee_created', 'time_off_enrolled', 'access_provisioning', 'accounting_sync']
  if (form.value.basic_salary) s.splice(1, 0, 'compensation_set')
  if (Object.values(custom.value).some((v) => v)) s.push('custom_fields_saved')
  if (!form.value.skip_onboarding) s.push('onboarding_checklist')
  if (form.value.offer_template_id) s.push('offer_letter_sent')
  if (form.value.send_invite) s.push('self_onboarding_invite')
  return s
})

function unwrap<T>(res: unknown): T {
  return ((res as { data?: T }).data ?? res) as T
}

async function loadLookups() {
  try {
    const [d, p, e, f] = await Promise.all([
      companyAPI.listDepartments(),
      companyAPI.listPositions(),
      employeeAPI.list({ status: 'active', limit: '500' }),
      lifecycleAPI.listCustomFields(),
    ])
    departments.value = (unwrap<{ id: number; name: string }[]>(d) || []).map((x) => ({ label: x.name, value: x.id }))
    positions.value = (unwrap<{ id: number; title?: string; name?: string }[]>(p) || []).map((x) => ({ label: x.title || x.name || String(x.id), value: x.id }))
    managers.value = (unwrap<{ id: number; first_name: string; last_name: string }[]>(e) || []).map((x) => ({ label: `${x.first_name} ${x.last_name}`, value: x.id }))
    customFields.value = unwrap<CustomField[]>(f) || []
    const tpls = unwrap<{ id: number; name: string; kind: string; is_active: boolean }[]>(await documentsAPI.templates()) || []
    offerTemplates.value = tpls.filter((x) => x.is_active).map((x) => ({ label: x.name, value: x.id }))
  } catch {
    message.error(t('common.loadFailed'))
  }
}

function validateStep(): boolean {
  if (current.value === 1) {
    if (!form.value.first_name.trim() || !form.value.last_name.trim()) {
      message.warning(`${t('auth.firstName')} / ${t('auth.lastName')}`)
      return false
    }
  }
  if (current.value === 2 && !form.value.hire_date) {
    message.warning(t('employee.hireDate'))
    return false
  }
  if (current.value === 3) {
    const missing = customFields.value.find((f) => f.is_required && !custom.value[f.field_key])
    if (missing) {
      message.warning(missing.label)
      return false
    }
  }
  return true
}

function next() {
  if (validateStep()) current.value++
}

async function submit() {
  submitting.value = true
  try {
    const f = form.value
    const payload: Record<string, unknown> = {
      first_name: f.first_name.trim(),
      last_name: f.last_name.trim(),
      hire_date: format(new Date(f.hire_date as number), 'yyyy-MM-dd'),
      employment_type: f.employment_type,
      skip_onboarding: f.skip_onboarding,
      send_invite: f.send_invite,
      email_links: f.email_links,
      custom_fields: Object.fromEntries(Object.entries(custom.value).filter(([, v]) => v !== '')),
    }
    if (f.email) payload.email = f.email
    if (f.phone) payload.phone = f.phone
    if (f.employee_no) payload.employee_no = f.employee_no
    if (f.department_id) payload.department_id = f.department_id
    if (f.position_id) payload.position_id = f.position_id
    if (f.manager_id) payload.manager_id = f.manager_id
    if (f.basic_salary) payload.basic_salary = f.basic_salary
    if (f.offer_template_id) payload.offer_template_id = f.offer_template_id
    const res = await lifecycleAPI.hire(payload)
    result.value = unwrap(res)
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } }
    message.error(err.data?.error?.message || t('common.saveFailed'))
  } finally {
    submitting.value = false
  }
}

function reset() {
  result.value = null
  current.value = 1
  custom.value = {}
  form.value = { ...form.value, first_name: '', last_name: '', email: '', phone: '', employee_no: '', basic_salary: null }
}

function boolOptions() {
  return [{ label: '✓', value: 'true' }, { label: '✗', value: 'false' }]
}

onMounted(loadLookups)
</script>

<template>
  <NSpace vertical :size="16" style="max-width: 820px">
    <div>
      <h2 style="margin: 0">{{ t('lifecycle.hireTitle') }}</h2>
      <div style="opacity: 0.7">{{ t('lifecycle.hireSubtitle') }}</div>
    </div>

    <NCard v-if="result">
      <NResult
        status="success"
        :title="t('lifecycle.hiredTitle', { name: `${result.employee.first_name} ${result.employee.last_name}` })"
      >
        <template #default>
          <div style="text-align: left">
            <h4>{{ t('lifecycle.whatWeDid') }}</h4>
            <NList>
              <NListItem v-for="s in result.steps" :key="s.key">
                <NSpace justify="space-between" align="center">
                  <span>{{ t('lifecycle.step_' + s.key) }}<span v-if="s.detail" style="opacity: 0.6"> — {{ s.detail }}</span></span>
                  <NTag size="small" :type="s.status === 'done' ? 'success' : s.status === 'scheduled' ? 'info' : 'default'">
                    {{ t('lifecycle.status_' + s.status) }}
                  </NTag>
                </NSpace>
              </NListItem>
            </NList>
            <NSpace vertical style="margin-top: 12px">
              <div v-if="result.invite_url"><b>{{ t('lifecycle.inviteLink') }}</b><br /><code style="word-break: break-all">{{ result.invite_url }}</code> <NButton size="tiny" @click="copy(result.invite_url)">{{ t('docs.copy') }}</NButton></div>
              <div v-if="result.signature_url"><b>{{ t('lifecycle.offerLink') }}</b><br /><code style="word-break: break-all">{{ result.signature_url }}</code> <NButton size="tiny" @click="copy(result.signature_url)">{{ t('docs.copy') }}</NButton></div>
            </NSpace>
          </div>
        </template>
        <template #footer>
          <NSpace justify="center">
            <NButton type="primary" @click="router.push({ name: 'employee-detail', params: { id: String(result!.employee.id) } })">
              {{ t('lifecycle.viewProfile') }}
            </NButton>
            <NButton @click="reset">{{ t('lifecycle.hireAnother') }}</NButton>
          </NSpace>
        </template>
      </NResult>
    </NCard>

    <NCard v-else>
      <NSteps :current="current" size="small" style="margin-bottom: 24px">
        <NStep :title="t('lifecycle.stepPerson')" />
        <NStep :title="t('lifecycle.stepRole')" />
        <NStep :title="t('lifecycle.stepPay')" />
        <NStep :title="t('lifecycle.stepReview')" />
      </NSteps>

      <NForm label-placement="top">
        <template v-if="current === 1">
          <NSpace :wrap="false" :size="12">
            <NFormItem :label="t('auth.firstName')" required style="flex: 1">
              <NInput v-model:value="form.first_name" />
            </NFormItem>
            <NFormItem :label="t('auth.lastName')" required style="flex: 1">
              <NInput v-model:value="form.last_name" />
            </NFormItem>
          </NSpace>
          <NFormItem :label="t('common.email')">
            <NInput v-model:value="form.email" placeholder="name@company.com" />
          </NFormItem>
          <NFormItem :label="t('common.phone')">
            <NInput v-model:value="form.phone" />
          </NFormItem>
        </template>

        <template v-else-if="current === 2">
          <NFormItem :label="t('employee.hireDate')" required>
            <NDatePicker v-model:value="form.hire_date" type="date" style="width: 100%" />
          </NFormItem>
          <NFormItem :label="t('employee.employmentType')">
            <NSelect v-model:value="form.employment_type" :options="employmentTypes" />
          </NFormItem>
          <NFormItem :label="t('employee.department')">
            <NSelect v-model:value="form.department_id" :options="departments" clearable filterable />
          </NFormItem>
          <NFormItem :label="t('employee.position')">
            <NSelect v-model:value="form.position_id" :options="positions" clearable filterable />
          </NFormItem>
          <NFormItem :label="t('userMgmt.manager')">
            <NSelect v-model:value="form.manager_id" :options="managers" clearable filterable />
          </NFormItem>
          <NFormItem :label="t('employee.employeeNo')">
            <NInput v-model:value="form.employee_no" :placeholder="t('lifecycle.employeeNoAuto')" />
          </NFormItem>
        </template>

        <template v-else-if="current === 3">
          <NFormItem :label="t('lifecycle.basicSalary')">
            <NInputNumber v-model:value="form.basic_salary" :min="0" :precision="2" style="width: 100%" />
          </NFormItem>
          <h4 style="margin: 8px 0">{{ t('lifecycle.customFields') }}</h4>
          <NEmpty v-if="customFields.length === 0" size="small" :description="t('lifecycle.noCustomFields')" />
          <NFormItem v-for="f in customFields" :key="f.id" :label="f.label" :required="f.is_required">
            <NSelect
              v-if="f.field_type === 'select'"
              v-model:value="custom[f.field_key]"
              :options="f.options.map((o) => ({ label: o, value: o }))"
              clearable
            />
            <NSelect
              v-else-if="f.field_type === 'boolean'"
              v-model:value="custom[f.field_key]"
              :options="boolOptions()"
              clearable
            />
            <NInput
              v-else
              v-model:value="custom[f.field_key]"
              :placeholder="f.field_type === 'date' ? 'YYYY-MM-DD' : ''"
            />
          </NFormItem>
          <h4 style="margin: 16px 0 8px">{{ t('lifecycle.newHireExperience') }}</h4>
          <NFormItem :label="t('lifecycle.offerLetter')">
            <NSelect v-model:value="form.offer_template_id" clearable :options="offerTemplates" :placeholder="t('lifecycle.offerNone')" />
          </NFormItem>
          <NSpace vertical>
            <NCheckbox v-model:checked="form.send_invite">{{ t('lifecycle.sendInvite') }}</NCheckbox>
            <NCheckbox v-model:checked="form.email_links">{{ t('lifecycle.emailLinks') }}</NCheckbox>
            <NCheckbox v-model:checked="form.skip_onboarding">{{ t('lifecycle.skipOnboarding') }}</NCheckbox>
          </NSpace>
        </template>

        <template v-else>
          <p>{{ t('lifecycle.reviewHint') }}</p>
          <NList>
            <NListItem v-for="k in plannedSteps" :key="k">✓ {{ t('lifecycle.step_' + k) }}</NListItem>
          </NList>
        </template>
      </NForm>

      <NSpace justify="space-between" style="margin-top: 16px">
        <NButton :disabled="current === 1" @click="current--">{{ t('common.back') }}</NButton>
        <NButton v-if="current < 4" type="primary" @click="next">{{ t('botSetup.nextStep') }}</NButton>
        <NButton v-else type="primary" :loading="submitting" @click="submit">{{ t('lifecycle.hireButton') }}</NButton>
      </NSpace>
    </NCard>
  </NSpace>
</template>
