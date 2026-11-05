<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { NCard, NForm, NFormItem, NInput, NButton, NSwitch, NTag, NSpace, NAlert, NDynamicTags, useMessage } from 'naive-ui'
import { authAPI } from '../api/client'

interface View { configured: boolean; issuer: string; client_id: string; email_domains: string[]; enabled: boolean; redirect_url: string; available: boolean }
const { t } = useI18n()
const message = useMessage()
const v = ref<View | null>(null)
const form = ref({ issuer: '', client_id: '', client_secret: '', email_domains: [] as string[], enabled: true })
const busy = ref(false)

async function load() {
  try {
    v.value = ((await authAPI.oidcProvider()) as { data: View }).data
    form.value = { issuer: v.value.issuer || 'https://accounts.google.com', client_id: v.value.client_id, client_secret: '', email_domains: [...v.value.email_domains], enabled: v.value.configured ? v.value.enabled : true }
  } catch { v.value = null }
}
async function save() {
  busy.value = true
  try {
    v.value = ((await authAPI.saveOidcProvider(form.value)) as { data: View }).data
    form.value.client_secret = ''
    message.success(t('oidc.saved'))
  } catch (e) {
    message.error((e as { data?: { error?: { message?: string } } }).data?.error?.message || t('common.saveFailed'))
  } finally { busy.value = false }
}
async function remove() {
  busy.value = true
  try { v.value = ((await authAPI.deleteOidcProvider()) as { data: View }).data } finally { busy.value = false }
}
async function copy(s: string) { await navigator.clipboard.writeText(s); message.success(t('mfa.copied')) }
onMounted(load)
</script>

<template>
  <NCard v-if="v" :title="t('oidc.title')" style="margin-top: 24px;">
    <template #header-extra>
      <NTag round :bordered="false" :type="v.configured && v.enabled ? 'success' : 'default'">{{ v.configured && v.enabled ? t('mfa.on') : t('mfa.off') }}</NTag>
    </template>
    <p style="margin-top: 0; opacity: .75;">{{ t('oidc.desc') }}</p>
    <NAlert v-if="!v.available" type="warning">{{ t('oidc.unavailable') }}</NAlert>
    <template v-else>
      <div class="redirect">
        <span>{{ t('oidc.redirect') }}</span>
        <code>{{ v.redirect_url }}</code>
        <NButton size="tiny" @click="copy(v.redirect_url)">{{ t('notices.copy') }}</NButton>
      </div>
      <NForm label-placement="left" label-width="160">
        <NFormItem :label="t('oidc.issuer')"><NInput v-model:value="form.issuer" placeholder="https://accounts.google.com" /></NFormItem>
        <NFormItem :label="t('oidc.clientId')"><NInput v-model:value="form.client_id" /></NFormItem>
        <NFormItem :label="t('oidc.clientSecret')"><NInput v-model:value="form.client_secret" type="password" show-password-on="click" :placeholder="v.configured ? t('oidc.secretKept') : ''" /></NFormItem>
        <NFormItem :label="t('oidc.domains')"><NDynamicTags v-model:value="form.email_domains" /></NFormItem>
        <NFormItem :label="t('oidc.enabled')"><NSwitch v-model:value="form.enabled" /></NFormItem>
      </NForm>
      <NSpace justify="end">
        <NButton v-if="v.configured" quaternary type="error" :loading="busy" @click="remove">{{ t('oidc.remove') }}</NButton>
        <NButton type="primary" :loading="busy" :disabled="!form.issuer || !form.client_id || !form.email_domains.length" @click="save">{{ t('common.save') }}</NButton>
      </NSpace>
    </template>
  </NCard>
</template>

<style scoped>
.redirect { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; font-size: 13px; margin-bottom: 14px; }
.redirect code { background: rgba(128,128,128,.12); padding: 2px 6px; border-radius: 4px; }
</style>
