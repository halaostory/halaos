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
  <div class="login-wrapper">
    <NCard style="width: 400px; max-width: 90vw;" :title="t('auth.loginTitle')">
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
      <template #footer>
        <NSpace justify="center">
          <span>{{ t('auth.noAccount') }}</span>
          <router-link to="/register">{{ t('auth.register') }}</router-link>
        </NSpace>
      </template>
    </NCard>
  </div>
</template>

<style scoped>
.login-wrapper {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  background: var(--bg-secondary);
}
</style>
