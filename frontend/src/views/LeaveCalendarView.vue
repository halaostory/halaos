<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { NSpace, NButton, NTag, NSpin } from 'naive-ui'
import { leaveAPI } from '../api/client'
import EmptyState from '../components/EmptyState.vue'

const { t } = useI18n()
const router = useRouter()

const loading = ref(false)
const currentYear = ref(new Date().getFullYear())
const currentMonth = ref(new Date().getMonth()) // 0-indexed

interface CalendarLeave {
  id: number
  employee_id: number
  start_date: string
  end_date: string
  days: number
  leave_type_name: string
  leave_type_code: string
  first_name: string
  last_name: string
  display_name: string | null
  department_name: string
}

const leaves = ref<CalendarLeave[]>([])

const monthLabel = computed(() => {
  const d = new Date(currentYear.value, currentMonth.value)
  return d.toLocaleDateString('en-US', { year: 'numeric', month: 'long' })
})

const daysOfWeek = computed(() => [
  t('analytics.sun'), t('analytics.mon'), t('analytics.tue'),
  t('analytics.wed'), t('analytics.thu'), t('analytics.fri'), t('analytics.sat'),
])

interface CalendarDay {
  date: number
  dateStr: string
  isCurrentMonth: boolean
  isToday: boolean
  leaves: CalendarLeave[]
}

const calendarGrid = computed<CalendarDay[][]>(() => {
  const year = currentYear.value
  const month = currentMonth.value
  const firstDay = new Date(year, month, 1)
  const lastDay = new Date(year, month + 1, 0)
  const startDow = firstDay.getDay()

  const today = new Date()
  const todayStr = `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, '0')}-${String(today.getDate()).padStart(2, '0')}`

  const days: CalendarDay[] = []

  // Previous month padding
  const prevLastDay = new Date(year, month, 0).getDate()
  for (let i = startDow - 1; i >= 0; i--) {
    const d = prevLastDay - i
    const m = month === 0 ? 12 : month
    const y = month === 0 ? year - 1 : year
    const dateStr = `${y}-${String(m).padStart(2, '0')}-${String(d).padStart(2, '0')}`
    days.push({ date: d, dateStr, isCurrentMonth: false, isToday: false, leaves: [] })
  }

  // Current month
  for (let d = 1; d <= lastDay.getDate(); d++) {
    const dateStr = `${year}-${String(month + 1).padStart(2, '0')}-${String(d).padStart(2, '0')}`
    const dayLeaves = leaves.value.filter(l => {
      const start = l.start_date.slice(0, 10)
      const end = l.end_date.slice(0, 10)
      return dateStr >= start && dateStr <= end
    })
    days.push({
      date: d,
      dateStr,
      isCurrentMonth: true,
      isToday: dateStr === todayStr,
      leaves: dayLeaves,
    })
  }

  // Next month padding to fill last row
  const remaining = 7 - (days.length % 7)
  if (remaining < 7) {
    for (let d = 1; d <= remaining; d++) {
      const m = month + 2 > 12 ? 1 : month + 2
      const y = month + 2 > 12 ? year + 1 : year
      const dateStr = `${y}-${String(m).padStart(2, '0')}-${String(d).padStart(2, '0')}`
      days.push({ date: d, dateStr, isCurrentMonth: false, isToday: false, leaves: [] })
    }
  }

  // Split into weeks
  const weeks: CalendarDay[][] = []
  for (let i = 0; i < days.length; i += 7) {
    weeks.push(days.slice(i, i + 7))
  }
  return weeks
})

async function fetchLeaves() {
  loading.value = true
  try {
    const year = currentYear.value
    const month = currentMonth.value
    const start = `${year}-${String(month + 1).padStart(2, '0')}-01`
    const lastDay = new Date(year, month + 1, 0).getDate()
    const end = `${year}-${String(month + 1).padStart(2, '0')}-${String(lastDay).padStart(2, '0')}`
    const res = await leaveAPI.calendar(start, end)
    const data = (res as any)?.data ?? res
    leaves.value = Array.isArray(data) ? data : []
  } catch {
    leaves.value = []
  } finally {
    loading.value = false
  }
}

function prevMonth() {
  if (currentMonth.value === 0) {
    currentMonth.value = 11
    currentYear.value--
  } else {
    currentMonth.value--
  }
}

function nextMonth() {
  if (currentMonth.value === 11) {
    currentMonth.value = 0
    currentYear.value++
  } else {
    currentMonth.value++
  }
}

function goToday() {
  currentYear.value = new Date().getFullYear()
  currentMonth.value = new Date().getMonth()
}

function getName(l: CalendarLeave) {
  return l.display_name || `${l.first_name} ${l.last_name}`
}

const leaveTypeColors: Record<string, string> = {
  VL: 'success',
  SL: 'warning',
  EL: 'error',
  ML: 'info',
  PL: 'info',
}

function getLeaveColor(code: string): string {
  return leaveTypeColors[code] || 'default'
}

watch([currentYear, currentMonth], fetchLeaves)

onMounted(fetchLeaves)
</script>

<template>
  <div>
    <NSpace justify="space-between" align="center" style="margin-bottom: 16px;">
      <h2 style="margin: 0;">{{ t('leaveCalendar.title') }}</h2>
      <NSpace :size="8">
        <NButton size="small" @click="prevMonth">{{ t('leaveCalendar.prevMonth') }}</NButton>
        <NButton size="small" @click="goToday">{{ t('leaveCalendar.today') }}</NButton>
        <NButton size="small" @click="nextMonth">{{ t('leaveCalendar.nextMonth') }}</NButton>
      </NSpace>
    </NSpace>

    <h3 style="text-align: center; margin-bottom: 12px;">{{ monthLabel }}</h3>

    <NSpin :show="loading">
      <table class="cal-table">
        <thead>
          <tr>
            <th v-for="d in daysOfWeek" :key="d">{{ d }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(week, wi) in calendarGrid" :key="wi">
            <td
              v-for="day in week"
              :key="day.dateStr"
              :class="{ 'cal-other': !day.isCurrentMonth, 'cal-today': day.isToday }"
            >
              <div class="cal-date">{{ day.date }}</div>
              <div class="cal-events">
                <NTag
                  v-for="l in day.leaves.slice(0, 3)"
                  :key="l.id"
                  size="tiny"
                  :type="(getLeaveColor(l.leave_type_code) as any)"
                  style="display: block; margin-bottom: 2px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 100%;"
                >
                  {{ getName(l) }}
                </NTag>
                <span v-if="day.leaves.length > 3" style="font-size: 10px; color: #999;">
                  +{{ day.leaves.length - 3 }} more
                </span>
              </div>
            </td>
          </tr>
        </tbody>
      </table>

      <EmptyState
        v-if="leaves.length === 0 && !loading"
        icon="🏖️"
        :title="t('emptyState.leaves.title')"
        :description="t('emptyState.leaves.desc')"
        :primaryAction="{ label: t('emptyState.leaves.cta'), handler: () => router.push({ name: 'leave' }) }"
        style="margin-top: 24px;"
      />
    </NSpin>
  </div>
</template>

<style scoped>
.cal-table {
  width: 100%;
  border-collapse: collapse;
  table-layout: fixed;
}
.cal-table th {
  padding: 8px 4px;
  text-align: center;
  font-weight: 600;
  font-size: 13px;
  border-bottom: 2px solid var(--n-border-color, #e0e0e0);
}
.cal-table td {
  border: 1px solid var(--n-border-color, #e0e0e0);
  vertical-align: top;
  padding: 4px 6px;
  min-height: 90px;
  height: 90px;
}
.cal-other {
  opacity: 0.35;
}
.cal-today {
  background: rgba(24, 160, 88, 0.08);
}
.cal-today .cal-date {
  color: #18a058;
  font-weight: 700;
}
.cal-date {
  font-size: 13px;
  font-weight: 500;
  margin-bottom: 4px;
}
.cal-events {
  display: flex;
  flex-direction: column;
  gap: 1px;
}
</style>
