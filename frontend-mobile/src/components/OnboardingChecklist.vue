<template>
  <div v-if="visible" class="onboarding-wrap">
    <!-- Welcome header -->
    <div class="onboarding-header">
      <div class="onboarding-skip" @click="handleDismiss">{{ t('onboarding.skip') }} ×</div>
      <div class="onboarding-title">{{ t('onboarding.welcome', { name: firstName }) }}</div>
      <div class="onboarding-subtitle">{{ t('onboarding.completeSteps') }}</div>
      <div class="onboarding-progress">
        <div class="onboarding-progress-bar">
          <div class="onboarding-progress-fill" :style="{ width: progressPct + '%' }" />
        </div>
        <div class="onboarding-progress-text">{{ t('onboarding.completed', { n: doneCount, total: steps.length }) }}</div>
      </div>
    </div>

    <!-- Steps -->
    <div class="onboarding-steps">
      <div
        v-for="(step, i) in steps"
        :key="step.key"
        class="onboarding-step"
        :class="{ done: step.done, current: !step.done && i === nextIndex }"
        @click="goToStep(step)"
      >
        <div class="step-circle" :class="{ done: step.done, current: !step.done && i === nextIndex }">
          <span v-if="step.done">✓</span>
          <span v-else>{{ i + 1 }}</span>
        </div>
        <div class="step-content">
          <div class="step-title" :class="{ done: step.done }">{{ step.title }}</div>
          <div class="step-desc">{{ step.desc }}</div>
        </div>
        <div v-if="!step.done" class="step-arrow">›</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '../stores/auth'
import { onboardingChecklistAPI } from '../api/client'
import type { ApiResponse } from '../types'

interface StepDef {
  key: string
  title: string
  desc: string
  route: string
  done: boolean
}

const { t } = useI18n()
const router = useRouter()
const auth = useAuthStore()

const visible = ref(false)
const stepData = ref<Record<string, { done: boolean; done_at?: string }>>({})

const firstName = computed(() => auth.user?.first_name || '')

const steps = computed<StepDef[]>(() => [
  { key: 'profile', title: t('onboarding.steps.profile'), desc: t('onboarding.steps.profileDesc'), route: 'profile', done: stepData.value.profile?.done ?? false },
  { key: 'first_clock', title: t('onboarding.steps.firstClock'), desc: t('onboarding.steps.firstClockDesc'), route: 'attendance', done: stepData.value.first_clock?.done ?? false },
  { key: 'view_leave', title: t('onboarding.steps.viewLeave'), desc: t('onboarding.steps.viewLeaveDesc'), route: 'leave', done: stepData.value.view_leave?.done ?? false },
  { key: 'view_payslip', title: t('onboarding.steps.viewPayslip'), desc: t('onboarding.steps.viewPayslipDesc'), route: 'payslips', done: stepData.value.view_payslip?.done ?? false },
  { key: 'ai_chat', title: t('onboarding.steps.aiChat'), desc: t('onboarding.steps.aiChatDesc'), route: 'ai-chat', done: stepData.value.ai_chat?.done ?? false },
])

const doneCount = computed(() => steps.value.filter(s => s.done).length)
const progressPct = computed(() => (doneCount.value / steps.value.length) * 100)
const nextIndex = computed(() => steps.value.findIndex(s => !s.done))

async function loadProgress() {
  try {
    const res = await onboardingChecklistAPI.getProgress('employee') as ApiResponse<{
      steps: Record<string, { done: boolean; done_at?: string }>
      dismissed: boolean
      completed_at: string | null
    }>
    const data = res.data ?? (res as any)
    if (data.dismissed || data.completed_at) {
      visible.value = false
      return
    }
    stepData.value = data.steps || {}
    visible.value = true
    // TODO: When all steps done, show brief celebration animation then auto-hide after 2s
  } catch {
    visible.value = false
  }
}

function goToStep(step: StepDef) {
  if (!step.done) {
    router.push({ name: step.route })
  }
}

async function handleDismiss() {
  try {
    await onboardingChecklistAPI.dismiss('employee')
  } catch { /* ignore */ }
  visible.value = false
}

onMounted(loadProgress)

defineExpose({ loadProgress })
</script>

<style scoped>
.onboarding-wrap { margin-bottom: 16px; }
.onboarding-header {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  border-radius: 16px;
  padding: 20px;
  color: white;
  position: relative;
  margin-bottom: 12px;
}
.onboarding-skip {
  position: absolute; top: 12px; right: 12px;
  font-size: 12px; opacity: 0.8; cursor: pointer;
}
.onboarding-title { font-size: 18px; font-weight: 700; margin-bottom: 4px; }
.onboarding-subtitle { font-size: 13px; opacity: 0.9; margin-bottom: 16px; }
.onboarding-progress-bar {
  background: rgba(255,255,255,0.25); border-radius: 8px; height: 6px; margin-bottom: 6px;
}
.onboarding-progress-fill {
  background: white; border-radius: 8px; height: 6px; transition: width 0.3s;
}
.onboarding-progress-text { font-size: 11px; opacity: 0.8; }
.onboarding-steps {
  background: white; border-radius: 12px; overflow: hidden;
  box-shadow: 0 1px 3px rgba(0,0,0,0.06);
}
.onboarding-step {
  display: flex; align-items: center; padding: 14px 16px;
  border-bottom: 1px solid #f0f0f0; cursor: pointer;
}
.onboarding-step:last-child { border-bottom: none; }
.onboarding-step.done { opacity: 0.6; }
.onboarding-step.current { background: #f0f7ff; }
.step-circle {
  width: 24px; height: 24px; border-radius: 50%;
  border: 2px solid #ddd; color: #bbb;
  display: flex; align-items: center; justify-content: center;
  font-size: 12px; font-weight: 700; flex-shrink: 0;
}
.step-circle.done { background: #18a058; color: white; border-color: #18a058; }
.step-circle.current { background: #2563eb; color: white; border-color: #2563eb; }
.step-content { margin-left: 12px; flex: 1; }
.step-title { font-size: 14px; font-weight: 500; color: #333; }
.step-title.done { text-decoration: line-through; color: #999; }
.onboarding-step.current .step-title { font-weight: 600; color: #2563eb; }
.step-desc { font-size: 12px; color: #999; margin-top: 2px; }
.step-arrow { font-size: 18px; color: #ccc; }
.onboarding-step.current .step-arrow { color: #2563eb; }
</style>
