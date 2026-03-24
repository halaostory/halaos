<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { NForm, NFormItem, NInput, NButton, NAlert, useMessage } from 'naive-ui'
import type { FormRules, FormInst } from 'naive-ui'
import { authAPI } from '../api/client'

const router = useRouter()
const route = useRoute()
const message = useMessage()

const formRef = ref<FormInst | null>(null)
const form = ref({ password: '', confirmPassword: '' })
const loading = ref(false)
const success = ref(false)
const errorCode = ref<string | null>(null)
const token = ref('')

onMounted(() => {
  token.value = (route.query.token as string) || ''
  if (!token.value) {
    errorCode.value = 'token_invalid'
  }
})

const rules: FormRules = {
  password: [
    { required: true, message: 'Password is required', trigger: ['blur', 'input'] },
    { min: 8, message: 'Password must be at least 8 characters', trigger: ['blur'] },
  ],
  confirmPassword: [
    { required: true, message: 'Please confirm your password', trigger: ['blur', 'input'] },
    {
      validator: (_rule: unknown, value: string) => {
        return value === form.value.password || new Error('Passwords do not match')
      },
      trigger: ['blur'],
    },
  ],
}

async function handleReset() {
  if (!formRef.value) return
  try {
    await formRef.value.validate()
  } catch {
    return
  }
  loading.value = true
  try {
    const res = await authAPI.resetPassword(token.value, form.value.password)
    const data = (res as any).data || res
    // Auto-login with returned tokens
    if ((data as any).token) {
      localStorage.setItem('access_token', (data as any).token)
      localStorage.setItem('refresh_token', (data as any).refresh_token)
    }
    success.value = true
    message.success('Password reset successful!')
    setTimeout(() => router.push('/dashboard'), 1500)
  } catch (err: any) {
    const code = err.data?.error?.code || err.response?.data?.error?.code
    if (code === 'token_expired' || code === 'token_invalid') {
      errorCode.value = code
    } else {
      message.error(err.data?.error?.message || 'Password reset failed')
    }
  } finally {
    loading.value = false
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
        <h2>Set new password</h2>
      </div>

      <!-- Error state -->
      <template v-if="errorCode">
        <NAlert
          :type="errorCode === 'token_expired' ? 'warning' : 'error'"
          :title="errorCode === 'token_expired' ? 'Link expired' : 'Invalid link'"
          style="margin-bottom: 16px;"
        >
          <template v-if="errorCode === 'token_expired'">
            This reset link has expired. Please request a new one.
          </template>
          <template v-else>
            This reset link is invalid. It may have already been used.
          </template>
        </NAlert>
        <NButton type="primary" block @click="router.push('/forgot-password')">
          Request New Reset Link
        </NButton>
      </template>

      <!-- Success state -->
      <template v-else-if="success">
        <NAlert type="success" title="Password reset successful!" style="margin-bottom: 16px;">
          Your password has been updated. Redirecting to dashboard...
        </NAlert>
      </template>

      <!-- Form state -->
      <template v-else>
        <NForm ref="formRef" :model="form" :rules="rules">
          <NFormItem path="password" label="New Password">
            <NInput
              v-model:value="form.password"
              type="password"
              show-password-on="click"
              placeholder="At least 8 characters"
            />
          </NFormItem>
          <NFormItem path="confirmPassword" label="Confirm Password">
            <NInput
              v-model:value="form.confirmPassword"
              type="password"
              show-password-on="click"
              placeholder="Re-enter your password"
              @keyup.enter="handleReset"
            />
          </NFormItem>
          <NButton type="primary" block :loading="loading" @click.prevent="handleReset">
            Reset Password
          </NButton>
        </NForm>
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
.brand-header h2 {
  font-size: 16px;
  font-weight: 500;
  color: #64748b;
  margin: 0;
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
