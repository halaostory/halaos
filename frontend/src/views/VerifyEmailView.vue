<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { NButton, NAlert, NInput, NFormItem, useMessage } from 'naive-ui'
import { authAPI } from '../api/client'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const route = useRoute()
const message = useMessage()
const auth = useAuthStore()

const loading = ref(true)
const status = ref<'verifying' | 'success' | 'already_verified' | 'token_expired' | 'token_invalid'>('verifying')
const resendEmail = ref('')
const resendLoading = ref(false)
const resendSent = ref(false)

onMounted(async () => {
  const token = route.query.token as string
  if (!token) {
    status.value = 'token_invalid'
    loading.value = false
    return
  }

  try {
    const res = await authAPI.verifyEmail(token)
    const data = res.data || res

    // Check for "already_verified" success response
    if (data.status === 'already_verified') {
      status.value = 'already_verified'
      loading.value = false
      return
    }

    // Normal verification success — auto-login
    if (data.token) {
      localStorage.setItem('access_token', data.token)
      localStorage.setItem('refresh_token', data.refresh_token)
      auth.setUser(data.user)
    }
    status.value = 'success'
    loading.value = false
    setTimeout(() => router.push('/setup'), 1500)
  } catch (e: unknown) {
    const err = e as { data?: { error?: { code?: string } }; response?: { data?: { error?: { code?: string } } } }
    const code = err.data?.error?.code || err.response?.data?.error?.code
    if (code === 'token_expired') {
      status.value = 'token_expired'
    } else {
      status.value = 'token_invalid'
    }
    loading.value = false
  }
})

async function handleResend() {
  const email = resendEmail.value.trim()
  if (!email) {
    message.warning('Please enter your email address')
    return
  }
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
    message.warning('Please enter a valid email address')
    return
  }
  resendLoading.value = true
  try {
    await authAPI.resendVerification(email)
    resendSent.value = true
    message.success('Verification email sent!')
  } catch {
    message.error('Failed to resend verification email')
  } finally {
    resendLoading.value = false
  }
}
</script>

<template>
  <div class="auth-wrapper">
    <div class="auth-card">
      <div class="brand-header">
        <router-link to="/" class="brand-logo">
          <span class="logo-icon">H</span>
          <span class="logo-text">HalaOS</span>
        </router-link>
      </div>

      <!-- Loading -->
      <div v-if="loading" style="text-align: center; padding: 40px 0;">
        <p style="color: #64748b;">Verifying your email...</p>
      </div>

      <!-- Success -->
      <template v-else-if="status === 'success'">
        <NAlert type="success" title="Email verified!">
          Your account is now active. Redirecting to setup...
        </NAlert>
      </template>

      <!-- Already verified -->
      <template v-else-if="status === 'already_verified'">
        <NAlert type="info" title="Email already verified">
          Your email is already verified. You can log in to your account.
        </NAlert>
        <NButton type="primary" block style="margin-top: 16px;" @click="router.push('/login')">
          Go to Login
        </NButton>
      </template>

      <!-- Token expired -->
      <template v-else-if="status === 'token_expired'">
        <NAlert type="warning" title="Verification link expired">
          This link has expired (valid for 24 hours). Enter your email below to receive a new one.
        </NAlert>
        <template v-if="!resendSent">
          <NFormItem label="Email" style="margin-top: 16px;">
            <NInput
              v-model:value="resendEmail"
              placeholder="email@company.com"
              :input-props="{ type: 'email' }"
              @keyup.enter="handleResend"
            />
          </NFormItem>
          <NButton type="primary" block :loading="resendLoading" @click="handleResend">
            Resend Verification Email
          </NButton>
        </template>
        <NAlert v-else type="success" title="Email sent!" style="margin-top: 16px;">
          Check your inbox for the new verification link.
        </NAlert>
      </template>

      <!-- Token invalid -->
      <template v-else>
        <NAlert type="error" title="Invalid verification link">
          This link is invalid. It may have already been used or was copied incorrectly.
        </NAlert>
        <NButton type="primary" block style="margin-top: 16px;" @click="router.push('/register')">
          Go to Register
        </NButton>
      </template>

      <div class="auth-footer">
        <router-link to="/login">Back to Login</router-link>
      </div>
    </div>
  </div>
</template>

<style scoped>
.auth-wrapper {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  background: linear-gradient(135deg, #f8fafc 0%, #eef2ff 50%, #f8fafc 100%);
}
.auth-card {
  width: 420px;
  max-width: 90vw;
  background: #fff;
  border-radius: 16px;
  padding: 40px 36px 32px;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.06);
}
.brand-header {
  text-align: center;
  margin-bottom: 24px;
}
.brand-logo {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  text-decoration: none;
  margin-bottom: 16px;
}
.logo-icon {
  width: 36px;
  height: 36px;
  background: linear-gradient(135deg, #2563eb, #1d4ed8);
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
.auth-footer {
  text-align: center;
  margin-top: 20px;
  padding-top: 20px;
  border-top: 1px solid #f1f5f9;
  font-size: 14px;
}
.auth-footer a {
  color: #2563eb;
  font-weight: 600;
  text-decoration: none;
}
</style>
