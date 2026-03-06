<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { NCard, NForm, NFormItem, NInput, NButton, NSpace, useMessage } from 'naive-ui'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const { t } = useI18n()
const message = useMessage()
const auth = useAuthStore()

const form = ref({
  company_name: '',
  email: '',
  password: '',
  first_name: '',
  last_name: '',
})
const loading = ref(false)

async function handleRegister() {
  loading.value = true
  try {
    await auth.register(form.value)
    router.push('/')
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } }
    message.error(err.data?.error?.message || 'Registration failed')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div style="display: flex; justify-content: center; align-items: center; min-height: 100vh; background: var(--bg-secondary);">
    <NCard style="width: 440px;" :title="t('auth.registerTitle')">
      <NForm @submit.prevent="handleRegister">
        <NFormItem :label="t('auth.companyName')">
          <NInput v-model:value="form.company_name" placeholder="Your Company Inc." />
        </NFormItem>
        <NSpace :size="12" style="width: 100%;">
          <NFormItem :label="t('auth.firstName')" style="flex: 1;">
            <NInput v-model:value="form.first_name" />
          </NFormItem>
          <NFormItem :label="t('auth.lastName')" style="flex: 1;">
            <NInput v-model:value="form.last_name" />
          </NFormItem>
        </NSpace>
        <NFormItem :label="t('auth.email')">
          <NInput v-model:value="form.email" type="text" placeholder="email@company.com" />
        </NFormItem>
        <NFormItem :label="t('auth.password')">
          <NInput v-model:value="form.password" type="password" show-password-on="click" placeholder="Min 8 characters" />
        </NFormItem>
        <NButton type="primary" block :loading="loading" attr-type="submit">
          {{ t('auth.register') }}
        </NButton>
      </NForm>
      <template #footer>
        <NSpace justify="center">
          <span>{{ t('auth.hasAccount') }}</span>
          <router-link to="/login">{{ t('auth.login') }}</router-link>
        </NSpace>
      </template>
    </NCard>
  </div>
</template>
