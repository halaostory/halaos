<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NButton, NDataTable, NModal, NForm, NFormItem, NInput, NInputNumber, NSelect, NSwitch, NTag, NSpace, NAlert,
  useMessage, type DataTableColumns,
} from 'naive-ui'
import { h } from 'vue'
import { leaveAPI, companyAPI, leavePolicyAPI } from '../../api/client'

interface Rule {
  id: number; leave_type_id: number; name: string; priority: number; department_ids: number[]; employment_types: string[]
  gender: string; min_tenure_months: number; max_tenure_months: number | null; days: string; is_active: boolean
}
interface Change { employee_name: string; leave_type: string; from: number; to: number; rule: string }

const { t } = useI18n()
const message = useMessage()
const rules = ref<Rule[]>([])
const types = ref<{ label: string; value: number }[]>([])
const depts = ref<{ label: string; value: number }[]>([])
const loading = ref(false)
const showForm = ref(false)
const editing = ref<number | null>(null)
const form = ref({ leave_type_id: null as number | null, name: '', priority: 0, department_ids: [] as number[], employment_types: [] as string[], gender: '', min_years: 0, max_years: null as number | null, days: 15, is_active: true })
const preview = ref<Change[] | null>(null)
const busy = ref(false)

const empTypes = computed(() => [
  { label: t('employee.regular'), value: 'regular' }, { label: t('employee.probationary'), value: 'probationary' },
  { label: t('employee.contractual'), value: 'contractual' }, { label: t('employee.partTime'), value: 'part_time' },
])
const genders = computed(() => [{ label: t('leavePolicy.anyGender'), value: '' }, { label: t('leavePolicy.female'), value: 'female' }, { label: t('leavePolicy.male'), value: 'male' }])
const typeName = (id: number) => types.value.find((x) => x.value === id)?.label || `#${id}`
const deptName = (id: number) => depts.value.find((x) => x.value === id)?.label || `#${id}`

function who(r: Rule) {
  const parts: string[] = []
  if (r.department_ids.length) parts.push(r.department_ids.map(deptName).join(', '))
  if (r.employment_types.length) parts.push(r.employment_types.join(', ').replace(/_/g, ' '))
  if (r.gender) parts.push(r.gender)
  const minY = r.min_tenure_months / 12, maxY = r.max_tenure_months != null ? r.max_tenure_months / 12 : null
  if (minY && maxY != null) parts.push(t('leavePolicy.tenureBetween', { a: +minY.toFixed(1), b: +maxY.toFixed(1) }))
  else if (minY) parts.push(t('leavePolicy.tenureFrom', { a: +minY.toFixed(1) }))
  else if (maxY != null) parts.push(t('leavePolicy.tenureUnder', { b: +maxY.toFixed(1) }))
  return parts.length ? parts.join(' · ') : t('leavePolicy.everyone')
}

const columns = computed<DataTableColumns<Rule>>(() => [
  { title: t('leavePolicy.rule'), key: 'name', render: (r) => h('div', [h('div', { style: 'font-weight:500' }, r.name), h('div', { style: 'font-size:12px;opacity:.65' }, who(r))]) },
  { title: t('leavePolicy.leaveType'), key: 'leave_type_id', width: 180, render: (r) => typeName(r.leave_type_id) },
  { title: t('leavePolicy.days'), key: 'days', width: 90, render: (r) => Number(r.days).toString() },
  { title: t('leavePolicy.priority'), key: 'priority', width: 90 },
  { title: '', key: 'state', width: 90, render: (r) => h(NTag, { size: 'small', round: true, bordered: false, type: r.is_active ? 'success' : 'default' }, () => (r.is_active ? t('leavePolicy.active') : t('leavePolicy.paused'))) },
  {
    title: '', key: 'actions', width: 150, render: (r) => h(NSpace, { size: 'small', justify: 'end' }, () => [
      h(NButton, { size: 'small', quaternary: true, onClick: () => edit(r) }, () => t('common.edit')),
      h(NButton, { size: 'small', quaternary: true, type: 'error', onClick: () => remove(r) }, () => t('common.delete')),
    ]),
  },
])

async function load() {
  loading.value = true
  try {
    const [r, lt, d] = await Promise.all([leavePolicyAPI.rules(), leaveAPI.listTypes(), companyAPI.listDepartments()])
    rules.value = (r as { data: Rule[] }).data
    types.value = ((lt as { data: { id: number; name: string }[] }).data || []).map((x) => ({ label: x.name, value: x.id }))
    depts.value = ((d as { data: { id: number; name: string }[] }).data || []).map((x) => ({ label: x.name, value: x.id }))
  } finally {
    loading.value = false
  }
}
function add() {
  editing.value = null
  form.value = { leave_type_id: types.value[0]?.value ?? null, name: '', priority: 0, department_ids: [], employment_types: [], gender: '', min_years: 0, max_years: null, days: 15, is_active: true }
  showForm.value = true
}
function edit(r: Rule) {
  editing.value = r.id
  form.value = { leave_type_id: r.leave_type_id, name: r.name, priority: r.priority, department_ids: [...r.department_ids], employment_types: [...r.employment_types], gender: r.gender,
    min_years: r.min_tenure_months / 12, max_years: r.max_tenure_months != null ? r.max_tenure_months / 12 : null, days: Number(r.days), is_active: r.is_active }
  showForm.value = true
}
async function save() {
  const f = form.value
  const payload = { leave_type_id: f.leave_type_id, name: f.name, priority: f.priority, department_ids: f.department_ids, employment_types: f.employment_types, gender: f.gender,
    min_tenure_months: Math.round((f.min_years || 0) * 12), max_tenure_months: f.max_years != null ? Math.round(f.max_years * 12) : null, days: f.days, is_active: f.is_active }
  busy.value = true
  try {
    if (editing.value) await leavePolicyAPI.update(editing.value, payload)
    else await leavePolicyAPI.create(payload)
    showForm.value = false
    await load()
  } catch (e) {
    message.error((e as { data?: { error?: { message?: string } } }).data?.error?.message || t('common.saveFailed'))
  } finally {
    busy.value = false
  }
}
async function remove(r: Rule) {
  await leavePolicyAPI.remove(r.id)
  await load()
}
async function runPreview() {
  busy.value = true
  try { preview.value = ((await leavePolicyAPI.preview()) as { data: { changes: Change[] } }).data.changes } finally { busy.value = false }
}
async function apply() {
  busy.value = true
  try {
    const n = ((await leavePolicyAPI.apply()) as { data: { changes: Change[] } }).data.changes.length
    message.success(t('leavePolicy.applied', { n }))
    preview.value = null
  } finally {
    busy.value = false
  }
}
onMounted(load)
</script>

<template>
  <div>
    <div class="head">
      <p class="muted">{{ t('leavePolicy.desc') }}</p>
      <NSpace>
        <NButton :loading="busy" @click="runPreview">{{ t('leavePolicy.preview') }}</NButton>
        <NButton type="primary" @click="add">{{ t('leavePolicy.add') }}</NButton>
      </NSpace>
    </div>
    <NDataTable :columns="columns" :data="rules" :loading="loading" />

    <NModal v-model:show="showForm" preset="card" :title="editing ? t('leavePolicy.edit') : t('leavePolicy.add')" style="max-width: 560px">
      <NForm label-placement="top">
        <NFormItem :label="t('leavePolicy.name')"><NInput v-model:value="form.name" :placeholder="t('leavePolicy.namePh')" /></NFormItem>
        <NFormItem :label="t('leavePolicy.leaveType')"><NSelect v-model:value="form.leave_type_id" :options="types" /></NFormItem>
        <NFormItem :label="t('leavePolicy.days')"><NInputNumber v-model:value="form.days" :min="0" :max="365" :step="0.5" /></NFormItem>
        <div class="muted small" style="margin: -8px 0 8px">{{ t('leavePolicy.whoHelp') }}</div>
        <NFormItem :label="t('leavePolicy.departments')"><NSelect v-model:value="form.department_ids" multiple :options="depts" :placeholder="t('leavePolicy.any')" /></NFormItem>
        <NFormItem :label="t('leavePolicy.employmentTypes')"><NSelect v-model:value="form.employment_types" multiple :options="empTypes" :placeholder="t('leavePolicy.any')" /></NFormItem>
        <NFormItem :label="t('leavePolicy.gender')"><NSelect v-model:value="form.gender" :options="genders" /></NFormItem>
        <NSpace>
          <NFormItem :label="t('leavePolicy.minYears')"><NInputNumber v-model:value="form.min_years" :min="0" :step="0.5" style="width: 140px" /></NFormItem>
          <NFormItem :label="t('leavePolicy.maxYears')"><NInputNumber v-model:value="form.max_years" :min="0" :step="0.5" clearable style="width: 140px" :placeholder="t('leavePolicy.noLimit')" /></NFormItem>
          <NFormItem :label="t('leavePolicy.priority')"><NInputNumber v-model:value="form.priority" style="width: 110px" /></NFormItem>
        </NSpace>
        <NFormItem :label="t('leavePolicy.active')"><NSwitch v-model:value="form.is_active" /></NFormItem>
      </NForm>
      <template #footer>
        <NSpace justify="end">
          <NButton @click="showForm = false">{{ t('common.cancel') }}</NButton>
          <NButton type="primary" :loading="busy" :disabled="!form.name || !form.leave_type_id" @click="save">{{ t('common.save') }}</NButton>
        </NSpace>
      </template>
    </NModal>

    <NModal :show="preview !== null" preset="card" :title="t('leavePolicy.previewTitle')" style="max-width: 640px" @update:show="(v: boolean) => { if (!v) preview = null }">
      <NAlert v-if="preview && !preview.length" type="success">{{ t('leavePolicy.noChanges') }}</NAlert>
      <table v-else-if="preview" class="changes">
        <tr v-for="(c, i) in preview" :key="i">
          <td>{{ c.employee_name }}</td><td>{{ c.leave_type }}</td><td class="num">{{ c.from }} → <strong>{{ c.to }}</strong></td><td class="muted small">{{ c.rule }}</td>
        </tr>
      </table>
      <template #footer>
        <NSpace justify="end">
          <NButton @click="preview = null">{{ t('common.cancel') }}</NButton>
          <NButton type="primary" :disabled="!preview?.length" :loading="busy" @click="apply">{{ t('leavePolicy.apply', { n: preview?.length || 0 }) }}</NButton>
        </NSpace>
      </template>
    </NModal>
  </div>
</template>

<style scoped>
.head { display: flex; justify-content: space-between; align-items: center; gap: 12px; flex-wrap: wrap; margin-bottom: 12px; }
.head p { margin: 0; max-width: 640px; }
.muted { opacity: .7; font-size: 13px; }
.small { font-size: 12px; }
.changes { width: 100%; border-collapse: collapse; font-size: 13px; }
.changes td { padding: 6px 8px; border-bottom: 1px solid rgba(128,128,128,.12); }
.num { white-space: nowrap; }
</style>
