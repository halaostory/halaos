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

const selectedJurisdiction = ref('PH')

const jurisdictions = [
  { code: 'PH', name: 'Philippines' },
  { code: 'SG', name: 'Singapore' },
  { code: 'LK', name: 'Sri Lanka' },
]

const rules = computed<FormRules>(() => ({
  email: [
    { required: true, message: t('auth.fieldRequired'), trigger: ['blur', 'input'] },
  ],
  password: [
    { required: true, message: t('auth.fieldRequired'), trigger: ['blur', 'input'] },
  ],
}))

async function handleLogin() {
  if (!formRef.value) return
  try {
    await formRef.value.validate()
  } catch (errors: unknown) {
    const errArr = errors as Array<Array<{ message?: string }>> | undefined
    const firstMsg = errArr?.[0]?.[0]?.message
    message.warning(firstMsg || t('auth.fieldRequired'))
    return
  }
  loading.value = true
  try {
    await auth.login(form.value.email, form.value.password)
    const redirect = (route.query.redirect as string) || '/dashboard'
    router.push(redirect)
  } catch (e: unknown) {
    const err = e as { response?: { data?: { error?: { message?: string } } }; data?: { error?: { message?: string } } }
    const msg = err.response?.data?.error?.message || err.data?.error?.message || t('auth.loginFailed')
    message.error(msg)
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
      <div class="product-switcher">
        <span class="product-btn active">
          <span class="product-icon">&#x1F465;</span> HR
        </span>
        <a href="https://finance.halaos.com/login" class="product-btn">
          <span class="product-icon">&#x1F4B0;</span> Finance
        </a>
      </div>
      <div class="jurisdiction-selector">
        <p class="jurisdiction-label">{{ t('auth.selectCountry') }}</p>
        <div class="jurisdiction-options">
          <button
            v-for="j in jurisdictions"
            :key="j.code"
            type="button"
            class="jurisdiction-btn"
            :class="{ active: selectedJurisdiction === j.code }"
            @click="selectedJurisdiction = j.code"
            :data-testid="'jurisdiction-' + j.code.toLowerCase()"
          >
            <span class="flag">{{ j.code }}</span>
            <span class="country-name">{{ j.name }}</span>
          </button>
        </div>
      </div>
      <NForm ref="formRef" :model="form" :rules="rules">
        <NFormItem path="email" :label="t('auth.email')">
          <NInput
            v-model:value="form.email"
            placeholder="email@company.com"
            autocomplete="username email"
            :input-props="{ type: 'email' }"
            data-testid="email-input"
          />
        </NFormItem>
        <NFormItem path="password" :label="t('auth.password')">
          <NInput
            v-model:value="form.password"
            type="password"
            show-password-on="click"
            :placeholder="t('auth.password')"
            autocomplete="current-password"
            data-testid="password-input"
          />
        </NFormItem>
        <NButton
          type="primary"
          block
          :loading="loading"
          style="margin-top: 8px;"
          data-testid="login-submit"
          @click.prevent="handleLogin"
        >
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
.product-switcher {
  display: flex;
  gap: 12px;
  justify-content: center;
  margin-bottom: 28px;
}
.product-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 10px 24px;
  border-radius: 10px;
  font-size: 14px;
  font-weight: 600;
  text-decoration: none;
  transition: all 0.2s;
  border: 2px solid #e2e8f0;
  color: #64748b;
  background: #f8fafc;
  cursor: pointer;
}
.product-btn:hover {
  border-color: #93c5fd;
  color: #2563eb;
  background: #eff6ff;
}
.product-btn.active {
  background: linear-gradient(135deg, #2563eb, #1d4ed8);
  color: #fff;
  border-color: transparent;
  cursor: default;
}
.product-icon {
  font-size: 16px;
}
.jurisdiction-selector {
  margin-bottom: 24px;
}
.jurisdiction-label {
  text-align: center;
  font-size: 14px;
  color: var(--text-secondary, #64748b);
  margin-bottom: 10px;
}
.jurisdiction-options {
  display: flex;
  gap: 12px;
  justify-content: center;
}
.jurisdiction-btn {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 12px 24px;
  border: 2px solid var(--border-default, #e5e7eb);
  border-radius: 12px;
  background: var(--bg-surface, #fff);
  cursor: pointer;
  transition: all 0.2s;
  min-width: 100px;
}
.jurisdiction-btn:hover {
  border-color: #93c5fd;
  background: #eff6ff;
}
.jurisdiction-btn.active {
  border-color: #2563eb;
  background: #eff6ff;
  box-shadow: 0 0 0 1px #2563eb;
}
.jurisdiction-btn .flag {
  font-size: 24px;
  font-weight: 700;
  color: #1e3a5f;
  margin-bottom: 4px;
}
.jurisdiction-btn .country-name {
  font-size: 12px;
  color: var(--text-secondary, #555);
}
@media (max-width: 768px) {
  .jurisdiction-options {
    gap: 8px;
  }
  .jurisdiction-btn {
    padding: 10px 16px;
    min-width: 80px;
  }
  .jurisdiction-btn .flag {
    font-size: 20px;
  }
  .jurisdiction-btn .country-name {
    font-size: 11px;
  }
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
  color: #2563eb;
  font-weight: 600;
  text-decoration: none;
  margin-left: 4px;
}
</style>
