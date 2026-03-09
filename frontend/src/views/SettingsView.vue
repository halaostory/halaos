<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { NCard, NForm, NFormItem, NInput, NSelect, NButton, NSpace, NUpload, NAvatar, NSwitch, NTag, NPopconfirm, NEmpty, useMessage } from 'naive-ui'
import type { UploadFileInfo } from 'naive-ui'
import { companyAPI, botAPI, byokAPI } from '../api/client'

const { t } = useI18n()
const message = useMessage()
const loading = ref(false)
const logoUploading = ref(false)
const logoUrl = ref<string | null>(null)

const form = ref({
  name: '',
  legal_name: '',
  tin: '',
  bir_rdo: '',
  address: '',
  city: '',
  province: '',
  zip_code: '',
  timezone: 'Asia/Manila',
  pay_frequency: 'semi_monthly',
  sss_er_no: '',
  philhealth_er_no: '',
  pagibig_er_no: '',
  bank_name: '',
  bank_branch: '',
  bank_account_no: '',
  bank_account_name: '',
  contact_person: '',
  contact_email: '',
  contact_phone: '',
})

const payFreqOptions = computed(() => [
  { label: t('settings.semiMonthly'), value: 'semi_monthly' },
  { label: t('settings.monthly'), value: 'monthly' },
  { label: t('settings.biWeekly'), value: 'bi_weekly' },
  { label: t('settings.weekly'), value: 'weekly' },
])

const apiBase = import.meta.env.VITE_API_URL || '/api'
const fullLogoUrl = computed(() => {
  if (!logoUrl.value) return null
  if (logoUrl.value.startsWith('http')) return logoUrl.value
  return `${apiBase.replace(/\/api$/, '')}${logoUrl.value}`
})

onMounted(async () => {
  try {
    const res = await companyAPI.get() as { data?: Record<string, string> }
    const company = (res.data || res) as Record<string, string>
    form.value.name = company.name || ''
    form.value.legal_name = company.legal_name || ''
    form.value.tin = company.tin || ''
    form.value.bir_rdo = company.bir_rdo || ''
    form.value.address = company.address || ''
    form.value.city = company.city || ''
    form.value.province = company.province || ''
    form.value.zip_code = company.zip_code || ''
    form.value.timezone = company.timezone || 'Asia/Manila'
    form.value.pay_frequency = company.pay_frequency || 'semi_monthly'
    form.value.sss_er_no = company.sss_er_no || ''
    form.value.philhealth_er_no = company.philhealth_er_no || ''
    form.value.pagibig_er_no = company.pagibig_er_no || ''
    form.value.bank_name = company.bank_name || ''
    form.value.bank_branch = company.bank_branch || ''
    form.value.bank_account_no = company.bank_account_no || ''
    form.value.bank_account_name = company.bank_account_name || ''
    form.value.contact_person = company.contact_person || ''
    form.value.contact_email = company.contact_email || ''
    form.value.contact_phone = company.contact_phone || ''
    logoUrl.value = company.logo_url || null
  } catch (e) {
    console.error('Failed to load company settings', e)
  }
  loadBotConfig()
})

async function handleSave() {
  loading.value = true
  try {
    await companyAPI.update(form.value)
    message.success(t('settings.saved'))
  } catch {
    message.error(t('settings.saveFailed'))
  } finally {
    loading.value = false
  }
}

async function handleLogoUpload({ file }: { file: UploadFileInfo }) {
  if (!file.file) return
  logoUploading.value = true
  try {
    const formData = new FormData()
    formData.append('logo', file.file)
    const res = await companyAPI.uploadLogo(formData) as { data?: { logo_url?: string } }
    const company = (res.data || res) as { logo_url?: string }
    if (company.logo_url) {
      logoUrl.value = company.logo_url
    }
    message.success(t('settings.logoUploaded'))
  } catch {
    message.error(t('settings.logoUploadFailed'))
  } finally {
    logoUploading.value = false
  }
}

// Bot Configuration
const botForm = ref({
  platform: 'telegram',
  bot_token: '',
  bot_username: '',
  is_active: false,
})
const botLoading = ref(false)

async function loadBotConfig() {
  try {
    const res = await botAPI.listBotConfigs() as { data?: Array<Record<string, unknown>> }
    const configs = res.data || (Array.isArray(res) ? res : [])
    const tg = (configs as Array<Record<string, unknown>>).find((c) => c.platform === 'telegram')
    if (tg) {
      botForm.value.bot_token = (tg.bot_token as string) || ''
      botForm.value.bot_username = (tg.bot_username as string) || ''
      botForm.value.is_active = !!tg.is_active
    }
  } catch {
    // no config yet
  }
}

async function saveBotConfig() {
  botLoading.value = true
  try {
    await botAPI.saveBotConfig({
      platform: 'telegram',
      bot_token: botForm.value.bot_token,
      bot_username: botForm.value.bot_username,
      is_active: botForm.value.is_active,
    })
    message.success(t('settings.saved'))
  } catch {
    message.error(t('settings.saveFailed'))
  } finally {
    botLoading.value = false
  }
}

async function handleRemoveLogo() {
  loading.value = true
  try {
    await companyAPI.update({ ...form.value, logo_url: null })
    logoUrl.value = null
    message.success(t('settings.saved'))
  } catch {
    message.error(t('settings.saveFailed'))
  } finally {
    loading.value = false
  }
}

// BYOK Key Management
interface ByokKey {
  id: string
  provider: string
  key_hint: string
  model_override: string
  label: string
  is_active: boolean
  user_id: number | null
  created_at: string
}

const byokKeys = ref<ByokKey[]>([])
const byokSaving = ref(false)
const showByokForm = ref(false)
const byokValidating = ref(false)

const byokForm = ref({
  provider: 'anthropic',
  api_key: '',
  model_override: '',
  label: '',
})

const providerOptions = [
  { label: 'Anthropic (Claude)', value: 'anthropic' },
  { label: 'OpenAI (GPT)', value: 'openai' },
  { label: 'Google (Gemini)', value: 'gemini' },
]

async function loadByokKeys() {
  try {
    const res = await byokAPI.listKeys() as { data?: ByokKey[] }
    byokKeys.value = (res.data || (Array.isArray(res) ? res : [])) as ByokKey[]
  } catch {
    // no keys yet
  }
}

async function saveByokKey() {
  if (!byokForm.value.api_key || byokForm.value.api_key.length < 10) {
    message.warning(t('settings.byokKeyTooShort'))
    return
  }
  byokSaving.value = true
  try {
    await byokAPI.createKey({
      provider: byokForm.value.provider,
      api_key: byokForm.value.api_key,
      model_override: byokForm.value.model_override || '',
      label: byokForm.value.label || '',
      user_id: null,
    })
    message.success(t('settings.byokKeySaved'))
    byokForm.value = { provider: 'anthropic', api_key: '', model_override: '', label: '' }
    showByokForm.value = false
    await loadByokKeys()
  } catch {
    message.error(t('settings.byokSaveFailed'))
  } finally {
    byokSaving.value = false
  }
}

async function validateByokKey() {
  if (!byokForm.value.api_key || byokForm.value.api_key.length < 10) {
    message.warning(t('settings.byokKeyTooShort'))
    return
  }
  byokValidating.value = true
  try {
    const res = await byokAPI.validateKey({
      provider: byokForm.value.provider,
      api_key: byokForm.value.api_key,
    }) as { data?: { valid: boolean; error?: string } }
    const result = res.data || (res as unknown as { valid: boolean; error?: string })
    if (result.valid) {
      message.success(t('settings.byokKeyValid'))
    } else {
      message.error(result.error || t('settings.byokKeyInvalid'))
    }
  } catch {
    message.error(t('settings.byokKeyInvalid'))
  } finally {
    byokValidating.value = false
  }
}

async function deleteByokKey(id: string) {
  try {
    await byokAPI.deleteKey(id)
    message.success(t('common.deleted'))
    await loadByokKeys()
  } catch {
    message.error(t('settings.byokSaveFailed'))
  }
}

onMounted(() => {
  loadByokKeys()
})
</script>

<template>
  <div>
    <!-- Logo / Branding Section -->
    <NCard :title="t('settings.companyLogo')" style="margin-bottom: 24px;">
      <div style="display: flex; align-items: center; gap: 20px;">
        <NAvatar
          v-if="fullLogoUrl"
          :src="fullLogoUrl"
          :size="80"
          style="border: 1px solid #e0e0e0; border-radius: 8px;"
          object-fit="contain"
        />
        <NAvatar v-else :size="80" style="border: 1px dashed #ccc; border-radius: 8px; background: #f5f5f5; font-size: 28px; color: #999;">
          {{ form.name?.charAt(0)?.toUpperCase() || '?' }}
        </NAvatar>
        <div>
          <NSpace>
            <NUpload
              :show-file-list="false"
              :custom-request="() => {}"
              accept=".png,.jpg,.jpeg,.svg,.webp"
              @change="handleLogoUpload"
            >
              <NButton :loading="logoUploading" size="small">{{ t('settings.uploadLogo') }}</NButton>
            </NUpload>
            <NButton v-if="logoUrl" size="small" quaternary type="error" @click="handleRemoveLogo">
              {{ t('settings.removeLogo') }}
            </NButton>
          </NSpace>
          <div style="font-size: 12px; color: #999; margin-top: 6px;">{{ t('settings.logoHint') }}</div>
        </div>
      </div>
    </NCard>

    <!-- Company Info -->
    <NCard :title="t('nav.settings')" style="margin-bottom: 24px;">
      <NForm @submit.prevent="handleSave" label-placement="left" label-width="140">
        <NFormItem :label="t('settings.companyName')">
          <NInput v-model:value="form.name" />
        </NFormItem>
        <NFormItem :label="t('settings.legalName')">
          <NInput v-model:value="form.legal_name" />
        </NFormItem>
        <NFormItem :label="t('settings.tin')">
          <NInput v-model:value="form.tin" />
        </NFormItem>
        <NFormItem :label="t('settings.birRdo')">
          <NInput v-model:value="form.bir_rdo" />
        </NFormItem>
        <NFormItem :label="t('settings.address')">
          <NInput v-model:value="form.address" />
        </NFormItem>
        <NSpace :size="12" style="width: 100%;">
          <NFormItem :label="t('settings.city')" style="flex: 1;">
            <NInput v-model:value="form.city" />
          </NFormItem>
          <NFormItem :label="t('settings.province')" style="flex: 1;">
            <NInput v-model:value="form.province" />
          </NFormItem>
          <NFormItem :label="t('settings.zipCode')" style="flex: 1;">
            <NInput v-model:value="form.zip_code" />
          </NFormItem>
        </NSpace>
        <NFormItem :label="t('settings.payFrequency')">
          <NSelect v-model:value="form.pay_frequency" :options="payFreqOptions" />
        </NFormItem>
        <NButton type="primary" :loading="loading" attr-type="submit">{{ t('common.save') }}</NButton>
      </NForm>
    </NCard>

    <!-- Government Registration -->
    <NCard :title="t('settings.govRegistration')" style="margin-bottom: 24px;">
      <NForm @submit.prevent="handleSave" label-placement="left" label-width="180">
        <NFormItem :label="t('settings.sssErNo')">
          <NInput v-model:value="form.sss_er_no" />
        </NFormItem>
        <NFormItem :label="t('settings.philhealthErNo')">
          <NInput v-model:value="form.philhealth_er_no" />
        </NFormItem>
        <NFormItem :label="t('settings.pagibigErNo')">
          <NInput v-model:value="form.pagibig_er_no" />
        </NFormItem>
        <NButton type="primary" :loading="loading" attr-type="submit">{{ t('common.save') }}</NButton>
      </NForm>
    </NCard>

    <!-- Banking Details -->
    <NCard :title="t('settings.bankingDetails')" style="margin-bottom: 24px;">
      <NForm @submit.prevent="handleSave" label-placement="left" label-width="140">
        <NFormItem :label="t('settings.bankName')">
          <NInput v-model:value="form.bank_name" />
        </NFormItem>
        <NFormItem :label="t('settings.bankBranch')">
          <NInput v-model:value="form.bank_branch" />
        </NFormItem>
        <NFormItem :label="t('settings.bankAccountNo')">
          <NInput v-model:value="form.bank_account_no" />
        </NFormItem>
        <NFormItem :label="t('settings.bankAccountName')">
          <NInput v-model:value="form.bank_account_name" />
        </NFormItem>
        <NButton type="primary" :loading="loading" attr-type="submit">{{ t('common.save') }}</NButton>
      </NForm>
    </NCard>

    <!-- Contact Person -->
    <NCard :title="t('settings.contactPerson')">
      <NForm @submit.prevent="handleSave" label-placement="left" label-width="140">
        <NFormItem :label="t('settings.contactPerson')">
          <NInput v-model:value="form.contact_person" />
        </NFormItem>
        <NFormItem :label="t('settings.contactEmail')">
          <NInput v-model:value="form.contact_email" />
        </NFormItem>
        <NFormItem :label="t('settings.contactPhone')">
          <NInput v-model:value="form.contact_phone" />
        </NFormItem>
        <NButton type="primary" :loading="loading" attr-type="submit">{{ t('common.save') }}</NButton>
      </NForm>
    </NCard>
    <!-- Bot Configuration -->
    <NCard :title="t('settings.botConfig')" style="margin-top: 24px;">
      <NForm @submit.prevent="saveBotConfig" label-placement="left" label-width="140">
        <NFormItem :label="t('settings.botPlatform')">
          <NInput :value="botForm.platform" disabled />
        </NFormItem>
        <NFormItem :label="t('settings.botToken')">
          <NInput v-model:value="botForm.bot_token" type="password" show-password-on="click" :placeholder="t('settings.botTokenHint')" />
        </NFormItem>
        <NFormItem :label="t('settings.botUsername')">
          <NInput v-model:value="botForm.bot_username" placeholder="@your_bot" />
        </NFormItem>
        <NFormItem :label="t('settings.botActive')">
          <NSpace align="center" :size="8">
            <NSwitch v-model:value="botForm.is_active" />
            <span v-if="botForm.is_active" style="color: #18a058; font-size: 12px;">● {{ t('settings.botRunning') }}</span>
          </NSpace>
        </NFormItem>
        <NButton type="primary" :loading="botLoading" attr-type="submit">{{ t('common.save') }}</NButton>
      </NForm>
    </NCard>

    <!-- BYOK API Key Management -->
    <NCard :title="t('settings.byokTitle')" style="margin-top: 24px;">
      <template #header-extra>
        <NButton size="small" type="primary" @click="showByokForm = !showByokForm">
          {{ showByokForm ? t('common.cancel') : t('settings.byokAddKey') }}
        </NButton>
      </template>

      <!-- Add Key Form -->
      <div v-if="showByokForm" style="margin-bottom: 20px; padding: 16px; background: #f9f9f9; border-radius: 8px;">
        <NForm label-placement="left" label-width="120">
          <NFormItem :label="t('settings.byokProvider')">
            <NSelect v-model:value="byokForm.provider" :options="providerOptions" />
          </NFormItem>
          <NFormItem :label="t('settings.byokApiKey')">
            <NInput v-model:value="byokForm.api_key" type="password" show-password-on="click" :placeholder="t('settings.byokApiKeyHint')" />
          </NFormItem>
          <NFormItem :label="t('settings.byokModel')">
            <NInput v-model:value="byokForm.model_override" :placeholder="t('settings.byokModelHint')" />
          </NFormItem>
          <NFormItem :label="t('settings.byokLabel')">
            <NInput v-model:value="byokForm.label" :placeholder="t('settings.byokLabelHint')" />
          </NFormItem>
          <NSpace>
            <NButton type="primary" :loading="byokSaving" @click="saveByokKey">{{ t('common.save') }}</NButton>
            <NButton :loading="byokValidating" @click="validateByokKey">{{ t('settings.byokValidate') }}</NButton>
          </NSpace>
        </NForm>
      </div>

      <!-- Keys List -->
      <div v-if="byokKeys.length > 0">
        <div v-for="key in byokKeys" :key="key.id" style="display: flex; align-items: center; justify-content: space-between; padding: 10px 0; border-bottom: 1px solid #f0f0f0;">
          <div style="display: flex; align-items: center; gap: 12px;">
            <NTag :type="key.is_active ? 'success' : 'default'" size="small">{{ key.provider }}</NTag>
            <span style="font-family: monospace; font-size: 13px;">{{ key.key_hint }}</span>
            <span v-if="key.label" style="color: #999; font-size: 12px;">{{ key.label }}</span>
            <span v-if="key.model_override" style="color: #666; font-size: 11px;">({{ key.model_override }})</span>
          </div>
          <NPopconfirm @positive-click="deleteByokKey(key.id)">
            <template #trigger>
              <NButton size="tiny" quaternary type="error">{{ t('common.delete') }}</NButton>
            </template>
            {{ t('settings.byokDeleteConfirm') }}
          </NPopconfirm>
        </div>
      </div>
      <NEmpty v-else :description="t('settings.byokNoKeys')" />
    </NCard>
  </div>
</template>
