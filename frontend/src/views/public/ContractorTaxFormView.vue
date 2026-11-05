<script setup lang="ts">
// Public page a contractor opens from HR's link: tax residency, TIN,
// address, VAT status and payout details, signed with their typed name.
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { NCard, NSpin, NResult, NForm, NFormItem, NInput, NSelect, NButton, NCheckbox, NRadioGroup, NRadio, NSpace, NAlert, useMessage } from 'naive-ui'
import { publicAPI } from '../../api/client'

const { t, locale } = useI18n()
const route = useRoute()
const message = useMessage()
const token = String(route.params.token || '')
const loading = ref(true)
const gone = ref(false)
const done = ref(false)
const saving = ref(false)
interface Country { code: string; label: string; declaration: string }
const info = ref<{ company: string; company_country: string; contractor: string; kind: string; expires_at: string; countries: Country[] } | null>(null)
const form = ref({ tax_country: 'PH', entity_type: 'individual', tin: '', address: '', is_vat_registered: false, bank_name: '', bank_account_no: '', bank_account_name: '', signer_name: '', agree: false })
// Countries and the exact declaration text come from the server, so what
// the contractor reads is what is stored as signed.
const countries = computed(() => (info.value?.countries || []).map((c) => ({ label: c.code === 'XX' ? t('taxForm.otherCountry') : c.label, value: c.code })))
const local = computed(() => !!info.value && info.value.company_country.startsWith(form.value.tax_country))
const declaration = computed(() => info.value?.countries.find((c) => c.code === form.value.tax_country)?.declaration || '')
const declarationZh = computed(() => {
  if (locale.value !== 'zh') return ''
  const c = info.value?.countries.find((x) => x.code === form.value.tax_country)
  return c ? t('taxForm.declaration', { country: c.code === 'XX' ? t('taxForm.otherCountry') : c.label, company: info.value?.company || '' }) : ''
})

onMounted(async () => {
  try {
    const d = ((await publicAPI.taxForm(token)) as { data: Record<string, unknown> }).data
    info.value = d as typeof info.value
    form.value.tin = String(d.tin || '')
    form.value.address = String(d.address || '')
    form.value.is_vat_registered = !!d.is_vat_registered
    form.value.entity_type = d.kind === 'company' ? 'company' : 'individual'
    form.value.signer_name = String(d.contractor || '')
  } catch {
    gone.value = true
  } finally {
    loading.value = false
  }
})

async function submit() {
  if (!form.value.agree || form.value.signer_name.trim().length < 2) {
    message.warning(t('taxForm.signRequired'))
    return
  }
  saving.value = true
  try {
    await publicAPI.submitTaxForm(token, { ...form.value })
    done.value = true
  } catch (e) {
    const err = e as { status?: number; data?: { error?: { message?: string } } }
    if (err.status === 410) gone.value = true
    else message.error(err.data?.error?.message || t('common.failed'))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="tf-page">
    <NSpin :show="loading">
      <NResult v-if="gone" status="warning" :title="t('taxForm.goneTitle')" :description="t('taxForm.gone')" />
      <NResult v-else-if="done" status="success" :title="t('taxForm.doneTitle')" :description="t('taxForm.done', { company: info?.company })" />
      <NCard v-else-if="info" :title="t('taxForm.title')">
        <p class="lead">{{ t('taxForm.intro', { company: info.company, name: info.contractor }) }}</p>
        <NForm label-placement="top" @submit.prevent="submit">
          <NFormItem :label="t('taxForm.country')" required>
            <NSelect v-model:value="form.tax_country" :options="countries" filterable />
          </NFormItem>
          <NFormItem :label="t('taxForm.entity')" required>
            <NRadioGroup v-model:value="form.entity_type">
              <NSpace><NRadio value="individual">{{ t('taxForm.individual') }}</NRadio><NRadio value="company">{{ t('taxForm.company') }}</NRadio></NSpace>
            </NRadioGroup>
          </NFormItem>
          <NFormItem :label="local ? t('taxForm.tinLocal') : t('taxForm.tinForeign')" :required="local">
            <NInput v-model:value="form.tin" maxlength="40" />
          </NFormItem>
          <NFormItem :label="t('taxForm.address')" required>
            <NInput v-model:value="form.address" type="textarea" :rows="2" maxlength="1000" />
          </NFormItem>
          <NFormItem v-if="local">
            <NCheckbox v-model:checked="form.is_vat_registered">{{ t('taxForm.vat') }}</NCheckbox>
          </NFormItem>
          <NAlert type="info" :show-icon="false" style="margin-bottom: 12px">{{ t('taxForm.bankHelp') }}</NAlert>
          <NFormItem :label="t('taxForm.bankName')"><NInput v-model:value="form.bank_name" /></NFormItem>
          <NFormItem :label="t('taxForm.bankAccountNo')"><NInput v-model:value="form.bank_account_no" /></NFormItem>
          <NFormItem :label="t('taxForm.bankAccountName')"><NInput v-model:value="form.bank_account_name" /></NFormItem>
          <div class="decl">
            {{ declaration }}
            <div v-if="declarationZh" class="decl-tr">{{ t('taxForm.translation') }}{{ declarationZh }}</div>
          </div>
          <NFormItem :label="t('taxForm.signName')" required>
            <NInput v-model:value="form.signer_name" maxlength="160" />
          </NFormItem>
          <NFormItem>
            <NCheckbox v-model:checked="form.agree">{{ t('taxForm.agree') }}</NCheckbox>
          </NFormItem>
          <NButton type="primary" block :loading="saving" attr-type="submit">{{ t('taxForm.sign') }}</NButton>
        </NForm>
      </NCard>
    </NSpin>
  </div>
</template>

<style scoped>
.tf-page { max-width: 640px; margin: 0 auto; padding: 24px 16px 48px; }
.lead { margin: 0 0 16px; opacity: .8; }
.decl-tr { margin-top: 8px; opacity: .7; }
.decl { padding: 12px; border-radius: 8px; background: rgba(128, 128, 128, .08); font-size: 13px; line-height: 1.5; margin-bottom: 12px; }
</style>
