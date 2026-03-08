<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NCard, NButton, NSpace, NTag, NModal, NForm, NFormItem, NTabs, NTabPane, NInput,
  NSelect, NDatePicker, NSwitch, NDataTable, NEmpty, useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { attendanceAPI, employeeAPI } from '../api/client'

const { t } = useI18n()
const message = useMessage()

interface Schedule {
  id: number
  employee_id: number
  shift_id: number
  work_date: string
  is_rest_day: boolean
  shift_name: string
  start_time: string
  end_time: string
  first_name: string
  last_name: string
  employee_no: string
}

interface Shift {
  id: number
  name: string
  start_time: string
  end_time: string
}

interface ScheduleTemplate {
  id: number
  name: string
  description: string
  is_active: boolean
}

interface TemplateDay {
  id: number
  template_id: number
  day_of_week: number
  shift_id: number | null
  is_rest_day: boolean
  shift_name: string
  start_time: string
  end_time: string
}

interface ScheduleAssignment {
  id: number
  employee_id: number
  template_id: number
  effective_from: string
  effective_to: string | null
  template_name: string
  first_name: string
  last_name: string
  employee_no: string
}

const schedules = ref<Schedule[]>([])
const shifts = ref<Shift[]>([])
const employees = ref<{ label: string; value: number }[]>([])
const loading = ref(false)

// Week navigation
const weekOffset = ref(0)

const weekStart = computed(() => {
  const now = new Date()
  const day = now.getDay() || 7
  const monday = new Date(now)
  monday.setDate(now.getDate() - day + 1 + weekOffset.value * 7)
  monday.setHours(0, 0, 0, 0)
  return monday
})

const weekEnd = computed(() => {
  const end = new Date(weekStart.value)
  end.setDate(end.getDate() + 6)
  return end
})

const weekDays = computed(() => {
  const days: { date: Date; label: string; dateStr: string }[] = []
  for (let i = 0; i < 7; i++) {
    const d = new Date(weekStart.value)
    d.setDate(d.getDate() + i)
    const dayNames = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']
    days.push({
      date: d,
      label: `${dayNames[d.getDay()]} ${d.getDate()}/${d.getMonth() + 1}`,
      dateStr: formatDate(d),
    })
  }
  return days
})

const weekLabel = computed(() => {
  return `${formatDate(weekStart.value)} ~ ${formatDate(weekEnd.value)}`
})

function formatDate(d: Date): string {
  return d.toISOString().split('T')[0]
}

function formatTime(t: string): string {
  if (!t) return ''
  const parts = t.split(':')
  if (parts.length >= 2) return `${parts[0]}:${parts[1]}`
  return t
}

// Group schedules by employee
const groupedSchedules = computed(() => {
  const map = new Map<number, { name: string; empNo: string; days: Map<string, Schedule> }>()
  for (const s of schedules.value) {
    if (!map.has(s.employee_id)) {
      map.set(s.employee_id, {
        name: `${s.last_name}, ${s.first_name}`,
        empNo: s.employee_no,
        days: new Map(),
      })
    }
    map.get(s.employee_id)!.days.set(s.work_date.split('T')[0], s)
  }
  return Array.from(map.entries()).map(([empId, data]) => ({
    empId,
    ...data,
    days: data.days,
  }))
})

const columns = computed<DataTableColumns<(typeof groupedSchedules.value)[0]>>(() => {
  const cols: DataTableColumns<(typeof groupedSchedules.value)[0]> = [
    {
      title: t('employee.name'),
      key: 'name',
      width: 180,
      fixed: 'left',
      render: (row) => h('div', [
        h('div', { style: 'font-weight: 600;' }, row.name),
        h('div', { style: 'font-size: 12px; color: #999;' }, row.empNo),
      ]),
    },
  ]

  for (const day of weekDays.value) {
    cols.push({
      title: day.label,
      key: day.dateStr,
      width: 130,
      align: 'center',
      render: (row) => {
        const schedule = row.days.get(day.dateStr)
        if (!schedule) return h('span', { style: 'color: #ccc;' }, '-')
        if (schedule.is_rest_day) {
          return h(NTag, { size: 'small', type: 'default' }, { default: () => t('attendance.restDay') })
        }
        return h('div', [
          h(NTag, { size: 'small', type: 'info' }, { default: () => schedule.shift_name }),
          h('div', { style: 'font-size: 11px; color: #666; margin-top: 2px;' },
            `${formatTime(schedule.start_time)}-${formatTime(schedule.end_time)}`),
        ])
      },
    })
  }

  return cols
})

// Assign modal
const showAssignModal = ref(false)
const assignForm = ref({
  employee_ids: [] as number[],
  shift_id: null as number | null,
  dates: null as [number, number] | null,
  is_rest_day: false,
})

const shiftOptions = computed(() =>
  shifts.value.map(s => ({
    label: `${s.name} (${formatTime(s.start_time)}-${formatTime(s.end_time)})`,
    value: s.id,
  }))
)

async function loadData() {
  loading.value = true
  try {
    const [schedRes, shiftRes] = await Promise.all([
      attendanceAPI.listSchedules({
        start: formatDate(weekStart.value),
        end: formatDate(weekEnd.value),
      }),
      attendanceAPI.listShifts(),
    ])
    const sd = (schedRes as any)?.data ?? schedRes
    schedules.value = Array.isArray(sd) ? sd : []
    const sh = (shiftRes as any)?.data ?? shiftRes
    shifts.value = Array.isArray(sh) ? sh : []
  } catch { message.error(t('common.loadFailed')) }
  loading.value = false
}

async function loadEmployees() {
  try {
    const res = await employeeAPI.list({ limit: '500' })
    const data = (res as any)?.data ?? res
    const arr = Array.isArray(data) ? data : []
    employees.value = arr.map((e: any) => ({
      label: `${e.employee_no} - ${e.last_name}, ${e.first_name}`,
      value: e.id,
    }))
  } catch { /* ignore */ }
}

function openAssign() {
  assignForm.value = { employee_ids: [], shift_id: null, dates: null, is_rest_day: false }
  showAssignModal.value = true
  if (employees.value.length === 0) loadEmployees()
}

async function handleBulkAssign() {
  if (!assignForm.value.employee_ids.length || !assignForm.value.shift_id || !assignForm.value.dates) {
    message.warning(t('common.fillAllFields'))
    return
  }

  const [startTs, endTs] = assignForm.value.dates
  const dates: string[] = []
  const cur = new Date(startTs)
  const end = new Date(endTs)
  while (cur <= end) {
    dates.push(formatDate(cur))
    cur.setDate(cur.getDate() + 1)
  }
  try {
    await attendanceAPI.bulkAssignSchedule({
      employee_ids: assignForm.value.employee_ids,
      shift_id: assignForm.value.shift_id,
      dates,
      is_rest_day: assignForm.value.is_rest_day,
    })
    message.success(t('attendance.assigned'))
    showAssignModal.value = false
    loadData()
  } catch {
    message.error(t('common.failed'))
  }
}

function prevWeek() {
  weekOffset.value--
  loadData()
}

function nextWeek() {
  weekOffset.value++
  loadData()
}

function thisWeek() {
  weekOffset.value = 0
  loadData()
}

// ========== Schedule Templates ==========
const templates = ref<ScheduleTemplate[]>([])
const templateLoading = ref(false)
const showTemplateModal = ref(false)
const editingTemplateId = ref<number | null>(null)

const dayLabels = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']

const templateForm = ref({
  name: '',
  description: '',
  days: Array.from({ length: 7 }, (_, i) => ({
    day_of_week: i,
    shift_id: null as number | null,
    is_rest_day: i === 0 || i === 6, // Default: Sun & Sat rest
  })),
})

async function loadTemplates() {
  templateLoading.value = true
  try {
    const res = await attendanceAPI.listScheduleTemplates()
    const data = (res as any)?.data ?? res
    templates.value = Array.isArray(data) ? data : []
  } catch { message.error(t('common.loadFailed')) }
  templateLoading.value = false
}

function openCreateTemplate() {
  editingTemplateId.value = null
  templateForm.value = {
    name: '',
    description: '',
    days: Array.from({ length: 7 }, (_, i) => ({
      day_of_week: i,
      shift_id: null as number | null,
      is_rest_day: i === 0 || i === 6,
    })),
  }
  showTemplateModal.value = true
}

async function openEditTemplate(tmpl: ScheduleTemplate) {
  editingTemplateId.value = tmpl.id
  templateForm.value = {
    name: tmpl.name,
    description: tmpl.description || '',
    days: Array.from({ length: 7 }, (_, i) => ({
      day_of_week: i,
      shift_id: null as number | null,
      is_rest_day: i === 0 || i === 6,
    })),
  }
  try {
    const res = await attendanceAPI.getScheduleTemplate(tmpl.id)
    const data = (res as any)?.data ?? res
    const daysList: TemplateDay[] = data?.days ?? []
    for (const d of daysList) {
      const slot = templateForm.value.days[d.day_of_week]
      if (slot) {
        slot.shift_id = d.shift_id
        slot.is_rest_day = d.is_rest_day
      }
    }
  } catch { message.error(t('common.loadFailed')) }
  showTemplateModal.value = true
}

async function handleSaveTemplate() {
  if (!templateForm.value.name.trim()) {
    message.warning(t('common.fillAllFields'))
    return
  }
  try {
    if (editingTemplateId.value) {
      await attendanceAPI.updateScheduleTemplate(editingTemplateId.value, {
        name: templateForm.value.name,
        description: templateForm.value.description,
        days: templateForm.value.days,
      })
    } else {
      await attendanceAPI.createScheduleTemplate({
        name: templateForm.value.name,
        description: templateForm.value.description,
        days: templateForm.value.days,
      })
    }
    message.success(t('common.save'))
    showTemplateModal.value = false
    loadTemplates()
  } catch {
    message.error(t('common.saveFailed'))
  }
}

async function handleDeleteTemplate(id: number) {
  try {
    await attendanceAPI.deleteScheduleTemplate(id)
    message.success(t('common.delete'))
    loadTemplates()
  } catch {
    message.error(t('common.failed'))
  }
}

// Template assignment
const showAssignTemplateModal = ref(false)
const assignTemplateForm = ref({
  employee_ids: [] as number[],
  template_id: null as number | null,
  effective_from: null as number | null,
  effective_to: null as number | null,
})

const templateOptions = computed(() =>
  templates.value.map(t => ({ label: t.name, value: t.id }))
)

const assignments = ref<ScheduleAssignment[]>([])

async function loadAssignments() {
  try {
    const res = await attendanceAPI.listScheduleAssignments()
    const data = (res as any)?.data ?? res
    assignments.value = Array.isArray(data) ? data : []
  } catch { message.error(t('common.loadFailed')) }
}

function openAssignTemplate() {
  assignTemplateForm.value = {
    employee_ids: [],
    template_id: null,
    effective_from: null,
    effective_to: null,
  }
  showAssignTemplateModal.value = true
  if (employees.value.length === 0) loadEmployees()
}

async function handleAssignTemplate() {
  if (!assignTemplateForm.value.employee_ids.length ||
      !assignTemplateForm.value.template_id ||
      !assignTemplateForm.value.effective_from) {
    message.warning(t('common.fillAllFields'))
    return
  }
  try {
    for (const empId of assignTemplateForm.value.employee_ids) {
      await attendanceAPI.assignScheduleTemplate(assignTemplateForm.value.template_id!, {
        employee_id: empId,
        effective_from: formatDate(new Date(assignTemplateForm.value.effective_from!)),
        effective_to: assignTemplateForm.value.effective_to
          ? formatDate(new Date(assignTemplateForm.value.effective_to))
          : undefined,
      })
    }
    message.success(t('attendance.assigned'))
    showAssignTemplateModal.value = false
    loadAssignments()
  } catch {
    message.error(t('common.failed'))
  }
}

const templateColumns: DataTableColumns<ScheduleTemplate> = [
  { title: () => t('common.name'), key: 'name' },
  { title: () => t('attendance.templateDescription'), key: 'description', ellipsis: { tooltip: true } },
  {
    title: () => t('common.actions'),
    key: 'actions',
    width: 180,
    render: (row) => h(NSpace, { size: 'small' }, {
      default: () => [
        h(NButton, { size: 'small', onClick: () => openEditTemplate(row) }, { default: () => t('common.edit') }),
        h(NButton, { size: 'small', type: 'error', onClick: () => handleDeleteTemplate(row.id) }, { default: () => t('common.delete') }),
      ],
    }),
  },
]

const assignmentColumns: DataTableColumns<ScheduleAssignment> = [
  {
    title: () => t('employee.name'),
    key: 'employee',
    render: (row) => `${row.last_name}, ${row.first_name} (${row.employee_no})`,
  },
  { title: () => t('attendance.template'), key: 'template_name' },
  {
    title: () => t('employee.effectiveFrom'),
    key: 'effective_from',
    render: (row) => row.effective_from?.split('T')[0] ?? '',
  },
  {
    title: () => t('employee.effectiveTo'),
    key: 'effective_to',
    render: (row) => row.effective_to?.split('T')[0] ?? '-',
  },
]

const activeTab = ref('weekly')

onMounted(() => {
  loadData()
  loadTemplates()
  loadAssignments()
})
</script>

<template>
  <NCard :title="t('attendance.schedules')">
    <NTabs v-model:value="activeTab" type="line">
      <!-- Weekly Schedule Tab -->
      <NTabPane name="weekly" :tab="t('attendance.weeklySchedule')">
        <NSpace vertical :size="16">
          <NSpace :size="12" align="center" justify="space-between">
            <NSpace :size="12" align="center">
              <NButton @click="prevWeek">{{ t('attendance.prevWeek') }}</NButton>
              <NButton @click="thisWeek">{{ t('attendance.thisWeek') }}</NButton>
              <NButton @click="nextWeek">{{ t('attendance.nextWeek') }}</NButton>
              <strong>{{ weekLabel }}</strong>
            </NSpace>
            <NButton type="primary" @click="openAssign">{{ t('attendance.bulkAssign') }}</NButton>
          </NSpace>

          <NDataTable
            v-if="groupedSchedules.length"
            :columns="columns"
            :data="groupedSchedules"
            :loading="loading"
            :row-key="(row: any) => row.empId"
            :scroll-x="1100"
            size="small"
          />
          <NEmpty v-else :description="t('attendance.noSchedules')" />
        </NSpace>
      </NTabPane>

      <!-- Templates Tab -->
      <NTabPane name="templates" :tab="t('attendance.templates')">
        <NSpace vertical :size="16">
          <NSpace justify="end">
            <NButton type="primary" @click="openCreateTemplate">{{ t('attendance.createTemplate') }}</NButton>
          </NSpace>

          <NDataTable
            v-if="templates.length"
            :columns="templateColumns"
            :data="templates"
            :loading="templateLoading"
            :row-key="(row: any) => row.id"
            size="small"
          />
          <NEmpty v-else :description="t('attendance.noTemplates')" />
        </NSpace>
      </NTabPane>

      <!-- Assignments Tab -->
      <NTabPane name="assignments" :tab="t('attendance.templateAssignments')">
        <NSpace vertical :size="16">
          <NSpace justify="end">
            <NButton type="primary" @click="openAssignTemplate">{{ t('attendance.assignTemplate') }}</NButton>
          </NSpace>

          <NDataTable
            v-if="assignments.length"
            :columns="assignmentColumns"
            :data="assignments"
            :row-key="(row: any) => row.id"
            size="small"
          />
          <NEmpty v-else :description="t('attendance.noAssignments')" />
        </NSpace>
      </NTabPane>
    </NTabs>
  </NCard>

  <!-- Bulk Assign Modal -->
  <NModal v-model:show="showAssignModal" preset="card" :title="t('attendance.bulkAssign')" style="max-width: 500px; width: 95vw;">
    <NForm label-placement="top">
      <NFormItem :label="t('attendance.selectEmployees')">
        <NSelect
          v-model:value="assignForm.employee_ids"
          :options="employees"
          multiple
          filterable
        />
      </NFormItem>
      <NFormItem :label="t('attendance.selectShift')">
        <NSelect v-model:value="assignForm.shift_id" :options="shiftOptions" />
      </NFormItem>
      <NFormItem :label="t('attendance.selectDates')">
        <NDatePicker v-model:value="assignForm.dates" type="daterange" style="width: 100%;" />
      </NFormItem>
      <NFormItem :label="t('attendance.restDay')">
        <NSwitch v-model:value="assignForm.is_rest_day" />
      </NFormItem>
    </NForm>
    <template #footer>
      <NSpace justify="end">
        <NButton @click="showAssignModal = false">{{ t('common.cancel') }}</NButton>
        <NButton type="primary" @click="handleBulkAssign">{{ t('common.save') }}</NButton>
      </NSpace>
    </template>
  </NModal>

  <!-- Create/Edit Template Modal -->
  <NModal v-model:show="showTemplateModal" preset="card"
    :title="editingTemplateId ? t('attendance.editTemplate') : t('attendance.createTemplate')"
    style="width: 600px;">
    <NForm label-placement="top">
      <NFormItem :label="t('common.name')">
        <NInput v-model:value="templateForm.name" />
      </NFormItem>
      <NFormItem :label="t('attendance.templateDescription')">
        <NInput v-model:value="templateForm.description" type="textarea" :rows="2" />
      </NFormItem>
      <NFormItem :label="t('attendance.weeklyPattern')">
        <div style="width: 100%;">
          <div v-for="(day, idx) in templateForm.days" :key="idx"
            style="display: flex; align-items: center; gap: 12px; padding: 6px 0; border-bottom: 1px solid #eee;">
            <span style="width: 40px; font-weight: 600;">{{ dayLabels[idx] }}</span>
            <NSwitch v-model:value="day.is_rest_day" :round="false" size="small">
              <template #checked>{{ t('attendance.restDay') }}</template>
              <template #unchecked>{{ t('attendance.workDay') }}</template>
            </NSwitch>
            <NSelect
              v-if="!day.is_rest_day"
              v-model:value="day.shift_id"
              :options="shiftOptions"
              :placeholder="t('attendance.selectShift')"
              style="flex: 1;"
              size="small"
            />
            <NTag v-else size="small" type="default">{{ t('attendance.restDay') }}</NTag>
          </div>
        </div>
      </NFormItem>
    </NForm>
    <template #footer>
      <NSpace justify="end">
        <NButton @click="showTemplateModal = false">{{ t('common.cancel') }}</NButton>
        <NButton type="primary" @click="handleSaveTemplate">{{ t('common.save') }}</NButton>
      </NSpace>
    </template>
  </NModal>

  <!-- Assign Template Modal -->
  <NModal v-model:show="showAssignTemplateModal" preset="card"
    :title="t('attendance.assignTemplate')" style="width: 500px;">
    <NForm label-placement="top">
      <NFormItem :label="t('attendance.selectEmployees')">
        <NSelect
          v-model:value="assignTemplateForm.employee_ids"
          :options="employees"
          multiple
          filterable
        />
      </NFormItem>
      <NFormItem :label="t('attendance.template')">
        <NSelect v-model:value="assignTemplateForm.template_id" :options="templateOptions" />
      </NFormItem>
      <NFormItem :label="t('employee.effectiveFrom')">
        <NDatePicker v-model:value="assignTemplateForm.effective_from" type="date" style="width: 100%;" />
      </NFormItem>
      <NFormItem :label="t('employee.effectiveTo')">
        <NDatePicker v-model:value="assignTemplateForm.effective_to" type="date" style="width: 100%;" clearable />
      </NFormItem>
    </NForm>
    <template #footer>
      <NSpace justify="end">
        <NButton @click="showAssignTemplateModal = false">{{ t('common.cancel') }}</NButton>
        <NButton type="primary" @click="handleAssignTemplate">{{ t('common.save') }}</NButton>
      </NSpace>
    </template>
  </NModal>
</template>
