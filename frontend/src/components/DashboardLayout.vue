<script setup lang="ts">
import { h, computed, ref, onMounted, onUnmounted, type Component } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  NLayout, NLayoutSider, NLayoutHeader, NLayoutContent,
  NMenu, NIcon, NButton, NSpace, NAvatar, NDropdown, NSwitch,
  NBadge, NPopover, NList, NListItem, NThing, NEmpty, NTag, NTime,
  type MenuOption,
} from 'naive-ui'
import {
  HomeOutline, PeopleOutline, TimeOutline, CalendarOutline,
  AlarmOutline, CheckmarkCircleOutline, WalletOutline, ReceiptOutline,
  BusinessOutline, BriefcaseOutline, SettingsOutline, PersonOutline,
  LogOutOutline, SunnyOutline, MoonOutline, CashOutline, ShieldCheckmarkOutline,
  TodayOutline, ClipboardOutline, RibbonOutline, PersonCircleOutline,
  BarChartOutline, CardOutline, NotificationsOutline, LibraryOutline,
  GridOutline, FileTrayFullOutline, CloudDownloadOutline, BookOutline, CalendarNumberOutline,
  MegaphoneOutline, DocumentTextOutline, SchoolOutline, AlertCircleOutline, MedkitOutline, FolderOpenOutline, ChatbubblesOutline, LocationOutline,
} from '@vicons/ionicons5'
import { useAuthStore } from '../stores/auth'
import { useThemeStore } from '../stores/theme'
import { notificationAPI } from '../api/client'
import ChatPanel from './ChatPanel.vue'

const router = useRouter()
const route = useRoute()
const { t, locale } = useI18n()
const auth = useAuthStore()
const themeStore = useThemeStore()

// Notifications
interface Notification {
  id: number
  title: string
  message: string
  category: string
  entity_type: string | null
  entity_id: number | null
  is_read: boolean
  created_at: string
}

const unreadCount = ref(0)
const notifications = ref<Notification[]>([])
const showNotifications = ref(false)
let pollTimer: ReturnType<typeof setInterval> | null = null

async function fetchUnreadCount() {
  try {
    const res = await notificationAPI.unreadCount()
    const data = (res as any)?.data ?? res
    unreadCount.value = data?.count ?? 0
  } catch { /* ignore */ }
}

async function fetchNotifications() {
  try {
    const res = await notificationAPI.list()
    const data = (res as any)?.data ?? res
    notifications.value = Array.isArray(data) ? data : []
  } catch { /* ignore */ }
}

async function handleMarkRead(id: number) {
  await notificationAPI.markRead(id)
  const n = notifications.value.find(x => x.id === id)
  if (n) n.is_read = true
  unreadCount.value = Math.max(0, unreadCount.value - 1)
}

async function handleMarkAllRead() {
  await notificationAPI.markAllRead()
  notifications.value.forEach(n => { n.is_read = true })
  unreadCount.value = 0
}

async function handleDeleteNotification(id: number) {
  const n = notifications.value.find(x => x.id === id)
  if (n && !n.is_read) unreadCount.value = Math.max(0, unreadCount.value - 1)
  notifications.value = notifications.value.filter(x => x.id !== id)
  await notificationAPI.delete(id)
}

function onNotifPopoverUpdate(visible: boolean) {
  showNotifications.value = visible
  if (visible) fetchNotifications()
}

const categoryColor: Record<string, string> = {
  info: 'default',
  leave: 'success',
  payroll: 'warning',
  performance: 'info',
  onboarding: 'info',
  loan: 'warning',
  approval: 'error',
}

onMounted(() => {
  fetchUnreadCount()
  pollTimer = setInterval(fetchUnreadCount, 30000)
})

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
})

function renderIcon(icon: Component) {
  return () => h(NIcon, null, { default: () => h(icon) })
}

const menuOptions = computed<MenuOption[]>(() => {
  const items: MenuOption[] = [
    { label: t('nav.dashboard'), key: 'dashboard', icon: renderIcon(HomeOutline) },
  ]

  items.push(
    { label: t('nav.announcements'), key: 'announcements', icon: renderIcon(MegaphoneOutline) },
    { label: t('nav.directory'), key: 'directory', icon: renderIcon(BookOutline) },
  )

  if (auth.isAdmin || auth.isManager) {
    items.push({ label: t('nav.employees'), key: 'employees', icon: renderIcon(PeopleOutline) })
  }

  items.push(
    { label: t('nav.attendance'), key: 'attendance', icon: renderIcon(TimeOutline) },
    { label: t('nav.leaves'), key: 'leaves', icon: renderIcon(CalendarOutline) },
    { label: t('nav.leaveCalendar'), key: 'leave-calendar', icon: renderIcon(CalendarNumberOutline) },
    { label: t('nav.leaveEncashment'), key: 'leave-encashment', icon: renderIcon(CashOutline) },
    { label: t('nav.overtime'), key: 'overtime', icon: renderIcon(AlarmOutline) },
  )

  if (auth.isManager) {
    items.push(
      { label: t('nav.attendanceReport'), key: 'attendance-report', icon: renderIcon(DocumentTextOutline) },
      { label: t('nav.dtr'), key: 'dtr', icon: renderIcon(ClipboardOutline) },
      { label: t('nav.schedules'), key: 'schedules', icon: renderIcon(GridOutline) },
      { label: t('nav.approvals'), key: 'approvals', icon: renderIcon(CheckmarkCircleOutline) },
    )
  }

  if (auth.isAdmin) {
    items.push(
      { label: t('nav.payroll'), key: 'payroll', icon: renderIcon(WalletOutline) },
      { label: t('nav.analytics'), key: 'analytics', icon: renderIcon(BarChartOutline) },
    )
  }

  if (auth.isAdmin || auth.isManager) {
    items.push(
      { label: t('nav.onboarding'), key: 'onboarding', icon: renderIcon(ClipboardOutline) },
      { label: t('nav.performance'), key: 'performance', icon: renderIcon(RibbonOutline) },
      { label: t('nav.clearance'), key: 'clearance', icon: renderIcon(DocumentTextOutline) },
    )
  }

  items.push(
    { label: t('nav.training'), key: 'training', icon: renderIcon(SchoolOutline) },
  )

  if (auth.isAdmin || auth.isManager) {
    items.push(
      { label: t('nav.disciplinary'), key: 'disciplinary', icon: renderIcon(AlertCircleOutline) },
      { label: t('nav.milestones'), key: 'milestones', icon: renderIcon(RibbonOutline) },
      { label: t('nav.file201'), key: '201file', icon: renderIcon(FolderOpenOutline) },
    )
  }

  items.push(
    { label: t('nav.payslips'), key: 'payslips', icon: renderIcon(ReceiptOutline) },
    { label: t('nav.loans'), key: 'loans', icon: renderIcon(CardOutline) },
    { label: t('nav.expenses'), key: 'expenses', icon: renderIcon(ReceiptOutline) },
    { label: t('nav.benefits'), key: 'benefits', icon: renderIcon(MedkitOutline) },
    { label: t('nav.grievance'), key: 'grievance', icon: renderIcon(ChatbubblesOutline) },
    { label: t('nav.policies'), key: 'policies', icon: renderIcon(DocumentTextOutline) },
    { label: t('nav.selfService'), key: 'self-service', icon: renderIcon(PersonCircleOutline) },
    { type: 'divider', key: 'd1' },
  )

  if (auth.isAdmin) {
    items.push(
      { label: t('nav.departments'), key: 'departments', icon: renderIcon(BusinessOutline) },
      { label: t('nav.positions'), key: 'positions', icon: renderIcon(BriefcaseOutline) },
      { label: t('nav.salary'), key: 'salary', icon: renderIcon(CashOutline) },
      { label: t('nav.finalPay'), key: 'final-pay', icon: renderIcon(WalletOutline) },
      { label: t('nav.compliance'), key: 'compliance', icon: renderIcon(ShieldCheckmarkOutline) },
      { label: t('nav.taxFilings'), key: 'tax-filings', icon: renderIcon(DocumentTextOutline) },
      { label: t('holiday.title'), key: 'holidays', icon: renderIcon(TodayOutline) },
      { label: t('nav.users'), key: 'users', icon: renderIcon(PeopleOutline) },
      { label: t('nav.knowledge'), key: 'knowledge', icon: renderIcon(LibraryOutline) },
      { label: t('nav.audit'), key: 'audit', icon: renderIcon(FileTrayFullOutline) },
      { label: t('nav.geofences'), key: 'geofences', icon: renderIcon(LocationOutline) },
      { label: t('nav.importExport'), key: 'import-export', icon: renderIcon(CloudDownloadOutline) },
      { label: t('nav.settings'), key: 'settings', icon: renderIcon(SettingsOutline) },
    )
  }

  return items
})

const activeKey = computed(() => route.name as string)

function handleMenuClick(key: string) {
  router.push({ name: key })
}

const userMenuOptions = [
  { label: t('nav.profile'), key: 'profile', icon: renderIcon(PersonOutline) },
  { type: 'divider', key: 'd' },
  { label: t('nav.logout'), key: 'logout', icon: renderIcon(LogOutOutline) },
]

function toggleLocale() {
  const next = locale.value === 'en' ? 'zh' : 'en'
  locale.value = next
  localStorage.setItem('locale', next)
}

function handleUserAction(key: string) {
  if (key === 'logout') {
    auth.logout()
    router.push({ name: 'login' })
  } else if (key === 'profile') {
    router.push({ name: 'profile' })
  }
}
</script>

<template>
  <NLayout has-sider style="min-height: 100vh">
    <NLayoutSider
      bordered
      :width="240"
      :collapsed-width="64"
      show-trigger
      collapse-mode="width"
    >
      <div style="padding: 16px 20px; font-size: 18px; font-weight: 700;">
        AigoNHR
      </div>
      <NMenu
        :options="menuOptions"
        :value="activeKey"
        @update:value="handleMenuClick"
      />
    </NLayoutSider>
    <NLayout>
      <NLayoutHeader bordered style="height: 56px; display: flex; align-items: center; justify-content: flex-end; padding: 0 24px;">
        <NSpace align="center" :size="16">
          <NPopover trigger="click" placement="bottom-end" :show="showNotifications" @update:show="onNotifPopoverUpdate" style="width: 380px; padding: 0;">
            <template #trigger>
              <NBadge :value="unreadCount" :max="99" :show="unreadCount > 0">
                <NButton quaternary circle>
                  <template #icon><NIcon :component="NotificationsOutline" /></template>
                </NButton>
              </NBadge>
            </template>
            <div style="padding: 12px 16px; display: flex; justify-content: space-between; align-items: center; border-bottom: 1px solid var(--n-border-color);">
              <strong>{{ t('notification.title') }}</strong>
              <NButton text size="small" @click="handleMarkAllRead" v-if="unreadCount > 0">{{ t('notification.markAllRead') }}</NButton>
            </div>
            <div style="max-height: 400px; overflow-y: auto;">
              <NEmpty v-if="notifications.length === 0" :description="t('notification.noNotifications')" style="padding: 24px;" />
              <NList v-else hoverable clickable>
                <NListItem v-for="n in notifications" :key="n.id" :style="{ opacity: n.is_read ? 0.6 : 1, background: n.is_read ? 'transparent' : 'var(--n-color-hover)' }">
                  <NThing :title="n.title" :description="n.message" content-style="margin-top: 4px;">
                    <template #header-extra>
                      <NSpace :size="4">
                        <NButton v-if="!n.is_read" text size="tiny" @click.stop="handleMarkRead(n.id)">{{ t('notification.markRead') }}</NButton>
                        <NButton text size="tiny" type="error" @click.stop="handleDeleteNotification(n.id)">{{ t('common.delete') }}</NButton>
                      </NSpace>
                    </template>
                    <template #footer>
                      <NSpace :size="8" align="center">
                        <NTag size="small" :type="(categoryColor[n.category] as any) || 'default'">{{ t(`notification.${n.category}`) || n.category }}</NTag>
                        <NTime :time="new Date(n.created_at)" type="relative" />
                      </NSpace>
                    </template>
                  </NThing>
                </NListItem>
              </NList>
            </div>
            <div style="padding: 8px 16px; border-top: 1px solid var(--n-border-color); text-align: center;">
              <NButton text type="primary" size="small" @click="showNotifications = false; router.push({ name: 'notifications' })">
                {{ t('notification.viewAll') }}
              </NButton>
            </div>
          </NPopover>
          <NButton quaternary size="small" @click="toggleLocale" style="font-weight: 600; min-width: 36px;">
            {{ locale === 'en' ? '中' : 'EN' }}
          </NButton>
          <NSwitch :value="themeStore.isDark" @update:value="themeStore.toggle()">
            <template #checked>
              <NIcon :component="MoonOutline" />
            </template>
            <template #unchecked>
              <NIcon :component="SunnyOutline" />
            </template>
          </NSwitch>
          <NDropdown :options="userMenuOptions" @select="handleUserAction">
            <NButton quaternary>
              <NSpace align="center" :size="8">
                <NAvatar :size="28" round>{{ auth.fullName?.charAt(0) || 'U' }}</NAvatar>
                <span>{{ auth.fullName }}</span>
              </NSpace>
            </NButton>
          </NDropdown>
        </NSpace>
      </NLayoutHeader>
      <NLayoutContent style="padding: 24px;">
        <router-view />
      </NLayoutContent>
    </NLayout>
    <ChatPanel />
  </NLayout>
</template>
