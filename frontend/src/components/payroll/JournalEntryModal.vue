<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { NModal, NButton, NSpin, NTable, NInput, NCollapse, NCollapseItem, NAlert, useMessage } from 'naive-ui'
import { payrollAPI, glAPI } from '../../api/client'

interface Line { role: string; account_code: string; account_name: string; description: string; debit: number; credit: number }
interface Journal { number: string; date: string; memo: string; currency: string; lines: Line[]; total_debit: number; total_credit: number }
interface Account { code: string; name: string }

const props = defineProps<{ show: boolean; cycleId: number | null; title: string }>()
const emit = defineEmits<{ (e: 'update:show', v: boolean): void }>()
const { t } = useI18n()
const message = useMessage()
const loading = ref(false)
const journal = ref<Journal | null>(null)
const error = ref('')
const roles = ref<string[]>([])
const accounts = ref<Record<string, Account>>({})
const saving = ref(false)
interface GLConn { provider: string; available: boolean; connected: boolean; tenant_name?: string }
const gl = ref<GLConn[]>([])
const pushing = ref('')
const glName: Record<string, string> = { xero: 'Xero', quickbooks: 'QuickBooks', netsuite: 'NetSuite' }
const nsOpen = ref(false)
const nsSaving = ref(false)
const ns = ref({ account_id: '', consumer_key: '', consumer_secret: '', token_id: '', token_secret: '', subsidiary_id: '' })
async function saveNetSuite() {
  nsSaving.value = true
  try {
    await glAPI.netsuite({ ...ns.value })
    message.success(t('journal.nsConnected'))
    nsOpen.value = false
    ns.value = { account_id: '', consumer_key: '', consumer_secret: '', token_id: '', token_secret: '', subsidiary_id: '' }
    await loadGL()
  } catch (e) {
    message.error((e as { data?: { error?: { message?: string } } }).data?.error?.message || t('common.failed'))
  } finally {
    nsSaving.value = false
  }
}
async function loadGL() {
  try { gl.value = ((await glAPI.connections()) as { data: GLConn[] }).data } catch { gl.value = [] }
}
async function connectGL(p: string) {
  if (p === 'netsuite') { nsOpen.value = !nsOpen.value; return }
  const { url } = ((await glAPI.connect(p)) as { data: { url: string } }).data
  window.location.href = url
}
async function pushGL(p: string) {
  if (!props.cycleId) return
  pushing.value = p
  try {
    const d = ((await glAPI.push(p, props.cycleId)) as { data: { already_pushed: boolean } }).data
    message.success(d.already_pushed ? t('journal.alreadySent', { app: glName[p] }) : t('journal.sent', { app: glName[p] }))
  } catch (e) {
    message.error((e as { data?: { error?: { message?: string } } }).data?.error?.message || t('common.failed'))
  } finally {
    pushing.value = ''
  }
}

async function load() {
  if (!props.cycleId) return
  loading.value = true
  error.value = ''
  journal.value = null
  try {
    journal.value = ((await payrollAPI.cycleJournal(props.cycleId)) as { data: Journal }).data
    const m = ((await payrollAPI.glAccounts()) as { data: { roles: string[]; accounts: Record<string, Account> } }).data
    roles.value = m.roles
    accounts.value = m.accounts
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}
watch(() => [props.show, props.cycleId], () => { if (props.show) { load(); loadGL() } }, { immediate: true })

async function saveAccounts() {
  saving.value = true
  try {
    await payrollAPI.saveGlAccounts(accounts.value)
    message.success(t('journal.saved'))
    await load()
  } catch (e) {
    message.error((e as Error).message)
  } finally {
    saving.value = false
  }
}

async function download(format: 'quickbooks' | 'xero') {
  if (!props.cycleId) return
  const blob = (await payrollAPI.cycleJournalCSV(props.cycleId, format)) as Blob
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = `journal-${journal.value?.number || props.cycleId}-${format}.csv`
  a.click()
}
const fmt = (v: number) => (v ? v.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 }) : '')
</script>

<template>
  <NModal :show="show" preset="card" :title="t('journal.title') + ' — ' + title" style="max-width: 860px" @update:show="(v: boolean) => emit('update:show', v)">
    <NSpin :show="loading">
      <NAlert v-if="error" type="warning">{{ error }}</NAlert>
      <template v-if="journal">
        <p class="muted">{{ journal.number }} · {{ journal.date }} · {{ journal.memo }}</p>
        <NTable size="small" :single-line="false">
          <thead>
            <tr><th>{{ t('journal.account') }}</th><th>{{ t('journal.description') }}</th><th class="num">{{ t('journal.debit') }}</th><th class="num">{{ t('journal.credit') }}</th></tr>
          </thead>
          <tbody>
            <tr v-for="(l, i) in journal.lines" :key="i">
              <td><strong>{{ l.account_code }}</strong> {{ l.account_name }}</td>
              <td class="muted">{{ l.description }}</td>
              <td class="num">{{ fmt(l.debit) }}</td>
              <td class="num">{{ fmt(l.credit) }}</td>
            </tr>
            <tr class="total">
              <td colspan="2">{{ t('journal.total') }} ({{ journal.currency }})</td>
              <td class="num">{{ fmt(journal.total_debit) }}</td>
              <td class="num">{{ fmt(journal.total_credit) }}</td>
            </tr>
          </tbody>
        </NTable>
        <div class="actions">
          <template v-for="g in gl" :key="g.provider">
            <NButton v-if="g.connected" type="primary" :loading="pushing === g.provider" @click="pushGL(g.provider)">{{ t('journal.sendTo', { app: glName[g.provider] }) }}</NButton>
            <NButton v-else-if="g.available" secondary @click="connectGL(g.provider)">{{ t('journal.connect', { app: glName[g.provider] }) }}</NButton>
          </template>
          <NButton @click="download('quickbooks')">{{ t('journal.quickbooks') }}</NButton>
          <NButton @click="download('xero')">{{ t('journal.xero') }}</NButton>
          <span class="muted small">{{ t('journal.hint') }}</span>
        </div>
        <div v-if="nsOpen" class="ns-form">
          <p class="muted small">{{ t('journal.nsHelp') }}</p>
          <div class="ns-grid">
            <NInput v-model:value="ns.account_id" :placeholder="t('journal.nsAccount')" />
            <NInput v-model:value="ns.subsidiary_id" :placeholder="t('journal.nsSubsidiary')" />
            <NInput v-model:value="ns.consumer_key" :placeholder="t('journal.nsConsumerKey')" />
            <NInput v-model:value="ns.consumer_secret" type="password" show-password-on="click" :placeholder="t('journal.nsConsumerSecret')" />
            <NInput v-model:value="ns.token_id" :placeholder="t('journal.nsTokenId')" />
            <NInput v-model:value="ns.token_secret" type="password" show-password-on="click" :placeholder="t('journal.nsTokenSecret')" />
          </div>
          <NButton type="primary" size="small" :loading="nsSaving" @click="saveNetSuite">{{ t('journal.nsSave') }}</NButton>
        </div>
        <NCollapse class="accounts">
          <NCollapseItem :title="t('journal.mapAccounts')" name="acc">
            <div v-for="r in roles" :key="r" class="acc-row">
              <span class="acc-role">{{ t('journal.role_' + r) }}</span>
              <NInput v-model:value="accounts[r].code" size="small" style="width: 110px" />
              <NInput v-model:value="accounts[r].name" size="small" />
            </div>
            <NButton size="small" type="primary" :loading="saving" @click="saveAccounts">{{ t('common.save') }}</NButton>
          </NCollapseItem>
        </NCollapse>
      </template>
    </NSpin>
  </NModal>
</template>

<style scoped>
.muted { opacity: .65; }
.small { font-size: 12px; }
.num { text-align: right; font-variant-numeric: tabular-nums; white-space: nowrap; }
.total td { font-weight: 600; }
.ns-form { margin-top: 12px; padding: 12px; border: 1px solid var(--n-border-color, rgba(128,128,128,.25)); border-radius: 8px; }
.ns-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 8px; margin-bottom: 8px; }
.actions { display: flex; gap: 8px; align-items: center; margin-top: 12px; flex-wrap: wrap; }
.accounts { margin-top: 16px; }
.acc-row { display: flex; gap: 8px; align-items: center; margin-bottom: 6px; }
.acc-role { width: 230px; font-size: 13px; flex: none; }
</style>
