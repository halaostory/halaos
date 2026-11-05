<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NCard, NSpace, NTabs, NTabPane, NButton, NDataTable, NTag, NModal, NForm, NFormItem, NInput, NSelect,
  NSwitch, NAlert, NDropdown, useMessage, useDialog, type DataTableColumns,
} from 'naive-ui'
import { devicesAPI, employeeAPI } from '../api/client'

interface Device { id: number; asset_tag: string; kind: string; make_model: string; serial_no: string; os: string; mdm_device_id: string | null; status: string; employee_name: string; assigned_employee_id: number | null }
interface Action { id: number; device_id: number; asset_tag: string; make_model: string; action: string; status: string; trigger: string; result: string | null; error_message: string | null; created_at: string }

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()
const unwrap = <T,>(r: unknown): T => ((r as { data?: T }).data ?? r) as T
const errMsg = (e: unknown) => (e as { data?: { error?: { message?: string } } }).data?.error?.message || t('common.saveFailed')

const devices = ref<Device[]>([])
const actions = ref<Action[]>([])
const employees = ref<{ label: string; value: number }[]>([])
const showAdd = ref(false)
const dev = ref({ asset_tag: '', kind: 'laptop', make_model: '', serial_no: '', os: '', mdm_device_id: '' })
const assignFor = ref<Device | null>(null)
const assignTo = ref<number | null>(null)
const mdm = ref({ provider: 'webhook', url: '', secret: '', auto_lock_on_termination: true, has_secret: false })

const devStatus: Record<string, 'default' | 'info' | 'success' | 'warning' | 'error'> = { in_stock: 'default', assigned: 'success', return_pending: 'warning', locked: 'warning', wiped: 'error', lost: 'error', retired: 'default' }
const actStatus: Record<string, 'default' | 'info' | 'success' | 'warning' | 'error'> = { pending: 'info', running: 'info', requires_approval: 'warning', manual: 'warning', completed: 'success', failed: 'error', cancelled: 'default' }

async function run(fn: () => Promise<unknown>) {
  try {
    await fn()
    message.success(t('common.saved'))
    await load()
  } catch (e) {
    message.error(errMsg(e))
  }
}
function request(d: Device, action: string) {
  const go = () => run(() => devicesAPI.action(d.id, action))
  if (action === 'wipe' || action === 'lock') {
    dialog.warning({ title: t('devices.act_' + action), content: t('devices.confirm_' + action, { tag: d.asset_tag }), positiveText: t('common.confirm'), negativeText: t('common.cancel'), onPositiveClick: go })
  } else go()
}

const devCols = computed<DataTableColumns<Device>>(() => [
  { title: t('devices.assetTag'), key: 'asset_tag', width: 120 },
  { title: t('devices.model'), key: 'make_model', render: (r) => `${r.make_model} ${r.serial_no ? '· ' + r.serial_no : ''}` },
  { title: t('devices.kind'), key: 'kind', width: 100 },
  { title: t('devices.assignedTo'), key: 'employee_name', width: 170 },
  { title: t('common.status'), key: 'status', width: 140, render: (r) => h(NTag, { size: 'small', type: devStatus[r.status] }, () => t('devices.st_' + r.status)) },
  { title: '', key: 'a', width: 220, render: (r) => h(NSpace, { size: 4 }, () => [
    r.status === 'in_stock' ? h(NButton, { size: 'small', onClick: () => { assignFor.value = r; assignTo.value = null } }, () => t('devices.assign')) : null,
    r.assigned_employee_id ? h(NButton, { size: 'small', onClick: () => run(() => devicesAPI.unassign(r.id)) }, () => t('devices.unassign')) : null,
    h(NDropdown, { trigger: 'click', options: ['lock', 'unlock', 'return_request', 'wipe'].map((a) => ({ label: t('devices.act_' + a), key: a })), onSelect: (k: string) => request(r, k) },
      () => h(NButton, { size: 'small' }, () => t('devices.mdm') + ' ▾')),
  ]) },
])
const actCols = computed<DataTableColumns<Action>>(() => [
  { title: t('devices.assetTag'), key: 'asset_tag', width: 110 },
  { title: t('devices.action'), key: 'action', width: 130, render: (r) => t('devices.act_' + r.action) },
  { title: t('devices.trigger'), key: 'trigger', width: 110 },
  { title: t('common.status'), key: 'status', width: 150, render: (r) => h(NTag, { size: 'small', type: actStatus[r.status] }, () => t('devices.ast_' + r.status)) },
  { title: '', key: 'msg', render: (r) => r.error_message || r.result || '' },
  { title: '', key: 'a', width: 200, render: (r) => h(NSpace, { size: 4 }, () => [
    r.status === 'requires_approval' ? h(NButton, { size: 'small', type: 'error', onClick: () => run(() => devicesAPI.approveAction(r.id)) }, () => t('devices.approve')) : null,
    ['manual', 'failed'].includes(r.status) ? h(NButton, { size: 'small', type: 'primary', onClick: () => run(() => devicesAPI.completeAction(r.id, '')) }, () => t('devices.markDone')) : null,
    ['pending', 'requires_approval', 'manual', 'failed'].includes(r.status) ? h(NButton, { size: 'small', quaternary: true, onClick: () => run(() => devicesAPI.cancelAction(r.id)) }, () => t('common.cancel')) : null,
  ]) },
])

async function load() {
  devices.value = unwrap<Device[]>(await devicesAPI.list()) || []
  actions.value = unwrap<Action[]>(await devicesAPI.actions()) || []
}
onMounted(async () => {
  await load()
  const e = unwrap<{ id: number; first_name: string; last_name: string }[]>(await employeeAPI.list({ status: 'active', limit: '500' })) || []
  employees.value = e.map((x) => ({ label: `${x.first_name} ${x.last_name}`, value: x.id }))
  const m = unwrap<{ connection: { provider: string; url: string; auto_lock_on_termination: boolean }; has_secret: boolean } | null>(await devicesAPI.mdm())
  if (m?.connection) mdm.value = { ...mdm.value, ...m.connection, has_secret: m.has_secret }
})
</script>

<template>
  <NSpace vertical :size="16">
    <div>
      <h2 style="margin: 0">{{ t('devices.title') }}</h2>
      <div style="opacity: 0.7">{{ t('devices.subtitle') }}</div>
    </div>
    <NTabs type="line">
      <NTabPane name="inventory" :tab="t('devices.tabInventory')">
        <NSpace vertical>
          <NButton type="primary" @click="showAdd = true">{{ t('devices.add') }}</NButton>
          <NDataTable :columns="devCols" :data="devices" :row-key="(r: Device) => r.id" />
        </NSpace>
      </NTabPane>
      <NTabPane name="actions" :tab="t('devices.tabActions')">
        <NDataTable :columns="actCols" :data="actions" :row-key="(r: Action) => r.id" />
      </NTabPane>
      <NTabPane name="mdm" :tab="t('devices.tabMdm')">
        <NCard size="small" style="max-width: 640px">
          <NAlert type="info" :show-icon="false" style="margin-bottom: 12px">{{ t('devices.mdmHint') }}</NAlert>
          <NForm label-placement="top">
            <NFormItem :label="t('devices.provider')">
              <NSelect v-model:value="mdm.provider" :options="['webhook', 'jamf', 'kandji', 'intune', 'google'].map((v) => ({ label: v, value: v }))" />
            </NFormItem>
            <NFormItem :label="t('devices.gatewayUrl')"><NInput v-model:value="mdm.url" placeholder="https://" /></NFormItem>
            <NFormItem :label="t('devices.secret')"><NInput v-model:value="mdm.secret" type="password" :placeholder="mdm.has_secret ? '••••••••' : ''" /></NFormItem>
            <NSpace align="center" style="margin-bottom: 12px"><NSwitch v-model:value="mdm.auto_lock_on_termination" /> {{ t('devices.autoLock') }}</NSpace>
            <NButton type="primary" :disabled="!mdm.url" @click="run(() => devicesAPI.saveMdm({ provider: mdm.provider, url: mdm.url, secret: mdm.secret, auto_lock_on_termination: mdm.auto_lock_on_termination }))">{{ t('common.save') }}</NButton>
          </NForm>
        </NCard>
      </NTabPane>
    </NTabs>

    <NModal v-model:show="showAdd" preset="card" style="max-width: 520px" :title="t('devices.add')">
      <NForm label-placement="top">
        <NFormItem :label="t('devices.assetTag')"><NInput v-model:value="dev.asset_tag" /></NFormItem>
        <NFormItem :label="t('devices.kind')"><NSelect v-model:value="dev.kind" :options="['laptop', 'desktop', 'phone', 'tablet', 'monitor', 'other'].map((v) => ({ label: v, value: v }))" /></NFormItem>
        <NFormItem :label="t('devices.model')"><NInput v-model:value="dev.make_model" /></NFormItem>
        <NFormItem :label="t('devices.serial')"><NInput v-model:value="dev.serial_no" /></NFormItem>
        <NFormItem label="OS"><NInput v-model:value="dev.os" /></NFormItem>
        <NFormItem :label="t('devices.mdmId')"><NInput v-model:value="dev.mdm_device_id" /></NFormItem>
      </NForm>
      <NSpace justify="end">
        <NButton @click="showAdd = false">{{ t('common.cancel') }}</NButton>
        <NButton type="primary" :disabled="!dev.asset_tag" @click="run(() => devicesAPI.create({ ...dev, mdm_device_id: dev.mdm_device_id || undefined })).then(() => (showAdd = false))">{{ t('common.save') }}</NButton>
      </NSpace>
    </NModal>
    <NModal :show="!!assignFor" preset="card" style="max-width: 420px" :title="t('devices.assign') + ' ' + (assignFor?.asset_tag || '')" @update:show="(v: boolean) => { if (!v) assignFor = null }">
      <NSelect v-model:value="assignTo" filterable :options="employees" />
      <NSpace justify="end" style="margin-top: 12px">
        <NButton type="primary" :disabled="!assignTo" @click="run(() => devicesAPI.assign(assignFor!.id, assignTo!)).then(() => (assignFor = null))">{{ t('common.save') }}</NButton>
      </NSpace>
    </NModal>
  </NSpace>
</template>
