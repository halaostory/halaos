<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NCard, NSpace, NTabs, NTabPane, NButton, NDataTable, NTag, NModal, NForm, NFormItem, NInput, NSelect,
  NCheckbox, NAlert, NText, useMessage, type DataTableColumns,
} from 'naive-ui'
import { documentsAPI, employeeAPI } from '../api/client'

interface Tpl { id: number; kind: string; name: string; body: string; is_active: boolean }
interface Req { id: number; employee_id: number; title: string; kind: string; signer_name: string; status: string; signed_at: string | null; created_at: string; has_pdf: boolean; declined_reason: string | null }

const { t } = useI18n()
const message = useMessage()
const unwrap = <T,>(r: unknown): T => ((r as { data?: T }).data ?? r) as T
const errMsg = (e: unknown) => (e as { data?: { error?: { message?: string } } }).data?.error?.message || t('common.saveFailed')

const templates = ref<Tpl[]>([])
const requests = ref<Req[]>([])
const fields = ref<string[]>([])
const employees = ref<{ label: string; value: number }[]>([])
const kinds = ['offer_letter', 'employment_contract', 'nda', 'policy_ack', 'other']
const kindOpts = computed(() => kinds.map((k) => ({ label: t('docs.kind_' + k), value: k })))

// templates
const showTpl = ref(false)
const tpl = ref({ id: 0, kind: 'offer_letter', name: '', body: '', is_active: true })
const aiInstr = ref('')
const drafting = ref(false)
function editTpl(x?: Tpl) {
  tpl.value = x ? { ...x } : { id: 0, kind: 'offer_letter', name: '', body: '', is_active: true }
  aiInstr.value = ''
  showTpl.value = true
}
async function aiDraft() {
  drafting.value = true
  try {
    tpl.value.body = unwrap<{ body: string }>(await documentsAPI.aiDraft(tpl.value.kind, aiInstr.value)).body
  } catch (e) {
    message.error(errMsg(e))
  } finally {
    drafting.value = false
  }
}
async function saveTpl() {
  try {
    const body = { kind: tpl.value.kind, name: tpl.value.name, body: tpl.value.body, is_active: tpl.value.is_active }
    if (tpl.value.id) await documentsAPI.updateTemplate(tpl.value.id, body)
    else await documentsAPI.createTemplate(body)
    showTpl.value = false
    message.success(t('common.saved'))
    await load()
  } catch (e) {
    message.error(errMsg(e))
  }
}
function insertField(f: string) {
  tpl.value.body += `{{${f}}}`
}

// requests
const showReq = ref(false)
const req = ref({ employee_id: null as number | null, template_id: null as number | null, send_email: false })
const lastLink = ref('')
async function sendReq() {
  try {
    const out = unwrap<{ link: { url: string; emailed: boolean } }>(await documentsAPI.createRequest({ ...req.value }))
    lastLink.value = out.link.url
    message.success(out.link.emailed ? t('docs.emailed') : t('docs.linkReady'))
    await load()
  } catch (e) {
    message.error(errMsg(e))
  }
}
async function pdf(r: Req) {
  const blob = (await documentsAPI.pdfBlob(r.id)) as Blob
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = `${r.title}_signed.pdf`
  a.click()
}
async function voidReq(r: Req) {
  try {
    await documentsAPI.voidRequest(r.id)
    await load()
  } catch (e) {
    message.error(errMsg(e))
  }
}
function copy(s: string) {
  navigator.clipboard?.writeText(s)
  message.success(t('docs.copied'))
}

const stType: Record<string, 'default' | 'info' | 'success' | 'warning' | 'error'> = { pending: 'info', viewed: 'info', signed: 'success', declined: 'error', voided: 'default', expired: 'warning' }
const tplCols = computed<DataTableColumns<Tpl>>(() => [
  { title: t('docs.name'), key: 'name' },
  { title: t('docs.kind'), key: 'kind', width: 180, render: (r) => t('docs.kind_' + r.kind) },
  { title: '', key: 'active', width: 100, render: (r) => (r.is_active ? '' : h(NTag, { size: 'small' }, () => t('docs.inactive'))) },
  { title: '', key: 'a', width: 90, render: (r) => h(NButton, { size: 'small', onClick: () => editTpl(r) }, () => t('common.edit')) },
])
const reqCols = computed<DataTableColumns<Req>>(() => [
  { title: t('docs.document'), key: 'title' },
  { title: t('docs.signer'), key: 'signer_name', width: 180 },
  { title: t('common.status'), key: 'status', width: 120, render: (r) => h(NTag, { size: 'small', type: stType[r.status] }, () => t('docs.st_' + r.status)) },
  { title: t('docs.sent'), key: 'created_at', width: 120, render: (r) => r.created_at.slice(0, 10) },
  { title: '', key: 'a', width: 170, render: (r) => h(NSpace, { size: 4 }, () => [
    r.has_pdf ? h(NButton, { size: 'small', onClick: () => pdf(r) }, () => 'PDF') : null,
    ['pending', 'viewed'].includes(r.status) ? h(NButton, { size: 'small', quaternary: true, type: 'error', onClick: () => voidReq(r) }, () => t('docs.void')) : null,
  ]) },
])

async function load() {
  templates.value = unwrap<Tpl[]>(await documentsAPI.templates()) || []
  requests.value = unwrap<Req[]>(await documentsAPI.requests()) || []
}
onMounted(async () => {
  await load()
  fields.value = unwrap<string[]>(await documentsAPI.mergeFields()) || []
  const e = unwrap<{ id: number; first_name: string; last_name: string }[]>(await employeeAPI.list({ limit: '500' })) || []
  employees.value = e.map((x) => ({ label: `${x.first_name} ${x.last_name}`, value: x.id }))
})
</script>

<template>
  <NSpace vertical :size="16">
    <div>
      <h2 style="margin: 0">{{ t('docs.title') }}</h2>
      <div style="opacity: 0.7">{{ t('docs.subtitle') }}</div>
    </div>
    <NTabs type="line">
      <NTabPane name="requests" :tab="t('docs.tabRequests')">
        <NSpace vertical>
          <NButton type="primary" @click="lastLink = ''; showReq = true">{{ t('docs.send') }}</NButton>
          <NDataTable :columns="reqCols" :data="requests" :row-key="(r: Req) => r.id" />
        </NSpace>
      </NTabPane>
      <NTabPane name="templates" :tab="t('docs.tabTemplates')">
        <NSpace vertical>
          <NButton type="primary" @click="editTpl()">{{ t('docs.newTemplate') }}</NButton>
          <NDataTable :columns="tplCols" :data="templates" :row-key="(r: Tpl) => r.id" />
        </NSpace>
      </NTabPane>
    </NTabs>

    <NModal v-model:show="showTpl" preset="card" style="max-width: 860px" :title="tpl.id ? t('common.edit') : t('docs.newTemplate')">
      <NForm label-placement="top">
        <NSpace :wrap="false">
          <NFormItem :label="t('docs.name')" style="flex: 1"><NInput v-model:value="tpl.name" /></NFormItem>
          <NFormItem :label="t('docs.kind')" style="width: 220px"><NSelect v-model:value="tpl.kind" :options="kindOpts" /></NFormItem>
        </NSpace>
        <NCard size="small" embedded style="margin-bottom: 12px">
          <NSpace align="center" :wrap="false">
            <NInput v-model:value="aiInstr" :placeholder="t('docs.aiPlaceholder')" />
            <NButton :loading="drafting" @click="aiDraft">✨ {{ t('docs.aiDraft') }}</NButton>
          </NSpace>
          <NText depth="3" style="font-size: 12px">{{ t('docs.aiHint') }}</NText>
        </NCard>
        <NFormItem :label="t('docs.body')">
          <NInput v-model:value="tpl.body" type="textarea" :rows="14" />
        </NFormItem>
        <NSpace :size="4" style="margin-bottom: 12px">
          <NTag v-for="f in fields" :key="f" size="small" style="cursor: pointer" @click="insertField(f)">{{ '{' + '{' + f + '}' + '}' }}</NTag>
        </NSpace>
        <NCheckbox v-if="tpl.id" v-model:checked="tpl.is_active">{{ t('lifecycle.active') }}</NCheckbox>
      </NForm>
      <NSpace justify="end"><NButton @click="showTpl = false">{{ t('common.cancel') }}</NButton><NButton type="primary" @click="saveTpl">{{ t('common.save') }}</NButton></NSpace>
    </NModal>

    <NModal v-model:show="showReq" preset="card" style="max-width: 520px" :title="t('docs.send')">
      <NForm label-placement="top">
        <NFormItem :label="t('docs.signer')"><NSelect v-model:value="req.employee_id" filterable :options="employees" /></NFormItem>
        <NFormItem :label="t('docs.template')">
          <NSelect v-model:value="req.template_id" :options="templates.filter((x) => x.is_active).map((x) => ({ label: x.name, value: x.id }))" />
        </NFormItem>
        <NCheckbox v-model:checked="req.send_email">{{ t('docs.sendEmail') }}</NCheckbox>
      </NForm>
      <NAlert v-if="lastLink" type="success" style="margin-top: 12px" :title="t('docs.linkReady')">
        <NSpace align="center"><code style="word-break: break-all">{{ lastLink }}</code><NButton size="small" @click="copy(lastLink)">{{ t('docs.copy') }}</NButton></NSpace>
      </NAlert>
      <NSpace justify="end" style="margin-top: 12px">
        <NButton @click="showReq = false">{{ t('common.cancel') }}</NButton>
        <NButton type="primary" :disabled="!req.employee_id || !req.template_id" @click="sendReq">{{ t('docs.send') }}</NButton>
      </NSpace>
    </NModal>
  </NSpace>
</template>
