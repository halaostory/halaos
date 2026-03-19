<script setup lang="ts">
import { ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { NInput, NButton, NSpace, NResult, useMessage } from 'naive-ui'
import { useAuthStore } from '../stores/auth'
import { authAPI } from '../api/client'

const route = useRoute()
const { t } = useI18n()
const message = useMessage()
const auth = useAuthStore()

const referralCode = (route.query.ref as string) || ''
const emailInput = ref('')
const loading = ref(false)
const emailSent = ref(false)
const resending = ref(false)

async function handleRegister() {
  const email = emailInput.value.trim()
  if (!email) {
    message.warning('Please enter your email address')
    return
  }
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
    message.warning('Please enter a valid email address')
    return
  }

  loading.value = true
  try {
    const result = await auth.register({
      email,
      company_name: '',
      password: '',
      first_name: '',
      last_name: '',
      referral_code: referralCode,
    })
    if (result.emailSent) {
      emailSent.value = true
    }
  } catch (e: unknown) {
    const err = e as { response?: { data?: { error?: { message?: string } } }; data?: { error?: { message?: string } } }
    const msg = err.response?.data?.error?.message || err.data?.error?.message || 'Registration failed. Please try again.'
    message.error(msg)
  } finally {
    loading.value = false
  }
}

async function handleResend() {
  resending.value = true
  try {
    await authAPI.resendVerification(emailInput.value.trim())
    message.success(t('auth.verificationResent'))
  } catch {
    message.error(t('auth.resendFailed'))
  } finally {
    resending.value = false
  }
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter') {
    handleRegister()
  }
}
</script>

<template>
  <div class="register-wrapper">
    <!-- Magic link sent -->
    <div v-if="emailSent" class="register-card">
      <div class="brand-header">
        <router-link to="/" class="brand-logo">
          <span class="logo-icon">H</span>
          <span class="logo-text">HalaOS</span>
        </router-link>
      </div>
      <NResult status="success" title="Check your email" description="We sent you a magic link to sign in.">
        <template #footer>
          <p style="color: #4f46e5; font-weight: 600; margin-bottom: 16px; font-size: 16px;">{{ emailInput }}</p>
          <p style="color: #64748b; font-size: 14px; margin-bottom: 20px;">
            Click the link in your email to activate your account. The link expires in 24 hours.
          </p>
          <NSpace vertical :size="12" style="width: 100%;">
            <NButton quaternary block :loading="resending" @click="handleResend">
              Didn't receive it? Resend
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
        <h2>Free HR, Payroll & Tax Compliance</h2>
        <p class="subtitle">Get started in 10 seconds. No credit card required.</p>
      </div>
      <div class="form-area">
        <NInput
          v-model:value="emailInput"
          placeholder="Enter your work email"
          size="large"
          :input-props="{ name: 'email', autocomplete: 'email', type: 'email' }"
          @keydown="handleKeydown"
        />
        <NButton type="primary" block size="large" :loading="loading" style="margin-top: 12px;" @click="handleRegister">
          Get Started — It's Free
        </NButton>
      </div>
      <p class="trust-text">
        Join 100+ companies using HalaOS. No spam, unsubscribe anytime.
      </p>
      <div class="auth-footer">
        <span>Already have an account?</span>
        <router-link to="/login">Log In</router-link>
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
  width: 440px;
  max-width: 90vw;
  background: #fff;
  border-radius: 16px;
  padding: 40px 36px 32px;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.06);
}
.brand-header {
  text-align: center;
  margin-bottom: 32px;
}
.brand-logo {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  text-decoration: none;
  margin-bottom: 16px;
}
.logo-icon {
  width: 40px;
  height: 40px;
  background: linear-gradient(135deg, #4f46e5, #7c3aed);
  color: #fff;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 22px;
  font-weight: 800;
}
.logo-text {
  font-size: 24px;
  font-weight: 700;
  color: #0f172a;
}
.brand-header h2 {
  font-size: 18px;
  font-weight: 600;
  color: #0f172a;
  margin: 0 0 4px;
}
.subtitle {
  font-size: 14px;
  color: #64748b;
  margin: 0;
}
.form-area {
  margin-bottom: 16px;
}
.trust-text {
  text-align: center;
  font-size: 12px;
  color: #94a3b8;
  margin: 0 0 16px;
}
.auth-footer {
  text-align: center;
  padding-top: 16px;
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
