<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  NModal, NInput, NSpin, NCard, NButton, NSpace, NTag, NEmpty,
} from 'naive-ui'
import { commandAPI } from '../api/client'

const router = useRouter()
const { t } = useI18n()

const STORAGE_KEY = 'aigonhr_recent_commands'
const MAX_RECENT = 5

// State
const visible = ref(false)
const query = ref('')
const loading = ref(false)
const errorMsg = ref('')
const inputRef = ref<InstanceType<typeof NInput> | null>(null)

interface CommandAction {
  label: string
  route?: string
  action?: string
  params?: Record<string, unknown>
}

interface CommandResult {
  type: 'action' | 'query' | 'info' | 'navigation'
  title: string
  message: string
  data?: Record<string, unknown>
  actions?: CommandAction[]
}

const result = ref<CommandResult | null>(null)
const recentCommands = ref<string[]>(loadRecent())
const isMac = typeof window !== 'undefined' && window.navigator.platform.toUpperCase().includes('MAC')

// Type icon/color mapping (no emojis, use text labels)
const typeConfig: Record<string, { icon: string; tagType: 'success' | 'info' | 'warning' | 'default' }> = {
  action: { icon: 'Action', tagType: 'success' },
  query: { icon: 'Query', tagType: 'warning' },
  info: { icon: 'Info', tagType: 'info' },
  navigation: { icon: 'Navigate', tagType: 'default' },
}

const resultTypeConfig = computed(() => {
  if (!result.value) return null
  return typeConfig[result.value.type] ?? typeConfig.info
})

// LocalStorage helpers
function loadRecent(): string[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed.slice(0, MAX_RECENT) : []
  } catch {
    return []
  }
}

function saveRecent(commands: readonly string[]): void {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(commands))
}

function addToRecent(cmd: string): void {
  const trimmed = cmd.trim()
  if (!trimmed) return
  const filtered = recentCommands.value.filter(c => c !== trimmed)
  const updated = [trimmed, ...filtered].slice(0, MAX_RECENT)
  recentCommands.value = updated
  saveRecent(updated)
}

// Open/Close
function open(): void {
  visible.value = true
  result.value = null
  errorMsg.value = ''
  query.value = ''
  nextTick(() => {
    inputRef.value?.focus()
  })
}

function close(): void {
  visible.value = false
  result.value = null
  errorMsg.value = ''
  query.value = ''
  loading.value = false
}

// Execute command
async function executeCommand(): Promise<void> {
  const trimmed = query.value.trim()
  if (!trimmed || loading.value) return

  loading.value = true
  errorMsg.value = ''
  result.value = null

  try {
    const res = await commandAPI.execute(trimmed)
    const data = (res as any)?.data ?? res
    const commandResult = data?.result ?? data
    result.value = commandResult as CommandResult
    addToRecent(trimmed)
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : String(err)
    errorMsg.value = message || t('commandPalette.executeFailed')
  } finally {
    loading.value = false
  }
}

// Handle action button click
function handleAction(action: CommandAction): void {
  if (action.route) {
    close()
    router.push(action.route)
  }
  // For non-route actions, emit or handle differently in the future
}

// Fill recent command into input
function fillRecent(cmd: string): void {
  query.value = cmd
  nextTick(() => {
    inputRef.value?.focus()
  })
}

// Keyboard shortcut handler
function handleKeyDown(e: KeyboardEvent): void {
  const isMac = navigator.platform.toUpperCase().includes('MAC')
  const modifier = isMac ? e.metaKey : e.ctrlKey

  if (modifier && e.key === 'k') {
    e.preventDefault()
    if (visible.value) {
      close()
    } else {
      open()
    }
  }

  if (e.key === 'Escape' && visible.value) {
    e.preventDefault()
    close()
  }
}

onMounted(() => {
  document.addEventListener('keydown', handleKeyDown)
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleKeyDown)
})

// Expose open for external trigger (mobile FAB)
defineExpose({ open })
</script>

<template>
  <!-- Mobile floating button -->
  <div class="command-palette-fab" @click="open">
    <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <circle cx="11" cy="11" r="8" />
      <line x1="21" y1="21" x2="16.65" y2="16.65" />
    </svg>
  </div>

  <NModal
    v-model:show="visible"
    :mask-closable="true"
    :close-on-esc="false"
    :auto-focus="false"
    transform-origin="center"
    @after-leave="close"
  >
    <div class="command-palette">
      <!-- Search input -->
      <div class="command-palette__header">
        <NInput
          ref="inputRef"
          v-model:value="query"
          size="large"
          :placeholder="t('commandPalette.placeholder')"
          clearable
          @keyup.enter="executeCommand"
        >
          <template #prefix>
            <svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="opacity: 0.5;">
              <circle cx="11" cy="11" r="8" />
              <line x1="21" y1="21" x2="16.65" y2="16.65" />
            </svg>
          </template>
          <template #suffix>
            <NTag size="small" :bordered="false" style="font-size: 11px; opacity: 0.6;">
              {{ isMac ? 'Cmd+K' : 'Ctrl+K' }}
            </NTag>
          </template>
        </NInput>
      </div>

      <!-- Recent commands -->
      <div v-if="!loading && !result && !errorMsg && recentCommands.length > 0" class="command-palette__recent">
        <div class="command-palette__recent-label">{{ t('commandPalette.recent') }}</div>
        <div class="command-palette__recent-list">
          <NButton
            v-for="cmd in recentCommands"
            :key="cmd"
            text
            size="small"
            @click="fillRecent(cmd)"
          >
            {{ cmd }}
          </NButton>
        </div>
      </div>

      <!-- Loading -->
      <div v-if="loading" class="command-palette__loading">
        <NSpin size="medium" />
        <span style="margin-left: 8px; opacity: 0.7;">{{ t('commandPalette.processing') }}</span>
      </div>

      <!-- Error -->
      <div v-if="errorMsg" class="command-palette__error">
        <NCard size="small">
          <div style="color: var(--n-text-color-error, #d03050);">{{ errorMsg }}</div>
        </NCard>
      </div>

      <!-- Result -->
      <div v-if="result" class="command-palette__result">
        <NCard size="small" :bordered="true">
          <template #header>
            <NSpace align="center" :size="8">
              <NTag
                v-if="resultTypeConfig"
                :type="resultTypeConfig.tagType"
                size="small"
              >
                {{ t(`commandPalette.type_${result.type}`) }}
              </NTag>
              <strong>{{ result.title }}</strong>
            </NSpace>
          </template>
          <div class="command-palette__message">{{ result.message }}</div>
          <template #action v-if="result.actions && result.actions.length > 0">
            <NSpace :size="8">
              <NButton
                v-for="(act, idx) in result.actions"
                :key="idx"
                size="small"
                :type="idx === 0 ? 'primary' : 'default'"
                @click="handleAction(act)"
              >
                {{ act.label }}
              </NButton>
            </NSpace>
          </template>
        </NCard>
      </div>

      <!-- Empty state hint -->
      <div v-if="!loading && !result && !errorMsg && recentCommands.length === 0" class="command-palette__hint">
        <NEmpty :description="t('commandPalette.hint')" size="small" />
      </div>
    </div>
  </NModal>
</template>

<style scoped>
.command-palette {
  width: 560px;
  max-width: 95vw;
  background: var(--n-color, #fff);
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 16px 48px rgba(0, 0, 0, 0.2);
}

.command-palette__header {
  padding: 16px 16px 12px;
}

.command-palette__recent {
  padding: 0 16px 12px;
  border-top: 1px solid var(--n-border-color, #e0e0e6);
}

.command-palette__recent-label {
  font-size: 12px;
  color: var(--n-text-color-3, #999);
  padding: 8px 0 4px;
}

.command-palette__recent-list {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 8px;
}

.command-palette__loading {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px 16px;
  border-top: 1px solid var(--n-border-color, #e0e0e6);
}

.command-palette__error {
  padding: 12px 16px 16px;
  border-top: 1px solid var(--n-border-color, #e0e0e6);
}

.command-palette__result {
  padding: 12px 16px 16px;
  border-top: 1px solid var(--n-border-color, #e0e0e6);
}

.command-palette__message {
  white-space: pre-wrap;
  line-height: 1.6;
}

.command-palette__hint {
  padding: 16px;
  border-top: 1px solid var(--n-border-color, #e0e0e6);
}

/* Mobile: full-screen modal */
@media (max-width: 640px) {
  .command-palette {
    width: 100vw;
    max-width: 100vw;
    height: 100vh;
    max-height: 100vh;
    border-radius: 0;
    display: flex;
    flex-direction: column;
  }

  .command-palette__result,
  .command-palette__loading,
  .command-palette__error,
  .command-palette__hint,
  .command-palette__recent {
    flex: 1;
    overflow-y: auto;
  }
}

/* Mobile FAB - only show on small screens */
.command-palette-fab {
  display: none;
  position: fixed;
  bottom: 80px;
  left: 20px;
  z-index: 100;
  width: 48px;
  height: 48px;
  border-radius: 50%;
  background: var(--n-color-primary, #2563eb);
  color: #fff;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  box-shadow: 0 4px 12px rgba(37, 99, 235, 0.4);
  transition: transform 0.2s, box-shadow 0.2s;
}

.command-palette-fab:hover {
  transform: scale(1.1);
  box-shadow: 0 6px 16px rgba(37, 99, 235, 0.5);
}

@media (max-width: 640px) {
  .command-palette-fab {
    display: flex;
  }
}
</style>
