<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { NCard, NForm, NFormItem, NInput, NButton, NSpace, NSelect, useMessage } from 'naive-ui'
import type { FormRules, FormInst } from 'naive-ui'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const { t } = useI18n()
const message = useMessage()
const auth = useAuthStore()

const formRef = ref<FormInst | null>(null)
const form = ref({
  company_name: '',
  email: '',
  password: '',
  first_name: '',
  last_name: '',
  country: 'PHL',
})

const countryOptions = [
  { label: 'Philippines', value: 'PHL' },
  { label: 'Sri Lanka', value: 'LKA' },
  { label: 'Singapore', value: 'SGP' },
  { label: 'Indonesia', value: 'IDN' },
]
const loading = ref(false)

const rules = computed<FormRules>(() => ({
  company_name: [
    { required: true, message: t('auth.fieldRequired'), trigger: 'blur' },
  ],
  first_name: [
    { required: true, message: t('auth.fieldRequired'), trigger: 'blur' },
  ],
  last_name: [
    { required: true, message: t('auth.fieldRequired'), trigger: 'blur' },
  ],
  email: [
    { required: true, message: t('auth.fieldRequired'), trigger: 'blur' },
    { type: 'email', message: t('auth.invalidEmail'), trigger: 'blur' },
  ],
  password: [
    { required: true, message: t('auth.fieldRequired'), trigger: 'blur' },
    { min: 8, message: t('auth.passwordMinLength'), trigger: 'blur' },
  ],
}))

async function handleRegister() {
  try {
    await formRef.value?.validate()
  } catch { return }
  loading.value = true
  try {
    await auth.register(form.value)
    router.push('/')
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } }
    message.error(err.data?.error?.message || t('auth.registerFailed'))
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div style="display: flex; justify-content: center; align-items: center; min-height: 100vh; background: var(--bg-secondary);">
    <NCard style="width: 440px;" :title="t('auth.registerTitle')">
      <NForm ref="formRef" :model="form" :rules="rules" @submit.prevent="handleRegister">
        <NFormItem :label="t('auth.companyName')" path="company_name">
          <NInput v-model:value="form.company_name" :placeholder="t('auth.companyPlaceholder')" />
        </NFormItem>
        <NFormItem :label="t('auth.country')" path="country">
          <NSelect v-model:value="form.country" :options="countryOptions" />
        </NFormItem>
        <NSpace :size="12" style="width: 100%;">
          <NFormItem :label="t('auth.firstName')" path="first_name" style="flex: 1;">
            <NInput v-model:value="form.first_name" />
          </NFormItem>
          <NFormItem :label="t('auth.lastName')" path="last_name" style="flex: 1;">
            <NInput v-model:value="form.last_name" />
          </NFormItem>
        </NSpace>
        <NFormItem :label="t('auth.email')" path="email">
          <NInput v-model:value="form.email" type="text" :placeholder="t('common.placeholder.email')" />
        </NFormItem>
        <NFormItem :label="t('auth.password')" path="password">
          <NInput v-model:value="form.password" type="password" show-password-on="click" :placeholder="t('common.placeholder.minChars')" />
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
