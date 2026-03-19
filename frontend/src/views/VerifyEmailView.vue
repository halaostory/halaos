<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { NCard, NResult, NButton, NSpin } from 'naive-ui'
import { authAPI } from '../api/client'
import { useAuthStore } from '../stores/auth'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const auth = useAuthStore()

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
    const raw = await authAPI.verifyEmail(token)
    const res = (raw as Record<string, unknown>).data as Record<string, unknown> | undefined
    const data = res ?? (raw as Record<string, unknown>)

    success.value = true

    // Auto-login if tokens are returned (magic link flow)
    if (data.token && typeof data.token === 'string') {
      localStorage.setItem('token', data.token)
      if (data.refresh_token && typeof data.refresh_token === 'string') {
        localStorage.setItem('refresh_token', data.refresh_token)
      }
      // Set user info in auth store
      if (data.user) {
        auth.setUser(data.user as { id: number; email: string; first_name: string; last_name: string; role: string; company_id: number })
      }
      await auth.fetchMe()
      // Redirect to setup wizard after short delay
      setTimeout(() => router.push('/setup'), 1500)
    }
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } }
    errorMsg.value = err.data?.error?.message || t('auth.verificationFailed')
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="verify-wrapper">
    <div class="verify-card">
      <div v-if="loading" style="text-align: center; padding: 48px;">
        <NSpin size="large" />
        <p style="margin-top: 16px; color: #64748b;">Verifying your email...</p>
      </div>

      <NResult
        v-else-if="success"
        status="success"
        title="Email Verified!"
        description="Your account is now active. Redirecting you to set up your company..."
      >
        <template #footer>
          <NButton type="primary" @click="router.push('/setup')">
            Continue to Setup
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
            Try Again
          </NButton>
        </template>
      </NResult>
    </div>
  </div>
</template>

<style scoped>
.verify-wrapper {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  background: linear-gradient(135deg, #f8fafc 0%, #eef2ff 50%, #f8fafc 100%);
}
.verify-card {
  width: 480px;
  max-width: 90vw;
  background: #fff;
  border-radius: 16px;
  padding: 32px;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.06);
}
</style>
