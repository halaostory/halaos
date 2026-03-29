<script setup lang="ts">
import { h, computed, ref, watch, onMounted, onUnmounted, nextTick, type Component } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  NLayout, NLayoutSider, NLayoutHeader, NLayoutContent,
  NMenu, NIcon, NButton, NSpace, NAvatar, NDropdown, NSwitch, NSelect,
  NBadge, NPopover, NList, NListItem, NThing, NEmpty, NTag, NTime,
  type MenuOption,
} from 'naive-ui'
import {
  HomeOutline, PeopleOutline, TimeOutline, CalendarOutline,
  AlarmOutline, CheckmarkCircleOutline, WalletOutline, ReceiptOutline,
  BusinessOutline, BriefcaseOutline, SettingsOutline, PersonOutline,
  LogOutOutline, SunnyOutline, MoonOutline, CashOutline, ShieldCheckmarkOutline, HelpCircleOutline,
  TodayOutline, ClipboardOutline, RibbonOutline, PersonCircleOutline,
  BarChartOutline, CardOutline, NotificationsOutline, LibraryOutline,
  GridOutline, FileTrayFullOutline, CloudDownloadOutline, BookOutline, CalendarNumberOutline,
  MegaphoneOutline, DocumentTextOutline, SchoolOutline, AlertCircleOutline, MedkitOutline, FolderOpenOutline, ChatbubblesOutline, LocationOutline,
  LinkOutline, PulseOutline, GitBranchOutline, TrendingUpOutline,
  FlashOutline, BulbOutline, HappyOutline, StarOutline, OpenOutline,
} from '@vicons/ionicons5'
import { useAuthStore } from '../stores/auth'
import { useThemeStore } from '../stores/theme'
import { notificationAPI, integrationAPI } from '../api/client'
import ChatPanel from './ChatPanel.vue'
import CommandPalette from './CommandPalette.vue'
import NPSModal from './NPSModal.vue'
import { useTour } from '../composables/useTour'

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

const { startTour, autoStartIfNeeded } = useTour()

onMounted(() => {
  fetchUnreadCount()
  pollTimer = setInterval(fetchUnreadCount, 30000)
  autoStartIfNeeded()
})

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
})

function renderIcon(icon: Component) {
  return () => h(NIcon, null, { default: () => h(icon) })
}

function mi(titleKey: string, key: string, icon: Component): MenuOption {
  return { label: t(titleKey), key, icon: renderIcon(icon) }
}

// Phase 1 feature flags — set to true to show in sidebar
const features: Record<string, boolean> = {
  dashboard: true, employees: true, directory: true, attendance: true,
  'attendance-report': true, dtr: true, leaves: true, 'leave-calendar': true,
  payroll: true, payslips: true, 'agent-hub': true, billing: false,
  users: true, departments: true, positions: true, salary: true,
  settings: true, announcements: true, approvals: true, integrations: true,
  overtime: true, 'leave-encashment': true, schedules: true, analytics: true,
  onboarding: true, performance: true, training: true, compliance: true,
  'tax-filings': true, knowledge: true, benefits: true, loans: true,
  expenses: true, disciplinary: true, grievance: true, clearance: true,
  'final-pay': true, '201file': true, milestones: true, geofences: true,
  'import-export': true, audit: true, policies: true, 'self-service': true,
  holidays: true, 'org-intelligence': true, 'workflow-rules': true, accounting: true,
  'workflow-analytics': true, 'workflow-triggers': true, 'workflow-decisions': true,
  'pulse-surveys': true, recognition: true, 'hr-requests': true,
  'virtual-office': true,
  'bot-setup': true,
}

function isEnabled(key: string): boolean {
  return features[key] !== false
}

/** Push item to array if feature is enabled */
function pushIf(arr: MenuOption[], key: string, titleKey: string, icon: Component) {
  if (isEnabled(key)) arr.push(mi(titleKey, key, icon))
}

const menuOptions = computed<MenuOption[]>(() => {
  const groups: MenuOption[] = []
  const isAdminOrManager = auth.isAdmin || auth.isManager

  // ── 1. My Workspace (all users) ──
  const workspace: MenuOption[] = []
  pushIf(workspace, 'dashboard', 'nav.dashboard', HomeOutline)
  pushIf(workspace, 'self-service', 'nav.selfService', PersonCircleOutline)
  pushIf(workspace, 'payslips', 'nav.payslips', ReceiptOutline)
  pushIf(workspace, 'announcements', 'nav.announcements', MegaphoneOutline)
  pushIf(workspace, 'hr-requests', 'nav.hrRequests', FileTrayFullOutline)
  pushIf(workspace, 'bot-setup', 'nav.botSetup', ChatbubblesOutline)
  if (workspace.length) {
    groups.push({ type: 'group', label: t('nav.groupWorkspace'), key: 'g-workspace', children: workspace })
  }

  // ── 2. People (Manager+, Directory visible to all) ──
  const people: MenuOption[] = []
  if (isAdminOrManager) pushIf(people, 'employees', 'nav.employees', PeopleOutline)
  pushIf(people, 'directory', 'nav.directory', BookOutline)
  if (isAdminOrManager) {
    pushIf(people, 'onboarding', 'nav.onboarding', ClipboardOutline)
    pushIf(people, '201file', 'nav.file201', FolderOpenOutline)
  }
  if (people.length) {
    groups.push({ type: 'group', label: t('nav.groupPeople'), key: 'g-people', children: people })
  }

  // ── 3. Time & Attendance ──
  const timeAtt: MenuOption[] = []
  pushIf(timeAtt, 'attendance', 'nav.attendance', TimeOutline)
  pushIf(timeAtt, 'leaves', 'nav.leaves', CalendarOutline)
  pushIf(timeAtt, 'leave-calendar', 'nav.leaveCalendar', CalendarNumberOutline)
  pushIf(timeAtt, 'leave-encashment', 'nav.leaveEncashment', CashOutline)
  pushIf(timeAtt, 'overtime', 'nav.overtime', AlarmOutline)
  if (isAdminOrManager) {
    pushIf(timeAtt, 'schedules', 'nav.schedules', GridOutline)
    pushIf(timeAtt, 'approvals', 'nav.approvals', CheckmarkCircleOutline)
    pushIf(timeAtt, 'attendance-report', 'nav.attendanceReport', DocumentTextOutline)
    pushIf(timeAtt, 'dtr', 'nav.dtr', ClipboardOutline)
  }
  if (timeAtt.length) {
    groups.push({ type: 'group', label: t('nav.groupTimeAttendance'), key: 'g-time', children: timeAtt })
  }

  // ── 4. Payroll & Compensation ──
  const payComp: MenuOption[] = []
  if (auth.isAdmin) pushIf(payComp, 'payroll', 'nav.payroll', WalletOutline)
  pushIf(payComp, 'loans', 'nav.loans', CardOutline)
  pushIf(payComp, 'expenses', 'nav.expenses', ReceiptOutline)
  pushIf(payComp, 'benefits', 'nav.benefits', MedkitOutline)
  if (auth.isAdmin) pushIf(payComp, 'final-pay', 'nav.finalPay', WalletOutline)
  if (auth.isAdmin) pushIf(payComp, 'accounting', 'nav.accounting', OpenOutline)
  if (payComp.length) {
    groups.push({ type: 'group', label: t('nav.groupPayroll'), key: 'g-payroll', children: payComp })
  }

  // ── 5. Talent & Development ──
  const talent: MenuOption[] = []
  if (isAdminOrManager) pushIf(talent, 'performance', 'nav.performance', RibbonOutline)
  pushIf(talent, 'training', 'nav.training', SchoolOutline)
  if (isAdminOrManager) {
    pushIf(talent, 'disciplinary', 'nav.disciplinary', AlertCircleOutline)
    pushIf(talent, 'clearance', 'nav.clearance', DocumentTextOutline)
    pushIf(talent, 'milestones', 'nav.milestones', RibbonOutline)
  }
  pushIf(talent, 'grievance', 'nav.grievance', ChatbubblesOutline)
  if (talent.length) {
    groups.push({ type: 'group', label: t('nav.groupTalent'), key: 'g-talent', children: talent })
  }

  // ── 6. Engagement ──
  const engagement: MenuOption[] = []
  pushIf(engagement, 'recognition', 'nav.recognition', StarOutline)
  pushIf(engagement, 'pulse-surveys', 'nav.pulseSurveys', HappyOutline)
  pushIf(engagement, 'virtual-office', 'nav.virtualOffice', BusinessOutline)
  pushIf(engagement, 'policies', 'nav.policies', DocumentTextOutline)
  if (engagement.length) {
    groups.push({ type: 'group', label: t('nav.groupEngagement'), key: 'g-engagement', children: engagement })
  }

  // ── 7. AI & Insights ──
  const ai: MenuOption[] = []
  pushIf(ai, 'agent-hub', 'nav.agentHub', ChatbubblesOutline)
  if (auth.isAdmin) pushIf(ai, 'analytics', 'nav.analytics', BarChartOutline)
  if (isAdminOrManager) pushIf(ai, 'org-intelligence', 'nav.orgIntelligence', PulseOutline)
  if (ai.length) {
    groups.push({ type: 'group', label: t('nav.groupAI'), key: 'g-ai', children: ai })
  }

  // ── 8. Administration (Admin only) ──
  if (auth.isAdmin) {
    const adminItems: MenuOption[] = []

    // Sub-group: Workflow Automation
    const wfItems: MenuOption[] = []
    pushIf(wfItems, 'workflow-rules', 'nav.workflowRules', GitBranchOutline)
    pushIf(wfItems, 'workflow-triggers', 'nav.workflowTriggers', FlashOutline)
    pushIf(wfItems, 'workflow-analytics', 'nav.workflowAnalytics', TrendingUpOutline)
    pushIf(wfItems, 'workflow-decisions', 'nav.workflowDecisions', BulbOutline)
    if (wfItems.length) {
      adminItems.push({ type: 'group', label: t('nav.groupAdminWorkflow'), key: 'g-admin-wf', children: wfItems })
    }

    // Sub-group: Company Setup
    const companyItems: MenuOption[] = []
    pushIf(companyItems, 'departments', 'nav.departments', BusinessOutline)
    pushIf(companyItems, 'positions', 'nav.positions', BriefcaseOutline)
    pushIf(companyItems, 'salary', 'nav.salary', CashOutline)
    pushIf(companyItems, 'holidays', 'nav.holidays', TodayOutline)
    pushIf(companyItems, 'compliance', 'nav.compliance', ShieldCheckmarkOutline)
    pushIf(companyItems, 'tax-filings', 'nav.taxFilings', DocumentTextOutline)
    if (companyItems.length) {
      adminItems.push({ type: 'group', label: t('nav.groupAdminCompany'), key: 'g-admin-company', children: companyItems })
    }

    // Sub-group: System Tools
    const sysItems: MenuOption[] = []
    pushIf(sysItems, 'users', 'nav.users', PeopleOutline)
    pushIf(sysItems, 'knowledge', 'nav.knowledge', LibraryOutline)
    pushIf(sysItems, 'integrations', 'nav.integrations', LinkOutline)
    pushIf(sysItems, 'geofences', 'nav.geofences', LocationOutline)
    pushIf(sysItems, 'import-export', 'nav.importExport', CloudDownloadOutline)
    pushIf(sysItems, 'audit', 'nav.audit', FileTrayFullOutline)
    pushIf(sysItems, 'billing', 'nav.billing', WalletOutline)
    pushIf(sysItems, 'referrals', 'nav.referrals', GitBranchOutline)
    pushIf(sysItems, 'bot-setup', 'nav.botSetup', ChatbubblesOutline)
    pushIf(sysItems, 'settings', 'nav.settings', SettingsOutline)
    if (sysItems.length) {
      adminItems.push({ type: 'group', label: t('nav.groupAdminSystem'), key: 'g-admin-sys', children: sysItems })
    }

    if (adminItems.length) {
      groups.push({ type: 'group', label: t('nav.groupAdmin'), key: 'g-admin', children: adminItems })
    }
  }

  return groups
})

const activeKey = computed(() => route.name as string)

const companyOptions = computed(() =>
  (auth.companies || []).map((c: { company_name: string; id: number }) => ({ label: c.company_name, value: c.id }))
)
const currentCompanyId = computed(() => auth.user?.company_id)

async function handleSwitchCompany(companyId: number) {
  // Will be implemented in Task 10 (auth store alignment)
  console.log('Switch company to', companyId)
}

const accountingLoading = ref(false)

async function openAccounting() {
  if (accountingLoading.value) return
  accountingLoading.value = true
  try {
    const res = await integrationAPI.getAccountingSSOToken()
    const data = (res as any)?.data ?? res
    const ssoToken = data?.sso_token
    const targetUrl = data?.target_url || 'https://finance.halaos.com'
    if (ssoToken) {
      window.open(`${targetUrl}/sso?token=${encodeURIComponent(ssoToken)}`, '_blank')
    }
  } catch {
    // If no accounting link configured, open AIStarlight login directly
    window.open('https://finance.halaos.com', '_blank')
  } finally {
    accountingLoading.value = false
  }
}

// Preserve sidebar scroll position across route changes
let siderScrollTop = 0
function saveSiderScroll() {
  const el = document.querySelector('.n-layout-sider-scroll-container')
  if (el) siderScrollTop = el.scrollTop
}
watch(() => route.name, () => {
  // Restore after NMenu re-renders (needs RAF + nextTick to be reliable)
  nextTick(() => {
    requestAnimationFrame(() => {
      const el = document.querySelector('.n-layout-sider-scroll-container')
      if (el) el.scrollTop = siderScrollTop
    })
  })
})

function handleMenuClick(key: string) {
  if (key === 'accounting') {
    openAccounting()
    return
  }
  saveSiderScroll()
  router.push({ name: key })
}

const userMenuOptions = [
  { label: t('nav.profile'), key: 'profile', icon: renderIcon(PersonOutline) },
  { label: t('tour.viewTour'), key: 'tour', icon: renderIcon(HelpCircleOutline) },
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
  } else if (key === 'tour') {
    startTour()
  }
}
</script>

<template>
  <NLayout has-sider style="min-height: 100vh">
    <NLayoutSider
      bordered
      :width="260"
      :collapsed-width="64"
      show-trigger
      collapse-mode="width"
      :collapsed="themeStore.sidebarCollapsed"
      @update:collapsed="(val: boolean) => themeStore.sidebarCollapsed = val"
    >
      <div id="app-logo" style="padding: 16px 20px; font-size: 18px; font-weight: 700; display: flex; align-items: center; gap: 8px;">
        HalaOS
        <span style="font-size: 11px; padding: 2px 8px; border-radius: 4px; background: rgba(37, 99, 235, 0.15); color: #3b82f6;">
          {{ auth.user?.company_country || 'PH' }}
        </span>
      </div>
      <div v-if="companyOptions.length > 1 && !themeStore.sidebarCollapsed" style="padding: 0 12px 8px;">
        <NSelect :value="currentCompanyId" :options="companyOptions" size="small" @update:value="handleSwitchCompany" />
      </div>
      <div v-else-if="auth.user && !themeStore.sidebarCollapsed" style="padding: 0 20px 8px; font-size: 13px; color: var(--text-muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis;">
        {{ auth.companyName }}
      </div>
      <NMenu
        id="sidebar-menu"
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
              <NBadge id="header-notifications" :value="unreadCount" :max="99" :show="unreadCount > 0">
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
          <NButton id="header-locale" quaternary size="small" @click="toggleLocale" style="font-weight: 600; min-width: 36px;">
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
          <NDropdown id="header-user" :options="userMenuOptions" @select="handleUserAction">
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
    <CommandPalette />
    <NPSModal />
  </NLayout>
</template>
