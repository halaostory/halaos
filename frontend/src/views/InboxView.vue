<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { NCard, NSpace, NTag, NButton, NSpin, NEmpty, NList, NListItem, NThing, NAlert, useMessage } from 'naive-ui'
import { lifecycleAPI } from '../api/client'

interface InboxItem {
  kind: string
  ref_id: number
  employee_id: number
  employee_name: string
  title: string
  due_date: string
  priority: string
  created_at: string
}
interface InboxData {
  items: InboxItem[]
  counts: Record<string, number>
  total: number
  high: number
}

const { t } = useI18n()
const router = useRouter()
const message = useMessage()
const loading = ref(false)
const data = ref<InboxData>({ items: [], counts: {}, total: 0, high: 0 })
const filter = ref<string>('')
const brief = ref<{ summary: string; actions: string[]; generated_by: string } | null>(null)
const briefLoading = ref(false)

async function loadBrief() {
  briefLoading.value = true
  try {
    brief.value = ((await lifecycleAPI.inboxBrief()) as { data: typeof brief.value }).data
  } catch {
    brief.value = null
  } finally {
    briefLoading.value = false
  }
}

// Where each kind of item gets resolved.
const kindRoute: Record<string, string> = {
  leave_approval: 'approvals',
  overtime_approval: 'approvals',
  attendance_correction: 'approvals',
  expense_approval: 'expenses',
  onboarding_task: 'onboarding',
  offboarding_task: 'onboarding',
  compliance_alert: 'compliance',
  contract_milestone: 'milestones',
  provisioning_approval: 'provisioning-jobs',
  gov_filing_approval: 'gov-filings',
  gov_filing_blocked: 'gov-filings',
  gov_filing_failed: 'gov-filings',
  gov_filing_manual: 'gov-filings',
  gov_human_input: 'gov-filings',
  signature_declined: 'documents',
  device_action: 'devices',
  final_pay_approval: 'final-pay',
}

const kinds = computed(() =>
  Object.entries(data.value.counts).sort((a, b) => b[1] - a[1]),
)

const visible = computed(() => {
  if (filter.value === 'urgent') return data.value.items.filter((i) => i.priority === 'high')
  if (filter.value) return data.value.items.filter((i) => i.kind === filter.value)
  return data.value.items
})

async function load() {
  loading.value = true
  try {
    const res = (await lifecycleAPI.inbox()) as { data: InboxData }
    data.value = res.data
  } catch {
    message.error(t('common.loadFailed'))
  } finally {
    loading.value = false
  }
}

function open(item: InboxItem) {
  if (['document_expiring', 'self_onboarding_review'].includes(item.kind) && item.employee_id) {
    router.push({ name: 'employee-detail', params: { id: String(item.employee_id) } })
    return
  }
  const name = kindRoute[item.kind]
  if (name) router.push({ name })
}

onMounted(() => { load(); loadBrief() })
</script>

<template>
  <NSpace vertical :size="16">
    <div>
      <h2 style="margin: 0">{{ t('lifecycle.inbox') }}</h2>
      <div style="opacity: 0.7">{{ t('lifecycle.inboxSubtitle') }}</div>
    </div>

    <NSpin :show="briefLoading">
      <NAlert v-if="brief" type="info" :title="brief.generated_by === 'ai' ? '✨ ' + t('lifecycle.aiBrief') : t('lifecycle.brief')">
        <div>{{ brief.summary }}</div>
        <ol v-if="brief.actions.length" style="margin: 6px 0 0; padding-left: 18px">
          <li v-for="(a, i) in brief.actions" :key="i">{{ a }}</li>
        </ol>
      </NAlert>
    </NSpin>

    <NSpace>
      <NButton size="small" :type="filter === '' ? 'primary' : 'default'" @click="filter = ''">
        {{ t('lifecycle.all') }} · {{ data.total }}
      </NButton>
      <NButton v-if="data.high" size="small" :type="filter === 'urgent' ? 'error' : 'default'" @click="filter = 'urgent'">
        {{ t('lifecycle.urgent') }} · {{ data.high }}
      </NButton>
      <NButton
        v-for="[kind, count] in kinds"
        :key="kind"
        size="small"
        :type="filter === kind ? 'primary' : 'default'"
        @click="filter = kind"
      >
        {{ t('lifecycle.kind_' + kind) }} · {{ count }}
      </NButton>
    </NSpace>

    <NSpin :show="loading">
      <NCard>
        <NEmpty v-if="!loading && visible.length === 0" :description="t('lifecycle.inboxEmpty')" />
        <NList v-else hoverable clickable>
          <NListItem v-for="item in visible" :key="item.kind + item.ref_id + item.title + item.employee_id" @click="open(item)">
            <NThing>
              <template #header>
                <NSpace align="center" :size="8">
                  <NTag size="small" :type="item.priority === 'high' ? 'error' : 'default'">
                    {{ t('lifecycle.kind_' + item.kind) }}
                  </NTag>
                  <span>{{ item.title }}</span>
                </NSpace>
              </template>
              <template #header-extra>
                <span v-if="item.due_date" :style="{ color: item.priority === 'high' ? 'var(--n-text-color-error, #d03050)' : undefined }">
                  {{ t('lifecycle.due') }} {{ item.due_date }}
                </span>
              </template>
              <template #description>
                <span v-if="item.employee_name">{{ item.employee_name }}</span>
              </template>
            </NThing>
          </NListItem>
        </NList>
      </NCard>
    </NSpin>
  </NSpace>
</template>
