<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { NSpin } from 'naive-ui'
import { useAuthStore } from '../stores/auth'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()
const auth = useAuthStore()
const error = ref('')
const loading = ref(true)

onMounted(async () => {
  const token = route.query.token as string
  if (!token) {
    error.value = 'No SSO token provided'
    loading.value = false
    return
  }
  try {
    await auth.loginWithSSO(token)
    router.replace('/')
  } catch {
    error.value = 'SSO login failed'
    loading.value = false
  }
})
</script>

<template>
  <div class="sso-page">
    <div class="sso-card">
      <template v-if="loading">
        <NSpin size="large" />
        <p style="margin-top: 16px; color: var(--text-muted);">{{ t('common.loading') }}</p>
      </template>
      <template v-else>
        <p style="color: var(--text-primary); font-size: 16px;">{{ error }}</p>
        <router-link to="/login" style="color: var(--brand-primary); margin-top: 12px; display: inline-block;">
          {{ t('auth.login') }}
        </router-link>
      </template>
    </div>
  </div>
</template>

<style scoped>
.sso-page {
  min-height: 100vh; display: flex; align-items: center; justify-content: center;
  background: var(--bg-app);
}
.sso-card {
  text-align: center; padding: 48px;
  background: var(--bg-surface); border-radius: 16px;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.06);
}
</style>
