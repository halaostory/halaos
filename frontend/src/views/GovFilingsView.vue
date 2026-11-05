<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, h } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NCard, NSpace, NTabs, NTabPane, NButton, NSelect, NInput, NDataTable, NTag, NDrawer, NDrawerContent,
  NAlert, NList, NListItem, NTimeline, NTimelineItem, NForm, NFormItem, NEmpty, NSwitch, NImage, NText,
  useMessage, useDialog, type DataTableColumns,
} from 'naive-ui'
import { govAPI } from '../api/client'

interface Form { code: string; country: string; agency: string; name: string; frequency: string; channels: string[]; portal: string; portal_url: string; steps: string[]; notes?: string; file_format: string }
interface Sub { id: number; agency: string; form_code: string; period: string; channel: string; status: string; due_date: string | null; reference_no: string | null; error_message: string | null; file_name: string | null; dry_run: boolean; created_at: string }
interface Issue { severity: string; code: string; message: string; source: string }
interface Preflight { issues: Issue[]; blocking: boolean; ai_summary?: string; ai_risk?: string }
interface Ev { id: number; kind: string; message: string; data: Record<string, string>; created_at: string }
interface HumanReq { id: number; submission_id: number; name: string; prompt: string; agency: string; form_code: string; period: string; expires_at: string }
interface Account { id: number; agency: string; portal: string; label: string; username: string; otp_channel: string; status: string; has_credentials: boolean; last_error: string | null; last_used_at: string | null }
interface Playbook { id: number; playbook_key: string; form_code: string; definition: Record<string, unknown>; verified: boolean; verified_at: string | null }

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()

const forms = ref<Form[]>([])
const subs = ref<Sub[]>([])
const counts = ref<Record<string, number>>({})
const statusFilter = ref<string | null>(null)
const loading = ref(false)
const humans = ref<HumanReq[]>([])
const shots = ref<Record<number, string>>({})
const answers = ref<Record<number, string>>({})

const unwrap = <T,>(r: unknown): T => ((r as { data?: T }).data ?? r) as T
const errMsg = (e: unknown) => (e as { data?: { error?: { message?: string } } }).data?.error?.message || t('common.saveFailed')

const statusType: Record<string, 'default' | 'info' | 'success' | 'warning' | 'error'> = {
  draft: 'error', ready: 'info', approved: 'info', running: 'warning', needs_human: 'warning', submitted: 'success', failed: 'error', cancelled: 'default',
}

// ── prepare ───────────────────────────────────────────────
const prep = ref({ form_code: null as string | null, period: '', channel: '' })
const selectedForm = computed(() => forms.value.find((f) => f.code === prep.value.form_code))
const periodHint = computed(() => ({ monthly: 'YYYY-MM', hire_month: 'YYYY-MM', quarterly: 'YYYY-Q1', annual: 'YYYY', per_state: 'CA' } as Record<string, string>)[selectedForm.value?.frequency || 'monthly'] || 'YYYY-MM')
const preparing = ref(false)
async function prepare() {
  if (!prep.value.form_code || !prep.value.period) return
  preparing.value = true
  try {
    const res = unwrap<{ submission: Sub; preflight: Preflight }>(await govAPI.prepare({ ...prep.value, channel: prep.value.channel || undefined }))
    message.success(res.preflight.blocking ? t('gov.preparedBlocked') : t('gov.prepared'))
    await loadSubs()
    openDetail(res.submission.id)
  } catch (e) {
    message.error(errMsg(e))
  } finally {
    preparing.value = false
  }
}

// ── list ──────────────────────────────────────────────────
async function loadSubs() {
  loading.value = true
  try {
    const res = unwrap<{ items: Sub[]; counts: Record<string, number> }>(await govAPI.submissions(statusFilter.value ? { status: statusFilter.value } : undefined))
    subs.value = res.items
    counts.value = res.counts
  } finally {
    loading.value = false
  }
}
const formName = (code: string) => forms.value.find((f) => f.code === code)?.name || code
const columns = computed<DataTableColumns<Sub>>(() => [
  { title: '#', key: 'id', width: 60 },
  { title: t('gov.form'), key: 'form_code', render: (r) => h('div', [h('b', r.agency + ' '), formName(r.form_code)]) },
  { title: t('gov.period'), key: 'period', width: 100 },
  { title: t('gov.channel'), key: 'channel', width: 100, render: (r) => t('gov.ch_' + r.channel) },
  { title: t('common.status'), key: 'status', width: 140, render: (r) => h(NTag, { size: 'small', type: statusType[r.status] }, () => t('gov.st_' + r.status) + (r.dry_run ? ' · dry run' : '')) },
  { title: t('lifecycle.due'), key: 'due_date', width: 110, render: (r) => (r.due_date || '').slice(0, 10) },
  { title: t('gov.reference'), key: 'reference_no', render: (r) => r.reference_no || '' },
  { title: '', key: 'a', width: 80, render: (r) => h(NButton, { size: 'small', onClick: () => openDetail(r.id) }, () => t('gov.open')) },
])

// ── detail ────────────────────────────────────────────────
const show = ref(false)
const detail = ref<{ submission: Sub; events: Ev[]; form: Form; preflight: Preflight } | null>(null)
const evidence = ref<{ name: string; url: string }[]>([])
const refInput = ref('')
async function openDetail(id: number) {
  show.value = true
  await loadDetail(id)
}
async function loadDetail(id: number) {
  const d = unwrap<{ submission: Sub; events: Ev[]; form: Form; preflight: Preflight }>(await govAPI.submission(id))
  const known = new Set(evidence.value.map((e) => e.name))
  if (detail.value?.submission.id !== id) {
    evidence.value.forEach((e) => URL.revokeObjectURL(e.url))
    evidence.value = []
    known.clear()
  }
  detail.value = d
  for (const ev of d.events.filter((e) => e.kind === 'evidence' && e.data?.file)) {
    if (known.has(ev.data.file)) continue
    try {
      const blob = (await govAPI.evidenceBlob(id, ev.data.file)) as Blob
      evidence.value.push({ name: ev.data.file, url: URL.createObjectURL(blob) })
    } catch { /* ignore */ }
  }
}
async function act(fn: () => Promise<unknown>, ok: string) {
  try {
    await fn()
    message.success(ok)
    await loadSubs()
    if (detail.value) await loadDetail(detail.value.submission.id)
  } catch (e) {
    message.error(errMsg(e))
  }
}
function approve(dry: boolean) {
  const s = detail.value!.submission
  dialog.warning({
    title: dry ? t('gov.dryRun') : t('gov.approveTitle'),
    content: dry ? t('gov.dryRunConfirm') : t('gov.approveConfirm', { form: `${s.agency} ${s.form_code} ${s.period}`, channel: t('gov.ch_' + s.channel) }),
    positiveText: t('common.confirm'), negativeText: t('common.cancel'),
    onPositiveClick: () => act(() => (dry ? govAPI.dryRun(s.id) : govAPI.approve(s.id)), t('gov.approved')),
  })
}
async function download() {
  const s = detail.value!.submission
  const blob = (await govAPI.fileBlob(s.id)) as Blob
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = s.file_name || 'return'
  a.click()
}

// ── human hand-off (CAPTCHA / OTP) ────────────────────────
async function loadHumans() {
  const list = unwrap<HumanReq[]>(await govAPI.humanRequests()) || []
  for (const r of list) {
    if (!shots.value[r.id]) {
      try {
        shots.value[r.id] = URL.createObjectURL((await govAPI.humanScreenshotBlob(r.id)) as Blob)
      } catch { /* no screenshot */ }
    }
  }
  humans.value = list
}
async function sendAnswer(r: HumanReq) {
  const a = (answers.value[r.id] || '').trim()
  if (!a) return
  try {
    await govAPI.answer(r.id, a)
    answers.value[r.id] = ''
    message.success(t('gov.answerSent'))
    await loadHumans()
  } catch (e) {
    message.error(errMsg(e))
  }
}

// ── accounts ──────────────────────────────────────────────
const accounts = ref<Account[]>([])
const acc = ref({ form_code: null as string | null, label: '', username: '', password: '', url: '', secret: '', otp_channel: 'sms' })
const accForm = computed(() => forms.value.find((f) => f.code === acc.value.form_code))
const accountTargets = computed(() => {
  const seen = new Set<string>()
  return forms.value.filter((f) => f.portal && !seen.has(f.agency + f.portal) && seen.add(f.agency + f.portal))
    .map((f) => ({ label: `${f.agency} · ${f.portal}`, value: f.code }))
})
const isPartner = computed(() => accForm.value?.channels.includes('api'))
async function loadAccounts() {
  accounts.value = unwrap<Account[]>(await govAPI.accounts()) || []
}
async function saveAccount() {
  const f = accForm.value
  if (!f) return
  const secrets: Record<string, string> = {}
  if (acc.value.password) secrets.password = acc.value.password
  if (acc.value.url) secrets.url = acc.value.url
  if (acc.value.secret) secrets.secret = acc.value.secret
  await act(() => govAPI.saveAccount({ agency: f.agency, portal: f.portal, label: acc.value.label || f.agency, username: acc.value.username, otp_channel: acc.value.otp_channel, secrets }), t('common.saved'))
  acc.value = { ...acc.value, password: '', secret: '' }
  await loadAccounts()
}
function removeAccount(a: Account) {
  dialog.warning({ title: t('common.confirmDelete'), positiveText: t('common.confirm'), negativeText: t('common.cancel'),
    onPositiveClick: async () => { await govAPI.deleteAccount(a.id); await loadAccounts() } })
}

// ── playbooks ─────────────────────────────────────────────
const playbooks = ref<Playbook[]>([])
const pbForm = ref<string | null>(null)
const pbJSON = ref('')
async function loadPlaybooks() {
  playbooks.value = unwrap<Playbook[]>(await govAPI.playbooks()) || []
}
function editPlaybook(p?: Playbook) {
  pbForm.value = p?.form_code || pbForm.value
  pbJSON.value = JSON.stringify(p?.definition || {
    id: 'company.' + (pbForm.value || 'form'), agency: '', portal: '', title: '', start_url: 'https://',
    required_outputs: ['reference_no'],
    steps: [
      { action: 'fill', selector: '#username', value: '{{.Creds.username}}' },
      { action: 'fill', selector: '#password', value: '{{.Creds.password}}' },
      { action: 'human_input', name: 'otp', prompt: 'Enter the OTP' },
      { action: 'fill', selector: '#otp', value: '{{.Input.otp}}' },
      { action: 'upload', selector: 'input[type=file]' },
      { action: 'click', selector: '#submit', final: true },
      { action: 'extract', selector: '#reference', name: 'reference_no' },
    ],
  }, null, 2)
}
async function savePlaybook() {
  let def: unknown
  try { def = JSON.parse(pbJSON.value) } catch { message.error('JSON'); return }
  await act(() => govAPI.savePlaybook({ form_code: pbForm.value, definition: def }), t('common.saved'))
  await loadPlaybooks()
}
async function toggleVerify(p: Playbook, v: boolean) {
  await act(() => govAPI.verifyPlaybook(p.id, v), t('common.saved'))
  await loadPlaybooks()
}

// ── polling ───────────────────────────────────────────────
let timer: number | undefined
onMounted(async () => {
  forms.value = unwrap<Form[]>(await govAPI.forms()) || []
  await Promise.all([loadSubs(), loadHumans(), loadAccounts(), loadPlaybooks()])
  timer = window.setInterval(async () => {
    await loadHumans()
    const st = detail.value?.submission.status
    if (show.value && detail.value && st && ['approved', 'running', 'needs_human'].includes(st)) {
      await loadDetail(detail.value.submission.id)
      if (detail.value.submission.status !== st) await loadSubs()
    }
  }, 3000)
})
onUnmounted(() => {
  window.clearInterval(timer)
  Object.values(shots.value).forEach((u) => URL.revokeObjectURL(u))
  evidence.value.forEach((e) => URL.revokeObjectURL(e.url))
})

const detailHumans = computed(() => humans.value.filter((r) => r.submission_id === detail.value?.submission.id))
const eventsDesc = computed(() => [...(detail.value?.events || [])].reverse())
const sevType = (s: string) => (s === 'error' ? 'error' : s === 'warning' ? 'warning' : 'info') as 'error' | 'warning' | 'info'
const statusOptions = computed(() => ['draft', 'ready', 'approved', 'running', 'needs_human', 'submitted', 'failed', 'cancelled'].map((s) => ({ label: `${t('gov.st_' + s)} (${counts.value[s] || 0})`, value: s })))
</script>

<template>
  <NSpace vertical :size="16">
    <div>
      <h2 style="margin: 0">{{ t('gov.title') }}</h2>
      <div style="opacity: 0.7">{{ t('gov.subtitle') }}</div>
    </div>

    <NCard v-if="humans.length" :title="t('gov.humanTitle')" style="border-color: #f0a020">
      <NSpace vertical>
        <div v-for="r in humans" :key="r.id" style="display: flex; gap: 16px; flex-wrap: wrap; align-items: flex-start">
          <NImage v-if="shots[r.id]" :src="shots[r.id]" width="520" style="border: 1px solid #ddd" />
          <NSpace vertical style="min-width: 260px; flex: 1">
            <NText strong>{{ r.agency }} {{ r.form_code }} {{ r.period }}</NText>
            <div>{{ r.prompt }}</div>
            <NSpace>
              <NInput v-model:value="answers[r.id]" :placeholder="r.name" style="width: 200px" @keyup.enter="sendAnswer(r)" />
              <NButton type="primary" @click="sendAnswer(r)">{{ t('gov.sendAnswer') }}</NButton>
            </NSpace>
            <NText depth="3" style="font-size: 12px">{{ t('gov.humanHint') }}</NText>
          </NSpace>
        </div>
      </NSpace>
    </NCard>

    <NTabs type="line" animated>
      <NTabPane name="filings" :tab="t('gov.tabFilings')">
        <NSpace vertical :size="16">
          <NCard size="small" :title="t('gov.prepareTitle')">
            <NSpace align="end" :wrap="true">
              <NFormItem :label="t('gov.form')" :show-feedback="false" style="min-width: 320px">
                <NSelect v-model:value="prep.form_code" filterable :options="forms.map((f) => ({ label: `${f.agency} · ${f.name}`, value: f.code }))" />
              </NFormItem>
              <NFormItem :label="t('gov.period')" :show-feedback="false">
                <NInput v-model:value="prep.period" :placeholder="periodHint" style="width: 130px" />
              </NFormItem>
              <NFormItem :label="t('gov.channel')" :show-feedback="false">
                <NSelect v-model:value="prep.channel" style="width: 160px"
                  :options="[{ label: t('gov.ch_auto'), value: '' }, ...(selectedForm?.channels || []).map((c) => ({ label: t('gov.ch_' + c), value: c }))]" />
              </NFormItem>
              <NButton type="primary" :loading="preparing" :disabled="!prep.form_code || !prep.period" @click="prepare">{{ t('gov.prepare') }}</NButton>
            </NSpace>
            <NText v-if="selectedForm?.notes" depth="3" style="display: block; margin-top: 8px; font-size: 12px">{{ selectedForm.notes }}</NText>
          </NCard>
          <NSpace>
            <NSelect v-model:value="statusFilter" clearable :placeholder="t('common.status')" :options="statusOptions" style="width: 220px" @update:value="loadSubs" />
          </NSpace>
          <NDataTable :columns="columns" :data="subs" :loading="loading" :row-key="(r: Sub) => r.id" />
        </NSpace>
      </NTabPane>

      <NTabPane name="accounts" :tab="t('gov.tabAccounts')">
        <NSpace vertical :size="16">
          <NAlert type="info" :show-icon="false">{{ t('gov.accountsHint') }}</NAlert>
          <NCard size="small">
            <NForm label-placement="top">
              <NSpace :wrap="true">
                <NFormItem :label="t('gov.portal')" style="min-width: 260px"><NSelect v-model:value="acc.form_code" :options="accountTargets" /></NFormItem>
                <NFormItem :label="t('gov.label')"><NInput v-model:value="acc.label" /></NFormItem>
                <NFormItem :label="isPartner ? t('gov.partnerUser') : t('gov.username')"><NInput v-model:value="acc.username" /></NFormItem>
                <NFormItem v-if="!isPartner" :label="t('gov.password')"><NInput v-model:value="acc.password" type="password" show-password-on="click" /></NFormItem>
                <template v-else>
                  <NFormItem :label="t('gov.partnerUrl')"><NInput v-model:value="acc.url" placeholder="https://" /></NFormItem>
                  <NFormItem :label="t('gov.partnerSecret')"><NInput v-model:value="acc.secret" type="password" /></NFormItem>
                </template>
                <NFormItem :label="t('gov.otp')">
                  <NSelect v-model:value="acc.otp_channel" style="width: 140px" :options="['none', 'sms', 'email', 'authenticator'].map((v) => ({ label: v, value: v }))" />
                </NFormItem>
              </NSpace>
              <NButton type="primary" :disabled="!acc.form_code" @click="saveAccount">{{ t('common.save') }}</NButton>
            </NForm>
          </NCard>
          <NList bordered>
            <NListItem v-for="a in accounts" :key="a.id">
              <NSpace justify="space-between" align="center">
                <div>
                  <b>{{ a.agency }}</b> · {{ a.portal }} — {{ a.label }} <span style="opacity: 0.7">({{ a.username || '—' }})</span>
                  <NTag size="tiny" :type="a.status === 'active' ? 'success' : 'warning'" style="margin-left: 6px">{{ a.status }}</NTag>
                  <NTag v-if="a.has_credentials" size="tiny" style="margin-left: 4px">🔒 {{ t('gov.encrypted') }}</NTag>
                  <div v-if="a.last_error" style="color: #d03050; font-size: 12px">{{ a.last_error }}</div>
                </div>
                <NButton size="small" quaternary type="error" @click="removeAccount(a)">{{ t('common.delete') }}</NButton>
              </NSpace>
            </NListItem>
          </NList>
        </NSpace>
      </NTabPane>

      <NTabPane name="playbooks" :tab="t('gov.tabPlaybooks')">
        <NSpace vertical :size="16">
          <NAlert type="warning" :show-icon="false">{{ t('gov.playbookHint') }}</NAlert>
          <NList bordered>
            <NListItem v-for="p in playbooks" :key="p.id">
              <NSpace justify="space-between" align="center">
                <div><b>{{ p.playbook_key }}</b> · {{ p.form_code }}</div>
                <NSpace align="center">
                  <span>{{ t('gov.verified') }}</span>
                  <NSwitch :value="p.verified" @update:value="(v: boolean) => toggleVerify(p, v)" />
                  <NButton size="small" @click="editPlaybook(p)">{{ t('common.edit') }}</NButton>
                </NSpace>
              </NSpace>
            </NListItem>
          </NList>
          <NCard size="small" :title="t('gov.playbookEditor')">
            <NSpace vertical>
              <NSpace>
                <NSelect v-model:value="pbForm" :options="forms.filter((f) => f.channels.includes('browser')).map((f) => ({ label: f.agency + ' · ' + f.name, value: f.code }))" style="width: 360px" />
                <NButton @click="editPlaybook()">{{ t('gov.newPlaybook') }}</NButton>
              </NSpace>
              <NInput v-model:value="pbJSON" type="textarea" :rows="16" style="font-family: monospace" />
              <NButton type="primary" :disabled="!pbForm || !pbJSON" @click="savePlaybook">{{ t('common.save') }}</NButton>
            </NSpace>
          </NCard>
        </NSpace>
      </NTabPane>
    </NTabs>

    <NDrawer v-model:show="show" :width="640" placement="right">
      <NDrawerContent v-if="detail" :title="`${detail.submission.agency} · ${detail.form.name || detail.submission.form_code} · ${detail.submission.period}`" closable>
        <NSpace vertical :size="14">
          <NSpace align="center">
            <NTag :type="statusType[detail.submission.status]">{{ t('gov.st_' + detail.submission.status) }}</NTag>
            <span>{{ t('gov.ch_' + detail.submission.channel) }}</span>
            <span v-if="detail.submission.reference_no"><b>{{ t('gov.reference') }}:</b> {{ detail.submission.reference_no }}</span>
          </NSpace>
          <NAlert v-if="detail.submission.error_message" type="error">{{ detail.submission.error_message }}</NAlert>

          <NCard v-for="r in detailHumans" :key="r.id" size="small" :title="t('gov.humanTitle')" style="border-color: #f0a020">
            <NImage v-if="shots[r.id]" :src="shots[r.id]" width="100%" style="border: 1px solid #ddd; margin-bottom: 8px" />
            <div style="margin-bottom: 8px">{{ r.prompt }}</div>
            <NSpace>
              <NInput v-model:value="answers[r.id]" :placeholder="r.name" style="width: 220px" @keyup.enter="sendAnswer(r)" />
              <NButton type="primary" @click="sendAnswer(r)">{{ t('gov.sendAnswer') }}</NButton>
            </NSpace>
          </NCard>

          <NCard size="small" :title="t('gov.preflight')">
            <NAlert v-if="detail.preflight.ai_summary" type="info" :title="t('gov.aiReview') + (detail.preflight.ai_risk ? ' · ' + detail.preflight.ai_risk : '')" style="margin-bottom: 8px">
              {{ detail.preflight.ai_summary }}
            </NAlert>
            <NEmpty v-if="!detail.preflight.issues?.length" size="small" :description="t('gov.noIssues')" />
            <NSpace vertical :size="6">
              <NAlert v-for="(i, k) in detail.preflight.issues" :key="k" :type="sevType(i.severity)" :show-icon="true" size="small">
                {{ i.message }} <NTag v-if="i.source === 'ai'" size="tiny">AI</NTag>
              </NAlert>
            </NSpace>
          </NCard>

          <NSpace>
            <template v-if="['ready', 'failed'].includes(detail.submission.status)">
              <NButton type="primary" @click="approve(false)">{{ detail.submission.status === 'failed' ? t('gov.retry') : t('gov.approve') }}</NButton>
              <NButton v-if="detail.submission.channel === 'browser'" @click="approve(true)">{{ t('gov.dryRun') }}</NButton>
            </template>
            <NButton v-if="detail.submission.file_name" @click="download">⬇ {{ detail.submission.file_name }}</NButton>
            <NButton v-if="['draft', 'ready', 'approved', 'failed', 'needs_human'].includes(detail.submission.status)" quaternary type="error"
              @click="act(() => govAPI.cancel(detail!.submission.id), t('gov.st_cancelled'))">{{ t('common.cancel') }}</NButton>
          </NSpace>

          <NCard v-if="(detail.submission.status === 'approved' && detail.submission.channel === 'manual') || detail.submission.status === 'failed'" size="small" :title="t('gov.recordTitle')">
            <NSpace>
              <NInput v-model:value="refInput" :placeholder="t('gov.reference')" style="width: 240px" />
              <NButton type="primary" :disabled="!refInput" @click="act(() => govAPI.record(detail!.submission.id, refInput), t('gov.st_submitted'))">{{ t('gov.recordBtn') }}</NButton>
            </NSpace>
          </NCard>

          <NCard size="small" :title="t('gov.instructions')">
            <ol style="margin: 0; padding-left: 18px">
              <li v-for="(s, i) in detail.form.steps" :key="i">{{ s }}</li>
            </ol>
            <a v-if="detail.form.portal_url" :href="detail.form.portal_url" target="_blank" rel="noopener">{{ detail.form.portal_url }}</a>
          </NCard>

          <NCard v-if="evidence.length" size="small" :title="t('gov.evidence')">
            <NSpace>
              <NImage v-for="e in evidence" :key="e.name" :src="e.url" width="180" style="border: 1px solid #ddd" />
            </NSpace>
          </NCard>

          <NCard size="small" :title="t('gov.activity')">
            <NTimeline>
              <NTimelineItem v-for="e in eventsDesc" :key="e.id"
                :type="e.kind === 'status' ? 'success' : e.kind === 'human_request' ? 'warning' : e.kind === 'ai' ? 'info' : 'default'"
                :title="e.message" :time="new Date(e.created_at).toLocaleString()" />
            </NTimeline>
          </NCard>
        </NSpace>
      </NDrawerContent>
    </NDrawer>
  </NSpace>
</template>
