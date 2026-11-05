<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { NCard, NButton, NInput, NTag, NAlert, useMessage } from 'naive-ui'
import { authAPI } from '../api/client'

const { t } = useI18n()
const message = useMessage()
const enabled = ref(false)
const left = ref(0)
const setup = ref<{ secret: string; otpauth_url: string } | null>(null)
const code = ref('')
const recovery = ref<string[]>([])
const disabling = ref(false)
const busy = ref(false)

async function load() {
  try {
    const d = ((await authAPI.mfaStatus()) as { data: { enabled: boolean; recovery_codes_left: number } }).data
    enabled.value = d.enabled
    left.value = d.recovery_codes_left
  } catch { /* ignore */ }
}
function errMsg(e: unknown) {
  const err = e as { data?: { error?: { message?: string } }; message?: string }
  return err.data?.error?.message || err.message || ''
}
async function start() {
  busy.value = true
  try { setup.value = ((await authAPI.mfaSetup()) as { data: { secret: string; otpauth_url: string } }).data } catch (e) { message.error(errMsg(e)) } finally { busy.value = false }
}
async function confirm() {
  busy.value = true
  try {
    const d = ((await authAPI.mfaEnable(code.value)) as { data: { recovery_codes: string[] } }).data
    recovery.value = d.recovery_codes
    setup.value = null
    code.value = ''
    message.success(t('mfa.enabled'))
    await load()
  } catch (e) { message.error(errMsg(e)) } finally { busy.value = false }
}
async function disable() {
  busy.value = true
  try {
    await authAPI.mfaDisable(code.value)
    code.value = ''
    disabling.value = false
    recovery.value = []
    message.success(t('mfa.disabled'))
    await load()
  } catch (e) { message.error(errMsg(e)) } finally { busy.value = false }
}
async function copy(text: string) {
  await navigator.clipboard.writeText(text)
  message.success(t('mfa.copied'))
}
onMounted(load)
</script>

<template>
  <NCard :title="t('mfa.title')">
    <template #header-extra>
      <NTag round :bordered="false" :type="enabled ? 'success' : 'default'">{{ enabled ? t('mfa.on') : t('mfa.off') }}</NTag>
    </template>
    <p class="muted">{{ t('mfa.desc') }}</p>

    <NAlert v-if="recovery.length" type="warning" :title="t('mfa.recoveryTitle')" style="margin-bottom: 12px">
      {{ t('mfa.recoveryHelp') }}
      <div class="codes">
        <code v-for="c in recovery" :key="c">{{ c }}</code>
      </div>
      <NButton size="small" @click="copy(recovery.join('\n'))">{{ t('notices.copy') }}</NButton>
    </NAlert>

    <template v-if="!enabled && !setup">
      <NButton type="primary" :loading="busy" data-testid="mfa-start" @click="start">{{ t('mfa.turnOn') }}</NButton>
    </template>

    <template v-if="setup">
      <p>{{ t('mfa.scan') }}</p>
      <div class="key">
        <span class="muted small">{{ t('mfa.key') }}</span>
        <code data-testid="mfa-secret">{{ setup.secret.match(/.{1,4}/g)?.join(' ') }}</code>
        <NButton size="tiny" @click="copy(setup.secret)">{{ t('notices.copy') }}</NButton>
        <a :href="setup.otpauth_url" class="small">{{ t('mfa.openApp') }}</a>
      </div>
      <p>{{ t('mfa.confirm') }}</p>
      <div class="row">
        <NInput v-model:value="code" :placeholder="t('mfa.codePh')" style="max-width: 160px" :input-props="{ inputmode: 'numeric', autocomplete: 'one-time-code' }" data-testid="mfa-enable-code" @keyup.enter="confirm" />
        <NButton type="primary" :loading="busy" :disabled="code.length < 6" data-testid="mfa-enable" @click="confirm">{{ t('mfa.turnOn') }}</NButton>
        <NButton quaternary @click="setup = null">{{ t('common.cancel') }}</NButton>
      </div>
    </template>

    <template v-if="enabled">
      <div class="muted small" style="margin-bottom: 8px">{{ t('mfa.codesLeft', { n: left }) }}</div>
      <NButton v-if="!disabling" @click="disabling = true">{{ t('mfa.turnOff') }}</NButton>
      <template v-else>
        <p>{{ t('mfa.disableHelp') }}</p>
        <div class="row">
          <NInput v-model:value="code" :placeholder="t('mfa.codePh')" style="max-width: 180px" @keyup.enter="disable" />
          <NButton type="error" :loading="busy" :disabled="!code" @click="disable">{{ t('mfa.turnOff') }}</NButton>
          <NButton quaternary @click="disabling = false">{{ t('common.cancel') }}</NButton>
        </div>
      </template>
    </template>
  </NCard>
</template>

<style scoped>
.muted { opacity: .7; }
.small { font-size: 12px; }
.key { display: flex; gap: 10px; align-items: center; flex-wrap: wrap; margin-bottom: 12px; }
.key code { font-size: 15px; letter-spacing: .05em; padding: 4px 8px; border-radius: 6px; background: rgba(128,128,128,.12); }
.row { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
.codes { display: grid; grid-template-columns: repeat(4, max-content); gap: 6px 16px; margin: 8px 0; }
.codes code { font-size: 14px; }
</style>
