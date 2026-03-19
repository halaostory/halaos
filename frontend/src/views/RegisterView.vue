<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { NForm, NFormItem, NInput, NButton, NSpace, NSelect, NResult, useMessage } from 'naive-ui'
import type { FormRules, FormInst } from 'naive-ui'
import { useAuthStore } from '../stores/auth'
import { authAPI } from '../api/client'

const router = useRouter()
const route = useRoute()
const { t } = useI18n()
const message = useMessage()
const auth = useAuthStore()

const referralCode = (route.query.ref as string) || ''

const formRef = ref<FormInst | null>(null)
const form = ref({
  company_name: '',
  email: '',
  password: '',
  first_name: '',
  last_name: '',
  country: 'PHL',
})

const countryOptions = [
  { label: 'Philippines', value: 'PHL' },
  { label: 'Sri Lanka', value: 'LKA' },
  { label: 'Singapore', value: 'SGP' },
  { label: 'Indonesia', value: 'IDN' },
]
const loading = ref(false)
const emailSent = ref(false)
const resending = ref(false)

const rules = computed<FormRules>(() => ({
  company_name: [
    { required: true, message: t('auth.fieldRequired'), trigger: 'blur' },
  ],
  first_name: [
    { required: true, message: t('auth.fieldRequired'), trigger: 'blur' },
  ],
  last_name: [
    { required: true, message: t('auth.fieldRequired'), trigger: 'blur' },
  ],
  email: [
    { required: true, message: t('auth.fieldRequired'), trigger: 'blur' },
    { type: 'email', message: t('auth.invalidEmail'), trigger: 'blur' },
  ],
  password: [
    { required: true, message: t('auth.fieldRequired'), trigger: 'blur' },
    { min: 8, message: t('auth.passwordMinLength'), trigger: 'blur' },
  ],
}))

async function handleRegister() {
  try {
    await formRef.value?.validate()
  } catch { return }
  loading.value = true
  try {
    const result = await auth.register({ ...form.value, referral_code: referralCode })
    if (result.emailSent) {
      emailSent.value = true
    } else {
      router.push('/setup')
    }
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } }
    message.error(err.data?.error?.message || t('auth.registerFailed'))
  } finally {
    loading.value = false
  }
}

async function handleResend() {
  resending.value = true
  try {
    await authAPI.resendVerification(form.value.email)
    message.success(t('auth.verificationResent'))
  } catch {
    message.error(t('auth.resendFailed'))
  } finally {
    resending.value = false
  }
}
</script>

<template>
  <div class="register-wrapper">
    <!-- Email sent confirmation -->
    <div v-if="emailSent" class="register-card">
      <div class="brand-header">
        <router-link to="/" class="brand-logo">
          <span class="logo-icon">H</span>
          <span class="logo-text">HalaOS</span>
        </router-link>
      </div>
      <NResult status="success" :title="t('auth.checkEmail')" :description="t('auth.verificationSent')">
        <template #footer>
          <p style="color: #64748b; margin-bottom: 16px;">{{ form.email }}</p>
          <NSpace vertical :size="12" style="width: 100%;">
            <NButton type="primary" block @click="router.push('/login')">
              {{ t('auth.goToLogin') }}
            </NButton>
            <NButton quaternary block :loading="resending" @click="handleResend">
              {{ t('auth.resendEmail') }}
            </NButton>
          </NSpace>
        </template>
      </NResult>
    </div>

    <!-- Registration form -->
    <div v-else class="register-card">
      <div class="brand-header">
        <router-link to="/" class="brand-logo">
          <span class="logo-icon">H</span>
          <span class="logo-text">HalaOS</span>
        </router-link>
        <h2>{{ t('auth.registerTitle') }}</h2>
      </div>
      <NForm ref="formRef" :model="form" :rules="rules" @submit.prevent="handleRegister">
        <NFormItem :label="t('auth.companyName')" path="company_name">
          <NInput v-model:value="form.company_name" :placeholder="t('auth.companyPlaceholder')" />
        </NFormItem>
        <NFormItem :label="t('auth.country')" path="country">
          <NSelect v-model:value="form.country" :options="countryOptions" />
        </NFormItem>
        <div class="name-row">
          <NFormItem :label="t('auth.firstName')" path="first_name" style="flex: 1;">
            <NInput v-model:value="form.first_name" />
          </NFormItem>
          <NFormItem :label="t('auth.lastName')" path="last_name" style="flex: 1;">
            <NInput v-model:value="form.last_name" />
          </NFormItem>
        </div>
        <NFormItem :label="t('auth.email')" path="email">
          <NInput v-model:value="form.email" type="text" :placeholder="t('common.placeholder.email')" />
        </NFormItem>
        <NFormItem :label="t('auth.password')" path="password">
          <NInput v-model:value="form.password" type="password" show-password-on="click" :placeholder="t('common.placeholder.minChars')" />
        </NFormItem>
        <NButton type="primary" block :loading="loading" attr-type="submit" @click="handleRegister">
          {{ t('auth.register') }}
        </NButton>
      </NForm>
      <div class="auth-footer">
        <span>{{ t('auth.hasAccount') }}</span>
        <router-link to="/login">{{ t('auth.login') }}</router-link>
      </div>
    </div>
  </div>
</template>

<style scoped>
.register-wrapper {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  background: linear-gradient(135deg, #f8fafc 0%, #eef2ff 50%, #f8fafc 100%);
  padding: 40px 16px;
}
.register-card {
  width: 460px;
  max-width: 90vw;
  background: #fff;
  border-radius: 16px;
  padding: 40px 36px 32px;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.06);
}
.brand-header {
  text-align: center;
  margin-bottom: 28px;
}
.brand-logo {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  text-decoration: none;
  margin-bottom: 12px;
}
.logo-icon {
  width: 36px;
  height: 36px;
  background: linear-gradient(135deg, #4f46e5, #7c3aed);
  color: #fff;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  font-weight: 800;
}
.logo-text {
  font-size: 22px;
  font-weight: 700;
  color: #0f172a;
}
.brand-header h2 {
  font-size: 16px;
  font-weight: 500;
  color: #64748b;
  margin: 0;
}
.name-row {
  display: flex;
  gap: 12px;
}
.auth-footer {
  text-align: center;
  margin-top: 20px;
  padding-top: 20px;
  border-top: 1px solid #f1f5f9;
  font-size: 14px;
  color: #64748b;
}
.auth-footer a {
  color: #4f46e5;
  font-weight: 600;
  text-decoration: none;
  margin-left: 4px;
}
</style>
