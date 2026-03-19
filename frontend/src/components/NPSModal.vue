<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { NModal, NButton, NInput, useMessage } from 'naive-ui'
import { npsAPI } from '../api/client'

const message = useMessage()
const show = ref(false)
const score = ref<number | null>(null)
const comment = ref('')
const submitting = ref(false)
const submitted = ref(false)

const STORAGE_KEY = 'halaos_nps_dismissed'
const DISMISS_DAYS = 7 // Don't show again for 7 days after dismissal

onMounted(async () => {
  // Don't show if recently dismissed
  const dismissed = localStorage.getItem(STORAGE_KEY)
  if (dismissed) {
    const dismissedAt = new Date(dismissed)
    if (Date.now() - dismissedAt.getTime() < DISMISS_DAYS * 86400000) return
  }

  // Check server-side eligibility (30-day cooldown)
  try {
    const res = await npsAPI.status()
    const data = (res as Record<string, unknown>)?.data ?? res
    if ((data as Record<string, unknown>)?.eligible) {
      // Show after 60s delay so it doesn't interrupt initial workflow
      setTimeout(() => { show.value = true }, 60000)
    }
  } catch { /* not eligible or error */ }
})

function dismiss() {
  localStorage.setItem(STORAGE_KEY, new Date().toISOString())
  show.value = false
}

async function submit() {
  if (score.value === null) {
    message.warning('Please select a score')
    return
  }
  submitting.value = true
  try {
    await npsAPI.submit({ score: score.value, comment: comment.value.trim() })
    submitted.value = true
    localStorage.removeItem(STORAGE_KEY)
    setTimeout(() => { show.value = false }, 2000)
  } catch {
    message.error('Failed to submit feedback. Please try again.')
  } finally {
    submitting.value = false
  }
}

function scoreLabel(n: number): string {
  if (n <= 2) return 'Very unlikely'
  if (n <= 4) return 'Unlikely'
  if (n <= 6) return 'Neutral'
  if (n <= 8) return 'Likely'
  return 'Very likely'
}

function scoreColor(n: number): string {
  if (n <= 4) return '#ef4444'
  if (n <= 6) return '#f59e0b'
  if (n <= 8) return '#22c55e'
  return '#16a34a'
}
</script>

<template>
  <NModal
    v-model:show="show"
    :mask-closable="true"
    @mask-click="dismiss"
    :close-on-esc="true"
    @esc="dismiss"
  >
    <div class="nps-card">
      <!-- Thank you state -->
      <div v-if="submitted" class="nps-thanks">
        <div class="nps-thanks-icon">&#10003;</div>
        <h3>Thank you for your feedback!</h3>
        <p>Your input helps us improve HalaOS.</p>
      </div>

      <!-- Survey form -->
      <template v-else>
        <div class="nps-header">
          <h3>How likely are you to recommend HalaOS?</h3>
          <p>On a scale of 0-10, how likely are you to recommend HalaOS to a friend or colleague?</p>
        </div>

        <div class="nps-scores">
          <button
            v-for="n in 11"
            :key="n - 1"
            class="nps-score-btn"
            :class="{ active: score === n - 1 }"
            :style="score === n - 1 ? { background: scoreColor(n - 1), borderColor: scoreColor(n - 1) } : {}"
            @click="score = n - 1"
          >
            {{ n - 1 }}
          </button>
        </div>
        <div class="nps-labels">
          <span>Not likely</span>
          <span v-if="score !== null" class="nps-selected">{{ scoreLabel(score) }}</span>
          <span>Very likely</span>
        </div>

        <NInput
          v-model:value="comment"
          type="textarea"
          placeholder="What's the main reason for your score? (optional)"
          :rows="3"
          style="margin-top: 16px;"
        />

        <div class="nps-actions">
          <NButton quaternary size="small" @click="dismiss">Maybe later</NButton>
          <NButton type="primary" :loading="submitting" :disabled="score === null" @click="submit">
            Submit Feedback
          </NButton>
        </div>
      </template>
    </div>
  </NModal>
</template>

<style scoped>
.nps-card {
  background: #fff;
  border-radius: 16px;
  padding: 32px;
  width: 480px;
  max-width: 90vw;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.12);
}
.nps-header h3 {
  font-size: 18px;
  font-weight: 700;
  color: #0f172a;
  margin: 0 0 8px;
}
.nps-header p {
  font-size: 14px;
  color: #64748b;
  margin: 0;
}
.nps-scores {
  display: flex;
  gap: 4px;
  margin-top: 20px;
  justify-content: center;
}
.nps-score-btn {
  width: 38px;
  height: 38px;
  border: 2px solid #e2e8f0;
  border-radius: 8px;
  background: #fff;
  font-size: 14px;
  font-weight: 600;
  color: #334155;
  cursor: pointer;
  transition: all 0.15s;
}
.nps-score-btn:hover {
  border-color: #4f46e5;
  color: #4f46e5;
}
.nps-score-btn.active {
  color: #fff;
  border-color: #4f46e5;
  background: #4f46e5;
}
.nps-labels {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 8px;
  font-size: 12px;
  color: #94a3b8;
}
.nps-selected {
  font-weight: 600;
  color: #4f46e5;
  font-size: 13px;
}
.nps-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 20px;
}
.nps-thanks {
  text-align: center;
  padding: 24px 0;
}
.nps-thanks-icon {
  width: 56px;
  height: 56px;
  background: #22c55e;
  color: #fff;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 28px;
  margin: 0 auto 16px;
}
.nps-thanks h3 {
  font-size: 18px;
  font-weight: 700;
  color: #0f172a;
  margin: 0 0 8px;
}
.nps-thanks p {
  color: #64748b;
  margin: 0;
}

/* Dark mode support */
:global(.dark) .nps-card {
  background: #1e293b;
}
:global(.dark) .nps-header h3,
:global(.dark) .nps-thanks h3 {
  color: #f1f5f9;
}
:global(.dark) .nps-score-btn {
  background: #1e293b;
  border-color: #475569;
  color: #e2e8f0;
}
:global(.dark) .nps-score-btn:hover {
  border-color: #818cf8;
  color: #818cf8;
}
</style>
