<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { NModal, NButton, NSpin, NAlert, NCheckbox, NSpace, NDescriptions, NDescriptionsItem, useMessage } from 'naive-ui'
import { contractorsAPI } from '../../api/client'

interface Quote { id: string; sourceCurrency: string; targetCurrency: string; sourceAmount: number; targetAmount: number; rate: number; fee: number }
interface Info { payment: { id: number; contractor_name: string; net_amount: number; invoice_no: string; status: string }; quote?: Quote; transfer?: { transfer_id: number; status: string; target_amount: string; target_currency: string }; blocker?: string }

const props = defineProps<{ show: boolean; paymentId: number | null }>()
const emit = defineEmits<{ (e: 'update:show', v: boolean): void; (e: 'paid'): void }>()
const { t } = useI18n()
const message = useMessage()
const info = ref<Info | null>(null)
const loading = ref(false)
const sending = ref(false)
const ok = ref(false)

async function load() {
  if (!props.paymentId) return
  loading.value = true
  ok.value = false
  try { info.value = ((await contractorsAPI.wiseQuote(props.paymentId)) as { data: Info }).data } finally { loading.value = false }
}
watch(() => [props.show, props.paymentId], () => { if (props.show) load() }, { immediate: true })

async function pay() {
  const q = info.value?.quote
  if (!q || !props.paymentId) return
  sending.value = true
  try {
    await contractorsAPI.wisePay(props.paymentId, { quote_id: q.id, confirm: true, target_amount: q.targetAmount, rate: q.rate })
    message.success(t('wise.sent'))
    emit('paid')
    await load()
  } catch (e) {
    message.error((e as { data?: { error?: { message?: string } } }).data?.error?.message || t('common.failed'))
  } finally {
    sending.value = false
  }
}
const fmt = (v: number, c: string) => v.toLocaleString(undefined, { style: 'currency', currency: c })
</script>

<template>
  <NModal :show="show" preset="card" :title="t('wise.title')" style="max-width: 560px" @update:show="(v: boolean) => emit('update:show', v)">
    <NSpin :show="loading">
      <template v-if="info">
        <p class="muted">{{ t('wise.desc') }}</p>
        <NAlert v-if="info.transfer" type="success">{{ t('wise.done', { id: info.transfer.transfer_id, status: info.transfer.status }) }}</NAlert>
        <NAlert v-else-if="info.blocker" type="warning">{{ info.blocker }}</NAlert>
        <NDescriptions v-if="info.quote" :column="1" label-placement="left" bordered size="small">
          <NDescriptionsItem :label="t('wise.payee')">{{ info.payment.contractor_name }} · {{ info.payment.invoice_no }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('wise.youSend')">{{ fmt(info.quote.sourceAmount, info.quote.sourceCurrency) }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('wise.fee')">{{ fmt(info.quote.fee, info.quote.sourceCurrency) }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('wise.rate')">1 {{ info.quote.sourceCurrency }} = {{ info.quote.rate }} {{ info.quote.targetCurrency }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('wise.theyGet')"><strong>{{ fmt(info.quote.targetAmount, info.quote.targetCurrency) }}</strong></NDescriptionsItem>
        </NDescriptions>
        <div v-if="info.quote" class="confirm">
          <NCheckbox v-model:checked="ok">{{ t('wise.confirm', { amount: fmt(info.quote.targetAmount, info.quote.targetCurrency), name: info.payment.contractor_name }) }}</NCheckbox>
        </div>
        <NSpace justify="end" style="margin-top: 12px">
          <NButton @click="emit('update:show', false)">{{ t('common.cancel') }}</NButton>
          <NButton v-if="info.quote" type="primary" :disabled="!ok" :loading="sending" @click="pay">{{ t('wise.pay') }}</NButton>
        </NSpace>
      </template>
    </NSpin>
  </NModal>
</template>

<style scoped>
.muted { opacity: .7; font-size: 13px; }
.confirm { margin-top: 14px; padding: 12px; border-radius: 8px; background: rgba(240, 160, 32, .08); }
</style>
