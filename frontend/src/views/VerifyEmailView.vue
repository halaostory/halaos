<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { NCard, NResult, NButton, NSpin } from 'naive-ui'
import { authAPI } from '../api/client'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()

const loading = ref(true)
const success = ref(false)
const errorMsg = ref('')

onMounted(async () => {
  const token = route.query.token as string
  if (!token) {
    errorMsg.value = t('auth.noVerificationToken')
    loading.value = false
    return
  }

  try {
    await authAPI.verifyEmail(token)
    success.value = true
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } }
    errorMsg.value = err.data?.error?.message || t('auth.verificationFailed')
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div style="display: flex; justify-content: center; align-items: center; min-height: 100vh; background: var(--bg-secondary);">
    <NCard style="width: 480px;">
      <div v-if="loading" style="text-align: center; padding: 48px;">
        <NSpin size="large" />
        <p style="margin-top: 16px; color: var(--text-secondary);">{{ t('auth.verifying') }}</p>
      </div>

      <NResult
        v-else-if="success"
        status="success"
        :title="t('auth.emailVerified')"
        :description="t('auth.emailVerifiedDesc')"
      >
        <template #footer>
          <NButton type="primary" @click="router.push('/login')">
            {{ t('auth.goToLogin') }}
          </NButton>
        </template>
      </NResult>

      <NResult
        v-else
        status="error"
        :title="t('auth.verificationFailed')"
        :description="errorMsg"
      >
        <template #footer>
          <NButton type="primary" @click="router.push('/register')">
            {{ t('auth.register') }}
          </NButton>
        </template>
      </NResult>
    </NCard>
  </div>
</template>
