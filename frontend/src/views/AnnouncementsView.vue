<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NCard, NButton, NSpace, NTag, NModal, NForm, NFormItem,
  NInput, NSelect, NDatePicker, useMessage, NTime, NPopconfirm,
} from 'naive-ui'
import { announcementAPI } from '../api/client'
import EmptyState from '../components/EmptyState.vue'
import { useAuthStore } from '../stores/auth'

const { t } = useI18n()
const message = useMessage()
const auth = useAuthStore()

interface Announcement {
  id: number
  title: string
  content: string
  priority: string
  published_at: string | null
  expires_at: string | null
  created_at: string
  author_first_name: string
  author_last_name: string
  target_roles?: string[]
  target_departments?: number[]
}

const announcements = ref<Announcement[]>([])
const showAll = ref(false)
const showCreateModal = ref(false)
const creating = ref(false)

const form = ref({
  title: '',
  content: '',
  priority: 'normal',
  expires_at: null as number | null,
})

const priorityOptions = computed(() => [
  { label: t('announcement.normal'), value: 'normal' },
  { label: t('announcement.important'), value: 'important' },
  { label: t('announcement.urgent'), value: 'urgent' },
])

const priorityColor: Record<string, 'default' | 'warning' | 'error'> = {
  normal: 'default',
  important: 'warning',
  urgent: 'error',
}

async function loadAnnouncements() {
  try {
    const res = showAll.value
      ? await announcementAPI.listAll()
      : await announcementAPI.list()
    const data = (res as any)?.data ?? res
    announcements.value = Array.isArray(data) ? data : []
  } catch { message.error(t('announcement.loadFailed')) }
}

onMounted(loadAnnouncements)

async function handleCreate() {
  if (!form.value.title.trim() || !form.value.content.trim()) return
  creating.value = true
  try {
    const payload: Record<string, unknown> = {
      title: form.value.title,
      content: form.value.content,
      priority: form.value.priority,
    }
    if (form.value.expires_at) {
      payload.expires_at = new Date(form.value.expires_at).toISOString()
    }
    await announcementAPI.create(payload)
    message.success(t('announcement.created'))
    showCreateModal.value = false
    form.value = { title: '', content: '', priority: 'normal', expires_at: null }
    await loadAnnouncements()
  } catch {
    message.error(t('announcement.createFailed'))
  } finally {
    creating.value = false
  }
}

async function handleDelete(id: number) {
  try {
    await announcementAPI.delete(id)
    message.success(t('announcement.deleted'))
    await loadAnnouncements()
  } catch {
    message.error(t('announcement.deleteFailed'))
  }
}

function toggleShowAll() {
  showAll.value = !showAll.value
  loadAnnouncements()
}
</script>

<template>
  <div>
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px;">
      <h2>{{ t('announcement.title') }}</h2>
      <NSpace v-if="auth.isAdmin">
        <NButton size="small" @click="toggleShowAll">
          {{ showAll ? t('announcement.title') : t('announcement.manageAll') }}
        </NButton>
        <NButton type="primary" @click="showCreateModal = true">
          {{ t('announcement.create') }}
        </NButton>
      </NSpace>
    </div>

    <EmptyState
      v-if="announcements.length === 0"
      icon="📢"
      :title="t('emptyState.announcements.title')"
      :description="t('emptyState.announcements.desc')"
    />

    <div style="display: flex; flex-direction: column; gap: 16px;">
      <NCard v-for="ann in announcements" :key="ann.id">
        <template #header>
          <div style="display: flex; align-items: center; gap: 10px;">
            <NTag :type="priorityColor[ann.priority] || 'default'" size="small">
              {{ t(`announcement.${ann.priority}`) }}
            </NTag>
            <span style="font-size: 16px; font-weight: 600;">{{ ann.title }}</span>
          </div>
        </template>
        <template #header-extra>
          <NPopconfirm v-if="auth.isAdmin" @positive-click="handleDelete(ann.id)">
            <template #trigger>
              <NButton text type="error" size="small">{{ t('common.delete') }}</NButton>
            </template>
            Are you sure?
          </NPopconfirm>
        </template>
        <div style="white-space: pre-wrap; line-height: 1.6;">{{ ann.content }}</div>
        <template #footer>
          <NSpace :size="12" align="center" style="font-size: 12px; color: #999;">
            <span>{{ t('announcement.postedBy') }} {{ ann.author_first_name }} {{ ann.author_last_name }}</span>
            <NTime v-if="ann.published_at" :time="new Date(ann.published_at)" type="relative" />
            <NTime v-else :time="new Date(ann.created_at)" type="relative" />
            <NTag v-if="ann.expires_at" size="tiny" type="warning">
              Expires <NTime :time="new Date(ann.expires_at)" type="relative" />
            </NTag>
          </NSpace>
        </template>
      </NCard>
    </div>

    <!-- Create Modal -->
    <NModal v-model:show="showCreateModal" preset="card" :title="t('announcement.create')" style="max-width: 560px; width: 95vw;">
      <NForm @submit.prevent="handleCreate" label-placement="top">
        <NFormItem :label="t('announcement.titleLabel')">
          <NInput v-model:value="form.title" />
        </NFormItem>
        <NFormItem :label="t('announcement.content')">
          <NInput v-model:value="form.content" type="textarea" :rows="5" />
        </NFormItem>
        <NFormItem :label="t('announcement.priority')">
          <NSelect v-model:value="form.priority" :options="priorityOptions" />
        </NFormItem>
        <NFormItem :label="t('announcement.expiresAt')">
          <NDatePicker v-model:value="form.expires_at" type="datetime" clearable :placeholder="t('announcement.noExpiry')" style="width: 100%;" />
        </NFormItem>
        <NButton type="primary" :loading="creating" attr-type="submit" :disabled="!form.title.trim() || !form.content.trim()">
          {{ t('announcement.create') }}
        </NButton>
      </NForm>
    </NModal>
  </div>
</template>
