<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick, computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  NModal, NInput, NSpin, NCard, NButton, NSpace, NTag, NEmpty,
  NScrollbar,
} from 'naive-ui'
import { commandAPI } from '../api/client'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const { t } = useI18n()
const auth = useAuthStore()

const STORAGE_KEY = 'halaos_recent_commands'
const MAX_RECENT = 5

// State
const visible = ref(false)
const query = ref('')
const loading = ref(false)
const errorMsg = ref('')
const inputRef = ref<InstanceType<typeof NInput> | null>(null)
const selectedIndex = ref(0)

interface CommandAction {
  label: string
  route?: string
  action?: string
  params?: Record<string, unknown>
}

interface CommandResult {
  type: 'action' | 'query' | 'info' | 'navigation'
  title: string
  message: string
  data?: Record<string, unknown>
  actions?: CommandAction[]
}

interface PaletteItem {
  id: string
  type: 'navigation' | 'action' | 'recent' | 'ai'
  label: string
  description: string
  route?: string
  keywords: string[]
}

const aiResult = ref<CommandResult | null>(null)
const recentCommands = ref<string[]>(loadRecent())
const isMac = typeof window !== 'undefined' && window.navigator.platform.toUpperCase().includes('MAC')

// Navigation items — all routable pages
const navigationItems: PaletteItem[] = [
  { id: 'nav-dashboard', type: 'navigation', label: 'Dashboard', description: 'Overview & analytics', route: '/', keywords: ['dashboard', 'home', 'overview'] },
  { id: 'nav-employees', type: 'navigation', label: 'Employees', description: 'Manage employees', route: '/employees', keywords: ['employees', 'staff', 'people', 'team'] },
  { id: 'nav-employee-new', type: 'navigation', label: 'Add Employee', description: 'Create new employee', route: '/employees/new', keywords: ['add', 'new', 'create', 'employee', 'hire'] },
  { id: 'nav-directory', type: 'navigation', label: 'Directory', description: 'Employee directory', route: '/directory', keywords: ['directory', 'contacts', 'search'] },
  { id: 'nav-attendance', type: 'navigation', label: 'Attendance', description: 'Clock in/out', route: '/attendance', keywords: ['attendance', 'clock', 'checkin', 'timekeeping'] },
  { id: 'nav-attendance-records', type: 'navigation', label: 'Attendance Records', description: 'View attendance records', route: '/attendance/records', keywords: ['attendance', 'records', 'history', 'log'] },
  { id: 'nav-attendance-report', type: 'navigation', label: 'Attendance Report', description: 'Attendance analytics', route: '/attendance/report', keywords: ['attendance', 'report', 'analytics'] },
  { id: 'nav-dtr', type: 'navigation', label: 'DTR Report', description: 'Daily time record', route: '/dtr', keywords: ['dtr', 'daily', 'time', 'record'] },
  { id: 'nav-leaves', type: 'navigation', label: 'Leaves', description: 'Leave requests', route: '/leaves', keywords: ['leaves', 'leave', 'vacation', 'sick', 'absence'] },
  { id: 'nav-leave-calendar', type: 'navigation', label: 'Leave Calendar', description: 'Team leave calendar', route: '/leave-calendar', keywords: ['leave', 'calendar', 'schedule'] },
  { id: 'nav-leave-encashment', type: 'navigation', label: 'Leave Encashment', description: 'Convert leave to cash', route: '/leave-encashment', keywords: ['leave', 'encashment', 'cash', 'convert'] },
  { id: 'nav-overtime', type: 'navigation', label: 'Overtime', description: 'OT requests', route: '/overtime', keywords: ['overtime', 'ot', 'extra', 'hours'] },
  { id: 'nav-approvals', type: 'navigation', label: 'Approvals', description: 'Pending approvals', route: '/approvals', keywords: ['approvals', 'approve', 'pending', 'review'] },
  { id: 'nav-payroll', type: 'navigation', label: 'Payroll', description: 'Run payroll', route: '/payroll', keywords: ['payroll', 'salary', 'pay', 'wages'] },
  { id: 'nav-payslips', type: 'navigation', label: 'My Payslips', description: 'View payslips', route: '/payslips', keywords: ['payslips', 'payslip', 'salary', 'slip'] },
  { id: 'nav-onboarding', type: 'navigation', label: 'Onboarding', description: 'New hire onboarding', route: '/onboarding', keywords: ['onboarding', 'new', 'hire', 'welcome'] },
  { id: 'nav-schedules', type: 'navigation', label: 'Schedules', description: 'Work schedules', route: '/schedules', keywords: ['schedules', 'shift', 'roster', 'timetable'] },
  { id: 'nav-loans', type: 'navigation', label: 'Loans', description: 'Employee loans', route: '/loans', keywords: ['loans', 'loan', 'borrow', 'advance'] },
  { id: 'nav-expenses', type: 'navigation', label: 'Expenses', description: 'Expense claims', route: '/expenses', keywords: ['expenses', 'expense', 'reimbursement', 'claim'] },
  { id: 'nav-analytics', type: 'navigation', label: 'Analytics', description: 'HR analytics', route: '/analytics', keywords: ['analytics', 'reports', 'data', 'insights'] },
  { id: 'nav-org-intelligence', type: 'navigation', label: 'Org Intelligence', description: 'AI organization insights', route: '/org-intelligence', keywords: ['org', 'intelligence', 'ai', 'insights'] },
  { id: 'nav-performance', type: 'navigation', label: 'Performance', description: 'Performance reviews', route: '/performance', keywords: ['performance', 'review', 'evaluation', 'kpi'] },
  { id: 'nav-training', type: 'navigation', label: 'Training', description: 'Training & certifications', route: '/training', keywords: ['training', 'certification', 'course', 'learning'] },
  { id: 'nav-knowledge', type: 'navigation', label: 'Knowledge Base', description: 'HR knowledge base', route: '/knowledge', keywords: ['knowledge', 'base', 'wiki', 'docs'] },
  { id: 'nav-self-service', type: 'navigation', label: 'My Dashboard', description: 'Employee self-service', route: '/self-service', keywords: ['self', 'service', 'my', 'dashboard'] },
  { id: 'nav-departments', type: 'navigation', label: 'Departments', description: 'Manage departments', route: '/departments', keywords: ['departments', 'dept', 'division'] },
  { id: 'nav-positions', type: 'navigation', label: 'Positions', description: 'Job positions', route: '/positions', keywords: ['positions', 'job', 'title', 'role'] },
  { id: 'nav-salary', type: 'navigation', label: 'Salary Config', description: 'Salary configuration', route: '/salary', keywords: ['salary', 'config', 'compensation', 'wage'] },
  { id: 'nav-compliance', type: 'navigation', label: 'Compliance', description: 'Regulatory compliance', route: '/compliance', keywords: ['compliance', 'regulatory', 'legal', 'government'] },
  { id: 'nav-tax-filings', type: 'navigation', label: 'Tax Filings', description: 'Tax filing management', route: '/tax-filings', keywords: ['tax', 'filing', 'bir', 'government'] },
  { id: 'nav-benefits', type: 'navigation', label: 'Benefits', description: 'Employee benefits', route: '/benefits', keywords: ['benefits', 'sss', 'philhealth', 'pagibig'] },
  { id: 'nav-policies', type: 'navigation', label: 'Policies', description: 'Company policies', route: '/policies', keywords: ['policies', 'policy', 'rules', 'handbook'] },
  { id: 'nav-holidays', type: 'navigation', label: 'Holidays', description: 'Holiday calendar', route: '/holidays', keywords: ['holidays', 'holiday', 'calendar'] },
  { id: 'nav-announcements', type: 'navigation', label: 'Announcements', description: 'Company announcements', route: '/announcements', keywords: ['announcements', 'news', 'notice'] },
  { id: 'nav-grievance', type: 'navigation', label: 'Grievance', description: 'Employee grievances', route: '/grievance', keywords: ['grievance', 'complaint', 'issue'] },
  { id: 'nav-disciplinary', type: 'navigation', label: 'Disciplinary', description: 'Disciplinary actions', route: '/disciplinary', keywords: ['disciplinary', 'warning', 'violation'] },
  { id: 'nav-milestones', type: 'navigation', label: 'Milestones', description: 'Contract milestones', route: '/milestones', keywords: ['milestones', 'contract', 'anniversary'] },
  { id: 'nav-clearance', type: 'navigation', label: 'Clearance', description: 'Employee clearance', route: '/clearance', keywords: ['clearance', 'exit', 'offboarding'] },
  { id: 'nav-final-pay', type: 'navigation', label: 'Final Pay', description: 'Final pay computation', route: '/final-pay', keywords: ['final', 'pay', 'last', 'separation'] },
  { id: 'nav-201file', type: 'navigation', label: '201 File', description: 'Employee documents', route: '/201file', keywords: ['201', 'file', 'documents', 'records'] },
  { id: 'nav-audit', type: 'navigation', label: 'Audit Trail', description: 'System audit log', route: '/audit', keywords: ['audit', 'trail', 'log', 'history'] },
  { id: 'nav-import-export', type: 'navigation', label: 'Import / Export', description: 'Data import & export', route: '/import-export', keywords: ['import', 'export', 'csv', 'data'] },
  { id: 'nav-geofences', type: 'navigation', label: 'Geofencing', description: 'Location geofences', route: '/geofences', keywords: ['geofencing', 'geofence', 'location', 'gps'] },
  { id: 'nav-users', type: 'navigation', label: 'User Management', description: 'Manage system users', route: '/users', keywords: ['users', 'user', 'management', 'accounts'] },
  { id: 'nav-settings', type: 'navigation', label: 'Settings', description: 'System settings', route: '/settings', keywords: ['settings', 'config', 'preferences'] },
  { id: 'nav-profile', type: 'navigation', label: 'My Profile', description: 'View & edit profile', route: '/profile', keywords: ['profile', 'my', 'account', 'personal'] },
  { id: 'nav-billing', type: 'navigation', label: 'Billing', description: 'Billing & subscription', route: '/billing', keywords: ['billing', 'subscription', 'plan', 'payment'] },
  { id: 'nav-agent-hub', type: 'navigation', label: 'AI Agents', description: 'AI agent hub', route: '/agent-hub', keywords: ['agent', 'ai', 'hub', 'bot'] },
  { id: 'nav-integrations', type: 'navigation', label: 'Integrations', description: 'Third-party integrations', route: '/integrations', keywords: ['integrations', 'connect', 'api', 'sync'] },
  { id: 'nav-workflow-rules', type: 'navigation', label: 'Workflow Rules', description: 'Automation rules', route: '/workflow-rules', keywords: ['workflow', 'rules', 'automation'] },
  { id: 'nav-workflow-triggers', type: 'navigation', label: 'Workflow Triggers', description: 'Event triggers', route: '/workflow-triggers', keywords: ['workflow', 'triggers', 'event', 'automation'] },
  { id: 'nav-workflow-decisions', type: 'navigation', label: 'AI Decisions', description: 'AI decision log', route: '/workflow-decisions', keywords: ['workflow', 'decisions', 'ai', 'agent'] },
  { id: 'nav-workflow-analytics', type: 'navigation', label: 'Workflow Analytics', description: 'Workflow analytics', route: '/workflow-analytics', keywords: ['workflow', 'analytics', 'stats'] },
  { id: 'nav-notifications', type: 'navigation', label: 'Notifications', description: 'View notifications', route: '/notifications', keywords: ['notifications', 'alerts', 'messages'] },
]

// Quick action items
const quickActions: PaletteItem[] = [
  { id: 'act-clock-in', type: 'action', label: 'Clock In', description: 'Record attendance', route: '/attendance', keywords: ['clock', 'in', 'checkin', 'punch'] },
  { id: 'act-request-leave', type: 'action', label: 'Request Leave', description: 'Submit leave request', route: '/leaves', keywords: ['request', 'leave', 'vacation', 'sick', 'absence', 'file'] },
  { id: 'act-request-ot', type: 'action', label: 'Request Overtime', description: 'Submit OT request', route: '/overtime', keywords: ['request', 'overtime', 'ot', 'extra'] },
  { id: 'act-approve', type: 'action', label: 'Review Approvals', description: 'Approve pending requests', route: '/approvals', keywords: ['approve', 'review', 'pending', 'reject'] },
  { id: 'act-run-payroll', type: 'action', label: 'Run Payroll', description: 'Process payroll', route: '/payroll', keywords: ['run', 'payroll', 'process', 'salary'] },
  { id: 'act-add-employee', type: 'action', label: 'Add Employee', description: 'Onboard new employee', route: '/employees/new', keywords: ['add', 'new', 'employee', 'hire', 'onboard'] },
  { id: 'act-view-payslip', type: 'action', label: 'View My Payslip', description: 'Check latest payslip', route: '/payslips', keywords: ['view', 'payslip', 'salary', 'my'] },
  { id: 'act-file-expense', type: 'action', label: 'File Expense', description: 'Submit expense claim', route: '/expenses', keywords: ['file', 'expense', 'claim', 'reimbursement'] },
]

// All static items combined
const allItems = [...quickActions, ...navigationItems]

// Fuzzy match score
function matchScore(item: PaletteItem, q: string): number {
  const lower = q.toLowerCase()
  const labelLower = item.label.toLowerCase()
  const descLower = item.description.toLowerCase()

  // Exact label match
  if (labelLower === lower) return 100
  // Label starts with query
  if (labelLower.startsWith(lower)) return 90
  // Label contains query
  if (labelLower.includes(lower)) return 80
  // Description contains query
  if (descLower.includes(lower)) return 60
  // Keyword match
  const words = lower.split(/\s+/)
  const keywordHits = words.filter(w =>
    item.keywords.some(k => k.startsWith(w)) ||
    labelLower.includes(w) ||
    descLower.includes(w)
  )
  if (keywordHits.length === words.length) return 70
  if (keywordHits.length > 0) return 40 + (keywordHits.length / words.length) * 20
  return 0
}

// Role-based filtering
function isAccessible(item: PaletteItem): boolean {
  if (!auth.user) return false
  const role = auth.user.role
  const adminOnly = [
    'nav-departments', 'nav-positions', 'nav-salary', 'nav-compliance',
    'nav-tax-filings', 'nav-holidays', 'nav-users', 'nav-settings',
    'nav-billing', 'nav-import-export', 'nav-geofences', 'nav-knowledge',
    'nav-audit', 'nav-analytics', 'nav-payroll', 'nav-workflow-rules',
    'nav-workflow-triggers', 'nav-employee-new', 'nav-integrations',
    'act-run-payroll', 'act-add-employee',
  ]
  const managerPlus = [
    'nav-employees', 'nav-approvals', 'nav-attendance-report', 'nav-dtr',
    'nav-onboarding', 'nav-schedules', 'nav-performance', 'nav-milestones',
    'nav-disciplinary', 'nav-clearance', 'nav-final-pay', 'nav-201file',
    'nav-org-intelligence', 'nav-workflow-decisions', 'nav-workflow-analytics',
    'act-approve',
  ]
  if (adminOnly.includes(item.id)) {
    return role === 'super_admin' || role === 'admin'
  }
  if (managerPlus.includes(item.id)) {
    return role === 'super_admin' || role === 'admin' || role === 'manager'
  }
  return true
}

// Filtered results based on query
const filteredItems = computed<PaletteItem[]>(() => {
  const q = query.value.trim()
  if (!q) {
    // Show quick actions + recent
    const recents: PaletteItem[] = recentCommands.value.map((cmd, i) => ({
      id: `recent-${i}`,
      type: 'recent' as const,
      label: cmd,
      description: 'Recent command',
      keywords: [],
    }))
    const accessible = quickActions.filter(isAccessible)
    return [...recents, ...accessible].slice(0, 10)
  }

  // Score and sort all items
  const scored = allItems
    .filter(isAccessible)
    .map(item => ({ item, score: matchScore(item, q) }))
    .filter(({ score }) => score > 0)
    .sort((a, b) => b.score - a.score)
    .slice(0, 10)
    .map(({ item }) => item)

  return scored
})

// Reset selected index when filtered items change
watch(filteredItems, () => {
  selectedIndex.value = 0
})

// Type colors
type TagType = 'info' | 'success' | 'error' | 'warning' | 'default'
const typeColors: Record<string, TagType> = {
  navigation: 'info',
  action: 'success',
  recent: 'default',
  ai: 'warning',
}

// LocalStorage helpers
function loadRecent(): string[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed.slice(0, MAX_RECENT) : []
  } catch {
    return []
  }
}

function saveRecent(commands: readonly string[]): void {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(commands))
}

function addToRecent(cmd: string): void {
  const trimmed = cmd.trim()
  if (!trimmed) return
  const filtered = recentCommands.value.filter(c => c !== trimmed)
  const updated = [trimmed, ...filtered].slice(0, MAX_RECENT)
  recentCommands.value = updated
  saveRecent(updated)
}

// Open/Close
function open(): void {
  visible.value = true
  aiResult.value = null
  errorMsg.value = ''
  query.value = ''
  selectedIndex.value = 0
  loading.value = false
  nextTick(() => {
    inputRef.value?.focus()
  })
}

function close(): void {
  visible.value = false
  aiResult.value = null
  errorMsg.value = ''
  query.value = ''
  loading.value = false
}

// Execute selected item
function executeItem(item: PaletteItem): void {
  if (item.type === 'recent') {
    query.value = item.label
    executeAI()
    return
  }
  if (item.route) {
    close()
    router.push(item.route)
  }
}

// Execute AI command
async function executeAI(): Promise<void> {
  const trimmed = query.value.trim()
  if (!trimmed || loading.value) return

  loading.value = true
  errorMsg.value = ''
  aiResult.value = null

  try {
    const res = await commandAPI.execute(trimmed)
    const data = (res as any)?.data ?? res
    const commandResult = data?.result ?? data
    aiResult.value = commandResult as CommandResult
    addToRecent(trimmed)
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : String(err)
    errorMsg.value = message || t('commandPalette.executeFailed')
  } finally {
    loading.value = false
  }
}

// Handle Enter key
function handleEnter(): void {
  if (aiResult.value) return // Already showing AI result

  const items = filteredItems.value
  if (items.length > 0 && selectedIndex.value < items.length) {
    executeItem(items[selectedIndex.value])
  } else {
    // No local match — ask AI
    executeAI()
  }
}

// Handle action button click in AI result
function handleAction(action: CommandAction): void {
  if (action.route) {
    close()
    router.push(action.route)
  }
}

// Keyboard navigation
function handleInputKeydown(e: KeyboardEvent): void {
  const items = filteredItems.value
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    selectedIndex.value = Math.min(selectedIndex.value + 1, items.length - 1)
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    selectedIndex.value = Math.max(selectedIndex.value - 1, 0)
  } else if (e.key === 'Tab') {
    e.preventDefault()
    // Tab cycles through items
    selectedIndex.value = (selectedIndex.value + 1) % Math.max(items.length, 1)
  }
}

// Global keyboard shortcut handler
function handleKeyDown(e: KeyboardEvent): void {
  const modifier = isMac ? e.metaKey : e.ctrlKey

  if (modifier && e.key === 'k') {
    e.preventDefault()
    if (visible.value) {
      close()
    } else {
      open()
    }
  }

  if (e.key === 'Escape' && visible.value) {
    e.preventDefault()
    close()
  }
}

onMounted(() => {
  document.addEventListener('keydown', handleKeyDown)
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleKeyDown)
})

// Expose open for external trigger (mobile FAB)
defineExpose({ open })
</script>

<template>
  <!-- Mobile floating button -->
  <div class="command-palette-fab" @click="open">
    <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <circle cx="11" cy="11" r="8" />
      <line x1="21" y1="21" x2="16.65" y2="16.65" />
    </svg>
  </div>

  <NModal
    v-model:show="visible"
    :mask-closable="true"
    :close-on-esc="false"
    :auto-focus="false"
    transform-origin="center"
    @after-leave="close"
  >
    <div class="command-palette">
      <!-- Search input -->
      <div class="command-palette__header">
        <NInput
          ref="inputRef"
          v-model:value="query"
          size="large"
          :placeholder="t('commandPalette.placeholder')"
          clearable
          @keyup.enter="handleEnter"
          @keydown="handleInputKeydown"
        >
          <template #prefix>
            <svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="opacity: 0.5;">
              <circle cx="11" cy="11" r="8" />
              <line x1="21" y1="21" x2="16.65" y2="16.65" />
            </svg>
          </template>
          <template #suffix>
            <NTag size="small" :bordered="false" style="font-size: 11px; opacity: 0.6;">
              {{ isMac ? 'Cmd+K' : 'Ctrl+K' }}
            </NTag>
          </template>
        </NInput>
      </div>

      <!-- Instant results list -->
      <div v-if="!loading && !aiResult && !errorMsg && filteredItems.length > 0" class="command-palette__list">
        <NScrollbar style="max-height: 360px">
          <div
            v-for="(item, idx) in filteredItems"
            :key="item.id"
            class="command-palette__item"
            :class="{ 'command-palette__item--selected': idx === selectedIndex }"
            @click="executeItem(item)"
            @mouseenter="selectedIndex = idx"
          >
            <div class="command-palette__item-left">
              <NTag :type="typeColors[item.type] || 'default'" size="tiny" style="min-width: 52px; text-align: center;">
                {{ item.type === 'navigation' ? 'Go to' : item.type === 'action' ? 'Action' : item.type === 'recent' ? 'Recent' : 'AI' }}
              </NTag>
              <div class="command-palette__item-text">
                <span class="command-palette__item-label">{{ item.label }}</span>
                <span class="command-palette__item-desc">{{ item.description }}</span>
              </div>
            </div>
            <span class="command-palette__item-shortcut">Enter</span>
          </div>
        </NScrollbar>
      </div>

      <!-- AI hint when items shown -->
      <div v-if="!loading && !aiResult && !errorMsg && query.trim() && filteredItems.length > 0" class="command-palette__ai-hint">
        <NButton text size="small" @click="executeAI" style="opacity: 0.6; font-size: 12px;">
          {{ t('commandPalette.askAI') }}
        </NButton>
      </div>

      <!-- Loading -->
      <div v-if="loading" class="command-palette__loading">
        <NSpin size="medium" />
        <span style="margin-left: 8px; opacity: 0.7;">{{ t('commandPalette.processing') }}</span>
      </div>

      <!-- Error -->
      <div v-if="errorMsg" class="command-palette__error">
        <NCard size="small">
          <div style="color: var(--n-text-color-error, #d03050);">{{ errorMsg }}</div>
        </NCard>
      </div>

      <!-- AI Result -->
      <div v-if="aiResult" class="command-palette__result">
        <NCard size="small" :bordered="true">
          <template #header>
            <NSpace align="center" :size="8">
              <NTag :type="typeColors[aiResult.type] || 'info'" size="small">
                {{ t(`commandPalette.type_${aiResult.type}`) }}
              </NTag>
              <strong>{{ aiResult.title }}</strong>
            </NSpace>
          </template>
          <div class="command-palette__message">{{ aiResult.message }}</div>
          <template #action v-if="aiResult.actions && aiResult.actions.length > 0">
            <NSpace :size="8">
              <NButton
                v-for="(act, idx) in aiResult.actions"
                :key="idx"
                size="small"
                :type="idx === 0 ? 'primary' : 'default'"
                @click="handleAction(act)"
              >
                {{ act.label }}
              </NButton>
            </NSpace>
          </template>
        </NCard>
      </div>

      <!-- Empty state hint -->
      <div v-if="!loading && !aiResult && !errorMsg && filteredItems.length === 0 && !query.trim()" class="command-palette__hint">
        <NEmpty :description="t('commandPalette.hint')" size="small" />
      </div>

      <!-- No local match — suggest AI -->
      <div v-if="!loading && !aiResult && !errorMsg && filteredItems.length === 0 && query.trim()" class="command-palette__no-match">
        <div style="text-align: center; padding: 16px;">
          <div style="opacity: 0.6; margin-bottom: 8px;">{{ t('commandPalette.noMatch') }}</div>
          <NButton type="primary" size="small" @click="executeAI">
            {{ t('commandPalette.askAI') }}
          </NButton>
        </div>
      </div>

      <!-- Footer hint -->
      <div class="command-palette__footer">
        <span>Arrow keys to navigate</span>
        <span>Enter to select</span>
        <span>Esc to close</span>
      </div>
    </div>
  </NModal>
</template>

<style scoped>
.command-palette {
  width: 600px;
  max-width: 95vw;
  background: var(--n-color, #fff);
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 16px 48px rgba(0, 0, 0, 0.2);
}

.command-palette__header {
  padding: 16px 16px 12px;
}

.command-palette__list {
  border-top: 1px solid var(--n-border-color, #e0e0e6);
}

.command-palette__item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 16px;
  cursor: pointer;
  transition: background-color 0.15s;
}

.command-palette__item:hover,
.command-palette__item--selected {
  background: var(--n-color-hover, rgba(0, 0, 0, 0.04));
}

.command-palette__item-left {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.command-palette__item-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.command-palette__item-label {
  font-size: 14px;
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.command-palette__item-desc {
  font-size: 12px;
  opacity: 0.5;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.command-palette__item-shortcut {
  font-size: 11px;
  opacity: 0;
  padding: 2px 6px;
  border-radius: 4px;
  background: var(--n-border-color, #e0e0e6);
  flex-shrink: 0;
}

.command-palette__item--selected .command-palette__item-shortcut,
.command-palette__item:hover .command-palette__item-shortcut {
  opacity: 0.6;
}

.command-palette__ai-hint {
  text-align: center;
  padding: 4px 16px 8px;
  border-top: 1px solid var(--n-border-color, #e0e0e6);
}

.command-palette__loading {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px 16px;
  border-top: 1px solid var(--n-border-color, #e0e0e6);
}

.command-palette__error {
  padding: 12px 16px 16px;
  border-top: 1px solid var(--n-border-color, #e0e0e6);
}

.command-palette__result {
  padding: 12px 16px 16px;
  border-top: 1px solid var(--n-border-color, #e0e0e6);
}

.command-palette__message {
  white-space: pre-wrap;
  line-height: 1.6;
}

.command-palette__hint,
.command-palette__no-match {
  border-top: 1px solid var(--n-border-color, #e0e0e6);
}

.command-palette__hint {
  padding: 16px;
}

.command-palette__footer {
  display: flex;
  justify-content: center;
  gap: 16px;
  padding: 8px 16px;
  font-size: 11px;
  opacity: 0.4;
  border-top: 1px solid var(--n-border-color, #e0e0e6);
}

/* Mobile: full-screen modal */
@media (max-width: 640px) {
  .command-palette {
    width: 100vw;
    max-width: 100vw;
    height: 100vh;
    max-height: 100vh;
    border-radius: 0;
    display: flex;
    flex-direction: column;
  }

  .command-palette__list,
  .command-palette__result,
  .command-palette__loading,
  .command-palette__error,
  .command-palette__hint,
  .command-palette__no-match {
    flex: 1;
    overflow-y: auto;
  }
}

/* Mobile FAB - only show on small screens */
.command-palette-fab {
  display: none;
  position: fixed;
  bottom: 80px;
  left: 20px;
  z-index: 100;
  width: 48px;
  height: 48px;
  border-radius: 50%;
  background: var(--n-color-primary, #2563eb);
  color: #fff;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  box-shadow: 0 4px 12px rgba(37, 99, 235, 0.4);
  transition: transform 0.2s, box-shadow 0.2s;
}

.command-palette-fab:hover {
  transform: scale(1.1);
  box-shadow: 0 6px 16px rgba(37, 99, 235, 0.5);
}

@media (max-width: 640px) {
  .command-palette-fab {
    display: flex;
  }
}
</style>
