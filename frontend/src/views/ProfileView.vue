<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NCard, NDescriptions, NDescriptionsItem, NForm, NFormItem,
  NInput, NButton, NSpace, NTag, NAvatar, NUpload, useMessage
} from 'naive-ui'
import type { UploadFileInfo } from 'naive-ui'
import { useAuthStore } from '../stores/auth'
import { authAPI, botAPI } from '../api/client'

const { t } = useI18n()
const message = useMessage()
const auth = useAuthStore()

const editMode = ref(false)
const profileLoading = ref(false)
const pwLoading = ref(false)
const avatarUploading = ref(false)
const avatarUrl = ref<string | null>(null)

const profileForm = ref({
  first_name: '',
  last_name: '',
})

const pwForm = ref({
  current_password: '',
  new_password: '',
  confirm_password: '',
})

const apiBase = import.meta.env.VITE_API_URL || '/api'
function fullAvatarUrl(url: string | null): string | null {
  if (!url) return null
  if (url.startsWith('http')) return url
  return `${apiBase.replace(/\/api$/, '')}${url}`
}

onMounted(() => {
  if (auth.user) {
    profileForm.value.first_name = auth.user.first_name
    profileForm.value.last_name = auth.user.last_name
    avatarUrl.value = (auth.user as any).avatar_url || null
  }
  loadBotStatus()
})

async function saveProfile() {
  if (!profileForm.value.first_name || !profileForm.value.last_name) {
    message.warning(t('profile.nameRequired'))
    return
  }
  profileLoading.value = true
  try {
    const res = await authAPI.updateProfile({
      first_name: profileForm.value.first_name,
      last_name: profileForm.value.last_name,
    }) as { id: number; email: string; first_name: string; last_name: string; role: string; company_id: number }
    auth.setUser(res)
    editMode.value = false
    message.success(t('profile.updated'))
  } catch {
    message.error(t('profile.updateFailed'))
  } finally {
    profileLoading.value = false
  }
}

async function handleAvatarUpload({ file }: { file: UploadFileInfo }) {
  if (!file.file) return
  avatarUploading.value = true
  try {
    const formData = new FormData()
    formData.append('avatar', file.file)
    const res = await authAPI.uploadAvatar(formData) as { avatar_url?: string; data?: { avatar_url?: string } }
    const data = res.data || res
    if ((data as any).avatar_url) {
      avatarUrl.value = (data as any).avatar_url
    }
    message.success(t('profile.avatarUploaded'))
  } catch {
    message.error(t('profile.avatarUploadFailed'))
  } finally {
    avatarUploading.value = false
  }
}

async function changePassword() {
  if (!pwForm.value.current_password || !pwForm.value.new_password) {
    message.warning(t('profile.fillAllFields'))
    return
  }
  if (pwForm.value.new_password.length < 8) {
    message.warning(t('profile.passwordMinLength'))
    return
  }
  if (pwForm.value.new_password !== pwForm.value.confirm_password) {
    message.warning(t('profile.passwordMismatch'))
    return
  }
  pwLoading.value = true
  try {
    await authAPI.changePassword({
      current_password: pwForm.value.current_password,
      new_password: pwForm.value.new_password,
    })
    message.success(t('profile.passwordChanged'))
    pwForm.value = { current_password: '', new_password: '', confirm_password: '' }
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } }
    message.error(err.data?.error?.message || t('profile.passwordChangeFailed'))
  } finally {
    pwLoading.value = false
  }
}

// Telegram Bot Link
const botLinked = ref(false)
const botUsername = ref('')
const botLinkCode = ref('')
const botLoading = ref(false)
const companyBotUsername = ref('')

async function loadBotStatus() {
  try {
    const res = await botAPI.getLinkStatus() as { data?: { linked: boolean; platform_username?: string } }
    const data = res.data || (res as unknown as { linked: boolean; platform_username?: string })
    botLinked.value = !!data.linked
    botUsername.value = data.platform_username || ''
  } catch {
    // not linked
  }
  // Also load company bot info for deeplink
  try {
    const res = await botAPI.getBotInfo() as { data?: { bot_username: string } }
    const data = (res as any)?.data ?? res
    companyBotUsername.value = (data.bot_username || '').replace(/^@/, '')
  } catch {
    // ignore
  }
}

const deeplinkUrl = computed(() => {
  if (!companyBotUsername.value || !botLinkCode.value) return ''
  return `https://t.me/${companyBotUsername.value}?start=${botLinkCode.value}`
})

async function generateLinkCode() {
  botLoading.value = true
  try {
    const res = await botAPI.getLinkCode() as { data?: { code: string } }
    const data = res.data || (res as unknown as { code: string })
    botLinkCode.value = data.code || ''
  } catch {
    message.error(t('common.failed'))
  } finally {
    botLoading.value = false
  }
}

async function unlinkTelegram() {
  botLoading.value = true
  try {
    await botAPI.unlinkPlatform('telegram')
    botLinked.value = false
    botUsername.value = ''
    botLinkCode.value = ''
    message.success(t('profile.telegramUnlinked'))
  } catch {
    message.error(t('common.failed'))
  } finally {
    botLoading.value = false
  }
}

const roleMap: Record<string, 'success' | 'warning' | 'info' | 'default'> = {
  super_admin: 'success',
  admin: 'success',
  manager: 'info',
  employee: 'default',
}
</script>

<template>
  <NSpace vertical :size="16">
    <h2>{{ t('nav.profile') }}</h2>

    <!-- Avatar Section -->
    <NCard :title="t('profile.avatar')">
      <div style="display: flex; align-items: center; gap: 20px;">
        <NAvatar
          v-if="fullAvatarUrl(avatarUrl)"
          :src="fullAvatarUrl(avatarUrl)!"
          :size="80"
          round
          object-fit="cover"
        />
        <NAvatar v-else :size="80" round style="font-size: 28px;">
          {{ auth.user?.first_name?.charAt(0)?.toUpperCase() || 'U' }}
        </NAvatar>
        <div>
          <NUpload
            :show-file-list="false"
            :custom-request="() => {}"
            accept=".png,.jpg,.jpeg,.webp"
            @change="handleAvatarUpload"
          >
            <NButton :loading="avatarUploading" size="small">{{ t('profile.uploadAvatar') }}</NButton>
          </NUpload>
          <div style="font-size: 12px; color: #999; margin-top: 6px;">{{ t('profile.avatarHint') }}</div>
        </div>
      </div>
    </NCard>

    <NCard :title="t('profile.basicInfo')">
      <template v-if="!editMode">
        <NDescriptions label-placement="left" :column="2" bordered v-if="auth.user">
          <NDescriptionsItem :label="t('auth.firstName')">{{ auth.user.first_name }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('auth.lastName')">{{ auth.user.last_name }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('auth.email')">{{ auth.user.email }}</NDescriptionsItem>
          <NDescriptionsItem :label="t('profile.role')">
            <NTag :type="roleMap[auth.user.role] || 'default'" size="small">{{ auth.user.role }}</NTag>
          </NDescriptionsItem>
        </NDescriptions>
        <NButton style="margin-top: 12px;" @click="editMode = true">{{ t('common.edit') }}</NButton>
      </template>
      <template v-else>
        <NForm @submit.prevent="saveProfile" label-placement="left" label-width="120">
          <NFormItem :label="t('auth.firstName')" required>
            <NInput v-model:value="profileForm.first_name" />
          </NFormItem>
          <NFormItem :label="t('auth.lastName')" required>
            <NInput v-model:value="profileForm.last_name" />
          </NFormItem>
          <NSpace>
            <NButton type="primary" :loading="profileLoading" attr-type="submit">{{ t('common.save') }}</NButton>
            <NButton @click="editMode = false">{{ t('common.cancel') }}</NButton>
          </NSpace>
        </NForm>
      </template>
    </NCard>

    <!-- Telegram Bot Link -->
    <NCard :title="t('profile.telegramBot')">
      <template v-if="botLinked">
        <NSpace align="center" :size="12">
          <NTag type="success" size="small">{{ t('profile.telegramConnected') }}</NTag>
          <span>@{{ botUsername }}</span>
          <NButton size="small" type="error" quaternary :loading="botLoading" @click="unlinkTelegram">
            {{ t('profile.telegramDisconnect') }}
          </NButton>
        </NSpace>
      </template>
      <template v-else>
        <template v-if="botLinkCode">
          <p style="margin-bottom: 8px;">{{ t('profile.telegramCodeInstructions') }}</p>
          <div style="font-size: 28px; font-weight: 700; letter-spacing: 4px; font-family: monospace; padding: 12px 0;">
            {{ botLinkCode }}
          </div>
          <NSpace v-if="deeplinkUrl" :size="12" style="margin-bottom: 8px;">
            <NButton tag="a" :href="deeplinkUrl" target="_blank" type="primary" size="small">
              {{ t('botSetup.openInTelegram') }}
            </NButton>
          </NSpace>
          <p style="font-size: 12px; color: var(--n-text-color3);">{{ t('profile.telegramCodeHint') }}</p>
        </template>
        <NButton v-else type="primary" :loading="botLoading" @click="generateLinkCode">
          {{ t('profile.telegramGenerate') }}
        </NButton>
        <div style="margin-top: 12px;">
          <RouterLink to="/bot-setup" style="font-size: 13px; color: var(--n-color-target);">
            {{ t('botSetup.wizardLink') }} &rarr;
          </RouterLink>
        </div>
      </template>
    </NCard>

    <NCard :title="t('profile.changePassword')">
      <NForm @submit.prevent="changePassword" label-placement="left" label-width="160">
        <NFormItem :label="t('profile.currentPassword')" required>
          <NInput v-model:value="pwForm.current_password" type="password" show-password-on="click" />
        </NFormItem>
        <NFormItem :label="t('profile.newPassword')" required>
          <NInput v-model:value="pwForm.new_password" type="password" show-password-on="click" />
        </NFormItem>
        <NFormItem :label="t('profile.confirmPassword')" required>
          <NInput v-model:value="pwForm.confirm_password" type="password" show-password-on="click" />
        </NFormItem>
        <NButton type="primary" :loading="pwLoading" attr-type="submit">{{ t('profile.changePassword') }}</NButton>
      </NForm>
    </NCard>
  </NSpace>
</template>
