<script setup lang="ts">
import { ref } from 'vue'
import { NCard, NButton, NSpin, NEmpty } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '../../stores/auth'
import { orgIntelligenceAPI } from '../../api/client'
import MarkdownIt from 'markdown-it'

defineProps<{
  briefing: {
    narrative: string
    week_date: string
    generated_at: string
    tokens_used: number
  } | null
}>()

const emit = defineEmits<{
  (e: 'refresh'): void
}>()

const { t } = useI18n()
const auth = useAuthStore()
const generating = ref(false)

const md = new MarkdownIt({
  html: false,
  linkify: true,
  breaks: true,
})

function renderMarkdown(text: string): string {
  if (!text) return ''
  return md.render(text)
}

function formatDate(dateStr: string): string {
  if (!dateStr) return ''
  return new Date(dateStr).toLocaleDateString()
}

async function handleGenerate() {
  generating.value = true
  try {
    await orgIntelligenceAPI.generateBriefing()
    emit('refresh')
  } catch (e) {
    console.error('Failed to generate briefing', e)
  } finally {
    generating.value = false
  }
}
</script>

<template>
  <NCard>
    <template #header>
      {{ t('orgIntelligence.executiveBriefing') }}
    </template>
    <template #header-extra>
      <div style="display: flex; align-items: center; gap: 12px;">
        <span v-if="briefing" style="font-size: 12px; opacity: 0.7;">
          {{ t('orgIntelligence.weekOf') }} {{ formatDate(briefing.week_date) }}
        </span>
        <NButton
          v-if="auth.isAdmin"
          size="small"
          type="primary"
          :loading="generating"
          @click="handleGenerate"
        >
          {{ generating ? t('orgIntelligence.generating') : t('orgIntelligence.generateNew') }}
        </NButton>
      </div>
    </template>

    <NSpin :show="generating">
      <NEmpty
        v-if="!briefing"
        :description="t('orgIntelligence.noBriefing')"
        style="padding: 40px 0;"
      />
      <div v-else>
        <div class="briefing-content" v-html="renderMarkdown(briefing.narrative)" />
        <div v-if="briefing.generated_at" style="margin-top: 16px; font-size: 12px; opacity: 0.6;">
          {{ t('orgIntelligence.generatedAt') }}: {{ formatDate(briefing.generated_at) }}
        </div>
      </div>
    </NSpin>
  </NCard>
</template>

<style scoped>
.briefing-content :deep(h1) { font-size: 1.3em; margin: 16px 0 8px; }
.briefing-content :deep(h2) { font-size: 1.15em; margin: 14px 0 6px; }
.briefing-content :deep(h3) { font-size: 1.05em; margin: 12px 0 4px; }
.briefing-content :deep(ul) { padding-left: 20px; }
.briefing-content :deep(li) { margin: 4px 0; }
.briefing-content :deep(p) { margin: 8px 0; }
.briefing-content :deep(strong) { color: var(--n-text-color); }
</style>
