<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { NCard, NForm, NFormItem, NInput, NButton, NSpace, useMessage } from 'naive-ui'
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
    { required: true, message: t('auth.email'), trigger: 'blur' },
    { type: 'email', message: t('auth.invalidEmail'), trigger: 'blur' },
  ],
  password: [
    { required: true, message: t('auth.password'), trigger: 'blur' },
  ],
}))

async function handleLogin() {
  try {
    await formRef.value?.validate()
  } catch { return }
  loading.value = true
  try {
    await auth.login(form.value.email, form.value.password)
    const redirect = (route.query.redirect as string) || '/'
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
  <div style="display: flex; justify-content: center; align-items: center; min-height: 100vh; background: var(--bg-secondary);">
    <NCard style="width: 400px;" :title="t('auth.loginTitle')">
      <NForm ref="formRef" :model="form" :rules="rules" @submit.prevent="handleLogin">
        <NFormItem :label="t('auth.email')" path="email">
          <NInput v-model:value="form.email" type="text" :placeholder="t('common.placeholder.email')" />
        </NFormItem>
        <NFormItem :label="t('auth.password')" path="password">
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
