<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  NButton, NSpin, NEmpty, NTag, NModal, NInput, NDrawer, NDrawerContent, NAlert, NIcon, NCollapse,
  NCollapseItem, NUpload, type UploadFileInfo, useMessage,
} from 'naive-ui'
import { SparklesOutline, AddOutline, DocumentAttachOutline, CopyOutline } from '@vicons/ionicons5'
import { differenceInCalendarDays, format } from 'date-fns'
import { govAPI } from '../api/client'

interface Notice {
  id: number; agency: string; notice_type: string; form_code: string; period: string; reference_no: string
  amount: string | null; respond_by: string | null; title: string; summary: string; recommendation: string
  resolution_path: string; draft_response: string; matched_submission_id: number | null; status: string
  auto_resolvable: boolean; triaged_by: string; source_text: string; file_name: string | null
  resolution_note: string; created_at: string; resolved_at: string | null
}
interface Submission { id: number; status: string; reference_no: string | null; submitted_at: string | null; period: string; form_code: string }

const { t } = useI18n()
const router = useRouter()
const message = useMessage()
const loading = ref(true)
const notices = ref<Notice[]>([])
const counts = ref<Record<string, number>>({})
const filter = ref('active')

const addOpen = ref(false)
const addText = ref('')
const addFile = ref<File | null>(null)
const adding = ref(false)

const detailOpen = ref(false)
const detail = ref<Notice | null>(null)
const detailSub = ref<Submission | null>(null)
const draft = ref('')
const note = ref('')
const busy = ref(false)

const tabs = computed(() => [
  { key: 'active', n: (counts.value.open || 0) + (counts.value.awaiting_approval || 0) + (counts.value.escalated || 0) },
  { key: 'awaiting_approval', n: counts.value.awaiting_approval || 0 },
  { key: 'escalated', n: counts.value.escalated || 0 },
  { key: 'resolved', n: counts.value.resolved || 0 },
])
const visible = computed(() =>
  filter.value === 'active' ? notices.value.filter((n) => n.status !== 'resolved') : notices.value.filter((n) => n.status === filter.value),
)
const autoShare = computed(() => {
  const all = notices.value.length
  return all ? Math.round((notices.value.filter((n) => n.auto_resolvable).length / all) * 100) : 0
})

async function load() {
  loading.value = true
  try {
    const d = ((await govAPI.notices()) as { data: { notices: Notice[]; counts: Record<string, number> } }).data
    notices.value = d.notices
    counts.value = d.counts
  } catch {
    message.error(t('common.loadFailed'))
  } finally {
    loading.value = false
  }
}

function onFile(opts: { fileList: UploadFileInfo[] }) {
  addFile.value = opts.fileList[0]?.file ?? null
}

async function submit() {
  if (!addText.value.trim() && !addFile.value) return
  adding.value = true
  try {
    const fd = new FormData()
    fd.append('text', addText.value)
    if (addFile.value) fd.append('file', addFile.value)
    const n = ((await govAPI.createNotice(fd)) as { data: Notice }).data
    addOpen.value = false
    addText.value = ''
    addFile.value = null
    await load()
    open(n)
  } catch (e) {
    message.error((e as Error).message || t('common.saveFailed'))
  } finally {
    adding.value = false
  }
}

async function open(n: Notice) {
  detail.value = n
  draft.value = n.draft_response
  note.value = n.resolution_note
  detailSub.value = null
  detailOpen.value = true
  try {
    const d = ((await govAPI.notice(n.id)) as { data: { notice: Notice; submission?: Submission } }).data
    detail.value = d.notice
    detailSub.value = d.submission || null
  } catch { /* keep list data */ }
}

async function setStatus(status: string, extra: { draft_response?: string; resolution_note?: string } = {}) {
  if (!detail.value) return
  busy.value = true
  try {
    const n = ((await govAPI.updateNotice(detail.value.id, { status, ...extra })) as { data: Notice }).data
    detail.value = n
    message.success(t('notices.updated'))
    load()
  } catch (e) {
    message.error((e as Error).message)
  } finally {
    busy.value = false
  }
}

async function approveReply() {
  await setStatus('resolved', { draft_response: draft.value, resolution_note: note.value || t('notices.replySent') })
}

async function retriage() {
  if (!detail.value) return
  busy.value = true
  try {
    const n = ((await govAPI.retriageNotice(detail.value.id)) as { data: Notice }).data
    await open(n)
    load()
  } finally {
    busy.value = false
  }
}

async function prepare() {
  if (!detail.value) return
  busy.value = true
  try {
    const d = ((await govAPI.prepareFromNotice(detail.value.id)) as { data: { notice: Notice } }).data
    await open(d.notice)
    message.success(t('notices.prepared'))
    load()
  } catch (e) {
    message.error((e as Error).message)
  } finally {
    busy.value = false
  }
}

async function downloadFile() {
  if (!detail.value) return
  const blob = (await govAPI.noticeFileBlob(detail.value.id)) as Blob
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = detail.value.file_name || 'notice'
  a.click()
}

async function copyDraft() {
  await navigator.clipboard.writeText(draft.value)
  message.success(t('notices.copied'))
}

function due(n: Notice) {
  if (!n.respond_by || n.status === 'resolved') return null
  const d = differenceInCalendarDays(new Date(n.respond_by), new Date())
  if (d < 0) return { text: t('home.overdueBy', { n: -d }), tone: 'error' as const }
  if (d === 0) return { text: t('home.dueToday'), tone: 'error' as const }
  return { text: t('home.dueIn', { n: d }), tone: d <= 5 ? 'warning' as const : 'default' as const }
}
const statusTone: Record<string, 'success' | 'warning' | 'error' | 'info' | 'default'> = {
  open: 'info', awaiting_approval: 'warning', escalated: 'error', resolved: 'success',
}
function money(v: string | null) {
  return v ? Number(v).toLocaleString(undefined, { minimumFractionDigits: 2 }) : ''
}

onMounted(load)
</script>

<template>
  <div class="notices">
    <header class="head">
      <div>
        <h1>{{ t('notices.title') }}</h1>
        <p class="muted">{{ t('notices.subtitle') }}</p>
      </div>
      <NButton type="primary" @click="addOpen = true">
        <template #icon><NIcon><AddOutline /></NIcon></template>{{ t('notices.add') }}
      </NButton>
    </header>

    <div class="tabs">
      <NButton v-for="tb in tabs" :key="tb.key" size="small" :type="filter === tb.key ? 'primary' : 'default'" secondary @click="filter = tb.key">
        {{ t('notices.tab_' + tb.key) }} · {{ tb.n }}
      </NButton>
      <span v-if="notices.length" class="muted small auto">
        <NIcon class="spark"><SparklesOutline /></NIcon>{{ t('notices.autoShare', { n: autoShare }) }}
      </span>
    </div>

    <NSpin :show="loading">
      <NEmpty v-if="!loading && !visible.length" :description="t('notices.empty')" class="empty" />
      <ul v-else class="list">
        <li v-for="n in visible" :key="n.id" class="item" @click="open(n)">
          <NTag size="small" :bordered="false">{{ n.agency || '—' }}</NTag>
          <div class="item-main">
            <div class="item-title">
              {{ n.title || t('notices.type_' + n.notice_type) }}
              <NIcon v-if="n.auto_resolvable && n.status !== 'resolved'" class="spark" :title="t('notices.autoResolvable')"><SparklesOutline /></NIcon>
            </div>
            <div class="item-sub">{{ n.summary }}</div>
          </div>
          <span v-if="n.amount" class="amount">{{ money(n.amount) }}</span>
          <NTag v-if="due(n)" size="small" round :type="due(n)!.tone">{{ due(n)!.text }}</NTag>
          <NTag size="small" round :type="statusTone[n.status]">{{ t('notices.status_' + n.status) }}</NTag>
        </li>
      </ul>
    </NSpin>

    <NModal v-model:show="addOpen" preset="card" :title="t('notices.add')" style="max-width: 640px">
      <p class="muted small">{{ t('notices.addHelp') }}</p>
      <NInput v-model:value="addText" type="textarea" :autosize="{ minRows: 8, maxRows: 16 }" :placeholder="t('notices.pastePh')" />
      <NUpload :max="1" :default-upload="false" accept=".pdf,.png,.jpg,.jpeg,.txt,.eml" class="upload" @change="onFile">
        <NButton size="small"><template #icon><NIcon><DocumentAttachOutline /></NIcon></template>{{ t('notices.attach') }}</NButton>
      </NUpload>
      <template #footer>
        <div class="modal-foot">
          <NButton @click="addOpen = false">{{ t('common.cancel') }}</NButton>
          <NButton type="primary" :loading="adding" :disabled="!addText.trim() && !addFile" @click="submit">{{ t('notices.triage') }}</NButton>
        </div>
      </template>
    </NModal>

    <NDrawer v-model:show="detailOpen" :width="720" placement="right">
      <NDrawerContent v-if="detail" :title="detail.title || t('notices.type_' + detail.notice_type)" closable>
        <div class="facts">
          <NTag size="small" round :type="statusTone[detail.status]">{{ t('notices.status_' + detail.status) }}</NTag>
          <span>{{ detail.agency || '—' }}</span>
          <span>{{ t('notices.type_' + detail.notice_type) }}</span>
          <span v-if="detail.period">{{ detail.period }}</span>
          <span v-if="detail.reference_no">#{{ detail.reference_no }}</span>
          <span v-if="detail.amount">{{ money(detail.amount) }}</span>
          <span v-if="detail.respond_by">{{ t('notices.respondBy') }} {{ format(new Date(detail.respond_by), 'MMM d, yyyy') }}</span>
        </div>
        <p class="summary">{{ detail.summary }}</p>

        <NAlert :type="detail.resolution_path === 'escalate' ? 'error' : detail.auto_resolvable ? 'success' : 'info'" :show-icon="true" class="rec">
          <template #header>
            <span class="rec-head"><NIcon class="spark"><SparklesOutline /></NIcon>{{ t('notices.path_' + detail.resolution_path) }}</span>
          </template>
          {{ detail.recommendation }}
          <div class="muted small triaged">{{ t('notices.triagedBy_' + detail.triaged_by) }}</div>
        </NAlert>

        <div v-if="detailSub" class="matched">
          <div class="label">{{ t('notices.ourRecord') }}</div>
          <div>
            {{ detailSub.form_code }} · {{ detailSub.period }} ·
            <NTag size="tiny" round :type="detailSub.status === 'submitted' ? 'success' : 'warning'">{{ detailSub.status }}</NTag>
            <template v-if="detailSub.reference_no"> · {{ t('notices.ref') }} {{ detailSub.reference_no }}</template>
            <template v-if="detailSub.submitted_at"> · {{ format(new Date(detailSub.submitted_at), 'MMM d, yyyy') }}</template>
            <NButton text type="primary" size="small" class="ml" @click="router.push({ name: 'gov-filings' })">{{ t('routines.open') }}</NButton>
          </div>
        </div>

        <template v-if="detail.resolution_path === 'prepare_filing' && detail.status !== 'resolved'">
          <NButton v-if="!detailSub" type="primary" :loading="busy" @click="prepare">{{ t('notices.prepareNow', { form: detail.form_code, period: detail.period }) }}</NButton>
          <NButton v-else type="primary" @click="router.push({ name: 'gov-filings' })">{{ t('notices.reviewPrepared') }}</NButton>
        </template>

        <template v-if="draft || detail.draft_response">
          <div class="label">{{ t('notices.draftReply') }}</div>
          <NInput v-model:value="draft" type="textarea" :autosize="{ minRows: 10, maxRows: 22 }" :disabled="detail.status === 'resolved'" />
          <div class="row-actions">
            <NButton size="small" @click="copyDraft"><template #icon><NIcon><CopyOutline /></NIcon></template>{{ t('notices.copy') }}</NButton>
            <span class="muted small">{{ t('notices.neverSent') }}</span>
          </div>
        </template>

        <div class="label">{{ t('notices.note') }}</div>
        <NInput v-model:value="note" :placeholder="t('notices.notePh')" :disabled="detail.status === 'resolved'" />

        <div class="actions">
          <template v-if="detail.status !== 'resolved'">
            <NButton v-if="detail.draft_response" type="primary" :loading="busy" @click="approveReply">{{ t('notices.approveReply') }}</NButton>
            <NButton v-else type="primary" :loading="busy" @click="setStatus('resolved', { resolution_note: note })">{{ t('notices.markResolved') }}</NButton>
            <NButton v-if="detail.status !== 'escalated'" :loading="busy" @click="setStatus('escalated', { resolution_note: note })">{{ t('notices.escalate') }}</NButton>
            <NButton :loading="busy" @click="retriage">{{ t('notices.retriage') }}</NButton>
          </template>
          <NButton v-else :loading="busy" @click="setStatus('open')">{{ t('notices.reopen') }}</NButton>
          <NButton v-if="detail.file_name" quaternary @click="downloadFile">⬇ {{ detail.file_name }}</NButton>
        </div>

        <NCollapse class="source">
          <NCollapseItem :title="t('notices.original')" name="src">
            <pre>{{ detail.source_text }}</pre>
          </NCollapseItem>
        </NCollapse>
      </NDrawerContent>
    </NDrawer>
  </div>
</template>

<style scoped>
.notices { max-width: 1100px; margin: 0 auto; }
.head { display: flex; justify-content: space-between; align-items: flex-end; gap: 12px; flex-wrap: wrap; margin-bottom: 14px; }
.head h1 { font-size: 24px; font-weight: 650; margin: 0 0 4px; }
.muted { opacity: .65; }
.small { font-size: 12px; }
.spark { color: #7c3aed; vertical-align: -2px; }
.tabs { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; margin-bottom: 12px; }
.auto { margin-left: auto; display: flex; align-items: center; gap: 4px; }
.empty { padding: 48px 0; }
.list { list-style: none; margin: 0; padding: 0; border: 1px solid rgba(128, 128, 128, .18); border-radius: 12px; }
.item { display: flex; align-items: center; gap: 10px; padding: 12px 14px; cursor: pointer; }
.item + .item { border-top: 1px solid rgba(128, 128, 128, .1); }
.item:hover { background: rgba(128, 128, 128, .06); }
.item-main { flex: 1; min-width: 0; }
.item-title { font-size: 14px; font-weight: 500; display: flex; align-items: center; gap: 6px; }
.item-sub { font-size: 12px; opacity: .65; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.amount { font-variant-numeric: tabular-nums; font-size: 13px; }
.facts { display: flex; gap: 10px; flex-wrap: wrap; align-items: center; font-size: 13px; opacity: .9; }
.summary { margin: 12px 0; }
.rec { margin-bottom: 14px; }
.rec-head { display: inline-flex; align-items: center; gap: 4px; }
.triaged { margin-top: 6px; }
.matched { margin: 10px 0 14px; font-size: 13px; }
.label { font-size: 12px; font-weight: 600; text-transform: uppercase; letter-spacing: .04em; opacity: .6; margin: 14px 0 6px; }
.row-actions { display: flex; gap: 10px; align-items: center; margin-top: 6px; }
.actions { display: flex; gap: 8px; flex-wrap: wrap; margin-top: 16px; }
.source { margin-top: 18px; }
.source pre { white-space: pre-wrap; font-size: 12px; margin: 0; }
.upload { margin-top: 10px; }
.modal-foot { display: flex; justify-content: flex-end; gap: 8px; }
.ml { margin-left: 6px; }
</style>
