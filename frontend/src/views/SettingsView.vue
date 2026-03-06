<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { NCard, NForm, NFormItem, NInput, NSelect, NButton, NSpace, NUpload, NAvatar, useMessage } from 'naive-ui'
import type { UploadFileInfo } from 'naive-ui'
import { companyAPI } from '../api/client'

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
    logoUrl.value = company.logo_url || null
  } catch {
    // ok
  }
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
    <NCard :title="t('nav.settings')">
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
  </div>
</template>
