<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NCard, NGrid, NGi, NTag, NButton, NSpace, NSpin, NEmpty,
} from 'naive-ui'
import { agentAPI } from '../api/client'

const { t } = useI18n()
const loading = ref(true)

// --- Types ---

interface Agent {
  id: number
  slug: string
  name: string
  description: string
  icon: string | null
  system_prompt: string
  model: string
  cost_multiplier: number
  available_tools: string[]
  is_active: boolean
}

// --- State ---

const agents = ref<Agent[]>([])

// --- Data Fetching ---

function extractData<T>(res: unknown): T {
  const r = res as { data?: T }
  return (r.data ?? res) as T
}

async function fetchAgents() {
  loading.value = true
  try {
    const res = await agentAPI.list()
    const data = extractData<Agent[]>(res)
    agents.value = Array.isArray(data) ? data.filter(a => a.is_active) : []
  } catch {
    agents.value = []
  } finally {
    loading.value = false
  }
}

// --- Actions ---

function openAgentChat(agent: Agent) {
  window.dispatchEvent(
    new CustomEvent('open-agent-chat', { detail: { slug: agent.slug } })
  )
}

function getAgentInitial(agent: Agent): string {
  if (agent.icon) return agent.icon
  return agent.name.charAt(0).toUpperCase()
}

function getCostColor(multiplier: number): string {
  if (multiplier <= 1.0) return 'success'
  if (multiplier <= 2.0) return 'warning'
  return 'error'
}

// --- Mount ---

onMounted(fetchAgents)
</script>

<template>
  <NSpin :show="loading">
    <div>
      <div style="margin-bottom: 24px;">
        <h2 style="margin-bottom: 4px;">{{ t('agentHub.title') }}</h2>
        <p style="color: #999; font-size: 14px; margin: 0;">
          {{ t('agentHub.subtitle') }}
        </p>
      </div>

      <!-- Agent Cards Grid -->
      <NGrid
        v-if="agents.length > 0"
        cols="1 s:2 m:3 l:3"
        :x-gap="16"
        :y-gap="16"
        responsive="screen"
        :item-responsive="true"
      >
        <NGi v-for="agent in agents" :key="agent.id">
          <NCard hoverable style="height: 100%;">
            <template #header>
              <div style="display: flex; align-items: center; gap: 12px;">
                <div
                  style="
                    width: 44px;
                    height: 44px;
                    border-radius: 12px;
                    background: linear-gradient(135deg, #18a058, #36ad6a);
                    color: white;
                    display: flex;
                    align-items: center;
                    justify-content: center;
                    font-size: 20px;
                    font-weight: 600;
                    flex-shrink: 0;
                  "
                >
                  {{ getAgentInitial(agent) }}
                </div>
                <div style="flex: 1; min-width: 0;">
                  <div style="font-weight: 600; font-size: 16px;">
                    {{ agent.name }}
                  </div>
                  <NTag
                    :type="getCostColor(agent.cost_multiplier) as any"
                    size="small"
                    round
                    style="margin-top: 4px;"
                  >
                    {{ t('agentHub.costMultiplier') }}: {{ agent.cost_multiplier }}x
                  </NTag>
                </div>
              </div>
            </template>

            <p style="color: #666; font-size: 13px; line-height: 1.5; margin: 0 0 12px 0; min-height: 40px;">
              {{ agent.description }}
            </p>

            <!-- Available Tools -->
            <div v-if="agent.available_tools && agent.available_tools.length > 0" style="margin-bottom: 16px;">
              <div style="font-size: 12px; color: #999; margin-bottom: 6px;">
                {{ t('agentHub.tools') }}
              </div>
              <NSpace :size="4">
                <NTag
                  v-for="tool in agent.available_tools.slice(0, 5)"
                  :key="tool"
                  size="small"
                  :bordered="false"
                >
                  {{ tool }}
                </NTag>
                <NTag
                  v-if="agent.available_tools.length > 5"
                  size="small"
                  :bordered="false"
                >
                  +{{ agent.available_tools.length - 5 }}
                </NTag>
              </NSpace>
            </div>

            <template #action>
              <NButton
                type="primary"
                block
                @click="openAgentChat(agent)"
              >
                {{ t('agentHub.tryIt') }}
              </NButton>
            </template>
          </NCard>
        </NGi>
      </NGrid>

      <!-- Empty State -->
      <NCard v-else-if="!loading">
        <NEmpty :description="t('agentHub.comingSoon')" style="padding: 48px 0;" />
      </NCard>
    </div>
  </NSpin>
</template>
