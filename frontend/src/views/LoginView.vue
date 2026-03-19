<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { NForm, NFormItem, NInput, NButton, useMessage } from 'naive-ui'
import type { FormRules, FormInst } from 'naive-ui'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const route = useRoute()
const { t } = useI18n()
const message = useMessage()
const auth = useAuthStore()

const formRef = ref<FormInst | null>(null)
const form = ref({ email: '', password: '' })
const loading = ref(false)

const rules = computed<FormRules>(() => ({
  email: [
    { required: true, message: t('auth.fieldRequired'), trigger: ['blur', 'input'] },
  ],
  password: [
    { required: true, message: t('auth.fieldRequired'), trigger: ['blur', 'input'] },
  ],
}))

async function handleLogin() {
  try {
    await formRef.value?.validate()
  } catch { return }
  loading.value = true
  try {
    await auth.login(form.value.email, form.value.password)
    const redirect = (route.query.redirect as string) || '/dashboard'
    router.push(redirect)
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } }
    message.error(err.data?.error?.message || t('auth.loginFailed'))
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-wrapper">
    <div class="login-card">
      <div class="brand-header">
        <router-link to="/" class="brand-logo">
          <span class="logo-icon">H</span>
          <span class="logo-text">HalaOS</span>
        </router-link>
        <h2>{{ t('auth.loginTitle') }}</h2>
      </div>
      <NForm ref="formRef" :model="form" :rules="rules" @submit.prevent="handleLogin">
        <NFormItem path="email" :label="t('auth.email')">
          <NInput
            v-model:value="form.email"
            placeholder="email@company.com"
            autocomplete="username email"
            :input-props="{ type: 'email' }"
          />
        </NFormItem>
        <NFormItem path="password" :label="t('auth.password')">
          <NInput
            v-model:value="form.password"
            type="password"
            show-password-on="click"
            :placeholder="t('auth.password')"
            autocomplete="current-password"
          />
        </NFormItem>
        <NButton type="primary" block :loading="loading" attr-type="submit" style="margin-top: 8px;">
          {{ t('auth.login') }}
        </NButton>
      </NForm>
      <div class="auth-footer">
        <span>{{ t('auth.noAccount') }}</span>
        <router-link to="/register">{{ t('auth.register') }}</router-link>
      </div>
    </div>
  </div>
</template>

<style scoped>
.login-wrapper {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  background: linear-gradient(135deg, #f8fafc 0%, #eef2ff 50%, #f8fafc 100%);
}
.login-card {
  width: 420px;
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
