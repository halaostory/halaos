<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NCard, NButton, NModal, NForm, NFormItem, NInput, NSelect,
  NSwitch, NSpace, NEmpty, NGrid, NGi, NStatistic, NTag,
  NAvatar, useMessage, NTabs, NTabPane,
} from 'naive-ui'
import { recognitionAPI, employeeAPI } from '../api/client'

const { t } = useI18n()
const message = useMessage()

const loading = ref(false)
const wall = ref<any[]>([])
const myRecs = ref<any[]>([])
const stats = ref<any>(null)
const showSendModal = ref(false)
const activeTab = ref('wall')

// Employee search for recipient
const employees = ref<any[]>([])
const employeeOptions = computed(() =>
  employees.value.map(e => ({
    label: `${e.first_name} ${e.last_name} (${e.employee_no})`,
    value: e.id,
  }))
)

const form = ref({
  to_employee_id: null as number | null,
  category: 'kudos',
  message: '',
  is_public: true,
})

const categoryOptions = [
  { label: t('recognition.kudos'), value: 'kudos' },
  { label: t('recognition.teamwork'), value: 'teamwork' },
  { label: t('recognition.innovation'), value: 'innovation' },
  { label: t('recognition.leadership'), value: 'leadership' },
  { label: t('recognition.aboveAndBeyond'), value: 'above_and_beyond' },
  { label: t('recognition.customerFocus'), value: 'customer_focus' },
]

const categoryEmoji: Record<string, string> = {
  kudos: '👏',
  teamwork: '🤝',
  innovation: '💡',
  leadership: '🌟',
  above_and_beyond: '🚀',
  customer_focus: '💎',
}

async function loadData() {
  loading.value = true
  try {
    const [wallRes, myRes, statsRes, empRes] = await Promise.allSettled([
      recognitionAPI.list(),
      recognitionAPI.listMy(),
      recognitionAPI.getStats(),
      employeeAPI.list(),
    ])

    if (wallRes.status === 'fulfilled') {
      const data = (wallRes.value as any)?.data ?? wallRes.value
      wall.value = Array.isArray(data) ? data : []
    }
    if (myRes.status === 'fulfilled') {
      const data = (myRes.value as any)?.data ?? myRes.value
      myRecs.value = Array.isArray(data) ? data : []
    }
    if (statsRes.status === 'fulfilled') {
      stats.value = (statsRes.value as any)?.data ?? statsRes.value
    }
    if (empRes.status === 'fulfilled') {
      const data = (empRes.value as any)?.data ?? empRes.value
      employees.value = Array.isArray(data) ? data : (data?.employees || [])
    }
  } catch {
    message.error('Failed to load data')
  } finally {
    loading.value = false
  }
}

function openSend() {
  form.value = { to_employee_id: null, category: 'kudos', message: '', is_public: true }
  showSendModal.value = true
}

async function sendRecognition() {
  if (!form.value.to_employee_id) {
    message.warning(t('recognition.selectRecipient'))
    return
  }
  if (!form.value.message.trim()) {
    message.warning(t('recognition.enterMessage'))
    return
  }

  try {
    await recognitionAPI.send({
      to_employee_id: form.value.to_employee_id,
      category: form.value.category,
      message: form.value.message,
      is_public: form.value.is_public,
      points: 1,
    })
    message.success(t('recognition.sent'))
    showSendModal.value = false
    loadData()
  } catch {
    message.error('Failed to send recognition')
  }
}

function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 60) return `${mins}m`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}h`
  const days = Math.floor(hours / 24)
  return `${days}d`
}

function initials(first: string, last: string): string {
  return `${(first || '')[0] || ''}${(last || '')[0] || ''}`.toUpperCase()
}

onMounted(loadData)
</script>

<template>
  <div style="max-width: 1200px; margin: 0 auto;">
    <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 24px;">
      <div>
        <h2 style="margin: 0;">{{ t('recognition.title') }}</h2>
        <p style="margin: 4px 0 0; opacity: 0.7;">{{ t('recognition.subtitle') }}</p>
      </div>
      <NButton type="primary" @click="openSend">
        {{ t('recognition.sendKudos') }}
      </NButton>
    </div>

    <!-- Stats -->
    <NGrid :cols="4" :x-gap="16" :y-gap="16" responsive="screen" :item-responsive="true" style="margin-bottom: 24px;">
      <NGi span="4 m:2 l:1">
        <NCard>
          <NStatistic :label="t('recognition.totalThisMonth')" :value="stats?.stats?.total_recognitions ?? 0" />
        </NCard>
      </NGi>
      <NGi span="4 m:2 l:1">
        <NCard>
          <NStatistic :label="t('recognition.uniqueGivers')" :value="stats?.stats?.unique_givers ?? 0" />
        </NCard>
      </NGi>
      <NGi span="4 m:2 l:1">
        <NCard>
          <NStatistic :label="t('recognition.uniqueReceivers')" :value="stats?.stats?.unique_receivers ?? 0" />
        </NCard>
      </NGi>
      <NGi span="4 m:2 l:1">
        <NCard>
          <div style="font-size: 13px; opacity: 0.7; margin-bottom: 4px;">{{ t('recognition.topStar') }}</div>
          <div v-if="stats?.top_recognized?.length" style="font-weight: 600;">
            {{ stats.top_recognized[0].first_name }} {{ stats.top_recognized[0].last_name }}
            <NTag type="warning" size="small" :bordered="false" style="margin-left: 4px;">
              {{ stats.top_recognized[0].total_points }} pts
            </NTag>
          </div>
          <div v-else style="opacity: 0.5;">-</div>
        </NCard>
      </NGi>
    </NGrid>

    <!-- Tabs -->
    <NTabs v-model:value="activeTab" type="line">
      <NTabPane name="wall" :tab="t('recognition.wall')">
        <NEmpty v-if="wall.length === 0 && !loading" :description="t('recognition.noRecognitions')" style="padding: 40px 0;" />
        <div v-else style="display: flex; flex-direction: column; gap: 12px; padding-top: 16px;">
          <NCard v-for="r in wall" :key="r.id" size="small">
            <div style="display: flex; gap: 12px; align-items: flex-start;">
              <NAvatar :size="36" round style="flex-shrink: 0; background: var(--n-color-target);">
                {{ initials(r.from_first_name, r.from_last_name) }}
              </NAvatar>
              <div style="flex: 1;">
                <div style="font-size: 13px;">
                  <strong>{{ r.from_first_name }} {{ r.from_last_name }}</strong>
                  <span style="opacity: 0.5;"> → </span>
                  <strong>{{ r.to_first_name }} {{ r.to_last_name }}</strong>
                  <NTag v-if="r.to_department" size="tiny" :bordered="false" style="margin-left: 4px;">
                    {{ r.to_department }}
                  </NTag>
                  <span style="float: right; opacity: 0.4; font-size: 12px;">{{ timeAgo(r.created_at) }}</span>
                </div>
                <div style="margin-top: 6px;">
                  <span style="margin-right: 4px;">{{ categoryEmoji[r.category] || '👏' }}</span>
                  {{ r.message }}
                </div>
                <NTag :type="r.category === 'innovation' ? 'info' : r.category === 'leadership' ? 'warning' : 'success'" size="small" :bordered="false" style="margin-top: 6px;">
                  {{ t('recognition.' + r.category) || r.category }}
                </NTag>
              </div>
            </div>
          </NCard>
        </div>
      </NTabPane>

      <NTabPane name="my" :tab="t('recognition.myRecognitions')">
        <NEmpty v-if="myRecs.length === 0" :description="t('recognition.noRecognitions')" style="padding: 40px 0;" />
        <div v-else style="display: flex; flex-direction: column; gap: 12px; padding-top: 16px;">
          <NCard v-for="r in myRecs" :key="r.id" size="small">
            <div style="display: flex; gap: 12px; align-items: flex-start;">
              <NAvatar :size="36" round style="flex-shrink: 0;">
                {{ initials(r.from_first_name, r.from_last_name) }}
              </NAvatar>
              <div style="flex: 1;">
                <div style="font-size: 13px;">
                  <strong>{{ r.from_first_name }} {{ r.from_last_name }}</strong>
                  <span style="opacity: 0.5;"> → </span>
                  <strong>{{ r.to_first_name }} {{ r.to_last_name }}</strong>
                  <span style="float: right; opacity: 0.4; font-size: 12px;">{{ timeAgo(r.created_at) }}</span>
                </div>
                <div style="margin-top: 6px;">
                  <span style="margin-right: 4px;">{{ categoryEmoji[r.category] || '👏' }}</span>
                  {{ r.message }}
                </div>
              </div>
            </div>
          </NCard>
        </div>
      </NTabPane>

      <NTabPane name="leaderboard" :tab="t('recognition.leaderboard')">
        <div v-if="stats?.top_recognized?.length" style="padding-top: 16px;">
          <NCard v-for="(emp, idx) in stats.top_recognized" :key="emp.employee_id" size="small" style="margin-bottom: 8px;">
            <div style="display: flex; align-items: center; gap: 12px;">
              <div style="width: 28px; text-align: center; font-weight: 700; font-size: 18px; opacity: 0.4;">
                {{ Number(idx) + 1 }}
              </div>
              <NAvatar :size="36" round>{{ initials(emp.first_name, emp.last_name) }}</NAvatar>
              <div style="flex: 1;">
                <strong>{{ emp.first_name }} {{ emp.last_name }}</strong>
                <div style="font-size: 12px; opacity: 0.6;">{{ emp.department || '' }}</div>
              </div>
              <div style="text-align: right;">
                <NTag type="warning" :bordered="false">{{ emp.total_points }} pts</NTag>
                <div style="font-size: 11px; opacity: 0.5; margin-top: 2px;">{{ emp.recognition_count }} {{ t('recognition.times') }}</div>
              </div>
            </div>
          </NCard>
        </div>
        <NEmpty v-else :description="t('recognition.noRecognitions')" style="padding: 40px 0;" />
      </NTabPane>
    </NTabs>

    <!-- Send Modal -->
    <NModal
      v-model:show="showSendModal"
      preset="card"
      :title="t('recognition.sendKudos')"
      style="max-width: 500px;"
    >
      <NForm label-placement="left" label-width="100">
        <NFormItem :label="t('recognition.to')">
          <NSelect
            v-model:value="form.to_employee_id"
            :options="employeeOptions"
            filterable
            :placeholder="t('recognition.selectRecipient')"
          />
        </NFormItem>
        <NFormItem :label="t('recognition.category')">
          <NSelect v-model:value="form.category" :options="categoryOptions" />
        </NFormItem>
        <NFormItem :label="t('recognition.message')">
          <NInput
            v-model:value="form.message"
            type="textarea"
            :rows="3"
            :placeholder="t('recognition.messagePlaceholder')"
          />
        </NFormItem>
        <NFormItem :label="t('recognition.public')">
          <NSwitch v-model:value="form.is_public" />
        </NFormItem>
      </NForm>

      <template #action>
        <NSpace justify="end">
          <NButton @click="showSendModal = false">{{ t('recognition.cancel') }}</NButton>
          <NButton type="primary" @click="sendRecognition">{{ t('recognition.send') }}</NButton>
        </NSpace>
      </template>
    </NModal>
  </div>
</template>
