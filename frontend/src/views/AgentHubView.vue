<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage, useDialog } from 'naive-ui'
import {
  NCard, NGrid, NGi, NTag, NButton, NSpace, NSpin, NEmpty,
  NModal, NForm, NFormItem, NInput, NInputNumber, NSelect,
  NSwitch, NSlider,
} from 'naive-ui'
import { agentAPI } from '../api/client'
import { useAuthStore } from '../stores/auth'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()
const authStore = useAuthStore()
const loading = ref(true)

// --- Types ---

interface Agent {
  slug: string
  name: string
  description: string
  icon: string | null
  cost_multiplier: number
  tools: string[]
  is_active: boolean
  is_system: boolean
  company_id: number
  is_autonomous: boolean
  max_rounds: number
  max_tokens: number
  model: string
}

interface ToolInfo {
  name: string
  description: string
}

// --- State ---

const agents = ref<Agent[]>([])
const availableTools = ref<ToolInfo[]>([])
const showBuilder = ref(false)
const builderSaving = ref(false)
const editingSlug = ref<string | null>(null)

const isAdmin = computed(() => authStore.isAdmin)

const defaultForm = () => ({
  slug: '',
  name: '',
  description: '',
  system_prompt: '',
  tools: [] as string[],
  cost_multiplier: 1.0,
  is_autonomous: false,
  max_rounds: 5,
  max_tokens: 4096,
  icon: '',
  model: '',
})

const builderForm = ref(defaultForm())

const isEditing = computed(() => editingSlug.value !== null)
const builderTitle = computed(() =>
  isEditing.value ? t('agentHub.builder.editTitle') : t('agentHub.builder.title')
)

const toolOptions = computed(() =>
  availableTools.value.map(tool => ({
    label: tool.name,
    value: tool.name,
    description: tool.description,
  }))
)

const modelOptions = computed(() => [
  { label: t('agentHub.builder.modelDefault'), value: '' },
  { label: 'Haiku', value: 'haiku' },
  { label: 'Sonnet', value: 'sonnet' },
])

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
    agents.value = Array.isArray(data) ? data.filter(a => a.is_active !== false) : []
  } catch {
    agents.value = []
  } finally {
    loading.value = false
  }
}

async function fetchTools() {
  try {
    const res = await agentAPI.listTools()
    const data = extractData<ToolInfo[]>(res)
    availableTools.value = Array.isArray(data) ? data : []
  } catch {
    availableTools.value = []
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

function openCreateBuilder() {
  editingSlug.value = null
  builderForm.value = defaultForm()
  showBuilder.value = true
}

function openEditBuilder(agent: Agent) {
  editingSlug.value = agent.slug
  builderForm.value = {
    slug: agent.slug,
    name: agent.name,
    description: agent.description || '',
    system_prompt: '',
    tools: agent.tools || [],
    cost_multiplier: agent.cost_multiplier || 1.0,
    is_autonomous: agent.is_autonomous || false,
    max_rounds: agent.max_rounds || 5,
    max_tokens: agent.max_tokens || 4096,
    icon: agent.icon || '',
    model: agent.model || '',
  }
  // Fetch full agent details to get system_prompt
  agentAPI.get(agent.slug).then(res => {
    const data = extractData<Agent & { system_prompt?: string }>(res)
    if (data.system_prompt) {
      builderForm.value.system_prompt = data.system_prompt
    }
  }).catch(() => { /* use empty system_prompt */ })
  showBuilder.value = true
}

async function saveAgent() {
  const form = builderForm.value
  if (!form.name.trim()) {
    message.error(t('agentHub.builder.namePlaceholder'))
    return
  }

  builderSaving.value = true
  try {
    if (isEditing.value) {
      await agentAPI.update(editingSlug.value!, {
        name: form.name,
        description: form.description,
        system_prompt: form.system_prompt,
        tools: form.tools,
        cost_multiplier: form.cost_multiplier,
        is_autonomous: form.is_autonomous,
        max_rounds: form.max_rounds,
        max_tokens: form.max_tokens,
        icon: form.icon,
        model: form.model,
      })
      message.success(t('agentHub.builder.updateSuccess'))
    } else {
      if (!form.slug.trim()) {
        message.error(t('agentHub.builder.slugPlaceholder'))
        builderSaving.value = false
        return
      }
      await agentAPI.create({
        slug: form.slug,
        name: form.name,
        description: form.description,
        system_prompt: form.system_prompt,
        tools: form.tools,
        cost_multiplier: form.cost_multiplier,
        is_autonomous: form.is_autonomous,
        max_rounds: form.max_rounds,
        max_tokens: form.max_tokens,
        icon: form.icon,
        model: form.model,
      })
      message.success(t('agentHub.builder.createSuccess'))
    }
    showBuilder.value = false
    await fetchAgents()
  } catch (err: unknown) {
    const errorMsg = isEditing.value
      ? t('agentHub.builder.updateError')
      : t('agentHub.builder.createError')
    const detail = (err as { data?: { error?: string } })?.data?.error
    message.error(detail || errorMsg)
  } finally {
    builderSaving.value = false
  }
}

function confirmDeleteAgent(agent: Agent) {
  dialog.warning({
    title: t('agentHub.deleteAgent'),
    content: t('agentHub.deleteConfirm'),
    positiveText: t('agentHub.deleteAgent'),
    negativeText: t('agentHub.builder.cancel'),
    onPositiveClick: async () => {
      try {
        await agentAPI.delete(agent.slug)
        message.success(t('agentHub.builder.deleteSuccess'))
        await fetchAgents()
      } catch {
        message.error(t('agentHub.builder.deleteError'))
      }
    },
  })
}

// --- Mount ---

onMounted(() => {
  fetchAgents()
  if (isAdmin.value) {
    fetchTools()
  }
})
</script>

<template>
  <NSpin :show="loading">
    <div>
      <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 24px;">
        <div>
          <h2 style="margin-bottom: 4px;">{{ t('agentHub.title') }}</h2>
          <p style="color: #999; font-size: 14px; margin: 0;">
            {{ t('agentHub.subtitle') }}
          </p>
        </div>
        <NButton
          v-if="isAdmin"
          type="primary"
          @click="openCreateBuilder"
        >
          + {{ t('agentHub.createAgent') }}
        </NButton>
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
        <NGi v-for="agent in agents" :key="agent.slug">
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
                  <NSpace :size="4" style="margin-top: 4px;">
                    <NTag
                      :type="getCostColor(agent.cost_multiplier) as any"
                      size="small"
                      round
                    >
                      {{ agent.cost_multiplier }}x
                    </NTag>
                    <NTag
                      :type="agent.is_system ? 'info' : 'default'"
                      size="small"
                      round
                    >
                      {{ agent.is_system ? t('agentHub.systemAgent') : t('agentHub.customAgent') }}
                    </NTag>
                  </NSpace>
                </div>
              </div>
            </template>

            <p style="color: #666; font-size: 13px; line-height: 1.5; margin: 0 0 12px 0; min-height: 40px;">
              {{ agent.description }}
            </p>

            <!-- Available Tools -->
            <div v-if="agent.tools && agent.tools.length > 0" style="margin-bottom: 16px;">
              <div style="font-size: 12px; color: #999; margin-bottom: 6px;">
                {{ t('agentHub.tools') }}
              </div>
              <NSpace :size="4">
                <NTag
                  v-for="tool in agent.tools.slice(0, 5)"
                  :key="tool"
                  size="small"
                  :bordered="false"
                >
                  {{ tool }}
                </NTag>
                <NTag
                  v-if="agent.tools.length > 5"
                  size="small"
                  :bordered="false"
                >
                  +{{ agent.tools.length - 5 }}
                </NTag>
              </NSpace>
            </div>

            <template #action>
              <NSpace justify="space-between" align="center">
                <NButton
                  type="primary"
                  @click="openAgentChat(agent)"
                >
                  {{ t('agentHub.tryIt') }}
                </NButton>
                <NSpace v-if="isAdmin && !agent.is_system" :size="8">
                  <NButton
                    size="small"
                    @click="openEditBuilder(agent)"
                  >
                    {{ t('agentHub.editAgent') }}
                  </NButton>
                  <NButton
                    size="small"
                    type="error"
                    ghost
                    @click="confirmDeleteAgent(agent)"
                  >
                    {{ t('agentHub.deleteAgent') }}
                  </NButton>
                </NSpace>
              </NSpace>
            </template>
          </NCard>
        </NGi>
      </NGrid>

      <!-- Empty State -->
      <NCard v-else-if="!loading">
        <NEmpty :description="t('agentHub.noAgents')" style="padding: 48px 0;" />
      </NCard>
    </div>
  </NSpin>

  <!-- Agent Builder Modal -->
  <NModal
    v-model:show="showBuilder"
    :title="builderTitle"
    preset="card"
    style="max-width: 640px;"
    :mask-closable="false"
  >
    <NForm label-placement="top" :model="builderForm">
      <!-- Slug (only when creating) -->
      <NFormItem
        v-if="!isEditing"
        :label="t('agentHub.builder.slug')"
        path="slug"
      >
        <NInput
          v-model:value="builderForm.slug"
          :placeholder="t('agentHub.builder.slugPlaceholder')"
        />
        <template #feedback>
          <span style="font-size: 12px; color: #999;">{{ t('agentHub.builder.slugHint') }}</span>
        </template>
      </NFormItem>

      <!-- Name -->
      <NFormItem :label="t('agentHub.builder.name')" path="name">
        <NInput
          v-model:value="builderForm.name"
          :placeholder="t('agentHub.builder.namePlaceholder')"
        />
      </NFormItem>

      <!-- Description -->
      <NFormItem :label="t('agentHub.builder.description')" path="description">
        <NInput
          v-model:value="builderForm.description"
          type="textarea"
          :placeholder="t('agentHub.builder.descriptionPlaceholder')"
          :rows="2"
        />
      </NFormItem>

      <!-- Icon -->
      <NFormItem :label="t('agentHub.builder.icon')" path="icon">
        <NInput
          v-model:value="builderForm.icon"
          :placeholder="t('agentHub.builder.iconPlaceholder')"
          style="max-width: 120px;"
        />
      </NFormItem>

      <!-- System Prompt -->
      <NFormItem :label="t('agentHub.builder.systemPrompt')" path="system_prompt">
        <NInput
          v-model:value="builderForm.system_prompt"
          type="textarea"
          :placeholder="t('agentHub.builder.systemPromptPlaceholder')"
          :rows="5"
        />
      </NFormItem>

      <!-- Tools -->
      <NFormItem :label="t('agentHub.builder.selectTools')" path="tools">
        <NSelect
          v-model:value="builderForm.tools"
          multiple
          filterable
          :options="toolOptions"
          :placeholder="t('agentHub.builder.selectTools')"
          :render-label="(option: any) => option.label"
          max-tag-count="responsive"
        />
      </NFormItem>

      <!-- Model -->
      <NFormItem :label="t('agentHub.builder.model')" path="model">
        <NSelect
          v-model:value="builderForm.model"
          :options="modelOptions"
          style="max-width: 200px;"
        />
      </NFormItem>

      <!-- Cost Multiplier -->
      <NFormItem :label="`${t('agentHub.builder.costMultiplier')}: ${builderForm.cost_multiplier}x`">
        <NSlider
          v-model:value="builderForm.cost_multiplier"
          :min="0.5"
          :max="3.0"
          :step="0.1"
        />
      </NFormItem>

      <!-- Max Rounds & Max Tokens -->
      <NSpace :size="16">
        <NFormItem :label="t('agentHub.builder.maxRounds')">
          <NInputNumber
            v-model:value="builderForm.max_rounds"
            :min="1"
            :max="10"
            style="width: 120px;"
          />
        </NFormItem>
        <NFormItem :label="t('agentHub.builder.maxTokens')">
          <NInputNumber
            v-model:value="builderForm.max_tokens"
            :min="500"
            :max="8192"
            :step="500"
            style="width: 140px;"
          />
        </NFormItem>
      </NSpace>

      <!-- Autonomous -->
      <NFormItem :label="t('agentHub.builder.autonomous')">
        <NSwitch v-model:value="builderForm.is_autonomous" />
      </NFormItem>
    </NForm>

    <template #action>
      <NSpace justify="end">
        <NButton @click="showBuilder = false">
          {{ t('agentHub.builder.cancel') }}
        </NButton>
        <NButton
          type="primary"
          :loading="builderSaving"
          @click="saveAgent"
        >
          {{ builderSaving ? t('agentHub.builder.saving') : t('agentHub.builder.save') }}
        </NButton>
      </NSpace>
    </template>
  </NModal>
</template>
