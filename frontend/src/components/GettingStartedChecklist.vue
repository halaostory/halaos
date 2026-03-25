<template>
  <NCard v-if="visible" style="margin-bottom: 20px;">
    <div class="gs-header">
      <div>
        <div class="gs-title">{{ t('gettingStarted.title') }}</div>
        <div class="gs-subtitle">{{ t('gettingStarted.subtitle') }}</div>
      </div>
      <div class="gs-header-right">
        <span class="gs-counter">{{ t('gettingStarted.completed', { n: doneCount, total: steps.length }) }}</span>
        <NButton text size="small" @click="handleDismiss">{{ t('gettingStarted.dismiss') }} ×</NButton>
      </div>
    </div>
    <div class="gs-progress">
      <div class="gs-progress-fill" :style="{ width: progressPct + '%' }" />
    </div>
    <div class="gs-grid">
      <div
        v-for="(step, i) in steps"
        :key="step.key"
        class="gs-step"
        :class="{ done: step.done, current: !step.done && i === nextIndex }"
        @click="goToStep(step)"
      >
        <div class="gs-circle" :class="{ done: step.done, current: !step.done && i === nextIndex }">
          <span v-if="step.done">✓</span>
          <span v-else>{{ i + 1 }}</span>
        </div>
        <div class="gs-content">
          <div class="gs-step-title" :class="{ done: step.done }">{{ step.title }}</div>
          <div class="gs-step-desc">{{ step.desc }}</div>
          <div v-if="!step.done && i === nextIndex" class="gs-step-link">
            {{ t('gettingStarted.goTo', { feature: step.linkLabel }) }} →
          </div>
        </div>
      </div>
    </div>
  </NCard>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { NCard, NButton } from 'naive-ui'
import { onboardingChecklistAPI } from '../api/client'

const { t } = useI18n()
const router = useRouter()

const visible = ref(false)
const stepData = ref<Record<string, { done: boolean; done_at?: string }>>({})

interface StepDef {
  key: string; title: string; desc: string; route: string; linkLabel: string; done: boolean
}

const steps = computed<StepDef[]>(() => [
  { key: 'company_info', title: t('gettingStarted.steps.companyInfo'), desc: t('gettingStarted.steps.companyInfoDesc'), route: 'settings', linkLabel: t('gettingStarted.links.companyInfo'), done: stepData.value.company_info?.done ?? false },
  { key: 'departments', title: t('gettingStarted.steps.departments'), desc: t('gettingStarted.steps.departmentsDesc'), route: 'departments', linkLabel: t('gettingStarted.links.departments'), done: stepData.value.departments?.done ?? false },
  { key: 'import_employees', title: t('gettingStarted.steps.importEmployees'), desc: t('gettingStarted.steps.importEmployeesDesc'), route: 'employees', linkLabel: t('gettingStarted.links.importEmployees'), done: stepData.value.import_employees?.done ?? false },
  { key: 'leave_policies', title: t('gettingStarted.steps.leavePolicies'), desc: t('gettingStarted.steps.leavePoliciesDesc'), route: 'settings', linkLabel: t('gettingStarted.links.leavePolicies'), done: stepData.value.leave_policies?.done ?? false },
  { key: 'schedules', title: t('gettingStarted.steps.schedules'), desc: t('gettingStarted.steps.schedulesDesc'), route: 'schedules', linkLabel: t('gettingStarted.links.schedules'), done: stepData.value.schedules?.done ?? false },
  { key: 'payroll_config', title: t('gettingStarted.steps.payrollConfig'), desc: t('gettingStarted.steps.payrollConfigDesc'), route: 'salary', linkLabel: t('gettingStarted.links.payrollConfig'), done: stepData.value.payroll_config?.done ?? false },
  { key: 'first_payroll', title: t('gettingStarted.steps.firstPayroll'), desc: t('gettingStarted.steps.firstPayrollDesc'), route: 'payroll', linkLabel: t('gettingStarted.links.firstPayroll'), done: stepData.value.first_payroll?.done ?? false },
])

const doneCount = computed(() => steps.value.filter(s => s.done).length)
const progressPct = computed(() => (doneCount.value / steps.value.length) * 100)
const nextIndex = computed(() => steps.value.findIndex(s => !s.done))

async function loadProgress() {
  try {
    const res = await onboardingChecklistAPI.getProgress('admin') as { data?: any }
    const data = (res.data || res) as any
    if (data.dismissed || data.completed_at) {
      visible.value = false
      return
    }
    stepData.value = data.steps || {}
    visible.value = true
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
  try { await onboardingChecklistAPI.dismiss('admin') } catch { /* ignore */ }
  visible.value = false
}

onMounted(loadProgress)
</script>

<style scoped>
.gs-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 16px; }
.gs-title { font-size: 18px; font-weight: 700; color: #111; }
.gs-subtitle { font-size: 13px; color: #888; margin-top: 4px; }
.gs-header-right { display: flex; align-items: center; gap: 12px; }
.gs-counter { font-size: 13px; color: #2563eb; font-weight: 600; }
.gs-progress { background: #f0f0f0; border-radius: 8px; height: 6px; margin-bottom: 20px; }
.gs-progress-fill { background: linear-gradient(90deg, #2563eb, #7c3aed); border-radius: 8px; height: 6px; transition: width 0.3s; }
.gs-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
@media (max-width: 768px) { .gs-grid { grid-template-columns: 1fr; } }
.gs-step {
  display: flex; align-items: flex-start; padding: 14px;
  border-radius: 10px; background: #fafafa; border: 1px solid #eee; cursor: pointer;
  transition: background 0.15s;
}
.gs-step:hover { background: #f5f5f5; }
.gs-step.done { background: #f8faf8; border-color: #d4edda; }
.gs-step.current { background: #eff6ff; border-color: #bfdbfe; }
.gs-circle {
  width: 28px; height: 28px; border-radius: 50%;
  border: 2px solid #ddd; color: #bbb;
  display: flex; align-items: center; justify-content: center;
  font-size: 12px; font-weight: 700; flex-shrink: 0; margin-top: 2px;
}
.gs-circle.done { background: #18a058; color: white; border-color: #18a058; }
.gs-circle.current { background: #2563eb; color: white; border-color: #2563eb; }
.gs-content { margin-left: 12px; }
.gs-step-title { font-size: 14px; font-weight: 600; color: #333; }
.gs-step-title.done { text-decoration: line-through; color: #999; }
.gs-step.current .gs-step-title { color: #2563eb; }
.gs-step-desc { font-size: 12px; color: #999; margin-top: 2px; }
.gs-step-link { font-size: 11px; color: #2563eb; margin-top: 6px; font-weight: 500; }
</style>
