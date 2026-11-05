<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { NModal, NButton, NSpin, NTable, NAlert, NCheckbox, NTag, NSpace, useMessage } from 'naive-ui'
import { payoutsAPI } from '../../api/client'

interface Line { employee_id: number; employee_no: string; name: string; amount: number; bank_name: string; bank_code: string; account_no: string; issue?: string }
interface Plan { cycle_id: number; cycle: string; cycle_status: string; lines: Line[]; total: number; issues: number; blocker?: string; sent?: { status: string; created_at: string } }

const props = defineProps<{ show: boolean; cycleId: number | null }>()
const emit = defineEmits<{ (e: 'update:show', v: boolean): void }>()
const { t } = useI18n()
const message = useMessage()
const plan = ref<Plan | null>(null)
const loading = ref(false)
const sending = ref(false)
const confirmed = ref(false)
const error = ref('')

const money = (v: number) => v.toLocaleString(undefined, { style: 'currency', currency: 'PHP' })
const masked = (n: string) => (n.length > 4 ? '•••• ' + n.slice(-4) : n)
const canSend = computed(() => plan.value && !plan.value.blocker && confirmed.value)

async function load() {
  if (!props.cycleId) return
  loading.value = true
  error.value = ''
  confirmed.value = false
  try {
    plan.value = ((await payoutsAPI.preview(props.cycleId)) as { data: Plan }).data
  } catch (e) {
    error.value = (e as { data?: { error?: { message?: string } } }).data?.error?.message || t('common.loadFailed')
  } finally {
    loading.value = false
  }
}
watch(() => [props.show, props.cycleId], () => { if (props.show) load() }, { immediate: true })

async function send() {
  if (!plan.value) return
  sending.value = true
  try {
    await payoutsAPI.send(plan.value.cycle_id, plan.value.total)
    message.success(t('payout.sentMsg'))
    await load()
  } catch (e) {
    message.error((e as { data?: { error?: { message?: string } } }).data?.error?.message || t('common.failed'))
  } finally {
    sending.value = false
  }
}
</script>

<template>
  <NModal :show="show" preset="card" :title="t('payout.title') + (plan ? ' — ' + plan.cycle : '')" style="max-width: 820px" @update:show="(v: boolean) => emit('update:show', v)">
    <NSpin :show="loading">
      <NAlert v-if="error" type="warning">{{ error }}</NAlert>
      <template v-if="plan">
        <p class="muted">{{ t('payout.desc') }}</p>
        <NAlert v-if="plan.sent" type="success" style="margin-bottom: 12px">{{ t('payout.alreadySent', { status: plan.sent.status }) }}</NAlert>
        <NAlert v-else-if="plan.blocker" type="warning" style="margin-bottom: 12px">{{ t('payout.blocked') }}: {{ plan.blocker }}</NAlert>
        <NTable size="small" :single-line="false">
          <thead><tr><th>{{ t('payout.employee') }}</th><th>{{ t('payout.bank') }}</th><th>{{ t('payout.account') }}</th><th class="num">{{ t('payout.amount') }}</th></tr></thead>
          <tbody>
            <tr v-for="l in plan.lines" :key="l.employee_id">
              <td>{{ l.name }} <span class="muted small">{{ l.employee_no }}</span></td>
              <td>
                <template v-if="l.bank_code">{{ l.bank_name }} <NTag size="tiny" :bordered="false">{{ l.bank_code }}</NTag></template>
                <NTag v-else size="small" type="error" round :bordered="false">{{ l.issue }}</NTag>
              </td>
              <td>{{ l.account_no ? masked(l.account_no) : '—' }}</td>
              <td class="num">{{ money(l.amount) }}</td>
            </tr>
            <tr class="total"><td colspan="3">{{ t('payout.total', { n: plan.lines.length }) }}</td><td class="num">{{ money(plan.total) }}</td></tr>
          </tbody>
        </NTable>
        <div v-if="!plan.blocker" class="confirm">
          <NCheckbox v-model:checked="confirmed">{{ t('payout.confirm', { total: money(plan.total), n: plan.lines.length }) }}</NCheckbox>
        </div>
        <NSpace justify="end" style="margin-top: 12px">
          <NButton @click="emit('update:show', false)">{{ t('common.cancel') }}</NButton>
          <NButton type="primary" :disabled="!canSend" :loading="sending" @click="send">{{ t('payout.send') }}</NButton>
        </NSpace>
      </template>
    </NSpin>
  </NModal>
</template>

<style scoped>
.muted { opacity: .7; font-size: 13px; }
.small { font-size: 12px; }
.num { text-align: right; font-variant-numeric: tabular-nums; }
.total td { font-weight: 600; }
.confirm { margin-top: 14px; padding: 12px; border-radius: 8px; background: rgba(240, 160, 32, .08); }
</style>
