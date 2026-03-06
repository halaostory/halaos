<script setup lang="ts">
import { ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { NCard, NForm, NFormItem, NInput, NButton, NSpace, useMessage } from 'naive-ui'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const route = useRoute()
const { t } = useI18n()
const message = useMessage()
const auth = useAuthStore()

const form = ref({ email: '', password: '' })
const loading = ref(false)

async function handleLogin() {
  loading.value = true
  try {
    await auth.login(form.value.email, form.value.password)
    const redirect = (route.query.redirect as string) || '/'
    router.push(redirect)
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } }
    message.error(err.data?.error?.message || 'Login failed')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div style="display: flex; justify-content: center; align-items: center; min-height: 100vh; background: var(--bg-secondary);">
    <NCard style="width: 400px;" :title="t('auth.loginTitle')">
      <NForm @submit.prevent="handleLogin">
        <NFormItem :label="t('auth.email')">
          <NInput v-model:value="form.email" type="text" placeholder="email@company.com" />
        </NFormItem>
        <NFormItem :label="t('auth.password')">
          <NInput v-model:value="form.password" type="password" show-password-on="click" />
        </NFormItem>
        <NButton type="primary" block :loading="loading" attr-type="submit">
          {{ t('auth.login') }}
        </NButton>
      </NForm>
      <template #footer>
        <NSpace justify="center">
          <span>{{ t('auth.noAccount') }}</span>
          <router-link to="/register">{{ t('auth.register') }}</router-link>
        </NSpace>
      </template>
    </NCard>
  </div>
</template>
