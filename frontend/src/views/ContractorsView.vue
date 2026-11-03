<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NSpace, NTabs, NTabPane, NButton, NDataTable, NTag, NModal, NForm, NFormItem, NInput, NInputNumber,
  NSelect, NCheckbox, NDatePicker, NAlert, NText, useMessage, useDialog, type DataTableColumns,
} from 'naive-ui'
import { format } from 'date-fns'
import { contractorsAPI } from '../api/client'
import WisePayModal from '../components/contractors/WisePayModal.vue'

interface ATC { code: string; description: string; rate: number; kind: string }
interface Contractor { id: number; kind: string; name: string; email: string | null; tin: string; address: string; is_vat_registered: boolean; ewt_rate: string; atc_code: string; bank_name: string | null; bank_account_no: string | null; bank_account_name: string | null; status: string; ytd_paid: number }
interface Payment { id: number; contractor_id: number; contractor_name: string; invoice_no: string; description: string; service_date: string; gross_amount: string; vat_amount: string; ewt_amount: string; net_amount: string; atc_code: string; status: string; paid_at: string | null; paid_reference: string | null }

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()
const unwrap = <T,>(r: unknown): T => ((r as { data?: T }).data ?? r) as T
const errMsg = (e: unknown) => (e as { data?: { error?: { message?: string } } }).data?.error?.message || t('common.saveFailed')
const peso = (v: string | number) => Number(v || 0).toLocaleString('en-PH', { minimumFractionDigits: 2, maximumFractionDigits: 2 })

const atcs = ref<ATC[]>([])
const contractors = ref<Contractor[]>([])
const payments = ref<Payment[]>([])
const statusFilter = ref<string | null>(null)

async function load() {
  contractors.value = unwrap<Contractor[]>(await contractorsAPI.list()) || []
  payments.value = unwrap<Payment[]>(await contractorsAPI.payments(statusFilter.value ? { status: statusFilter.value } : undefined)) || []
}
async function run(fn: () => Promise<unknown>, ok = t('common.saved')) {
  try { await fn(); message.success(ok); await load() } catch (e) { message.error(errMsg(e)) }
}

// ── contractor form ──
const showC = ref(false)
const blankC = () => ({ id: 0, kind: 'individual', name: '', email: '', tin: '', address: '', is_vat_registered: false, atc_code: 'WI010', bank_name: '', bank_account_no: '', bank_account_name: '', status: 'active' })
const cf = ref(blankC())
const atcOptions = computed(() => atcs.value.filter((a) => a.kind === cf.value.kind).map((a) => ({ label: `${a.code} · ${(a.rate * 100).toFixed(0)}% · ${a.description}`, value: a.code })))
function editC(c?: Contractor) {
  cf.value = c ? { ...blankC(), ...c, email: c.email || '', bank_name: c.bank_name || '', bank_account_no: c.bank_account_no || '', bank_account_name: c.bank_account_name || '' } : blankC()
  showC.value = true
}
function onKind(k: string) {
  cf.value.kind = k
  cf.value.atc_code = k === 'company' ? 'WC010' : 'WI010'
}
async function saveC() {
  const body: Record<string, unknown> = { ...cf.value }
  delete body.id
  await run(() => (cf.value.id ? contractorsAPI.update(cf.value.id, body) : contractorsAPI.create(body)))
  showC.value = false
}

// ── payment form ──
const showP = ref(false)
const pf = ref({ contractor_id: null as number | null, invoice_no: '', description: '', service_date: Date.now() as number | null, gross_amount: null as number | null })
const sel = computed(() => contractors.value.find((c) => c.id === pf.value.contractor_id))
const preview = computed(() => {
  const c = sel.value
  const g = pf.value.gross_amount || 0
  if (!c || g <= 0) return null
  const rate = Number(c.ewt_rate)
  const base = c.is_vat_registered ? Math.round((g / 1.12) * 100) / 100 : g
  const ewt = Math.round(base * rate * 100) / 100
  return { base, ewt, net: Math.round((g - ewt) * 100) / 100, rate }
})
async function saveP() {
  await run(() => contractorsAPI.createPayment({
    contractor_id: pf.value.contractor_id, invoice_no: pf.value.invoice_no, description: pf.value.description,
    gross_amount: pf.value.gross_amount, service_date: pf.value.service_date ? format(new Date(pf.value.service_date), 'yyyy-MM-dd') : undefined,
  }))
  showP.value = false
}
// Cross-border payouts through Wise
const wiseOpen = ref(false)
const wisePayment = ref<number | null>(null)
const linkOpen = ref(false)
const linkFor = ref<Contractor | null>(null)
const linkForm = ref({ recipient_id: null as number | null, currency: 'USD', label: '' })
async function linkWise(c: Contractor) {
  linkFor.value = c
  linkForm.value = { recipient_id: null, currency: 'USD', label: '' }
  try {
    const a = ((await contractorsAPI.payoutAccount(c.id)) as { data: { account: { recipient_id: number; currency: string; label: string } | null } }).data.account
    if (a) linkForm.value = { recipient_id: a.recipient_id, currency: a.currency, label: a.label }
  } catch { /* none yet */ }
  linkOpen.value = true
}
async function saveLink() {
  if (!linkFor.value || !linkForm.value.recipient_id) return
  try {
    await contractorsAPI.setPayoutAccount(linkFor.value.id, { recipient_id: linkForm.value.recipient_id, currency: linkForm.value.currency, label: linkForm.value.label })
    message.success(t('common.saved'))
    linkOpen.value = false
  } catch (e) {
    message.error((e as { data?: { error?: { message?: string } } }).data?.error?.message || t('common.saveFailed'))
  }
}

function markPaid(p: Payment) {
  const ref = { value: '' }
  dialog.info({
    title: t('contractors.markPaid'),
    content: () => h(NInput, { placeholder: t('contractors.bankReference'), onUpdateValue: (v: string) => (ref.value = v) }),
    positiveText: t('common.confirm'), negativeText: t('common.cancel'),
    onPositiveClick: () => run(() => contractorsAPI.pay(p.id, { reference: ref.value })),
  })
}
async function bankFile() {
  const blob = (await contractorsAPI.bankFileBlob()) as Blob
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = `contractor_payments_${format(new Date(), 'yyyyMMdd')}.csv`
  a.click()
}

// ── 2307 ──
const quarter = ref(`${new Date().getFullYear()}-Q${Math.floor(new Date().getMonth() / 3) + 1}`)
async function cert(c: Contractor) {
  try {
    const blob = (await contractorsAPI.certificateBlob(c.id, quarter.value)) as Blob
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob)
    a.download = `BIR2307_${c.name}_${quarter.value}.pdf`
    a.click()
  } catch (e) {
    message.error(errMsg(e))
  }
}

const stType: Record<string, 'default' | 'info' | 'success' | 'warning' | 'error'> = { draft: 'default', approved: 'info', paid: 'success', void: 'error' }
const cCols = computed<DataTableColumns<Contractor>>(() => [
  { title: t('contractors.name'), key: 'name', render: (r) => h('div', [h('b', r.name), h('div', { style: 'opacity:.6;font-size:12px' }, `${t('contractors.kind_' + r.kind)} · TIN ${r.tin || '—'}`)]) },
  { title: 'ATC', key: 'atc_code', width: 150, render: (r) => `${r.atc_code} · ${(Number(r.ewt_rate) * 100).toFixed(0)}%${r.is_vat_registered ? ' · VAT' : ''}` },
  { title: t('contractors.ytdPaid'), key: 'ytd_paid', width: 140, render: (r) => '₱' + peso(r.ytd_paid) },
  { title: t('common.status'), key: 'status', width: 100, render: (r) => h(NTag, { size: 'small', type: r.status === 'active' ? 'success' : 'default' }, () => r.status) },
  { title: '', key: 'a', width: 200, render: (r) => h(NSpace, { size: 4 }, () => [
    h(NButton, { size: 'small', onClick: () => editC(r) }, () => t('common.edit')),
    h(NButton, { size: 'small', onClick: () => cert(r) }, () => `2307 ${quarter.value}`),
    h(NButton, { size: 'small', quaternary: true, onClick: () => linkWise(r) }, () => t('wise.link')),
  ]) },
])
const pCols = computed<DataTableColumns<Payment>>(() => [
  { title: t('contractors.payee'), key: 'contractor_name' },
  { title: t('contractors.invoice'), key: 'invoice_no', width: 110 },
  { title: t('contractors.gross'), key: 'gross_amount', width: 120, render: (r) => peso(r.gross_amount) },
  { title: 'EWT', key: 'ewt_amount', width: 110, render: (r) => `${peso(r.ewt_amount)} (${r.atc_code})` },
  { title: t('contractors.net'), key: 'net_amount', width: 120, render: (r) => h('b', peso(r.net_amount)) },
  { title: t('common.status'), key: 'status', width: 100, render: (r) => h(NTag, { size: 'small', type: stType[r.status] }, () => t('contractors.st_' + r.status)) },
  { title: '', key: 'a', width: 210, render: (r) => h(NSpace, { size: 4 }, () => [
    r.status === 'draft' ? h(NButton, { size: 'small', type: 'primary', onClick: () => run(() => contractorsAPI.approve(r.id)) }, () => t('common.approve')) : null,
    r.status === 'approved' ? h(NButton, { size: 'small', type: 'primary', secondary: true, onClick: () => { wisePayment.value = r.id; wiseOpen.value = true } }, () => t('wise.payVia')) : null,
    r.status === 'approved' ? h(NButton, { size: 'small', onClick: () => markPaid(r) }, () => t('contractors.markPaid')) : null,
    ['draft', 'approved'].includes(r.status) ? h(NButton, { size: 'small', quaternary: true, type: 'error', onClick: () => run(() => contractorsAPI.void(r.id)) }, () => t('contractors.void')) : null,
    r.paid_reference ? h(NText, { depth: 3 }, () => r.paid_reference) : null,
  ]) },
])

onMounted(async () => {
  atcs.value = unwrap<ATC[]>(await contractorsAPI.atcs()) || []
  await load()
})
</script>

<template>
  <NSpace vertical :size="16">
    <div>
      <h2 style="margin: 0">{{ t('contractors.title') }}</h2>
      <div style="opacity: 0.7">{{ t('contractors.subtitle') }}</div>
      <WisePayModal v-model:show="wiseOpen" :payment-id="wisePayment" @paid="load" />
    <NModal v-model:show="linkOpen" preset="card" :title="t('wise.linkTitle') + (linkFor ? ' — ' + linkFor.name : '')" style="max-width: 480px">
      <p style="opacity:.7;font-size:13px;margin-top:0">{{ t('wise.linkHelp') }}</p>
      <NForm label-placement="top">
        <NFormItem :label="t('wise.recipientId')"><NInputNumber v-model:value="linkForm.recipient_id" :show-button="false" style="width: 100%" /></NFormItem>
        <NFormItem :label="t('wise.currency')"><NInput v-model:value="linkForm.currency" style="width: 100px" /></NFormItem>
        <NFormItem :label="t('wise.label')"><NInput v-model:value="linkForm.label" :placeholder="t('wise.labelPh')" /></NFormItem>
      </NForm>
      <NSpace justify="end">
        <NButton @click="linkOpen = false">{{ t('common.cancel') }}</NButton>
        <NButton type="primary" :disabled="!linkForm.recipient_id" @click="saveLink">{{ t('common.save') }}</NButton>
      </NSpace>
    </NModal>
  </div>
    <NTabs type="line">
      <NTabPane name="payments" :tab="t('contractors.tabPayments')">
        <NSpace vertical>
          <NSpace>
            <NButton type="primary" :disabled="!contractors.length" @click="showP = true">{{ t('contractors.newPayment') }}</NButton>
            <NButton @click="bankFile">⬇ {{ t('contractors.bankFile') }}</NButton>
            <NSelect v-model:value="statusFilter" clearable style="width: 160px" :placeholder="t('common.status')"
              :options="['draft', 'approved', 'paid', 'void'].map((s) => ({ label: t('contractors.st_' + s), value: s }))" @update:value="load" />
          </NSpace>
          <NDataTable :columns="pCols" :data="payments" :row-key="(r: Payment) => r.id" />
        </NSpace>
      </NTabPane>
      <NTabPane name="contractors" :tab="t('contractors.tabContractors')">
        <NSpace vertical>
          <NSpace align="center">
            <NButton type="primary" @click="editC()">{{ t('contractors.add') }}</NButton>
            <span>{{ t('contractors.quarterFor2307') }}</span>
            <NInput v-model:value="quarter" style="width: 110px" placeholder="2026-Q3" />
          </NSpace>
          <NDataTable :columns="cCols" :data="contractors" :row-key="(r: Contractor) => r.id" />
        </NSpace>
      </NTabPane>
    </NTabs>

    <NModal v-model:show="showC" preset="card" style="max-width: 600px" :title="cf.id ? t('common.edit') : t('contractors.add')">
      <NForm label-placement="top">
        <NFormItem :label="t('contractors.kind')">
          <NSelect :value="cf.kind" :options="[{ label: t('contractors.kind_individual'), value: 'individual' }, { label: t('contractors.kind_company'), value: 'company' }]" @update:value="onKind" />
        </NFormItem>
        <NFormItem :label="t('contractors.name')"><NInput v-model:value="cf.name" /></NFormItem>
        <NSpace :wrap="false">
          <NFormItem label="TIN"><NInput v-model:value="cf.tin" placeholder="000-000-000-000" /></NFormItem>
          <NFormItem :label="t('common.email')"><NInput v-model:value="cf.email" /></NFormItem>
        </NSpace>
        <NFormItem :label="t('contractors.address')"><NInput v-model:value="cf.address" /></NFormItem>
        <NFormItem label="ATC"><NSelect v-model:value="cf.atc_code" :options="atcOptions" /></NFormItem>
        <NCheckbox v-model:checked="cf.is_vat_registered">{{ t('contractors.vatRegistered') }}</NCheckbox>
        <NSpace :wrap="false" style="margin-top: 12px">
          <NFormItem :label="t('contractors.bank')"><NInput v-model:value="cf.bank_name" /></NFormItem>
          <NFormItem :label="t('contractors.accountNo')"><NInput v-model:value="cf.bank_account_no" /></NFormItem>
        </NSpace>
        <NFormItem v-if="cf.id" :label="t('common.status')">
          <NSelect v-model:value="cf.status" :options="[{ label: 'active', value: 'active' }, { label: 'inactive', value: 'inactive' }]" />
        </NFormItem>
      </NForm>
      <NSpace justify="end"><NButton @click="showC = false">{{ t('common.cancel') }}</NButton><NButton type="primary" :disabled="!cf.name" @click="saveC">{{ t('common.save') }}</NButton></NSpace>
    </NModal>

    <NModal v-model:show="showP" preset="card" style="max-width: 520px" :title="t('contractors.newPayment')">
      <NForm label-placement="top">
        <NFormItem :label="t('contractors.payee')">
          <NSelect v-model:value="pf.contractor_id" filterable :options="contractors.filter((c) => c.status === 'active').map((c) => ({ label: c.name, value: c.id }))" />
        </NFormItem>
        <NSpace :wrap="false">
          <NFormItem :label="t('contractors.invoice')"><NInput v-model:value="pf.invoice_no" /></NFormItem>
          <NFormItem :label="t('contractors.serviceDate')"><NDatePicker v-model:value="pf.service_date" type="date" /></NFormItem>
        </NSpace>
        <NFormItem :label="t('contractors.description')"><NInput v-model:value="pf.description" /></NFormItem>
        <NFormItem :label="t('contractors.grossHint')"><NInputNumber v-model:value="pf.gross_amount" :min="0" :precision="2" style="width: 100%" /></NFormItem>
      </NForm>
      <NAlert v-if="preview" type="info" :show-icon="false">
        {{ t('contractors.preview', { base: peso(preview.base), rate: (preview.rate * 100).toFixed(0), ewt: peso(preview.ewt), net: peso(preview.net) }) }}
      </NAlert>
      <NSpace justify="end" style="margin-top: 12px"><NButton @click="showP = false">{{ t('common.cancel') }}</NButton><NButton type="primary" :disabled="!pf.contractor_id || !pf.gross_amount" @click="saveP">{{ t('common.save') }}</NButton></NSpace>
    </NModal>
  </NSpace>
</template>
