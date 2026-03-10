<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NCard, NButton, NEmpty, NTag, NRate, NInput,
  NRadioGroup, NRadio, NSpace, useMessage,
} from 'naive-ui'
import { pulseAPI } from '../api/client'

const { t } = useI18n()
const message = useMessage()

const loading = ref(true)
const surveys = ref<any[]>([])

// Response state
const selectedSurvey = ref<any>(null)
const openRound = ref<any>(null)
const questions = ref<any[]>([])
const responses = ref<Record<number, { rating?: number; answer_text?: string }>>({})
const responded = ref(false)
const submitting = ref(false)

async function loadActiveSurveys() {
  loading.value = true
  try {
    const res = await pulseAPI.listActive()
    const data = (res as any)?.data ?? res
    surveys.value = Array.isArray(data) ? data : []
  } catch {
    message.error('Failed to load surveys')
  } finally {
    loading.value = false
  }
}

async function openSurvey(survey: any) {
  selectedSurvey.value = survey
  try {
    const res = await pulseAPI.getOpenRound(survey.id)
    const data = (res as any)?.data ?? res
    openRound.value = data.round
    questions.value = data.questions || []
    responded.value = data.responded || false

    // Init response map
    const r: Record<number, { rating?: number; answer_text?: string }> = {}
    for (const q of questions.value) {
      r[q.id] = {}
    }
    responses.value = r
  } catch {
    message.warning(t('pulse.noOpenRound'))
    selectedSurvey.value = null
  }
}

async function submitResponses() {
  if (!openRound.value) return
  submitting.value = true

  const responseList = []
  for (const q of questions.value) {
    const r = responses.value[q.id]
    if (!r) continue

    if (q.question_type === 'rating' && q.is_required && !r.rating) {
      message.warning(`${t('pulse.pleaseRate')}: ${q.question}`)
      submitting.value = false
      return
    }
    if (q.question_type === 'text' && q.is_required && !r.answer_text?.trim()) {
      message.warning(`${t('pulse.pleaseAnswer')}: ${q.question}`)
      submitting.value = false
      return
    }

    responseList.push({
      question_id: q.id,
      rating: r.rating || null,
      answer_text: r.answer_text || null,
    })
  }

  try {
    await pulseAPI.submitResponse(openRound.value.id, { responses: responseList })
    message.success(t('pulse.submitSuccess'))
    responded.value = true
  } catch {
    message.error('Failed to submit response')
  } finally {
    submitting.value = false
  }
}

function goBack() {
  selectedSurvey.value = null
  openRound.value = null
  questions.value = []
  responses.value = {}
  responded.value = false
}

onMounted(loadActiveSurveys)
</script>

<template>
  <div style="max-width: 800px; margin: 0 auto;">
    <h2 style="margin: 0 0 8px;">{{ t('pulse.respondTitle') }}</h2>
    <p style="margin: 0 0 24px; opacity: 0.7;">{{ t('pulse.respondSubtitle') }}</p>

    <!-- Survey list -->
    <template v-if="!selectedSurvey">
      <div v-if="loading" style="padding: 40px; text-align: center;">{{ t('pulse.loading') }}</div>
      <NEmpty v-else-if="surveys.length === 0" :description="t('pulse.noActiveSurveys')" style="padding: 40px 0;" />
      <div v-else style="display: flex; flex-direction: column; gap: 12px;">
        <NCard
          v-for="s in surveys"
          :key="s.id"
          hoverable
          style="cursor: pointer;"
          @click="s.open_round && !s.responded ? openSurvey(s) : null"
        >
          <div style="display: flex; justify-content: space-between; align-items: center;">
            <div>
              <strong>{{ s.title }}</strong>
              <p v-if="s.description" style="margin: 4px 0 0; opacity: 0.7; font-size: 13px;">{{ s.description }}</p>
            </div>
            <NSpace size="small">
              <NTag v-if="s.is_anonymous" type="info" size="small" :bordered="false">
                {{ t('pulse.anonymousTag') }}
              </NTag>
              <NTag v-if="s.responded" type="success" size="small">
                {{ t('pulse.alreadyResponded') }}
              </NTag>
              <NTag v-else-if="s.open_round" type="warning" size="small">
                {{ t('pulse.pendingResponse') }}
              </NTag>
              <NTag v-else type="default" size="small">
                {{ t('pulse.noOpenRound') }}
              </NTag>
            </NSpace>
          </div>
        </NCard>
      </div>
    </template>

    <!-- Response form -->
    <template v-else>
      <NButton quaternary @click="goBack" style="margin-bottom: 16px;">
        &larr; {{ t('pulse.backToList') }}
      </NButton>

      <NCard :title="selectedSurvey.title">
        <p v-if="selectedSurvey.description" style="margin: 0 0 16px; opacity: 0.7;">
          {{ selectedSurvey.description }}
        </p>

        <NTag v-if="selectedSurvey.is_anonymous" type="info" size="small" :bordered="false" style="margin-bottom: 16px;">
          {{ t('pulse.anonymousNotice') }}
        </NTag>

        <div v-if="responded" style="text-align: center; padding: 40px;">
          <h3>{{ t('pulse.thankYou') }}</h3>
          <p style="opacity: 0.7;">{{ t('pulse.alreadySubmitted') }}</p>
        </div>

        <div v-else>
          <div v-for="q in questions" :key="q.id" style="margin-bottom: 20px;">
            <div style="font-weight: 500; margin-bottom: 8px;">
              {{ q.question }}
              <span v-if="q.is_required" style="color: red;">*</span>
            </div>

            <!-- Rating -->
            <div v-if="q.question_type === 'rating'">
              <NRate
                :value="responses[q.id]?.rating || 0"
                :count="5"
                @update:value="(v: number) => { responses[q.id] = { ...responses[q.id], rating: v } }"
              />
              <span style="margin-left: 8px; opacity: 0.5;">
                {{ responses[q.id]?.rating ? `${responses[q.id].rating}/5` : '' }}
              </span>
            </div>

            <!-- Text -->
            <div v-else-if="q.question_type === 'text'">
              <NInput
                :value="responses[q.id]?.answer_text || ''"
                type="textarea"
                :rows="3"
                :placeholder="t('pulse.typePlaceholder')"
                @update:value="(v: string) => { responses[q.id] = { ...responses[q.id], answer_text: v } }"
              />
            </div>

            <!-- Yes/No -->
            <div v-else-if="q.question_type === 'yes_no'">
              <NRadioGroup
                :value="responses[q.id]?.answer_text || ''"
                @update:value="(v: string) => { responses[q.id] = { ...responses[q.id], answer_text: v } }"
              >
                <NRadio value="yes">{{ t('pulse.yes') }}</NRadio>
                <NRadio value="no">{{ t('pulse.no') }}</NRadio>
              </NRadioGroup>
            </div>
          </div>

          <NButton
            type="primary"
            block
            :loading="submitting"
            @click="submitResponses"
          >
            {{ t('pulse.submit') }}
          </NButton>
        </div>
      </NCard>
    </template>
  </div>
</template>
