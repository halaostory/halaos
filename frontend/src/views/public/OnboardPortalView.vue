<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { NCard, NSpin, NResult, NSteps, NStep, NForm, NFormItem, NInput, NSelect, NButton, NSpace, NAlert, NTag, NList, NListItem } from 'naive-ui'
import { publicAPI } from '../../api/client'
import SignDocumentPanel from '../../components/lifecycle/SignDocumentPanel.vue'

interface Portal { company_name: string; country: string; first_name: string; last_name: string; position: string; start_date: string; status: string; profile: Record<string, string>; documents: { id: number; title: string; kind: string; status: string }[] }
interface Doc { title: string; body: string; company_name: string; signer_name: string; status: string; signed_at?: string }

const route = useRoute()
const { t } = useI18n()
const token = String(route.params.token)
const portal = ref<Portal | null>(null)
const gone = ref('')
const loading = ref(true)
const step = ref(1)
const saving = ref(false)
const fieldErrors = ref<Record<string, string>>({})
const formError = ref('')
const form = ref<Record<string, string>>({})
const openDoc = ref<{ id: number; doc: Doc } | null>(null)

const isPH = computed(() => portal.value?.country === 'PHL')
const govFields = computed(() => isPH.value
  ? [['tin', 'TIN'], ['sss_no', 'SSS'], ['philhealth_no', 'PhilHealth'], ['pagibig_no', 'Pag-IBIG MID']]
  : [['tin', t('portal.taxId')]])
const pendingDocs = computed(() => (portal.value?.documents || []).filter((d) => d.status === 'pending' || d.status === 'viewed'))

async function load() {
  try {
    portal.value = ((await publicAPI.portal(token)) as { data: Portal }).data
    form.value = { ...portal.value.profile }
  } catch (e: unknown) {
    gone.value = (e as { data?: { error?: { message?: string } } }).data?.error?.message || t('portal.invalidLink')
  } finally {
    loading.value = false
  }
}

async function save(complete: boolean) {
  saving.value = true
  fieldErrors.value = {}
  formError.value = ''
  try {
    await publicAPI.saveProfile(token, { ...form.value, complete })
    if (complete) await load()
    else step.value++
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string; fields?: Record<string, string> } } }
    fieldErrors.value = err.data?.error?.fields || {}
    formError.value = err.data?.error?.message || t('common.saveFailed')
    if (Object.keys(fieldErrors.value).some((k) => govFields.value.some((g) => g[0] === k))) step.value = 2
  } finally {
    saving.value = false
  }
}

async function showDoc(id: number) {
  openDoc.value = { id, doc: ((await publicAPI.portalDoc(token, id)) as { data: Doc }).data }
}
async function signDoc(payload: Record<string, unknown>) {
  if (!openDoc.value) return
  await publicAPI.portalSign(token, openDoc.value.id, payload)
  await showDoc(openDoc.value.id)
  await load()
}
onMounted(load)
</script>

<template>
  <div class="public-wrap">
    <NSpin :show="loading">
      <NResult v-if="gone" status="warning" :title="gone" />
      <template v-else-if="portal">
        <div class="brand">{{ portal.company_name }}</div>
        <h2 style="margin: 4px 0 2px">{{ t('portal.welcome', { name: portal.first_name }) }}</h2>
        <div style="opacity: 0.75; margin-bottom: 20px">
          {{ portal.position ? portal.position + ' · ' : '' }}{{ t('portal.starts', { date: portal.start_date }) }}
        </div>

        <NCard v-if="pendingDocs.length || openDoc" :title="t('portal.documents')" style="margin-bottom: 16px">
          <NList v-if="!openDoc">
            <NListItem v-for="d in portal.documents" :key="d.id">
              <NSpace justify="space-between" align="center">
                <span>{{ d.title }}</span>
                <NButton size="small" type="primary" @click="showDoc(d.id)">{{ t('portal.reviewSign') }}</NButton>
              </NSpace>
            </NListItem>
          </NList>
          <template v-else>
            <NButton text style="margin-bottom: 8px" @click="openDoc = null">← {{ t('common.back') }}</NButton>
            <SignDocumentPanel :doc="openDoc.doc" :submit="signDoc" />
          </template>
        </NCard>

        <NCard v-if="portal.status === 'submitted'">
          <NResult status="success" :title="t('portal.doneTitle')" :description="t('portal.doneDesc')" />
        </NCard>
        <NCard v-else>
          <NSteps :current="step" size="small" style="margin-bottom: 20px">
            <NStep :title="t('portal.stepPersonal')" />
            <NStep :title="t('portal.stepIds')" />
            <NStep :title="t('portal.stepBank')" />
            <NStep :title="t('portal.stepEmergency')" />
          </NSteps>
          <NAlert v-if="formError" type="error" :show-icon="false" style="margin-bottom: 12px">{{ formError }}</NAlert>
          <NForm label-placement="top">
            <template v-if="step === 1">
              <NFormItem :label="t('common.phone')"><NInput v-model:value="form.phone" /></NFormItem>
              <NFormItem :label="t('employee.birthDate')"><NInput v-model:value="form.birth_date" placeholder="YYYY-MM-DD" /></NFormItem>
              <NFormItem :label="t('employee.gender')">
                <NSelect v-model:value="form.gender" clearable :options="[{ label: t('employee.male'), value: 'male' }, { label: t('employee.female'), value: 'female' }]" />
              </NFormItem>
              <NFormItem :label="t('employee.civilStatus')"><NInput v-model:value="form.civil_status" /></NFormItem>
              <NFormItem :label="t('portal.address')"><NInput v-model:value="form.address_line1" /></NFormItem>
              <NSpace :wrap="false">
                <NFormItem :label="t('portal.city')"><NInput v-model:value="form.city" /></NFormItem>
                <NFormItem :label="t('portal.province')"><NInput v-model:value="form.province" /></NFormItem>
                <NFormItem :label="t('portal.zip')"><NInput v-model:value="form.zip_code" /></NFormItem>
              </NSpace>
            </template>
            <template v-else-if="step === 2">
              <NAlert type="info" :show-icon="false" style="margin-bottom: 12px">{{ t('portal.idsHint') }}</NAlert>
              <NFormItem v-for="[k, label] in govFields" :key="k" :label="label" :validation-status="fieldErrors[k] ? 'error' : undefined" :feedback="fieldErrors[k]">
                <NInput v-model:value="form[k]" />
              </NFormItem>
            </template>
            <template v-else-if="step === 3">
              <NFormItem :label="t('portal.bankName')"><NInput v-model:value="form.bank_name" /></NFormItem>
              <NFormItem :label="t('portal.accountNo')"><NInput v-model:value="form.bank_account_no" /></NFormItem>
              <NFormItem :label="t('portal.accountName')"><NInput v-model:value="form.bank_account_name" /></NFormItem>
            </template>
            <template v-else>
              <NFormItem :label="t('portal.emergencyName')"><NInput v-model:value="form.emergency_name" /></NFormItem>
              <NFormItem :label="t('portal.emergencyPhone')"><NInput v-model:value="form.emergency_phone" /></NFormItem>
              <NFormItem :label="t('portal.emergencyRelation')"><NInput v-model:value="form.emergency_relation" /></NFormItem>
            </template>
          </NForm>
          <NSpace justify="space-between">
            <NButton :disabled="step === 1" @click="step--">{{ t('common.back') }}</NButton>
            <NButton v-if="step < 4" type="primary" :loading="saving" @click="save(false)">{{ t('portal.saveContinue') }}</NButton>
            <NButton v-else type="primary" :loading="saving" @click="save(true)">{{ t('portal.submit') }}</NButton>
          </NSpace>
          <div v-if="pendingDocs.length" style="margin-top: 12px"><NTag type="warning" size="small">{{ t('portal.docsPending', { n: pendingDocs.length }) }}</NTag></div>
        </NCard>
        <p style="opacity: 0.6; font-size: 12px; margin-top: 16px">{{ t('portal.privacy') }}</p>
      </template>
    </NSpin>
  </div>
</template>

<style scoped>
.public-wrap { max-width: 760px; margin: 0 auto; padding: 32px 16px; min-height: 100vh; }
.brand { font-size: 14px; font-weight: 700; letter-spacing: .04em; text-transform: uppercase; opacity: .7; }
</style>
