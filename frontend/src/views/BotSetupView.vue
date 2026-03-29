<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NCard, NButton, NInput, NSwitch, NSpace, NAlert, NTag, NSteps, NStep,
  NFormItem, NForm, NDivider, NCollapse, NCollapseItem, useMessage,
} from 'naive-ui'
import { useAuthStore } from '../stores/auth'
import { botAPI } from '../api/client'

const { t } = useI18n()
const message = useMessage()
const auth = useAuthStore()

const isAdmin = computed(() => auth.isAdmin)

// ── Admin Wizard State ──
const currentStep = ref(1)
const tokenInput = ref('')
const usernameInput = ref('')
const isActive = ref(false)
const testing = ref(false)
const testResult = ref<{ ok: boolean; bot_username?: string; bot_name?: string; error?: string } | null>(null)
const saving = ref(false)

// Load existing config if available
const existingLoaded = ref(false)
onMounted(async () => {
  if (isAdmin.value) {
    await loadExistingConfig()
  }
  await loadBotInfo()
  await loadLinkStatus()
})

async function loadExistingConfig() {
  try {
    const res = await botAPI.listBotConfigs() as { data?: Array<{ bot_token?: string; bot_username?: string; is_active?: boolean }> }
    const configs = (res as any)?.data ?? res
    if (Array.isArray(configs) && configs.length > 0) {
      const cfg = configs[0]
      // Token is masked from API, don't populate
      usernameInput.value = cfg.bot_username || ''
      isActive.value = !!cfg.is_active
      existingLoaded.value = true
    }
  } catch {
    // No existing config
  }
}

async function handleTestToken() {
  if (!tokenInput.value.trim()) {
    message.warning(t('common.fillAllFields'))
    return
  }
  testing.value = true
  testResult.value = null
  try {
    const res = await botAPI.testBotToken(tokenInput.value.trim()) as { data?: { ok: boolean; bot_username: string; bot_name: string } }
    const data = (res as any)?.data ?? res
    testResult.value = { ok: true, bot_username: data.bot_username, bot_name: data.bot_name }
    // Auto-fill username from test result
    if (data.bot_username) {
      usernameInput.value = data.bot_username
    }
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } }
    testResult.value = { ok: false, error: err.data?.error?.message || t('botSetup.testFailed') }
  } finally {
    testing.value = false
  }
}

async function handleSaveConfig() {
  if (!tokenInput.value.trim() && !existingLoaded.value) {
    message.warning(t('common.fillAllFields'))
    return
  }
  saving.value = true
  try {
    const payload: Record<string, unknown> = {
      platform: 'telegram',
      bot_username: usernameInput.value.replace(/^@/, ''),
      is_active: isActive.value,
    }
    // Only send token if user entered a new one
    if (tokenInput.value.trim()) {
      payload.bot_token = tokenInput.value.trim()
    }
    await botAPI.saveBotConfig(payload)
    message.success(t('botSetup.configSaved'))
  } catch {
    message.error(t('botSetup.configSaveFailed'))
  } finally {
    saving.value = false
  }
}

// ── Employee Flow State ──
const botInfo = ref<{ bot_username: string; is_active: boolean; is_shared?: boolean }>({ bot_username: '', is_active: false })
const botLinked = ref(false)
const linkCode = ref('')
const linkLoading = ref(false)

async function loadBotInfo() {
  try {
    const res = await botAPI.getBotInfo() as { data?: { bot_username: string; is_active: boolean; is_shared?: boolean } }
    const data = (res as any)?.data ?? res
    botInfo.value = { bot_username: data.bot_username || '', is_active: !!data.is_active, is_shared: !!data.is_shared }
  } catch {
    // No bot info
  }
}

async function loadLinkStatus() {
  try {
    const res = await botAPI.getLinkStatus() as { data?: { linked: boolean } }
    const data = (res as any)?.data ?? res
    botLinked.value = !!data.linked
  } catch {
    // Not linked
  }
}

async function handleGenerateCode() {
  linkLoading.value = true
  try {
    const res = await botAPI.getLinkCode() as { data?: { code: string } }
    const data = (res as any)?.data ?? res
    linkCode.value = data.code || ''
  } catch {
    message.error(t('common.failed'))
  } finally {
    linkLoading.value = false
  }
}

async function handleUnlink() {
  linkLoading.value = true
  try {
    await botAPI.unlinkPlatform('telegram')
    botLinked.value = false
    linkCode.value = ''
    message.success(t('profile.telegramUnlinked'))
  } catch {
    message.error(t('common.failed'))
  } finally {
    linkLoading.value = false
  }
}

const deeplinkUrl = computed(() => {
  const username = botInfo.value.bot_username.replace(/^@/, '')
  if (!username || !linkCode.value) return ''
  return `https://t.me/${username}?start=${linkCode.value}`
})

const botHasConfig = computed(() => !!botInfo.value.bot_username)
const hasCustomBot = computed(() => existingLoaded.value && !botInfo.value.is_shared)
</script>

<template>
  <NSpace vertical :size="16">
    <h2>{{ t('botSetup.title') }}</h2>
    <p style="color: var(--n-text-color3);">{{ t('botSetup.subtitle') }}</p>

    <!-- ══════════════ SHARED BOT INFO (everyone sees this) ══════════════ -->
    <NCard v-if="botHasConfig && botInfo.is_shared" :title="t('botSetup.employeeTitle')">
      <NSpace vertical :size="16">
        <NAlert type="info">{{ t('botSetup.sharedBotExplanation') }}</NAlert>

        <template v-if="botLinked">
          <NSpace align="center" :size="12">
            <NTag type="success" size="small">{{ t('profile.telegramConnected') }}</NTag>
            <span>{{ t('botSetup.alreadyLinked') }}</span>
            <NButton size="small" type="error" quaternary :loading="linkLoading" @click="handleUnlink">
              {{ t('profile.telegramDisconnect') }}
            </NButton>
          </NSpace>
        </template>

        <template v-else>
          <p>{{ t('botSetup.botAvailable') }} <NTag type="info">{{ botInfo.bot_username }}</NTag></p>

          <template v-if="linkCode">
            <div>
              <p style="margin-bottom: 8px; font-weight: 500;">{{ t('botSetup.linkCodeLabel') }}</p>
              <div style="font-size: 28px; font-weight: 700; letter-spacing: 4px; font-family: monospace; padding: 12px 0;">
                {{ linkCode }}
              </div>
            </div>
            <p>{{ t('botSetup.linkCodeInstructions') }}</p>
            <NSpace>
              <NButton tag="a" :href="deeplinkUrl" target="_blank" type="primary" :disabled="!deeplinkUrl">
                {{ t('botSetup.openInTelegram') }}
              </NButton>
            </NSpace>
            <p style="font-size: 12px; color: var(--n-text-color3);">{{ t('botSetup.codeExpiry') }}</p>
          </template>

          <NButton v-else type="primary" :loading="linkLoading" @click="handleGenerateCode">
            {{ t('botSetup.generateCode') }}
          </NButton>
        </template>
      </NSpace>
    </NCard>

    <!-- ══════════════ CUSTOM BOT: LINK ACCOUNT (all users when bot configured) ══════════════ -->
    <template v-if="!botInfo.is_shared && botHasConfig">
      <!-- Already linked -->
      <NCard v-if="botLinked" :title="t('botSetup.employeeTitle')">
        <NSpace align="center" :size="12">
          <NTag type="success" size="small">{{ t('profile.telegramConnected') }}</NTag>
          <span>{{ t('botSetup.alreadyLinked') }}</span>
          <NButton size="small" type="error" quaternary :loading="linkLoading" @click="handleUnlink">
            {{ t('profile.telegramDisconnect') }}
          </NButton>
        </NSpace>
      </NCard>

      <!-- Bot configured, not linked -->
      <NCard v-else :title="t('botSetup.employeeTitle')">
        <NSpace vertical :size="16">
          <p>{{ t('botSetup.botAvailable') }} <NTag type="info">{{ botInfo.bot_username }}</NTag></p>

          <template v-if="linkCode">
            <div>
              <p style="margin-bottom: 8px; font-weight: 500;">{{ t('botSetup.linkCodeLabel') }}</p>
              <div style="font-size: 28px; font-weight: 700; letter-spacing: 4px; font-family: monospace; padding: 12px 0;">
                {{ linkCode }}
              </div>
            </div>
            <p>{{ t('botSetup.linkCodeInstructions') }}</p>
            <NSpace>
              <NButton tag="a" :href="deeplinkUrl" target="_blank" type="primary" :disabled="!deeplinkUrl">
                {{ t('botSetup.openInTelegram') }}
              </NButton>
            </NSpace>
            <p style="font-size: 12px; color: var(--n-text-color3);">{{ t('botSetup.codeExpiry') }}</p>
          </template>

          <NButton v-else type="primary" :loading="linkLoading" @click="handleGenerateCode">
            {{ t('botSetup.generateCode') }}
          </NButton>
        </NSpace>
      </NCard>
    </template>

    <!-- No bot configured (non-admin only) -->
    <NCard v-if="!isAdmin && !botInfo.is_shared && !botHasConfig" :title="t('botSetup.employeeTitle')">
      <NAlert type="warning">
        {{ t('botSetup.noBotConfigured') }}
      </NAlert>
    </NCard>

    <!-- ══════════════ ADMIN: CUSTOM BOT WIZARD (Advanced) ══════════════ -->
    <template v-if="isAdmin">
      <NCollapse>
        <NCollapseItem :title="t('botSetup.advancedCustomBot')" name="custom-bot">
          <template #header-extra>
            <NTag v-if="hasCustomBot" type="success" size="small">{{ t('common.active') }}</NTag>
            <NTag v-else size="small">{{ t('botSetup.optional') }}</NTag>
          </template>

          <NSteps :current="currentStep" style="margin-bottom: 24px;">
            <NStep :title="t('botSetup.step1Title')" :description="t('botSetup.step1Desc')" />
            <NStep :title="t('botSetup.step2Title')" :description="t('botSetup.step2Desc')" />
            <NStep :title="t('botSetup.step3Title')" :description="t('botSetup.step3Desc')" />
            <NStep :title="t('botSetup.step4Title')" :description="t('botSetup.step4Desc')" />
          </NSteps>

          <!-- Step 1: Create Bot -->
          <NCard v-if="currentStep === 1">
            <template #header>
              <NSpace align="center" :size="8">
                {{ t('botSetup.step1Title') }}
                <NTag size="small" type="info">Step 1 of 4</NTag>
              </NSpace>
            </template>
            <p style="margin-bottom: 16px;">{{ t('botSetup.botFatherInstructions') }}</p>
            <ol style="padding-left: 20px; line-height: 2;">
              <li>{{ t('botSetup.botFatherStep1') }}</li>
              <li>{{ t('botSetup.botFatherStep2') }}</li>
              <li>{{ t('botSetup.botFatherStep3') }}</li>
              <li>{{ t('botSetup.botFatherStep4') }}</li>
              <li>{{ t('botSetup.botFatherStep5') }}</li>
            </ol>
            <NDivider />
            <NSpace>
              <NButton tag="a" href="https://t.me/BotFather" target="_blank" type="primary">
                {{ t('botSetup.openBotFather') }}
              </NButton>
              <NButton @click="currentStep = 2">{{ t('botSetup.nextStep') }} &rarr;</NButton>
            </NSpace>
          </NCard>

          <!-- Step 2: Enter Token -->
          <NCard v-if="currentStep === 2">
            <template #header>
              <NSpace align="center" :size="8">
                {{ t('botSetup.step2Title') }}
                <NTag size="small" type="info">Step 2 of 4</NTag>
              </NSpace>
            </template>
            <NForm label-placement="left" label-width="160">
              <NFormItem :label="t('botSetup.tokenLabel')">
                <NInput
                  v-model:value="tokenInput"
                  type="password"
                  show-password-on="click"
                  :placeholder="t('botSetup.tokenPlaceholder')"
                />
              </NFormItem>
              <p style="font-size: 12px; color: var(--n-text-color3); margin: -8px 0 16px 160px;">
                {{ t('botSetup.tokenHint') }}
              </p>
              <NFormItem :label="t('botSetup.usernameLabel')">
                <NInput v-model:value="usernameInput" :placeholder="t('botSetup.usernamePlaceholder')" />
              </NFormItem>
            </NForm>
            <NDivider />
            <NSpace>
              <NButton @click="currentStep = 1">&larr; {{ t('botSetup.prevStep') }}</NButton>
              <NButton type="primary" @click="currentStep = 3" :disabled="!tokenInput.trim()">
                {{ t('botSetup.nextStep') }} &rarr;
              </NButton>
            </NSpace>
          </NCard>

          <!-- Step 3: Test Connection -->
          <NCard v-if="currentStep === 3">
            <template #header>
              <NSpace align="center" :size="8">
                {{ t('botSetup.step3Title') }}
                <NTag size="small" type="info">Step 3 of 4</NTag>
              </NSpace>
            </template>
            <NSpace vertical :size="16">
              <NButton type="primary" :loading="testing" @click="handleTestToken">
                {{ t('botSetup.testToken') }}
              </NButton>

              <NAlert v-if="testResult?.ok" type="success" :title="t('botSetup.testSuccess')">
                <p><strong>{{ t('botSetup.testBotName') }}:</strong> {{ testResult.bot_name }}</p>
                <p><strong>{{ t('botSetup.testBotUsername') }}:</strong> @{{ testResult.bot_username }}</p>
              </NAlert>
              <NAlert v-else-if="testResult && !testResult.ok" type="error" :title="t('botSetup.testFailed')">
                {{ testResult.error }}
              </NAlert>
            </NSpace>
            <NDivider />
            <NSpace>
              <NButton @click="currentStep = 2">&larr; {{ t('botSetup.prevStep') }}</NButton>
              <NButton type="primary" @click="currentStep = 4" :disabled="!testResult?.ok">
                {{ t('botSetup.nextStep') }} &rarr;
              </NButton>
            </NSpace>
          </NCard>

          <!-- Step 4: Activate -->
          <NCard v-if="currentStep === 4">
            <template #header>
              <NSpace align="center" :size="8">
                {{ t('botSetup.step4Title') }}
                <NTag size="small" type="info">Step 4 of 4</NTag>
              </NSpace>
            </template>
            <NSpace vertical :size="16">
              <NForm label-placement="left" label-width="160">
                <NFormItem :label="t('botSetup.activateLabel')">
                  <NSpace align="center" :size="8">
                    <NSwitch v-model:value="isActive" />
                    <span v-if="isActive" style="color: #18a058; font-size: 12px;">{{ t('common.active') }}</span>
                  </NSpace>
                </NFormItem>
              </NForm>

              <NAlert type="info">
                {{ t('botSetup.autoStartMessage') }}
              </NAlert>

              <NButton type="primary" :loading="saving" @click="handleSaveConfig">
                {{ t('botSetup.saveConfig') }}
              </NButton>
            </NSpace>
            <NDivider />
            <NButton @click="currentStep = 3">&larr; {{ t('botSetup.prevStep') }}</NButton>
          </NCard>
        </NCollapseItem>
      </NCollapse>
    </template>
  </NSpace>
</template>
