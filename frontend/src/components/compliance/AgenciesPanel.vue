<script setup lang="ts">
import { ref, h, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { NDataTable, NTag, NEmpty, NText, type DataTableColumns } from 'naive-ui'
import { homeAPI } from '../../api/client'

// Warp-style compliance overview: one row per agency with its account
// number, status, last filing, next due return, open notices and the
// last thing the agent did there.
interface Row {
  code: string; name: string; status: string; detail: string; employer_no?: string; link: string
  employees_registered: number; employees_total: number
  last_filed?: { form: string; period: string; at: string; reference_no?: string }
  next_due?: { filing: string; date: string; overdue: boolean }
  open_notices: number
  last_activity?: { kind: string; title: string; status: string; at: string; link: string }
}

const { t, locale } = useI18n()
const router = useRouter()
const rows = ref<Row[]>([])
const loading = ref(true)
const tagType: Record<string, 'success' | 'error' | 'warning' | 'info' | 'default'> = {
  active: 'success', needs_info: 'error', needs_approval: 'warning', filing_started: 'info', awaiting_agency: 'default',
}
const attention = computed(() => rows.value.filter(r => r.status !== 'active').length)

function day(s: string) { return new Date(s).toLocaleDateString(locale.value === 'zh' ? 'zh-CN' : 'en-PH', { month: 'short', day: 'numeric', year: 'numeric' }) }
function ago(s: string) {
  const days = Math.floor((Date.now() - new Date(s).getTime()) / 86400000)
  return days <= 0 ? t('agencies.today') : t('agencies.daysAgo', { n: days })
}
function filingLabel(f: string) { return f.replace(/^[a-z]+_/, '').toUpperCase() }

const columns = computed<DataTableColumns<Row>>(() => [
  {
    title: t('agencies.agency'), key: 'name', minWidth: 150,
    render: r => h('div', [h('strong', r.name), r.employer_no ? h('div', { class: 'sub' }, r.employer_no) : h('div', { class: 'sub warn' }, t('agencies.noAccount'))]),
  },
  {
    title: t('agencies.status'), key: 'status', minWidth: 170,
    render: r => h('div', [h(NTag, { size: 'small', round: true, bordered: false, type: tagType[r.status] || 'default' }, () => t('home.reg_' + r.status)), h('div', { class: 'sub' }, r.detail)]),
  },
  {
    title: t('agencies.lastFiled'), key: 'last_filed', minWidth: 150,
    render: r => r.last_filed
      ? h('div', [h("div", `${filingLabel(r.last_filed.form)} · ${r.last_filed.period}`), h('div', { class: 'sub' }, day(r.last_filed.at) + (r.last_filed.reference_no ? ' · ' + r.last_filed.reference_no : ''))])
      : h(NText, { depth: 3 }, () => '—'),
  },
  {
    title: t('agencies.nextDue'), key: 'next_due', minWidth: 140,
    render: r => r.next_due
      ? h('div', [h('div', filingLabel(r.next_due.filing)), h('div', { class: r.next_due.overdue ? 'sub warn' : 'sub' }, (r.next_due.overdue ? t('agencies.overdue') + ' · ' : '') + day(r.next_due.date))])
      : h(NText, { depth: 3 }, () => '—'),
  },
  {
    title: t('agencies.notices'), key: 'open_notices', width: 90, align: 'center',
    render: r => r.open_notices ? h(NTag, { size: 'small', type: 'warning', bordered: false }, () => String(r.open_notices)) : h(NText, { depth: 3 }, () => '0'),
  },
  {
    title: t('agencies.agentActivity'), key: 'last_activity', minWidth: 220,
    render: r => r.last_activity
      ? h('div', [h('div', `${t('agencies.kind_' + r.last_activity.kind)}: ${r.last_activity.kind === 'filing' ? r.last_activity.title.replace(/^\S+/, filingLabel) : r.last_activity.title}`), h('div', { class: 'sub' }, `${r.last_activity.status} · ${ago(r.last_activity.at)}`)])
      : h(NText, { depth: 3 }, () => t('agencies.noActivity')),
  },
])

function rowProps(r: Row) {
  return { style: 'cursor: pointer', onClick: () => router.push(r.last_activity?.link || r.link) }
}

onMounted(async () => {
  try { rows.value = ((await homeAPI.agencies()) as { data: Row[] }).data || [] } catch { rows.value = [] } finally { loading.value = false }
})
</script>

<template>
  <div class="agencies">
    <p class="lead">
      <template v-if="!loading && rows.length">{{ attention ? t('agencies.attention', { n: attention, total: rows.length }) : t('agencies.allGood', { total: rows.length }) }}</template>
    </p>
    <NDataTable :columns="columns" :data="rows" :loading="loading" :row-props="rowProps" size="small" :bordered="false" :scroll-x="900" />
    <NEmpty v-if="!loading && !rows.length" :description="t('agencies.empty')" style="margin-top: 24px" />
  </div>
</template>

<style scoped>
.lead { margin: 0 0 12px; opacity: .8; }
.agencies :deep(.sub) { font-size: 12px; opacity: .6; margin-top: 2px; }
.agencies :deep(.sub.warn) { color: var(--n-error-color, #d03050); opacity: 1; }
</style>
