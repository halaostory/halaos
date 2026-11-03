<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  NButton, NSpin, NSwitch, NRadioGroup, NRadioButton, NInput, NTag, NDrawer, NDrawerContent, NTimeline,
  NTimelineItem, NEmpty, NIcon, NTooltip, NCheckbox, NCheckboxGroup, useMessage,
} from 'naive-ui'
import { SparklesOutline, LockClosedOutline, FlashOutline, TimeOutline } from '@vicons/ionicons5'
import { formatDistanceToNow, format } from 'date-fns'
import { routinesAPI, alertsAPI } from '../api/client'

interface RunRow {
  id: number; routine_key: string; trigger: string; subject_type: string; subject_id: number; subject_name: string
  autonomy: string; status: string; summary: string; actions: Action[]; created_at: string
}
interface Action { key: string; title: string; status: string; rationale?: string; link?: string }
interface Routine {
  key: string; name: string; description: string; trigger: string; trigger_type: string; steps: string[]
  hard_limits: string[]; default_autonomy: string; max_autonomy: string
  enabled: boolean; autonomy: string; instructions: string
  last_run: RunRow | null; runs_30d: number; needs_attention_30d: number
}

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const message = useMessage()
const loading = ref(true)
const routines = ref<Routine[]>([])
const drafts = ref<Record<string, string>>({})
const saving = ref<Record<string, boolean>>({})

const drawer = ref(false)
const drawerTitle = ref('')
const runs = ref<RunRow[]>([])
const runsLoading = ref(false)
const selected = ref<RunRow | null>(null)

// Slack alerts
interface SlackView { connected: boolean; hint: string; events: string[]; enabled: boolean; all_events: string[]; can_store: boolean }
const slack = ref<SlackView | null>(null)
const slackUrl = ref('')
const slackBusy = ref(false)
async function loadSlack() {
  try { slack.value = ((await alertsAPI.get()) as { data: SlackView }).data } catch { slack.value = null }
}
async function saveSlack(patch: { webhook_url?: string; events?: string[]; enabled?: boolean }) {
  slackBusy.value = true
  try {
    slack.value = ((await alertsAPI.save(patch)) as { data: SlackView }).data
    slackUrl.value = ''
    message.success(t('routines.saved'))
  } catch (e) {
    message.error((e as Error).message)
  } finally {
    slackBusy.value = false
  }
}
async function testSlack() {
  slackBusy.value = true
  try { await alertsAPI.test(); message.success(t('slack.testSent')) } catch (e) { message.error((e as Error).message) } finally { slackBusy.value = false }
}
async function disconnectSlack() {
  slackBusy.value = true
  try { slack.value = ((await alertsAPI.disconnect()) as { data: SlackView }).data } finally { slackBusy.value = false }
}

const levels = ['recommend', 'approve', 'autonomous'] as const
const rank: Record<string, number> = { recommend: 0, approve: 1, autonomous: 2 }

async function load() {
  loading.value = true
  try {
    routines.value = ((await routinesAPI.list()) as { data: Routine[] }).data
    for (const r of routines.value) drafts.value[r.key] = r.instructions
  } catch {
    message.error(t('common.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function save(r: Routine, patch: { enabled?: boolean; autonomy?: string; instructions?: string }) {
  saving.value[r.key] = true
  try {
    await routinesAPI.update(r.key, patch)
    Object.assign(r, patch)
    message.success(t('routines.saved'))
  } catch (e) {
    message.error((e as Error).message || t('common.saveFailed'))
  } finally {
    saving.value[r.key] = false
  }
}

async function openRuns(r: Routine | null) {
  drawerTitle.value = r ? t('routines.name_' + r.key) : t('routines.allRuns')
  drawer.value = true
  runsLoading.value = true
  selected.value = null
  try {
    const params: Record<string, string> = { limit: '50' }
    if (r) params.routine = r.key
    runs.value = ((await routinesAPI.runs(params)) as { data: RunRow[] }).data
    selected.value = runs.value[0] || null
  } finally {
    runsLoading.value = false
  }
}

async function openRun(id: number) {
  drawer.value = true
  drawerTitle.value = t('routines.runLog')
  runsLoading.value = true
  try {
    const run = ((await routinesAPI.run(id)) as { data: RunRow }).data
    runs.value = [run]
    selected.value = run
  } catch {
    message.error(t('common.loadFailed'))
  } finally {
    runsLoading.value = false
  }
}

const statusTone: Record<string, 'success' | 'warning' | 'error' | 'info' | 'default'> = {
  completed: 'success', needs_attention: 'warning', failed: 'error',
}
const actionTone: Record<string, 'success' | 'warning' | 'error' | 'info' | 'default'> = {
  done: 'success', person: 'warning', proposed: 'info', failed: 'error', waiting: 'default', skipped: 'default',
}
function ago(ts: string) {
  return formatDistanceToNow(new Date(ts), { addSuffix: true })
}
function followLink(a: Action) {
  if (a.link) router.push(a.link)
}

onMounted(async () => {
  loadSlack()
  await load()
  if (route.query.run) openRun(Number(route.query.run))
})
watch(() => route.query.run, (id) => { if (id) openRun(Number(id)) })
</script>

<template>
  <div class="routines">
    <header class="head">
      <div>
        <h1><NIcon class="spark"><SparklesOutline /></NIcon>{{ t('routines.title') }}</h1>
        <p class="muted">{{ t('routines.subtitle') }}</p>
      </div>
      <NButton @click="openRuns(null)">{{ t('routines.allRuns') }}</NButton>
    </header>

    <NSpin :show="loading">
      <NEmpty v-if="!loading && !routines.length" :description="t('routines.none')" />
      <div class="cards">
        <article v-for="r in routines" :key="r.key" class="card" :class="{ off: !r.enabled }">
          <div class="card-head">
            <div>
              <h2>{{ t('routines.name_' + r.key) }}</h2>
              <div class="trigger">
                <NIcon><FlashOutline v-if="r.trigger_type === 'event'" /><TimeOutline v-else /></NIcon>
                {{ t('routines.trigger_' + r.key) }}
              </div>
            </div>
            <NSwitch :value="r.enabled" :loading="saving[r.key]" @update:value="(v: boolean) => save(r, { enabled: v })" />
          </div>
          <p class="desc">{{ t('routines.desc_' + r.key) }}</p>

          <ol class="steps">
            <li v-for="(s, i) in r.steps" :key="i">{{ s }}</li>
          </ol>

          <div class="label">{{ t('routines.autonomy') }}</div>
          <NRadioGroup :value="r.autonomy" size="small" :disabled="!r.enabled" @update:value="(v: string) => save(r, { autonomy: v })">
            <NTooltip v-for="lv in levels" :key="lv" :disabled="rank[lv] <= rank[r.max_autonomy]">
              <template #trigger>
                <NRadioButton :value="lv" :disabled="rank[lv] > rank[r.max_autonomy]">{{ t('routines.level_' + lv) }}</NRadioButton>
              </template>
              {{ t('routines.capped') }}
            </NTooltip>
          </NRadioGroup>
          <div class="level-help muted">{{ t('routines.levelHelp_' + r.autonomy) }}</div>

          <div class="limits">
            <div v-for="l in r.hard_limits" :key="l" class="limit"><NIcon><LockClosedOutline /></NIcon>{{ l }}</div>
          </div>

          <div class="label">{{ t('routines.instructions') }}</div>
          <NInput
            v-model:value="drafts[r.key]" type="textarea" :autosize="{ minRows: 2, maxRows: 6 }"
            :placeholder="t('routines.instructionsPh_' + r.key)" :disabled="!r.enabled"
          />
          <div class="instr-actions">
            <span class="muted small">{{ t('routines.instructionsHelp') }}</span>
            <NButton size="small" :disabled="drafts[r.key] === r.instructions" :loading="saving[r.key]" @click="save(r, { instructions: drafts[r.key] })">
              {{ t('common.save') }}
            </NButton>
          </div>

          <footer class="card-foot" @click="openRuns(r)">
            <template v-if="r.last_run">
              <NTag size="small" round :type="statusTone[r.last_run.status]">{{ t('routines.status_' + r.last_run.status) }}</NTag>
              <span class="foot-text">{{ r.last_run.summary }} · {{ ago(r.last_run.created_at) }}</span>
            </template>
            <span v-else class="foot-text muted">{{ t('routines.neverRan') }}</span>
            <span class="foot-count">{{ t('routines.runs30', { n: r.runs_30d }) }}</span>
          </footer>
        </article>
      </div>
    </NSpin>

    <section v-if="slack" class="slack">
      <div class="slack-main">
        <h2>{{ t('slack.title') }}</h2>
        <p class="muted small">{{ t('slack.desc') }}</p>
        <NCheckboxGroup :value="slack.events" :disabled="!slack.connected" @update:value="(v: (string | number)[]) => saveSlack({ events: v.map(String) })">
          <NCheckbox v-for="e in slack.all_events" :key="e" :value="e" :label="t('slack.event_' + e)" style="margin-right: 14px" />
        </NCheckboxGroup>
      </div>
      <div class="slack-side">
        <template v-if="slack.connected">
          <div class="small"><NTag size="small" round type="success" :bordered="false">{{ t('slack.connected') }}</NTag> <span class="muted">{{ slack.hint }}</span></div>
          <div class="slack-btns">
            <NSwitch :value="slack.enabled" @update:value="(v: boolean) => saveSlack({ enabled: v })" />
            <NButton size="small" :loading="slackBusy" @click="testSlack">{{ t('slack.test') }}</NButton>
            <NButton size="small" quaternary :loading="slackBusy" @click="disconnectSlack">{{ t('slack.disconnect') }}</NButton>
          </div>
        </template>
        <template v-else>
          <NInput v-model:value="slackUrl" size="small" placeholder="https://hooks.slack.com/services/…" :disabled="!slack.can_store" />
          <NButton size="small" type="primary" :disabled="!slackUrl" :loading="slackBusy" @click="saveSlack({ webhook_url: slackUrl })">{{ t('slack.connect') }}</NButton>
          <div v-if="!slack.can_store" class="muted small">{{ t('slack.needKey') }}</div>
        </template>
      </div>
    </section>

    <NDrawer v-model:show="drawer" :width="760" placement="right">
      <NDrawerContent :title="drawerTitle" closable>
        <NSpin :show="runsLoading">
          <NEmpty v-if="!runsLoading && !runs.length" :description="t('routines.noRuns')" />
          <div v-else class="log">
            <ul class="run-list">
              <li v-for="run in runs" :key="run.id" :class="{ active: selected?.id === run.id }" @click="selected = run">
                <NTag size="tiny" round :type="statusTone[run.status]">{{ t('routines.status_' + run.status) }}</NTag>
                <div class="run-title">{{ run.subject_name || t('routines.name_' + run.routine_key) }}</div>
                <div class="run-sub">{{ format(new Date(run.created_at), 'MMM d, HH:mm') }}</div>
              </li>
            </ul>
            <div v-if="selected" class="run-detail">
              <div class="run-head">
                <div class="muted small">{{ t('routines.name_' + selected.routine_key) }} · {{ t('routines.level_' + selected.autonomy) }}</div>
                <h3>{{ selected.trigger }}</h3>
                <div class="muted">{{ selected.summary }}</div>
              </div>
              <NTimeline>
                <NTimelineItem
                  v-for="(a, i) in selected.actions" :key="i"
                  :type="actionTone[a.status] || 'default'"
                  :title="a.title"
                >
                  <div class="act">
                    <NTag size="tiny" :bordered="false" :type="actionTone[a.status] || 'default'">{{ t('routines.act_' + a.status) }}</NTag>
                    <span v-if="a.rationale" class="muted small">{{ a.rationale }}</span>
                    <NButton v-if="a.link" text type="primary" size="tiny" @click="followLink(a)">{{ t('routines.open') }}</NButton>
                  </div>
                </NTimelineItem>
              </NTimeline>
            </div>
          </div>
        </NSpin>
      </NDrawerContent>
    </NDrawer>
  </div>
</template>

<style scoped>
.routines { max-width: 1180px; margin: 0 auto; }
.head { display: flex; justify-content: space-between; align-items: flex-end; gap: 12px; margin-bottom: 18px; flex-wrap: wrap; }
.head h1 { font-size: 24px; font-weight: 650; margin: 0 0 4px; display: flex; align-items: center; gap: 8px; }
.spark { color: #7c3aed; }
.muted { opacity: .65; }
.small { font-size: 12px; }
.slack { margin-top: 20px; border: 1px solid rgba(128, 128, 128, .18); border-radius: 12px; padding: 16px; display: flex; gap: 16px; justify-content: space-between; flex-wrap: wrap; }
.slack h2 { font-size: 16px; margin: 0 0 4px; }
.slack-main { flex: 1; min-width: 280px; }
.slack-side { display: flex; flex-direction: column; gap: 8px; min-width: 280px; align-items: flex-start; }
.slack-btns { display: flex; gap: 8px; align-items: center; }
.cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(340px, 1fr)); gap: 16px; }
.card { border: 1px solid rgba(128, 128, 128, .18); border-radius: 12px; padding: 16px; display: flex; flex-direction: column; gap: 8px; }
.card.off { opacity: .6; }
.card-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 8px; }
.card h2 { font-size: 16px; margin: 0; font-weight: 600; }
.trigger { font-size: 12px; opacity: .7; display: flex; align-items: center; gap: 4px; margin-top: 2px; }
.desc { margin: 0; font-size: 13px; opacity: .85; }
.steps { margin: 0; padding-left: 18px; font-size: 13px; opacity: .8; }
.steps li { margin: 2px 0; }
.label { font-size: 12px; font-weight: 600; text-transform: uppercase; letter-spacing: .04em; opacity: .6; margin-top: 6px; }
.level-help { font-size: 12px; }
.limits { display: flex; flex-direction: column; gap: 2px; }
.limit { font-size: 12px; display: flex; align-items: center; gap: 6px; opacity: .75; }
.instr-actions { display: flex; justify-content: space-between; align-items: center; gap: 8px; }
.card-foot { margin-top: auto; border-top: 1px solid rgba(128, 128, 128, .12); padding-top: 10px; display: flex; align-items: center; gap: 8px; cursor: pointer; font-size: 12px; }
.card-foot:hover .foot-text { text-decoration: underline; }
.foot-text { flex: 1; min-width: 0; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.foot-count { opacity: .55; white-space: nowrap; }
.log { display: grid; grid-template-columns: 220px 1fr; gap: 16px; }
@media (max-width: 700px) { .log { grid-template-columns: 1fr; } }
.run-list { list-style: none; margin: 0; padding: 0; border-right: 1px solid rgba(128, 128, 128, .12); max-height: 75vh; overflow: auto; }
.run-list li { padding: 8px; border-radius: 8px; cursor: pointer; margin-right: 8px; }
.run-list li:hover { background: rgba(128, 128, 128, .07); }
.run-list li.active { background: rgba(124, 58, 237, .09); }
.run-title { font-size: 13px; margin-top: 4px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.run-sub { font-size: 11px; opacity: .6; }
.run-head { margin-bottom: 14px; }
.run-head h3 { margin: 4px 0; font-size: 16px; }
.act { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
</style>
