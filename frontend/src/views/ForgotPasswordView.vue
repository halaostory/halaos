<script setup lang="ts">
import { ref } from 'vue'
import { NForm, NFormItem, NInput, NButton, NAlert, useMessage } from 'naive-ui'
import type { FormRules, FormInst } from 'naive-ui'
import { authAPI } from '../api/client'

const formRef = ref<FormInst | null>(null)
const email = ref('')
const loading = ref(false)
const sent = ref(false)
const message = useMessage()

const rules: FormRules = {
  email: [
    { required: true, message: 'Email is required', trigger: ['blur', 'input'] },
    { type: 'email', message: 'Please enter a valid email', trigger: ['blur'] },
  ],
}

async function handleSubmit() {
  if (!formRef.value) return
  try {
    await formRef.value.validate()
  } catch {
    return
  }
  loading.value = true
  try {
    await authAPI.forgotPassword(email.value)
    sent.value = true
  } catch (err: any) {
    message.error(err.data?.error?.message || 'Something went wrong. Please try again.')
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
        <h2>Reset your password</h2>
      </div>

      <template v-if="!sent">
        <p class="auth-description">
          Enter your email address and we'll send you a link to reset your password.
        </p>
        <NForm ref="formRef" :model="{ email }" :rules="rules">
          <NFormItem path="email" label="Email">
            <NInput
              v-model:value="email"
              placeholder="email@company.com"
              :input-props="{ type: 'email' }"
              @keyup.enter="handleSubmit"
            />
          </NFormItem>
          <NButton type="primary" block :loading="loading" @click.prevent="handleSubmit">
            Send Reset Link
          </NButton>
        </NForm>
      </template>

      <template v-else>
        <NAlert type="success" title="Check your email" style="margin-bottom: 16px;">
          If an account exists for <strong>{{ email }}</strong>, we've sent a password reset link.
          The link expires in 1 hour.
        </NAlert>
        <p class="auth-description">
          Didn't receive the email? Check your spam folder or
          <a href="#" @click.prevent="sent = false; loading = false">try again</a>.
        </p>
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
.auth-description {
  font-size: 14px;
  color: #64748b;
  margin-bottom: 20px;
  line-height: 1.5;
}
.auth-description a {
  color: #2563eb;
  text-decoration: none;
  font-weight: 600;
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
