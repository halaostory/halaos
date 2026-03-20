<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  NCard, NGrid, NGi, NButton, NTag, NBadge, NAlert,
  NSpace, NSkeleton, NDivider, useMessage,
} from 'naive-ui'
import { briefingAPI, attendanceAPI } from '../api/client'
import { useAuthStore } from '../stores/auth'

interface LeaveBalance {
  leave_type: string
  code: string
  earned: string
  used: string
  remaining: string
}

interface ManagerAlert {
  type: string
  message: string
  count: number
}

interface BriefingData {
  greeting: string
  date: string
  day_of_week: string
  schedule: {
    shift_name: string
    start_time: string
    end_time: string
  } | null
  leave_balances: LeaveBalance[]
  next_payday: {
    cycle_name: string
    pay_date: string
  } | null
  pending_expenses: number
  unread_notifications: number
  manager: {
    pending_leave_approvals: number
    pending_ot_approvals: number
    today_present: number
    today_total: number
    today_late: number
    upcoming_payroll: {
      cycle_name: string
      pay_date: string
    } | null
    alerts: ManagerAlert[]
  } | null
}

const router = useRouter()
const { t } = useI18n()
const message = useMessage()
const auth = useAuthStore()

const briefing = ref<BriefingData | null>(null)
const loading = ref(true)
const error = ref(false)
const clockedIn = ref(false)
const clockLoading = ref(false)

const formattedDate = computed(() => {
  if (!briefing.value) return ''
  const d = new Date(briefing.value.date + 'T00:00:00')
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
})

const formattedPayday = computed(() => {
  if (!briefing.value?.next_payday) return ''
  const d = new Date(briefing.value.next_payday.pay_date + 'T00:00:00')
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
})

const formattedManagerPayday = computed(() => {
  if (!briefing.value?.manager?.upcoming_payroll) return ''
  const d = new Date(briefing.value.manager.upcoming_payroll.pay_date + 'T00:00:00')
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
})

function formatTime(time: string): string {
  const [h, m] = time.split(':')
  const hour = parseInt(h, 10)
  const ampm = hour >= 12 ? 'PM' : 'AM'
  const displayHour = hour === 0 ? 12 : hour > 12 ? hour - 12 : hour
  return `${displayHour}:${m} ${ampm}`
}

function getDashboardLocation(): Promise<{ lat: string; lng: string } | null> {
  if (!navigator.geolocation) return Promise.resolve(null)
  return new Promise((resolve) => {
    navigator.geolocation.getCurrentPosition(
      (pos) => resolve({ lat: pos.coords.latitude.toFixed(7), lng: pos.coords.longitude.toFixed(7) }),
      () => resolve(null),
      { enableHighAccuracy: true, timeout: 10000, maximumAge: 60000 },
    )
  })
}

async function handleClockIn() {
  clockLoading.value = true
  try {
    const loc = await getDashboardLocation()
    await attendanceAPI.clockIn({ source: 'web', lat: loc?.lat, lng: loc?.lng })
    message.success(t('dashboard.clockInSuccess'))
    clockedIn.value = true
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } }
    message.error(err.data?.error?.message || t('dashboard.clockInFailed'))
  } finally {
    clockLoading.value = false
  }
}

async function handleClockOut() {
  clockLoading.value = true
  try {
    const loc = await getDashboardLocation()
    await attendanceAPI.clockOut({ source: 'web', lat: loc?.lat, lng: loc?.lng })
    message.success(t('dashboard.clockOutSuccess'))
    clockedIn.value = false
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } }
    message.error(err.data?.error?.message || t('dashboard.clockOutFailed'))
  } finally {
    clockLoading.value = false
  }
}

onMounted(async () => {
  try {
    const [briefRes, attRes] = await Promise.allSettled([
      briefingAPI.get(),
      attendanceAPI.getSummary(),
    ])

    if (briefRes.status === 'fulfilled') {
      const raw = briefRes.value as { data?: BriefingData } & BriefingData
      briefing.value = (raw.data ?? raw) as BriefingData
    } else {
      error.value = true
    }

    if (attRes.status === 'fulfilled') {
      const res = attRes.value as { data?: Record<string, unknown> }
      const data = res.data || (res as unknown as Record<string, unknown>)
      if (data.clock_in_at && !data.clock_out_at) {
        clockedIn.value = true
      }
    }
  } catch {
    error.value = true
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div v-if="!error" style="margin-bottom: 24px;">
    <!-- Loading skeleton -->
    <NCard v-if="loading">
      <NSkeleton text style="width: 60%; margin-bottom: 12px;" />
      <NSkeleton text style="width: 40%; margin-bottom: 16px;" />
      <NSpace>
        <NSkeleton :width="100" :height="34" :sharp="false" />
        <NSkeleton :width="120" :height="34" :sharp="false" />
        <NSkeleton :width="110" :height="34" :sharp="false" />
      </NSpace>
    </NCard>

    <!-- Briefing content -->
    <NCard v-else-if="briefing">
      <!-- Header: Greeting + Schedule + Payday -->
      <div style="margin-bottom: 16px;">
        <div style="display: flex; align-items: center; gap: 8px; font-size: 18px; font-weight: 600; margin-bottom: 4px;">
          <span>{{ briefing.greeting }}! {{ t('briefing.todayIs') }} {{ t(`briefing.${(briefing.day_of_week || '').toLowerCase()}`) }}, {{ formattedDate }}</span>
          <NTag size="small" :bordered="false" type="info">{{ t('briefing.aiPowered') }}</NTag>
        </div>
        <div style="font-size: 14px; color: #666; display: flex; flex-wrap: wrap; gap: 16px;">
          <span v-if="briefing.schedule">
            {{ t('briefing.shift') }}: {{ formatTime(briefing.schedule.start_time) }} - {{ formatTime(briefing.schedule.end_time) }}
          </span>
          <span v-if="briefing.next_payday">
            {{ t('briefing.nextPayday') }}: {{ formattedPayday }}
          </span>
        </div>
      </div>

      <!-- Quick Actions -->
      <NSpace style="margin-bottom: 16px;">
        <NButton
          v-if="!clockedIn"
          type="primary"
          :loading="clockLoading"
          @click="handleClockIn"
        >
          {{ t('dashboard.clockIn') }}
        </NButton>
        <template v-else>
          <NTag type="success" size="large">{{ t('dashboard.clockedIn') }}</NTag>
          <NButton type="warning" :loading="clockLoading" @click="handleClockOut">
            {{ t('dashboard.clockOut') }}
          </NButton>
        </template>
        <NButton @click="router.push('/leaves')">
          {{ t('briefing.requestLeave') }}
        </NButton>
        <NButton @click="router.push('/payslips')">
          {{ t('briefing.viewPayslip') }}
        </NButton>
      </NSpace>

      <NDivider style="margin: 12px 0;" />

      <!-- Leave Balances + Notifications row -->
      <NGrid :cols="1" :x-gap="12" :y-gap="12" responsive="screen" item-responsive>
        <NGi span="0:1">
          <div>
            <!-- Leave Balances -->
            <div v-if="briefing.leave_balances && briefing.leave_balances.length > 0" style="margin-bottom: 12px;">
              <div style="font-weight: 600; font-size: 14px; margin-bottom: 8px;">{{ t('briefing.leaveBalances') }}:</div>
              <NSpace>
                <NTag
                  v-for="lb in briefing.leave_balances"
                  :key="lb.code"
                  :type="parseFloat(lb.remaining) <= 2 ? 'warning' : 'success'"
                  round
                >
                  {{ lb.code }}: {{ lb.remaining }} {{ t('briefing.daysRemaining') }}
                </NTag>
              </NSpace>
            </div>

            <!-- Pending expenses + Unread notifications -->
            <NSpace>
              <NBadge
                v-if="briefing.pending_expenses > 0"
                :value="briefing.pending_expenses"
                :max="99"
                type="info"
              >
                <NButton text @click="router.push('/expenses')">
                  {{ t('briefing.pendingExpenses') }}
                </NButton>
              </NBadge>
              <NBadge
                v-if="briefing.unread_notifications > 0"
                :value="briefing.unread_notifications"
                :max="99"
                type="warning"
              >
                <NButton text @click="router.push('/notifications')">
                  {{ t('briefing.unreadNotifications') }}
                </NButton>
              </NBadge>
            </NSpace>
          </div>
        </NGi>
      </NGrid>

      <!-- Manager Section -->
      <template v-if="briefing.manager && (auth.isAdmin || auth.isManager)">
        <NDivider style="margin: 12px 0;" />
        <div>
          <div style="font-weight: 600; font-size: 15px; margin-bottom: 12px;">
            {{ t('briefing.managerActions') }}
          </div>

          <NGrid :cols="1" :x-gap="12" :y-gap="8" responsive="screen" item-responsive>
            <!-- Pending Leave Approvals -->
            <NGi v-if="briefing.manager.pending_leave_approvals > 0" span="0:1">
              <div style="display: flex; align-items: center; justify-content: space-between; padding: 8px 12px; border-radius: 6px; background: var(--n-color-hover, #f5f5f5);">
                <div style="display: flex; align-items: center; gap: 8px;">
                  <NBadge :value="briefing.manager.pending_leave_approvals" :max="99" type="warning" />
                  <span>{{ t('briefing.pendingLeaveApprovals') }}</span>
                </div>
                <NButton text type="primary" @click="router.push('/approvals')">
                  {{ t('briefing.reviewAll') }}
                </NButton>
              </div>
            </NGi>

            <!-- Pending OT Approvals -->
            <NGi v-if="briefing.manager.pending_ot_approvals > 0" span="0:1">
              <div style="display: flex; align-items: center; justify-content: space-between; padding: 8px 12px; border-radius: 6px; background: var(--n-color-hover, #f5f5f5);">
                <div style="display: flex; align-items: center; gap: 8px;">
                  <NBadge :value="briefing.manager.pending_ot_approvals" :max="99" type="warning" />
                  <span>{{ t('briefing.pendingOTApprovals') }}</span>
                </div>
                <NButton text type="primary" @click="router.push('/approvals')">
                  {{ t('briefing.review') }}
                </NButton>
              </div>
            </NGi>

            <!-- Today's Attendance -->
            <NGi span="0:1">
              <div style="display: flex; align-items: center; gap: 12px; padding: 8px 12px; border-radius: 6px; background: var(--n-color-hover, #f5f5f5);">
                <span>
                  {{ t('briefing.todayAttendance') }}: {{ briefing.manager.today_present }}/{{ briefing.manager.today_total }} {{ t('briefing.present') }}
                </span>
                <NTag v-if="briefing.manager.today_late > 0" type="warning" size="small">
                  {{ briefing.manager.today_late }} {{ t('briefing.late') }}
                </NTag>
              </div>
            </NGi>

            <!-- Upcoming Payroll -->
            <NGi v-if="briefing.manager.upcoming_payroll" span="0:1">
              <div style="display: flex; align-items: center; justify-content: space-between; padding: 8px 12px; border-radius: 6px; background: var(--n-color-hover, #f5f5f5);">
                <span>
                  {{ t('briefing.upcomingPayroll') }}: {{ briefing.manager.upcoming_payroll.cycle_name }} ({{ formattedManagerPayday }})
                </span>
                <NButton v-if="auth.isAdmin" text type="primary" @click="router.push('/payroll')">
                  {{ t('common.view') }}
                </NButton>
              </div>
            </NGi>
          </NGrid>

          <!-- Manager Alerts -->
          <div v-if="briefing.manager.alerts && briefing.manager.alerts.length > 0" style="margin-top: 12px;">
            <NAlert
              v-for="(alert, idx) in briefing.manager.alerts"
              :key="idx"
              :type="alert.type === 'consecutive_absence' ? 'warning' : 'info'"
              :title="alert.message"
              style="margin-bottom: 8px;"
              closable
            >
              <NButton
                v-if="alert.type === 'consecutive_absence'"
                text
                type="primary"
                size="small"
                @click="router.push('/attendance/records')"
              >
                {{ t('common.view') }}
              </NButton>
            </NAlert>
          </div>
        </div>
      </template>
    </NCard>
  </div>
</template>
