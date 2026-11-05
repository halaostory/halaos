<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { NButton, NUpload, NAlert, NTable, NTag, NSpace, NEmpty, useMessage, useDialog, type UploadFileInfo } from 'naive-ui'
import { payrollAPI } from '../../api/client'

interface Period { period_start: string; period_end: string; pay_date: string; employees: number; gross: number; net: number; withholding_tax: number }
interface Plan { rows: number; employees: number; periods: Period[]; issues: { line: number; message: string }[]; imported: boolean }
interface Imported { id: number; name: string; pay_date: string; total_employees: number; total_gross: number; total_net: number }

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()
const file = ref<File | null>(null)
const plan = ref<Plan | null>(null)
const busy = ref(false)
const history = ref<Imported[]>([])
const columns = ref<string[]>([])

async function load() {
  const d = ((await payrollAPI.history()) as { data: { periods: Imported[]; columns: string[] } }).data
  history.value = d.periods
  columns.value = d.columns
}
function onFile(o: { fileList: UploadFileInfo[] }) {
  file.value = o.fileList[0]?.file ?? null
  plan.value = null
  if (file.value) run(true)
}
async function run(dry: boolean) {
  if (!file.value) return
  busy.value = true
  try {
    const fd = new FormData()
    fd.append('file', file.value)
    plan.value = ((await payrollAPI.importHistory(fd, dry)) as { data: Plan }).data
    if (plan.value.imported) {
      message.success(t('payrollHistory.done', { n: plan.value.rows }))
      await load()
    }
  } catch (e) {
    message.error((e as { data?: { error?: { message?: string } } }).data?.error?.message || t('common.failed'))
  } finally {
    busy.value = false
  }
}
function remove(p: Imported) {
  dialog.warning({
    title: t('payrollHistory.deleteTitle'), content: p.name, positiveText: t('common.delete'), negativeText: t('common.cancel'),
    onPositiveClick: async () => { await payrollAPI.deleteHistory(p.id); await load() },
  })
}
function template() {
  const header = columns.value.join(',')
  const sample = 'EMP-00001,2026-01-01,2026-01-31,2026-01-31,42000,35200,42000,39400,6800,4200,1350,2700,30,1050,1050,200,200'
  const a = document.createElement('a')
  a.href = URL.createObjectURL(new Blob([header + '\n' + sample + '\n'], { type: 'text/csv' }))
  a.download = 'payroll-history-template.csv'
  a.click()
}
const fmt = (v: number) => v.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })
onMounted(load)
</script>

<template>
  <div class="hist">
    <p class="muted">{{ t('payrollHistory.desc') }}</p>
    <NSpace align="center" style="margin-bottom: 12px">
      <NUpload :max="1" :default-upload="false" accept=".csv" :show-file-list="false" @change="onFile">
        <NButton type="primary">{{ t('payrollHistory.choose') }}</NButton>
      </NUpload>
      <NButton quaternary @click="template">{{ t('payrollHistory.template') }}</NButton>
      <span v-if="file" class="muted small">{{ file.name }}</span>
    </NSpace>

    <template v-if="plan && !plan.imported">
      <NAlert v-if="plan.issues.length" type="error" :title="t('payrollHistory.issues', { n: plan.issues.length })" style="margin-bottom: 12px">
        <div v-for="i in plan.issues.slice(0, 20)" :key="i.line + i.message" class="small">{{ t('payrollHistory.line', { n: i.line }) }}: {{ i.message }}</div>
        <div class="small" style="margin-top: 6px">{{ t('payrollHistory.fixAndRetry') }}</div>
      </NAlert>
      <NAlert v-else type="success" style="margin-bottom: 12px">{{ t('payrollHistory.ready', { rows: plan.rows, employees: plan.employees, periods: plan.periods.length }) }}</NAlert>
      <NTable v-if="plan.periods.length" size="small" :single-line="false">
        <thead><tr><th>{{ t('payrollHistory.period') }}</th><th>{{ t('payroll.payDate') }}</th><th class="num">{{ t('payrollHistory.people') }}</th><th class="num">{{ t('payrollHistory.gross') }}</th><th class="num">{{ t('payrollHistory.tax') }}</th><th class="num">{{ t('payrollHistory.net') }}</th></tr></thead>
        <tbody>
          <tr v-for="p in plan.periods" :key="p.period_start + p.pay_date">
            <td>{{ p.period_start }} – {{ p.period_end }}</td><td>{{ p.pay_date }}</td><td class="num">{{ p.employees }}</td>
            <td class="num">{{ fmt(p.gross) }}</td><td class="num">{{ fmt(p.withholding_tax) }}</td><td class="num">{{ fmt(p.net) }}</td>
          </tr>
        </tbody>
      </NTable>
      <NSpace justify="end" style="margin-top: 12px">
        <NButton type="primary" :disabled="!!plan.issues.length || !plan.rows" :loading="busy" @click="run(false)">{{ t('payrollHistory.import', { n: plan.rows }) }}</NButton>
      </NSpace>
    </template>

    <h3 class="sub">{{ t('payrollHistory.imported') }}</h3>
    <NEmpty v-if="!history.length" :description="t('payrollHistory.none')" size="small" />
    <NTable v-else size="small" :single-line="false">
      <tbody>
        <tr v-for="p in history" :key="p.id">
          <td>{{ p.name }} <NTag size="tiny" round :bordered="false">{{ t('payrollHistory.tag') }}</NTag></td>
          <td class="num">{{ p.total_employees }}</td><td class="num">{{ fmt(p.total_gross) }}</td>
          <td class="num"><NButton text type="error" size="tiny" @click="remove(p)">{{ t('common.delete') }}</NButton></td>
        </tr>
      </tbody>
    </NTable>
  </div>
</template>

<style scoped>
.muted { opacity: .7; font-size: 13px; max-width: 760px; }
.small { font-size: 12px; }
.num { text-align: right; font-variant-numeric: tabular-nums; }
.sub { font-size: 15px; margin: 24px 0 8px; }
</style>
