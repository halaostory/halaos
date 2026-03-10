<script setup lang="ts">
import { ref, onMounted, h } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NCard, NButton, NDataTable, NTag, NModal, NForm, NFormItem,
  NInput, NSelect, NSwitch, NSpace, NEmpty,
  NGrid, NGi, NStatistic, NIcon, useMessage, NPopconfirm,
} from 'naive-ui'
import { AddOutline, TrashOutline } from '@vicons/ionicons5'
import { pulseAPI } from '../api/client'

const { t } = useI18n()
const message = useMessage()

const loading = ref(false)
const surveys = ref<any[]>([])
const showModal = ref(false)
const editingSurvey = ref<any>(null)

const form = ref({
  title: '',
  description: '',
  frequency: 'weekly',
  is_anonymous: true,
  is_active: true,
  questions: [
    { question: '', question_type: 'rating', sort_order: 0, is_required: true },
  ] as any[],
})

const frequencyOptions = [
  { label: t('pulse.weekly'), value: 'weekly' },
  { label: t('pulse.biweekly'), value: 'biweekly' },
  { label: t('pulse.monthly'), value: 'monthly' },
  { label: t('pulse.oneTime'), value: 'one_time' },
]

const questionTypeOptions = [
  { label: t('pulse.rating'), value: 'rating' },
  { label: t('pulse.text'), value: 'text' },
  { label: t('pulse.yesNo'), value: 'yes_no' },
]

const columns = [
  { title: t('pulse.surveyTitle'), key: 'title', ellipsis: { tooltip: true } },
  {
    title: t('pulse.frequency'),
    key: 'frequency',
    width: 120,
    render: (row: any) => h(NTag, { type: 'info', size: 'small', bordered: false }, () => row.frequency),
  },
  {
    title: t('pulse.anonymous'),
    key: 'is_anonymous',
    width: 100,
    render: (row: any) => h(NTag, {
      type: row.is_anonymous ? 'success' : 'default',
      size: 'small',
      bordered: false,
    }, () => row.is_anonymous ? t('pulse.yes') : t('pulse.no')),
  },
  {
    title: t('pulse.status'),
    key: 'is_active',
    width: 100,
    render: (row: any) => h(NTag, {
      type: row.is_active ? 'success' : 'default',
      size: 'small',
      bordered: false,
    }, () => row.is_active ? t('pulse.active') : t('pulse.inactive')),
  },
  {
    title: t('pulse.created'),
    key: 'created_at',
    width: 120,
    render: (row: any) => new Date(row.created_at).toLocaleDateString(),
  },
  {
    title: t('pulse.actions'),
    key: 'actions',
    width: 200,
    render: (row: any) => h(NSpace, { size: 'small' }, () => [
      h(NButton, {
        size: 'small',
        onClick: () => openEdit(row),
      }, () => t('pulse.edit')),
      h(NButton, {
        size: 'small',
        type: 'info',
        onClick: () => viewResults(row),
      }, () => t('pulse.results')),
      row.is_active ? h(NPopconfirm, {
        onPositiveClick: () => deactivate(row.id),
      }, {
        trigger: () => h(NButton, { size: 'small', type: 'error' }, () => t('pulse.deactivate')),
        default: () => t('pulse.confirmDeactivate'),
      }) : null,
    ]),
  },
]

async function loadSurveys() {
  loading.value = true
  try {
    const res = await pulseAPI.list()
    const data = (res as any)?.data ?? res
    surveys.value = Array.isArray(data) ? data : []
  } catch {
    message.error('Failed to load surveys')
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingSurvey.value = null
  form.value = {
    title: '',
    description: '',
    frequency: 'weekly',
    is_anonymous: true,
    is_active: true,
    questions: [
      { question: '', question_type: 'rating', sort_order: 0, is_required: true },
    ],
  }
  showModal.value = true
}

async function openEdit(survey: any) {
  editingSurvey.value = survey
  try {
    const res = await pulseAPI.get(survey.id)
    const data = (res as any)?.data ?? res
    const s = data.survey || data
    const qs = data.questions || []
    form.value = {
      title: s.title,
      description: s.description || '',
      frequency: s.frequency,
      is_anonymous: s.is_anonymous,
      is_active: s.is_active,
      questions: qs.length > 0 ? qs.map((q: any) => ({
        question: q.question,
        question_type: q.question_type,
        sort_order: q.sort_order,
        is_required: q.is_required,
      })) : [{ question: '', question_type: 'rating', sort_order: 0, is_required: true }],
    }
    showModal.value = true
  } catch {
    message.error('Failed to load survey details')
  }
}

function addQuestion() {
  form.value.questions.push({
    question: '',
    question_type: 'rating',
    sort_order: form.value.questions.length,
    is_required: true,
  })
}

function removeQuestion(index: number) {
  if (form.value.questions.length > 1) {
    form.value.questions.splice(index, 1)
  }
}

async function saveSurvey() {
  const validQuestions = form.value.questions.filter((q: any) => q.question.trim())
  if (validQuestions.length === 0) {
    message.warning(t('pulse.needQuestion'))
    return
  }

  const payload = {
    ...form.value,
    questions: validQuestions.map((q: any, i: number) => ({
      ...q,
      sort_order: i,
    })),
  }

  try {
    if (editingSurvey.value) {
      await pulseAPI.update(editingSurvey.value.id, payload)
      message.success(t('pulse.updated'))
    } else {
      await pulseAPI.create(payload)
      message.success(t('pulse.created'))
    }
    showModal.value = false
    loadSurveys()
  } catch {
    message.error('Failed to save survey')
  }
}

async function deactivate(id: number) {
  try {
    await pulseAPI.deactivate(id)
    message.success(t('pulse.deactivated'))
    loadSurveys()
  } catch {
    message.error('Failed to deactivate survey')
  }
}

// Results
const showResults = ref(false)
const resultsData = ref<any>(null)
const resultsLoading = ref(false)

async function viewResults(survey: any) {
  resultsLoading.value = true
  showResults.value = true
  try {
    const res = await pulseAPI.getResults(survey.id)
    resultsData.value = (res as any)?.data ?? res
  } catch {
    message.error('Failed to load results')
  } finally {
    resultsLoading.value = false
  }
}

onMounted(loadSurveys)
</script>

<template>
  <div style="max-width: 1200px; margin: 0 auto;">
    <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 24px;">
      <div>
        <h2 style="margin: 0;">{{ t('pulse.title') }}</h2>
        <p style="margin: 4px 0 0; opacity: 0.7;">{{ t('pulse.subtitle') }}</p>
      </div>
      <NButton type="primary" @click="openCreate">
        {{ t('pulse.createSurvey') }}
      </NButton>
    </div>

    <!-- Stats -->
    <NGrid :cols="4" :x-gap="16" :y-gap="16" responsive="screen" :item-responsive="true" style="margin-bottom: 24px;">
      <NGi span="4 m:2 l:1">
        <NCard>
          <NStatistic :label="t('pulse.totalSurveys')" :value="surveys.length" />
        </NCard>
      </NGi>
      <NGi span="4 m:2 l:1">
        <NCard>
          <NStatistic :label="t('pulse.activeSurveys')" :value="surveys.filter(s => s.is_active).length" />
        </NCard>
      </NGi>
    </NGrid>

    <!-- Surveys Table -->
    <NCard>
      <NEmpty v-if="surveys.length === 0 && !loading" :description="t('pulse.noSurveys')" style="padding: 40px 0;" />
      <NDataTable
        v-else
        :columns="columns"
        :data="surveys"
        :loading="loading"
        :bordered="false"
        :single-line="false"
      />
    </NCard>

    <!-- Create/Edit Modal -->
    <NModal
      v-model:show="showModal"
      preset="card"
      :title="editingSurvey ? t('pulse.editSurvey') : t('pulse.createSurvey')"
      style="max-width: 700px;"
    >
      <NForm label-placement="left" label-width="120">
        <NFormItem :label="t('pulse.surveyTitle')">
          <NInput v-model:value="form.title" :placeholder="t('pulse.titlePlaceholder')" />
        </NFormItem>
        <NFormItem :label="t('pulse.description')">
          <NInput v-model:value="form.description" type="textarea" :rows="2" />
        </NFormItem>
        <NFormItem :label="t('pulse.frequency')">
          <NSelect v-model:value="form.frequency" :options="frequencyOptions" />
        </NFormItem>
        <NFormItem :label="t('pulse.anonymous')">
          <NSwitch v-model:value="form.is_anonymous" />
        </NFormItem>
        <NFormItem v-if="editingSurvey" :label="t('pulse.active')">
          <NSwitch v-model:value="form.is_active" />
        </NFormItem>

        <div style="margin-bottom: 8px; font-weight: 600;">{{ t('pulse.questions') }}</div>

        <div v-for="(q, idx) in form.questions" :key="idx" style="display: flex; gap: 8px; margin-bottom: 8px; align-items: flex-start;">
          <NInput v-model:value="q.question" :placeholder="t('pulse.questionPlaceholder')" style="flex: 1;" />
          <NSelect v-model:value="q.question_type" :options="questionTypeOptions" style="width: 120px;" />
          <NSwitch v-model:value="q.is_required" size="small" style="margin-top: 6px;" />
          <NButton
            v-if="form.questions.length > 1"
            size="small"
            type="error"
            quaternary
            @click="removeQuestion(idx)"
          >
            <template #icon><NIcon :component="TrashOutline" /></template>
          </NButton>
        </div>

        <NButton dashed block @click="addQuestion" style="margin-bottom: 16px;">
          <template #icon><NIcon :component="AddOutline" /></template>
          {{ t('pulse.addQuestion') }}
        </NButton>
      </NForm>

      <template #action>
        <NSpace justify="end">
          <NButton @click="showModal = false">{{ t('pulse.cancel') }}</NButton>
          <NButton type="primary" @click="saveSurvey">{{ t('pulse.save') }}</NButton>
        </NSpace>
      </template>
    </NModal>

    <!-- Results Modal -->
    <NModal
      v-model:show="showResults"
      preset="card"
      :title="t('pulse.surveyResults')"
      style="max-width: 800px;"
    >
      <div v-if="resultsLoading" style="padding: 40px; text-align: center;">
        {{ t('pulse.loading') }}
      </div>
      <div v-else-if="resultsData">
        <div v-if="!resultsData.results?.length">
          <NEmpty :description="t('pulse.noResults')" />
        </div>
        <div v-for="round in resultsData.results" :key="round.round_id" style="margin-bottom: 24px;">
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px;">
            <strong>{{ round.round_date }}</strong>
            <NSpace size="small">
              <NTag :type="round.status === 'open' ? 'success' : 'default'" size="small">
                {{ round.status }}
              </NTag>
              <NTag type="info" size="small" :bordered="false">
                {{ round.total_responded }}/{{ round.total_sent }} {{ t('pulse.responded') }}
              </NTag>
            </NSpace>
          </div>
          <NCard v-for="q in round.questions" :key="q.question_id" size="small" style="margin-bottom: 8px;">
            <div style="font-weight: 500; margin-bottom: 4px;">{{ q.question }}</div>
            <div v-if="q.question_type === 'rating'">
              <NTag type="warning" size="small" :bordered="false">
                {{ t('pulse.avgRating') }}: {{ q.avg_rating ? q.avg_rating.toFixed(1) : 'N/A' }}
              </NTag>
              <span style="margin-left: 8px; opacity: 0.6; font-size: 12px;">
                ({{ q.response_count }} {{ t('pulse.responses') }})
              </span>
            </div>
            <div v-else>
              <div v-if="q.answers?.length" style="font-size: 13px;">
                <div v-for="(a, i) in q.answers" :key="i" style="padding: 4px 0; border-bottom: 1px solid var(--n-border-color);">
                  {{ a }}
                </div>
              </div>
              <span v-else style="opacity: 0.5;">{{ t('pulse.noResponses') }}</span>
            </div>
          </NCard>
        </div>
      </div>
    </NModal>
  </div>
</template>
