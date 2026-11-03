<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { NButton, NInput, NSpin, NEmpty, NTag, NProgress, NIcon, NTooltip, useMessage } from 'naive-ui'
import {
  PersonAddOutline, WalletOutline, SparklesOutline, ChevronForwardOutline, CheckmarkCircle,
  AlertCircle, EllipseOutline, TimeOutline, CloseCircle,
} from '@vicons/ionicons5'
import { formatDistanceToNow, format, differenceInCalendarDays } from 'date-fns'
import { homeAPI } from '../api/client'
import { useAuthStore } from '../stores/auth'
import { featureEnabled } from '../edition'
import { inboxRoute, prettyTitle, type InboxItem } from '../composables/inboxRoute'
import { useCurrency } from '../composables/useCurrency'

interface Agency { code: string; name: string; status: string; detail: string; employer_no?: string; employees_registered: number; employees_total: number; link: string }
interface Run { id: number; routine_key: string; trigger: string; subject_name: string; status: string; summary: string; created_at: string }
interface HomeData {
  company: string
  country: string
  headcount: { active: number; starting_soon: number; joined_this_month: number; leaving_soon: number; contractors: number } | null
  todos: InboxItem[]
  todo_total: number
  starts: { id: number; first_name: string; last_name: string; hire_date: string; position: string }[]
  leavers: { employee_id: number; first_name: string; last_name: string; last_working_day: string; separation_type: string }[]
  next_payroll?: { id: number; name: string; period_start: string; period_end: string; pay_date: string; status: string }
  last_payroll?: { name: string; pay_date: string; total_employees: number; total_gross: number; total_net: number }
  filings_overdue: number
  filings: { id: number; filing_type: string; period_year: number; period_month: number | null; period_quarter: number | null; due_date: string; status: string }[]
  registrations?: { country: string; active: number; total: number; counts: Record<string, number>; agencies: Agency[] }
  agent_runs: Run[]
}

const { t } = useI18n()
const router = useRouter()
const auth = useAuthStore()
const message = useMessage()
const { formatCurrency } = useCurrency()
const loading = ref(true)
const data = ref<HomeData | null>(null)
const inviteEmail = ref('')

const firstName = computed(() => auth.user?.first_name || '')
const greeting = computed(() => {
  const h = new Date().getHours()
  return t(h < 12 ? 'home.goodMorning' : h < 18 ? 'home.goodAfternoon' : 'home.goodEvening')
})
const today = format(new Date(), 'EEEE, MMMM d')

async function load() {
  loading.value = true
  try {
    data.value = ((await homeAPI.get()) as { data: HomeData }).data
  } catch {
    message.error(t('common.loadFailed'))
  } finally {
    loading.value = false
  }
}

function go(name: string, query?: Record<string, string>) {
  router.push({ name, query })
}
function openTodo(item: InboxItem) {
  const to = inboxRoute(item)
  router.push(to ?? { name: 'inbox' })
}
function startOnboarding() {
  go('hire', { email: inviteEmail.value.trim(), invite: '1' })
}

const statusTone: Record<string, 'success' | 'warning' | 'error' | 'info' | 'default'> = {
  active: 'success', needs_info: 'error', needs_approval: 'warning', filing_started: 'info', awaiting_agency: 'default',
}
const regPct = computed(() => {
  const r = data.value?.registrations
  return r && r.total ? Math.round((r.active / r.total) * 100) : 0
})

const filingNames: Record<string, string> = {
  bir_1601c: 'BIR 1601-C', bir_1604c: 'BIR 1604-C', bir_2316: 'BIR 2316', bir_0619e: 'BIR 0619-E', bir_1601eq: 'BIR 1601-EQ',
  sss_r3: 'SSS R-3', philhealth_rf1: 'PhilHealth RF-1', pagibig_ml1: 'Pag-IBIG MCRF',
}
function filingName(f: HomeData['filings'][number]) {
  const name = filingNames[f.filing_type] || f.filing_type.replace(/_/g, ' ').toUpperCase()
  const period = f.period_month ? format(new Date(f.period_year, f.period_month - 1, 1), 'MMM yyyy')
    : f.period_quarter ? `Q${f.period_quarter} ${f.period_year}` : String(f.period_year)
  return `${name} · ${period}`
}
function dueIn(date: string) {
  const d = differenceInCalendarDays(new Date(date), new Date())
  if (d < 0) return { text: t('home.overdueBy', { n: -d }), tone: 'error' as const }
  if (d === 0) return { text: t('home.dueToday'), tone: 'warning' as const }
  return { text: t('home.dueIn', { n: d }), tone: d <= 3 ? 'warning' as const : 'default' as const }
}
const payrollIn = computed(() => {
  const p = data.value?.next_payroll
  return p ? differenceInCalendarDays(new Date(p.pay_date), new Date()) : null
})
const routineNames: Record<string, string> = {
  new_hire: 'routines.name_new_hire', offboarding: 'routines.name_offboarding',
  pre_payroll_check: 'routines.name_pre_payroll_check', filing_autopilot: 'routines.name_filing_autopilot',
}
function ago(ts: string) {
  return formatDistanceToNow(new Date(ts), { addSuffix: true })
}

onMounted(load)
</script>

<template>
  <div class="home">
    <header class="home-head">
      <div>
        <div class="home-date">{{ today }}</div>
        <h1 class="home-title">{{ greeting }}<template v-if="firstName">, {{ firstName }}</template></h1>
        <div v-if="data" class="home-sub">
          {{ data.todo_total ? t('home.subtitleTodos', { n: data.todo_total }) : t('home.subtitleClear') }}
        </div>
      </div>
      <div class="home-actions">
        <NButton v-if="auth.isAdmin && featureEnabled('hire')" type="primary" @click="go('hire')">
          <template #icon><NIcon><PersonAddOutline /></NIcon></template>{{ t('home.hire') }}
        </NButton>
        <NButton v-if="auth.isAdmin && featureEnabled('payroll')" @click="go('payroll')">
          <template #icon><NIcon><WalletOutline /></NIcon></template>{{ t('home.runPayroll') }}
        </NButton>
        <NButton v-if="featureEnabled('agent-hub')" secondary @click="go('agent-hub')">
          <template #icon><NIcon><SparklesOutline /></NIcon></template>{{ t('home.askAgent') }}
        </NButton>
      </div>
    </header>

    <NSpin :show="loading">
      <div v-if="data" class="home-body">
        <!-- Headcount strip -->
        <div v-if="data.headcount" class="stats">
          <div class="stat" @click="go('employees')">
            <div class="stat-n">{{ data.headcount.active }}</div><div class="stat-l">{{ t('home.statActive') }}</div>
          </div>
          <div class="stat" @click="go('employees')">
            <div class="stat-n">{{ data.headcount.starting_soon }}</div><div class="stat-l">{{ t('home.statStarting') }}</div>
          </div>
          <div class="stat" @click="go('employees')">
            <div class="stat-n">{{ data.headcount.leaving_soon }}</div><div class="stat-l">{{ t('home.statLeaving') }}</div>
          </div>
          <div v-if="featureEnabled('contractors')" class="stat" @click="go('contractors')">
            <div class="stat-n">{{ data.headcount.contractors }}</div><div class="stat-l">{{ t('home.statContractors') }}</div>
          </div>
        </div>

        <div class="grid">
          <div class="col-main">
            <!-- To-dos -->
            <section v-if="auth.isAdmin && featureEnabled('inbox')" class="panel">
              <div class="panel-head">
                <h2>{{ t('home.todos') }}<span v-if="data.todo_total" class="count">{{ data.todo_total }}</span></h2>
                <NButton text type="primary" @click="go('inbox')">{{ t('home.viewAll') }}</NButton>
              </div>
              <div v-if="!data.todos.length" class="empty-ok">
                <NIcon size="28" color="#18a058"><CheckmarkCircle /></NIcon>
                <div>{{ t('home.allCaughtUp') }}</div>
              </div>
              <ul v-else class="rows">
                <li v-for="item in data.todos" :key="item.kind + item.ref_id + item.title" class="row clickable" @click="openTodo(item)">
                  <span class="dot" :class="item.priority === 'high' ? 'dot-high' : 'dot-normal'" />
                  <div class="row-main">
                    <div class="row-title">{{ prettyTitle(item.title) }}</div>
                    <div class="row-sub">
                      <template v-if="item.employee_name">{{ item.employee_name }} · </template>{{ t('lifecycle.kind_' + item.kind) }}
                    </div>
                  </div>
                  <span v-if="item.due_date" class="row-meta">{{ item.due_date }}</span>
                  <NIcon class="chev"><ChevronForwardOutline /></NIcon>
                </li>
              </ul>
            </section>

            <!-- Onboard a new employee -->
            <section v-if="auth.isAdmin && featureEnabled('hire')" class="panel onboard">
              <div>
                <h2>{{ t('home.onboardTitle') }}</h2>
                <p class="muted">{{ t('home.onboardDesc') }}</p>
              </div>
              <div class="onboard-form">
                <NInput v-model:value="inviteEmail" :placeholder="t('home.onboardEmail')" @keyup.enter="startOnboarding" />
                <NButton type="primary" @click="startOnboarding">{{ t('home.onboardStart') }}</NButton>
              </div>
            </section>

            <!-- Agent activity -->
            <section v-if="auth.isAdmin && featureEnabled('routines')" class="panel">
              <div class="panel-head">
                <h2><NIcon class="spark"><SparklesOutline /></NIcon>{{ t('home.agentActivity') }}</h2>
                <NButton text type="primary" @click="go('routines')">{{ t('home.routines') }}</NButton>
              </div>
              <NEmpty v-if="!data.agent_runs.length" :description="t('home.noAgentRuns')" size="small" />
              <ul v-else class="rows">
                <li v-for="r in data.agent_runs" :key="r.id" class="row clickable" @click="router.push({ name: 'routines', query: { run: String(r.id) } })">
                  <NIcon size="18" :color="r.status === 'completed' ? '#18a058' : r.status === 'failed' ? '#d03050' : '#f0a020'">
                    <CheckmarkCircle v-if="r.status === 'completed'" />
                    <CloseCircle v-else-if="r.status === 'failed'" />
                    <AlertCircle v-else />
                  </NIcon>
                  <div class="row-main">
                    <div class="row-title">{{ t(routineNames[r.routine_key] || r.routine_key) }} — {{ r.trigger }}</div>
                    <div class="row-sub">{{ r.summary }}</div>
                  </div>
                  <span class="row-meta">{{ ago(r.created_at) }}</span>
                </li>
              </ul>
            </section>

            <!-- Starting & leaving -->
            <section v-if="data.starts.length || data.leavers.length" class="panel">
              <div class="panel-head"><h2>{{ t('home.comingAndGoing') }}</h2></div>
              <ul class="rows">
                <li v-for="s in data.starts" :key="'s' + s.id" class="row clickable" @click="router.push({ name: 'employee-detail', params: { id: String(s.id) } })">
                  <NTag size="small" type="success" round>{{ t('home.starts') }}</NTag>
                  <div class="row-main">
                    <div class="row-title">{{ s.first_name }} {{ s.last_name }}</div>
                    <div class="row-sub">{{ s.position || '—' }}</div>
                  </div>
                  <span class="row-meta">{{ format(new Date(s.hire_date), 'MMM d') }}</span>
                </li>
                <li v-for="l in data.leavers" :key="'l' + l.employee_id" class="row clickable" @click="router.push({ name: 'employee-detail', params: { id: String(l.employee_id) } })">
                  <NTag size="small" round>{{ t('home.leaves') }}</NTag>
                  <div class="row-main">
                    <div class="row-title">{{ l.first_name }} {{ l.last_name }}</div>
                    <div class="row-sub">{{ l.separation_type.replace(/_/g, ' ') }}</div>
                  </div>
                  <span class="row-meta">{{ format(new Date(l.last_working_day), 'MMM d') }}</span>
                </li>
              </ul>
            </section>
          </div>

          <div class="col-side">
            <!-- Next payroll -->
            <section v-if="auth.isAdmin && featureEnabled('payroll')" class="panel">
              <div class="panel-head">
                <h2>{{ t('home.nextPayroll') }}</h2>
                <NButton text type="primary" @click="go('payroll')">{{ t('home.open') }}</NButton>
              </div>
              <template v-if="data.next_payroll">
                <div class="payday">
                  <div class="payday-n">{{ payrollIn !== null && payrollIn >= 0 ? payrollIn : 0 }}</div>
                  <div>
                    <div class="row-title">{{ payrollIn === 0 ? t('home.payToday') : t('home.daysToPayday') }}</div>
                    <div class="row-sub">{{ data.next_payroll.name }} · {{ format(new Date(data.next_payroll.pay_date), 'EEE, MMM d') }}</div>
                  </div>
                  <NTag size="small" round :type="data.next_payroll.status === 'approved' ? 'success' : 'warning'">{{ data.next_payroll.status }}</NTag>
                </div>
              </template>
              <div v-else class="muted">{{ t('home.noPayroll') }}</div>
              <div v-if="data.last_payroll" class="last-pay muted">
                {{ t('home.lastPayroll', { name: data.last_payroll.name, n: data.last_payroll.total_employees }) }}
                <strong>{{ formatCurrency(data.last_payroll.total_net) }}</strong>
              </div>
            </section>

            <!-- Registrations -->
            <section v-if="data.registrations && data.registrations.total" class="panel">
              <div class="panel-head">
                <h2>{{ t('home.registrations') }}</h2>
                <span class="muted">{{ t('home.registeredOf', { a: data.registrations.active, b: data.registrations.total }) }}</span>
              </div>
              <NProgress type="line" :percentage="regPct" :show-indicator="false" :height="6" status="success" />
              <div class="pill-counts">
                <NTag v-for="(n, st) in data.registrations.counts" :key="st" size="small" round :type="statusTone[st] || 'default'">
                  {{ t('home.reg_' + st) }} · {{ n }}
                </NTag>
              </div>
              <ul class="rows">
                <li v-for="a in data.registrations.agencies" :key="a.code" class="row clickable" @click="router.push(a.link)">
                  <NIcon :color="a.status === 'active' ? '#18a058' : a.status === 'needs_info' ? '#d03050' : '#f0a020'">
                    <CheckmarkCircle v-if="a.status === 'active'" />
                    <TimeOutline v-else-if="a.status === 'awaiting_agency' || a.status === 'filing_started'" />
                    <EllipseOutline v-else />
                  </NIcon>
                  <div class="row-main">
                    <div class="row-title">{{ a.name }}</div>
                    <div class="row-sub">{{ a.detail }}</div>
                  </div>
                  <NTooltip v-if="a.employees_total">
                    <template #trigger><span class="row-meta">{{ a.employees_registered }}/{{ a.employees_total }}</span></template>
                    {{ t('home.employeesWithId') }}
                  </NTooltip>
                  <NTag size="small" round :type="statusTone[a.status] || 'default'">{{ t('home.reg_' + a.status) }}</NTag>
                </li>
              </ul>
            </section>

            <!-- Upcoming filings -->
            <section v-if="auth.isAdmin && featureEnabled('gov-filings')" class="panel">
              <div class="panel-head">
                <h2>{{ t('home.filings') }}</h2>
                <NButton text type="primary" @click="go('gov-filings')">{{ t('home.open') }}</NButton>
              </div>
              <div v-if="data.filings_overdue" class="overdue" @click="go('tax-filings')">
                <NIcon color="#d03050"><AlertCircle /></NIcon>{{ t('home.overdueTotal', { n: data.filings_overdue }) }}
              </div>
              <div v-if="!data.filings.length" class="muted">{{ t('home.noFilings') }}</div>
              <ul v-else class="rows">
                <li v-for="f in data.filings" :key="f.id" class="row clickable" @click="go('gov-filings')">
                  <div class="row-main">
                    <div class="row-title">{{ filingName(f) }}</div>
                    <div class="row-sub">{{ format(new Date(f.due_date), 'MMM d') }} · {{ f.status }}</div>
                  </div>
                  <NTag size="small" round :type="dueIn(f.due_date).tone">{{ dueIn(f.due_date).text }}</NTag>
                </li>
              </ul>
            </section>
          </div>
        </div>
      </div>
    </NSpin>
  </div>
</template>

<style scoped>
.home { max-width: 1180px; margin: 0 auto; }
.home-head { display: flex; justify-content: space-between; align-items: flex-end; gap: 16px; flex-wrap: wrap; margin-bottom: 20px; }
.home-date { font-size: 13px; color: var(--n-text-color-3, #8a8f98); text-transform: uppercase; letter-spacing: .04em; }
.home-title { font-size: 28px; font-weight: 650; margin: 4px 0; letter-spacing: -.01em; }
.home-sub { color: var(--n-text-color-2, #6b7280); }
.home-actions { display: flex; gap: 8px; flex-wrap: wrap; }
.stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(140px, 1fr)); gap: 12px; margin-bottom: 16px; }
.stat { border: 1px solid rgba(128, 128, 128, .18); border-radius: 12px; padding: 14px 16px; cursor: pointer; transition: border-color .15s; }
.stat:hover { border-color: rgba(128, 128, 128, .4); }
.stat-n { font-size: 24px; font-weight: 650; }
.stat-l { font-size: 13px; opacity: .7; }
.grid { display: grid; grid-template-columns: minmax(0, 1.6fr) minmax(0, 1fr); gap: 16px; align-items: start; }
@media (max-width: 900px) { .grid { grid-template-columns: 1fr; } }
.col-main, .col-side { display: flex; flex-direction: column; gap: 16px; min-width: 0; }
.panel { border: 1px solid rgba(128, 128, 128, .18); border-radius: 12px; padding: 16px; }
.panel h2 { font-size: 15px; font-weight: 600; margin: 0; display: flex; align-items: center; gap: 6px; }
.panel-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; }
.count { font-size: 12px; font-weight: 600; background: rgba(240, 160, 32, .16); color: #c07a00; border-radius: 10px; padding: 1px 8px; margin-left: 6px; }
.spark { color: #7c3aed; }
.rows { list-style: none; margin: 0; padding: 0; }
.row { display: flex; align-items: center; gap: 10px; padding: 9px 6px; border-radius: 8px; }
.row + .row { border-top: 1px solid rgba(128, 128, 128, .1); }
.clickable { cursor: pointer; }
.clickable:hover { background: rgba(128, 128, 128, .07); }
.row-main { flex: 1; min-width: 0; }
.row-title { font-size: 14px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.row-sub { font-size: 12px; opacity: .65; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.row-meta { font-size: 12px; opacity: .65; white-space: nowrap; }
.chev { opacity: .4; }
.dot { width: 8px; height: 8px; border-radius: 50%; flex: none; }
.dot-high { background: #d03050; }
.dot-normal { background: #f0a020; }
.empty-ok { display: flex; align-items: center; gap: 10px; padding: 8px 4px; opacity: .85; }
.muted { opacity: .65; font-size: 13px; }
.onboard { display: flex; justify-content: space-between; align-items: center; gap: 16px; flex-wrap: wrap; background: linear-gradient(135deg, rgba(124, 58, 237, .07), rgba(24, 160, 88, .06)); }
.onboard p { margin: 4px 0 0; }
.onboard-form { display: flex; gap: 8px; min-width: 280px; flex: 1; max-width: 420px; }
.payday { display: flex; align-items: center; gap: 12px; }
.payday-n { font-size: 34px; font-weight: 700; min-width: 48px; text-align: center; }
.payday > div:nth-child(2) { flex: 1; min-width: 0; }
.last-pay { margin-top: 10px; }
.overdue { display: flex; align-items: center; gap: 6px; font-size: 13px; color: #d03050; cursor: pointer; margin-bottom: 6px; }
.pill-counts { display: flex; gap: 6px; flex-wrap: wrap; margin: 10px 0 4px; }
</style>
