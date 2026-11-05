<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NCard, NSpace, NTag, NButton, NProgress, NDescriptions, NDescriptionsItem, NModal,
  NForm, NFormItem, NSelect, NInput, NDatePicker, NCheckbox, NList, NListItem, NAlert,
  useMessage, useDialog,
} from 'naive-ui'
import { format } from 'date-fns'
import { lifecycleAPI, documentsAPI } from '../../api/client'

const props = defineProps<{ employeeId: number; employeeName: string; isAdmin: boolean }>()
const emit = defineEmits<{ (e: 'changed'): void }>()

interface Progress { total: number; completed: number; skipped: number; remaining: number }
interface Overview {
  stage: string
  status: string
  onboarding: Progress | null
  offboarding: Progress | null
  separation: { status: string; separation_type: string; last_working_day: string; rehire_eligible: boolean } | null
  clearance: { id: number; status: string } | null
  final_pay: { status: string; total_final_pay: string } | null
  provisioning: { id: string; provider: string; action_type: string; status: string }[]
}
interface CustomFieldValue {
  field_id: number
  field_key: string
  label: string
  field_type: string
  options: string[]
  is_required: boolean
  is_sensitive: boolean
  value: string | null
}
interface Step { key: string; status: string; detail?: string }

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()

const ov = ref<Overview | null>(null)
const fields = ref<CustomFieldValue[]>([])
const editingFields = ref(false)
const fieldDraft = ref<Record<string, string>>({})
const savingFields = ref(false)

const stageColor: Record<string, 'default' | 'info' | 'success' | 'warning' | 'error'> = {
  pre_start: 'info', onboarding: 'info', active: 'success', offboarding: 'warning', separated: 'error',
}

const separationTypes = ['resignation', 'termination', 'end_of_contract', 'retirement', 'layoff', 'death', 'other']

function pct(p: Progress | null): number {
  if (!p || p.total === 0) return 0
  return Math.round(((p.completed + p.skipped) / p.total) * 100)
}

function fmt(d?: string | null) {
  if (!d) return '-'
  try { return format(new Date(d), 'yyyy-MM-dd') } catch { return d }
}

async function load() {
  try {
    const [o, f] = await Promise.all([
      lifecycleAPI.overview(props.employeeId),
      lifecycleAPI.getEmployeeCustomFields(props.employeeId),
    ])
    ov.value = (o as { data: Overview }).data
    fields.value = (f as { data: CustomFieldValue[] }).data || []
  } catch {
    /* card is supplementary; stay silent */
  }
}

// --- self-onboarding invite ---------------------------------------------
const invite = ref<{ status: string; expires_at: string } | null>(null)
const inviteLink = ref('')
async function loadInvite() {
  try {
    invite.value = ((await documentsAPI.getInvite(props.employeeId)) as { data: typeof invite.value }).data
  } catch { invite.value = null }
}
async function sendInvite() {
  try {
    const l = ((await documentsAPI.createInvite(props.employeeId, { send_email: true })) as { data: { url: string; emailed: boolean } }).data
    inviteLink.value = l.url
    message.success(l.emailed ? t('docs.emailed') : t('docs.linkReady'))
    await loadInvite()
  } catch {
    message.error(t('common.saveFailed'))
  }
}

// --- custom fields -------------------------------------------------------
function startEditFields() {
  fieldDraft.value = Object.fromEntries(fields.value.map((f) => [f.field_key, f.value ?? '']))
  editingFields.value = true
}

async function saveFields() {
  savingFields.value = true
  try {
    const res = await lifecycleAPI.updateEmployeeCustomFields(props.employeeId, fieldDraft.value)
    fields.value = (res as { data: CustomFieldValue[] }).data || []
    editingFields.value = false
    message.success(t('common.saved'))
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } }
    message.error(err.data?.error?.message || t('common.saveFailed'))
  } finally {
    savingFields.value = false
  }
}

// --- terminate -----------------------------------------------------------
const showTerminate = ref(false)
const terminating = ref(false)
const termSteps = ref<Step[] | null>(null)
const term = ref({
  separation_type: 'resignation',
  reason: '',
  notice_date: Date.now() as number | null,
  last_working_day: null as number | null,
  rehire_eligible: true,
  revoke_access: true,
})

const canTerminate = computed(() => props.isAdmin && ov.value && !['offboarding', 'separated'].includes(ov.value.stage))
const canCancel = computed(() => props.isAdmin && ov.value?.separation?.status === 'scheduled')

function openTerminate() {
  termSteps.value = null
  showTerminate.value = true
}

async function submitTerminate() {
  if (!term.value.last_working_day) {
    message.warning(t('lifecycle.lastWorkingDay'))
    return
  }
  terminating.value = true
  try {
    const d = (n: number | null) => (n ? format(new Date(n), 'yyyy-MM-dd') : undefined)
    const res = await lifecycleAPI.terminate(props.employeeId, {
      separation_type: term.value.separation_type,
      reason: term.value.reason || undefined,
      notice_date: d(term.value.notice_date),
      last_working_day: d(term.value.last_working_day),
      rehire_eligible: term.value.rehire_eligible,
      revoke_access: term.value.revoke_access,
    })
    const data = (res as { data: { effective: boolean; steps: Step[] } }).data
    termSteps.value = data.steps
    message.success(data.effective ? t('lifecycle.terminatedNow') : t('lifecycle.terminatedScheduled'))
    await load()
    emit('changed')
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } }
    message.error(err.data?.error?.message || t('common.saveFailed'))
  } finally {
    terminating.value = false
  }
}

function cancelTermination() {
  dialog.warning({
    title: t('lifecycle.cancelTermination'),
    content: t('lifecycle.cancelTerminationConfirm'),
    positiveText: t('common.confirm'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await lifecycleAPI.cancelTermination(props.employeeId)
        message.success(t('common.saved'))
        await load()
        emit('changed')
      } catch {
        message.error(t('common.saveFailed'))
      }
    },
  })
}

watch(() => props.employeeId, () => { load(); loadInvite() })
onMounted(() => { load(); loadInvite() })
</script>

<template>
  <NCard v-if="ov" :title="t('lifecycle.lifecycle')">
    <template #header-extra>
      <NSpace>
        <NTag :type="stageColor[ov.stage] || 'default'">{{ t('lifecycle.stage_' + ov.stage) }}</NTag>
        <NButton v-if="canCancel" size="small" @click="cancelTermination">{{ t('lifecycle.cancelTermination') }}</NButton>
        <NButton v-if="canTerminate" size="small" type="error" ghost @click="openTerminate">{{ t('lifecycle.terminate') }}</NButton>
      </NSpace>
    </template>

    <NDescriptions label-placement="left" :column="2" bordered size="small">
      <NDescriptionsItem v-if="ov.onboarding" :label="t('lifecycle.onboardingProgress')">
        <NProgress type="line" :percentage="pct(ov.onboarding)" :show-indicator="true" style="max-width: 220px" />
      </NDescriptionsItem>
      <NDescriptionsItem v-if="ov.offboarding" :label="t('lifecycle.offboardingProgress')">
        <NProgress type="line" status="warning" :percentage="pct(ov.offboarding)" style="max-width: 220px" />
      </NDescriptionsItem>
      <NDescriptionsItem v-if="ov.separation" :label="t('lifecycle.separation')">
        {{ t('lifecycle.sep_' + ov.separation.separation_type) }} ·
        {{ t('lifecycle.lastWorkingDay') }} {{ fmt(ov.separation.last_working_day) }}
        <NTag size="tiny" style="margin-left: 6px">{{ ov.separation.status }}</NTag>
      </NDescriptionsItem>
      <NDescriptionsItem v-if="ov.clearance" :label="t('lifecycle.clearance')">{{ ov.clearance.status }}</NDescriptionsItem>
      <NDescriptionsItem v-if="ov.final_pay" :label="t('lifecycle.finalPay')">
        {{ ov.final_pay.status }} · {{ ov.final_pay.total_final_pay }}
      </NDescriptionsItem>
      <NDescriptionsItem :label="t('lifecycle.selfOnboarding')">
        <NSpace align="center" :size="6">
          <NTag v-if="invite" size="small" :type="invite.status === 'submitted' ? 'success' : 'info'">{{ t('lifecycle.invite_' + invite.status) }}</NTag>
          <span v-else style="opacity: 0.6">—</span>
          <NButton v-if="isAdmin && ov.stage !== 'separated'" size="tiny" @click="sendInvite">{{ invite ? t('lifecycle.resendInvite') : t('lifecycle.sendInviteShort') }}</NButton>
        </NSpace>
        <div v-if="inviteLink" style="font-size: 12px; word-break: break-all; margin-top: 4px"><code>{{ inviteLink }}</code></div>
      </NDescriptionsItem>
      <NDescriptionsItem :label="t('lifecycle.provisioning')">
        <span v-if="ov.provisioning.length === 0" style="opacity: 0.6">{{ t('lifecycle.noProvisioning') }}</span>
        <NSpace v-else :size="4">
          <NTag
            v-for="j in ov.provisioning"
            :key="j.id"
            size="small"
            :type="j.status === 'completed' ? 'success' : ['failed', 'dead'].includes(j.status) ? 'error' : 'default'"
          >
            {{ j.provider }} · {{ j.action_type }} · {{ j.status }}
          </NTag>
        </NSpace>
      </NDescriptionsItem>
    </NDescriptions>

    <template v-if="fields.length">
      <NSpace justify="space-between" align="center" style="margin: 16px 0 8px">
        <h4 style="margin: 0">{{ t('lifecycle.customFields') }}</h4>
        <NSpace v-if="isAdmin">
          <NButton v-if="!editingFields" size="small" @click="startEditFields">{{ t('common.edit') }}</NButton>
          <template v-else>
            <NButton size="small" @click="editingFields = false">{{ t('common.cancel') }}</NButton>
            <NButton size="small" type="primary" :loading="savingFields" @click="saveFields">{{ t('common.save') }}</NButton>
          </template>
        </NSpace>
      </NSpace>
      <NDescriptions v-if="!editingFields" label-placement="left" :column="2" bordered size="small">
        <NDescriptionsItem v-for="f in fields" :key="f.field_id" :label="f.label">
          {{ f.value ?? (f.is_sensitive && !isAdmin ? '••••' : '-') }}
        </NDescriptionsItem>
      </NDescriptions>
      <NForm v-else label-placement="left" label-width="160">
        <NFormItem v-for="f in fields" :key="f.field_id" :label="f.label" :required="f.is_required">
          <NSelect
            v-if="f.field_type === 'select' || f.field_type === 'boolean'"
            v-model:value="fieldDraft[f.field_key]"
            :options="(f.field_type === 'boolean' ? ['true', 'false'] : f.options).map((o) => ({ label: o, value: o }))"
            clearable
          />
          <NInput v-else v-model:value="fieldDraft[f.field_key]" :placeholder="f.field_type === 'date' ? 'YYYY-MM-DD' : ''" />
        </NFormItem>
      </NForm>
    </template>

    <NModal
      v-model:show="showTerminate"
      preset="card"
      style="max-width: 560px"
      :title="t('lifecycle.terminateTitle', { name: employeeName })"
    >
      <template v-if="termSteps">
        <NList>
          <NListItem v-for="s in termSteps" :key="s.key">
            <NSpace justify="space-between">
              <span>{{ t('lifecycle.step_' + s.key) }}<span v-if="s.detail" style="opacity: 0.6"> — {{ s.detail }}</span></span>
              <NTag size="small" :type="s.status === 'done' ? 'success' : 'info'">{{ t('lifecycle.status_' + s.status) }}</NTag>
            </NSpace>
          </NListItem>
        </NList>
        <NSpace justify="end" style="margin-top: 12px">
          <NButton type="primary" @click="showTerminate = false">{{ t('common.confirm') }}</NButton>
        </NSpace>
      </template>
      <template v-else>
        <NAlert type="info" :show-icon="false" style="margin-bottom: 12px">{{ t('lifecycle.terminateHint') }}</NAlert>
        <NForm label-placement="top">
          <NFormItem :label="t('lifecycle.separationType')" required>
            <NSelect
              v-model:value="term.separation_type"
              :options="separationTypes.map((s) => ({ label: t('lifecycle.sep_' + s), value: s }))"
            />
          </NFormItem>
          <NSpace :wrap="false" :size="12">
            <NFormItem :label="t('lifecycle.noticeDate')" style="flex: 1">
              <NDatePicker v-model:value="term.notice_date" type="date" style="width: 100%" />
            </NFormItem>
            <NFormItem :label="t('lifecycle.lastWorkingDay')" required style="flex: 1">
              <NDatePicker v-model:value="term.last_working_day" type="date" style="width: 100%" />
            </NFormItem>
          </NSpace>
          <NFormItem :label="t('lifecycle.reason')">
            <NInput v-model:value="term.reason" type="textarea" :rows="2" />
          </NFormItem>
          <NSpace vertical>
            <NCheckbox v-model:checked="term.rehire_eligible">{{ t('lifecycle.rehireEligible') }}</NCheckbox>
            <NCheckbox v-model:checked="term.revoke_access">{{ t('lifecycle.revokeAccess') }}</NCheckbox>
          </NSpace>
        </NForm>
        <NSpace justify="end" style="margin-top: 16px">
          <NButton @click="showTerminate = false">{{ t('common.cancel') }}</NButton>
          <NButton type="error" :loading="terminating" @click="submitTerminate">{{ t('lifecycle.terminate') }}</NButton>
        </NSpace>
      </template>
    </NModal>
  </NCard>
</template>
