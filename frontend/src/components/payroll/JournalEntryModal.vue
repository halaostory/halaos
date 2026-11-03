<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { NModal, NButton, NSpin, NTable, NInput, NCollapse, NCollapseItem, NAlert, useMessage } from 'naive-ui'
import { payrollAPI } from '../../api/client'

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
watch(() => [props.show, props.cycleId], () => { if (props.show) load() }, { immediate: true })

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
          <NButton type="primary" @click="download('quickbooks')">{{ t('journal.quickbooks') }}</NButton>
          <NButton @click="download('xero')">{{ t('journal.xero') }}</NButton>
          <span class="muted small">{{ t('journal.hint') }}</span>
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
.actions { display: flex; gap: 8px; align-items: center; margin-top: 12px; flex-wrap: wrap; }
.accounts { margin-top: 16px; }
.acc-row { display: flex; gap: 8px; align-items: center; margin-bottom: 6px; }
.acc-role { width: 230px; font-size: 13px; flex: none; }
</style>
