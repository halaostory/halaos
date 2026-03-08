<script setup lang="ts">
import { ref, nextTick, watch, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { NButton, NInput, NSpin, NSelect, NTag } from 'naive-ui'
import { aiAPI, billingAPI } from '../api/client'

const { t, locale } = useI18n()
const router = useRouter()

const isOpen = ref(false)
const message = ref('')
const loading = ref(false)
const sessionId = ref<string | undefined>()
const messagesContainer = ref<HTMLElement>()
const showHistory = ref(false)
const currentSessionId = ref<string | null>(null)

// Agent selector
const selectedAgent = ref<string>('general')
const agents = ref<Array<{ slug: string; name: string; description: string; cost_multiplier: number; icon: string }>>([])
const agentOptions = computed(() =>
  agents.value.map(a => ({
    label: `${a.icon || '🤖'} ${a.name} (${a.cost_multiplier}x)`,
    value: a.slug,
  }))
)

// Token balance
const tokenBalance = ref<number | null>(null)
const insufficientBalance = ref(false)

async function loadAgents() {
  try {
    const res = await aiAPI.listAgents()
    agents.value = res.data || []
  } catch {
    // fallback: just general agent
    agents.value = [{ slug: 'general', name: 'General', description: 'General HR assistant', cost_multiplier: 1.0, icon: '🤖' }]
  }
}

async function loadBalance() {
  try {
    const res = await billingAPI.getBalance()
    tokenBalance.value = res.data?.balance ?? null
  } catch {
    tokenBalance.value = null
  }
}

interface ChatMessage {
  role: 'user' | 'assistant' | 'system'
  content: string
  tools?: string[]
  tokensUsed?: number
}

interface ChatSession {
  id: string
  title: string
  messages: ChatMessage[]
  updatedAt: string
}

interface StoredSessions {
  sessions: ChatSession[]
}

const STORAGE_KEY = 'aigonhr_chat_sessions'
const MAX_SESSIONS = 20

function generateId(): string {
  return Date.now().toString(36) + Math.random().toString(36).slice(2, 8)
}

function getSystemMessage(): ChatMessage {
  return {
    role: 'system',
    content: locale.value === 'zh'
      ? '你好！我是 AigoNHR AI 助手。我可以帮你查询假期余额、薪资信息、考勤状况、菲律宾劳工法规等。请问有什么可以帮你的？'
      : 'Hello! I\'m the AigoNHR AI Assistant. I can help you check leave balances, payroll info, attendance, and Philippine labor regulations. How can I help you?'
  }
}

const messages = ref<ChatMessage[]>([getSystemMessage()])

// --- localStorage helpers (immutable) ---

function loadSessions(): ChatSession[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return []
    const parsed: StoredSessions = JSON.parse(raw)
    return Array.isArray(parsed.sessions) ? parsed.sessions : []
  } catch {
    return []
  }
}

function saveSessions(sessions: ChatSession[]): void {
  const data: StoredSessions = { sessions }
  localStorage.setItem(STORAGE_KEY, JSON.stringify(data))
}

function deriveTitle(msgs: ChatMessage[]): string {
  const firstUser = msgs.find(m => m.role === 'user')
  if (!firstUser) return 'New Chat'
  const text = firstUser.content.trim()
  return text.length > 30 ? text.slice(0, 30) + '...' : text
}

function saveCurrentSession(): void {
  // Only save if there are user messages beyond the system greeting
  const hasUserMessages = messages.value.some(m => m.role === 'user')
  if (!hasUserMessages) return

  const now = new Date().toISOString()
  const sessId = currentSessionId.value || generateId()

  if (!currentSessionId.value) {
    currentSessionId.value = sessId
  }

  const updatedSession: ChatSession = {
    id: sessId,
    title: deriveTitle(messages.value),
    messages: [...messages.value.map(m => ({ ...m, tools: m.tools ? [...m.tools] : undefined }))],
    updatedAt: now,
  }

  const existing = loadSessions()
  const withoutCurrent = existing.filter(s => s.id !== sessId)
  // Prepend current session, then trim to MAX_SESSIONS
  const merged = [updatedSession, ...withoutCurrent].slice(0, MAX_SESSIONS)
  saveSessions(merged)
}

function deleteSession(sessIdToDelete: string): void {
  const existing = loadSessions()
  const filtered = existing.filter(s => s.id !== sessIdToDelete)
  saveSessions(filtered)

  // If we deleted the active session, start a new chat
  if (currentSessionId.value === sessIdToDelete) {
    startNewChat()
  }
}

function loadSession(sess: ChatSession): void {
  currentSessionId.value = sess.id
  messages.value = [...sess.messages.map(m => ({ ...m, tools: m.tools ? [...m.tools] : undefined }))]
  sessionId.value = undefined // reset API session — backend doesn't persist
  showHistory.value = false
  scrollToBottom()
}

function startNewChat(): void {
  currentSessionId.value = null
  sessionId.value = undefined
  messages.value = [getSystemMessage()]
  showHistory.value = false
}

// Reactive list of sessions for the sidebar
const storedSessions = ref<ChatSession[]>([])

function refreshStoredSessions(): void {
  storedSessions.value = loadSessions()
}

// --- Relative time ---

function relativeTime(isoString: string): string {
  const now = Date.now()
  const then = new Date(isoString).getTime()
  const diffMs = now - then
  const diffMin = Math.floor(diffMs / 60000)

  if (diffMin < 1) return locale.value === 'zh' ? '刚刚' : 'just now'
  if (diffMin < 60) return locale.value === 'zh' ? `${diffMin}分钟前` : `${diffMin}m ago`
  const diffHr = Math.floor(diffMin / 60)
  if (diffHr < 24) return locale.value === 'zh' ? `${diffHr}小时前` : `${diffHr}h ago`
  const diffDay = Math.floor(diffHr / 24)
  if (diffDay < 30) return locale.value === 'zh' ? `${diffDay}天前` : `${diffDay}d ago`
  const diffMon = Math.floor(diffDay / 30)
  return locale.value === 'zh' ? `${diffMon}个月前` : `${diffMon}mo ago`
}

// --- Lifecycle ---

onMounted(() => {
  refreshStoredSessions()
  loadAgents()
  loadBalance()

  // Listen for agent-hub "Try It" button
  window.addEventListener('open-agent-chat', ((e: CustomEvent) => {
    const slug = e.detail?.slug
    if (slug) {
      selectedAgent.value = slug
      isOpen.value = true
      startNewChat()
    }
  }) as EventListener)
})

watch(locale, (newLocale) => {
  if (messages.value.length === 1 && messages.value[0].role === 'system') {
    messages.value = [
      {
        ...messages.value[0],
        content: newLocale === 'zh'
          ? '你好！我是 AigoNHR AI 助手。我可以帮你查询假期余额、薪资信息、考勤状况、菲律宾劳工法规等。请问有什么可以帮你的？'
          : 'Hello! I\'m the AigoNHR AI Assistant. I can help you check leave balances, payroll info, attendance, and Philippine labor regulations. How can I help you?'
      }
    ]
  }
})

function scrollToBottom() {
  nextTick(() => {
    if (messagesContainer.value) {
      messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
    }
  })
}

async function sendMessage() {
  const text = message.value.trim()
  if (!text || loading.value) return

  // Immutable push — create new array with new user message
  messages.value = [...messages.value, { role: 'user', content: text }]
  message.value = ''
  loading.value = true
  scrollToBottom()

  // Add empty assistant message for streaming
  const assistantMsg: ChatMessage = { role: 'assistant', content: '', tools: [] }
  messages.value = [...messages.value, assistantMsg]

  try {
    const agentSlug = selectedAgent.value || undefined
    const stream = aiAPI.streamChat(text, sessionId.value, agentSlug)
    for await (const chunk of stream) {
      switch (chunk.type) {
        case 'text':
          assistantMsg.content += chunk.text || ''
          scrollToBottom()
          break
        case 'tool':
          if (chunk.name) {
            assistantMsg.tools = [...(assistantMsg.tools || []), chunk.name]
          }
          break
        case 'error':
          if (chunk.code === 402) {
            insufficientBalance.value = true
            assistantMsg.content = locale.value === 'zh'
              ? '⚠️ Token 余额不足，请充值后继续使用 AI 功能。'
              : '⚠️ Insufficient token balance. Please top up to continue using AI features.'
          } else {
            assistantMsg.content += `\n\n${chunk.message || 'An error occurred'}`
          }
          break
        case 'done':
          if (chunk.tokens_used) {
            assistantMsg.tokensUsed = chunk.tokens_used
          }
          // Refresh balance after usage
          loadBalance()
          break
      }
    }
  } catch (e: unknown) {
    const err = e as Error
    assistantMsg.content = `${err.message || 'Failed to get AI response'}`
  } finally {
    loading.value = false
    scrollToBottom()
    // Save to localStorage after assistant response completes
    saveCurrentSession()
    refreshStoredSessions()
  }
}

function handleKeyDown(e: KeyboardEvent) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    sendMessage()
  }
}

function togglePanel() {
  isOpen.value = !isOpen.value
  if (isOpen.value) {
    refreshStoredSessions()
    loadAgents()
    loadBalance()
    insufficientBalance.value = false
  }
}

function goToBilling() {
  isOpen.value = false
  router.push({ name: 'billing' })
}

function toggleHistory() {
  showHistory.value = !showHistory.value
  if (showHistory.value) {
    refreshStoredSessions()
  }
}

const chatPanelWidth = computed(() => showHistory.value ? '600px' : '400px')
</script>

<template>
  <!-- Floating toggle button -->
  <div
    class="chat-fab"
    @click="togglePanel"
    :title="isOpen ? 'Close AI Assistant' : 'AI Assistant'"
  >
    <span v-if="!isOpen" style="font-size: 20px;">🤖</span>
    <span v-else style="font-size: 18px;">✕</span>
  </div>

  <!-- Chat panel -->
  <Transition name="slide">
    <div v-if="isOpen" class="chat-panel" :style="{ width: chatPanelWidth }">
      <div class="chat-header">
        <div class="chat-header-left">
          <NButton
            text
            size="small"
            class="history-toggle-btn"
            @click="toggleHistory"
            :title="locale === 'zh' ? '聊天记录' : 'Chat History'"
          >
            <span style="font-size: 16px;">&#9776;</span>
          </NButton>
          <span class="chat-title">AigoNHR AI</span>
          <NTag v-if="tokenBalance !== null" size="small" round class="balance-badge">
            {{ tokenBalance.toLocaleString() }} tokens
          </NTag>
        </div>
        <NButton text size="small" @click="isOpen = false" class="chat-close-btn">✕</NButton>
      </div>

      <!-- Agent selector bar -->
      <div class="agent-selector-bar">
        <NSelect
          v-model:value="selectedAgent"
          :options="agentOptions"
          size="small"
          :placeholder="locale === 'zh' ? '选择 Agent' : 'Select Agent'"
          style="flex: 1;"
        />
      </div>

      <div class="chat-body">
        <!-- History sidebar -->
        <Transition name="history-slide">
          <div v-if="showHistory" class="chat-history-sidebar">
            <div class="history-header">
              <span class="history-title">{{ locale === 'zh' ? '聊天记录' : 'History' }}</span>
              <NButton size="tiny" type="primary" @click="startNewChat" class="new-chat-btn">
                {{ locale === 'zh' ? '新对话' : 'New Chat' }}
              </NButton>
            </div>
            <div class="history-list">
              <div
                v-for="sess in storedSessions"
                :key="sess.id"
                :class="['history-item', { 'history-item-active': sess.id === currentSessionId }]"
                @click="loadSession(sess)"
              >
                <div class="history-item-content">
                  <div class="history-item-title">{{ sess.title }}</div>
                  <div class="history-item-time">{{ relativeTime(sess.updatedAt) }}</div>
                </div>
                <NButton
                  text
                  size="tiny"
                  class="history-delete-btn"
                  @click.stop="deleteSession(sess.id)"
                  :title="locale === 'zh' ? '删除' : 'Delete'"
                >
                  ✕
                </NButton>
              </div>
              <div v-if="storedSessions.length === 0" class="history-empty">
                {{ locale === 'zh' ? '暂无聊天记录' : 'No chat history' }}
              </div>
            </div>
          </div>
        </Transition>

        <!-- Main chat area -->
        <div class="chat-main">
          <div ref="messagesContainer" class="chat-messages">
            <div
              v-for="(msg, i) in messages"
              :key="i"
              :class="['chat-msg', `chat-msg-${msg.role}`]"
            >
              <div v-if="msg.tools && msg.tools.length" class="chat-tools">
                <span v-for="tool in msg.tools" :key="tool" class="chat-tool-tag">
                  🔧 {{ tool }}
                </span>
              </div>
              <div class="chat-bubble" v-html="renderMarkdown(msg.content)" />
              <div v-if="msg.tokensUsed" class="chat-tokens-used">
                {{ msg.tokensUsed.toLocaleString() }} tokens
              </div>
            </div>

            <div v-if="loading" class="chat-msg chat-msg-assistant">
              <div class="chat-bubble">
                <NSpin size="small" />
                <span style="margin-left: 8px; color: #999;">{{ t('common.loading') }}</span>
              </div>
            </div>
          </div>

          <!-- Insufficient balance banner -->
          <div v-if="insufficientBalance" class="insufficient-banner">
            <span>{{ locale === 'zh' ? 'Token 余额不足' : 'Insufficient token balance' }}</span>
            <NButton size="tiny" type="warning" @click="goToBilling">
              {{ locale === 'zh' ? '去充值' : 'Top Up' }}
            </NButton>
          </div>

          <div class="chat-input">
            <NInput
              v-model:value="message"
              type="textarea"
              :autosize="{ minRows: 1, maxRows: 4 }"
              :placeholder="locale === 'zh' ? '输入消息...' : 'Type a message...'"
              @keydown="handleKeyDown"
              :disabled="loading"
            />
            <NButton
              type="primary"
              size="small"
              :disabled="!message.trim() || loading"
              @click="sendMessage"
              style="margin-left: 8px; align-self: flex-end;"
            >
              {{ locale === 'zh' ? '发送' : 'Send' }}
            </NButton>
          </div>
        </div>
      </div>
    </div>
  </Transition>
</template>

<script lang="ts">
// Simple markdown renderer (no external dep)
function renderMarkdown(text: string): string {
  if (!text) return ''
  return text
    // Code blocks
    .replace(/```(\w*)\n([\s\S]*?)```/g, '<pre><code>$2</code></pre>')
    // Inline code
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    // Bold
    .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
    // Italic
    .replace(/\*(.+?)\*/g, '<em>$1</em>')
    // Headers
    .replace(/^### (.+)$/gm, '<h4>$1</h4>')
    .replace(/^## (.+)$/gm, '<h3>$1</h3>')
    .replace(/^# (.+)$/gm, '<h2>$1</h2>')
    // Lists
    .replace(/^- (.+)$/gm, '<li>$1</li>')
    .replace(/(<li>.*<\/li>)/s, '<ul>$1</ul>')
    // Line breaks
    .replace(/\n\n/g, '<br/><br/>')
    .replace(/\n/g, '<br/>')
    // Tables (basic)
    .replace(/\|(.+)\|/g, (match) => {
      const cells = match.split('|').filter(c => c.trim())
      if (cells.every(c => c.trim().match(/^[-:]+$/))) return ''
      const tag = 'td'
      return '<tr>' + cells.map(c => `<${tag}>${c.trim()}</${tag}>`).join('') + '</tr>'
    })
}
</script>

<style scoped>
.chat-fab {
  position: fixed;
  bottom: 24px;
  right: 24px;
  width: 52px;
  height: 52px;
  border-radius: 50%;
  background: var(--n-color-primary, #18a058);
  color: white;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  z-index: 1000;
  transition: transform 0.2s, background 0.2s;
}
.chat-fab:hover {
  transform: scale(1.1);
}

.chat-panel {
  position: fixed;
  bottom: 88px;
  right: 24px;
  height: 540px;
  background: var(--n-color, #fff);
  border: 1px solid var(--n-border-color, #e0e0e0);
  border-radius: 12px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.12);
  display: flex;
  flex-direction: column;
  z-index: 999;
  overflow: hidden;
  transition: width 0.3s ease;
}

.chat-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-bottom: 1px solid var(--n-border-color, #e0e0e0);
  background: var(--n-color-primary, #18a058);
  color: white;
  flex-shrink: 0;
}
.chat-header-left {
  display: flex;
  align-items: center;
  gap: 8px;
}
.history-toggle-btn {
  color: white !important;
  opacity: 0.85;
  transition: opacity 0.2s;
}
.history-toggle-btn:hover {
  opacity: 1;
}
.chat-close-btn {
  color: white !important;
}
.chat-title {
  font-weight: 600;
  font-size: 15px;
}

.chat-body {
  display: flex;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

/* --- History sidebar --- */
.chat-history-sidebar {
  width: 200px;
  min-width: 200px;
  border-right: 1px solid var(--n-border-color, #e0e0e0);
  display: flex;
  flex-direction: column;
  background: var(--n-color-modal, #fafafa);
  overflow: hidden;
}
.history-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  border-bottom: 1px solid var(--n-border-color, #e0e0e0);
  flex-shrink: 0;
}
.history-title {
  font-weight: 600;
  font-size: 13px;
  color: var(--n-text-color, #333);
}
.new-chat-btn {
  font-size: 11px;
}
.history-list {
  flex: 1;
  overflow-y: auto;
  padding: 4px 0;
}
.history-item {
  display: flex;
  align-items: center;
  padding: 8px 12px;
  cursor: pointer;
  transition: background 0.15s;
  gap: 4px;
}
.history-item:hover {
  background: var(--n-color-hover, #f0f0f0);
}
.history-item-active {
  background: rgba(24, 160, 88, 0.08);
  border-left: 3px solid var(--n-color-primary, #18a058);
  padding-left: 9px;
}
.history-item-content {
  flex: 1;
  min-width: 0;
}
.history-item-title {
  font-size: 12px;
  font-weight: 500;
  color: var(--n-text-color, #333);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.history-item-time {
  font-size: 10px;
  color: #999;
  margin-top: 2px;
}
.history-delete-btn {
  opacity: 0;
  color: #999 !important;
  font-size: 12px;
  flex-shrink: 0;
  transition: opacity 0.15s;
}
.history-item:hover .history-delete-btn {
  opacity: 1;
}
.history-delete-btn:hover {
  color: #e06060 !important;
}
.history-empty {
  padding: 20px 12px;
  text-align: center;
  color: #999;
  font-size: 12px;
}

/* History slide animation */
.history-slide-enter-active,
.history-slide-leave-active {
  transition: all 0.25s ease;
}
.history-slide-enter-from,
.history-slide-leave-to {
  width: 0;
  min-width: 0;
  opacity: 0;
}

/* --- Main chat area --- */
.chat-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  overflow: hidden;
}

.chat-messages {
  flex: 1;
  overflow-y: auto;
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.chat-msg {
  display: flex;
  flex-direction: column;
  max-width: 85%;
}
.chat-msg-user {
  align-self: flex-end;
}
.chat-msg-user .chat-bubble {
  background: var(--n-color-primary, #18a058);
  color: white;
  border-radius: 12px 12px 2px 12px;
}
.chat-msg-assistant,
.chat-msg-system {
  align-self: flex-start;
}
.chat-msg-assistant .chat-bubble,
.chat-msg-system .chat-bubble {
  background: var(--n-color-hover, #f5f5f5);
  color: var(--n-text-color, #333);
  border-radius: 12px 12px 12px 2px;
}

.chat-bubble {
  padding: 10px 14px;
  font-size: 14px;
  line-height: 1.5;
  word-break: break-word;
}
.chat-bubble :deep(code) {
  background: rgba(0, 0, 0, 0.06);
  padding: 2px 4px;
  border-radius: 3px;
  font-size: 13px;
}
.chat-bubble :deep(pre) {
  background: rgba(0, 0, 0, 0.06);
  padding: 8px;
  border-radius: 6px;
  overflow-x: auto;
  margin: 4px 0;
}
.chat-bubble :deep(table) {
  border-collapse: collapse;
  margin: 4px 0;
  font-size: 13px;
}
.chat-bubble :deep(td) {
  border: 1px solid #ddd;
  padding: 4px 8px;
}
.chat-bubble :deep(strong) {
  font-weight: 600;
}
.chat-bubble :deep(h2),
.chat-bubble :deep(h3),
.chat-bubble :deep(h4) {
  margin: 6px 0 4px;
  font-weight: 600;
}

.chat-tools {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-bottom: 4px;
}
.chat-tool-tag {
  font-size: 11px;
  padding: 2px 6px;
  background: rgba(24, 160, 88, 0.1);
  border-radius: 4px;
  color: #18a058;
}

/* --- Agent selector bar --- */
.agent-selector-bar {
  display: flex;
  align-items: center;
  padding: 6px 12px;
  border-bottom: 1px solid var(--n-border-color, #e0e0e0);
  background: var(--n-color-modal, #fafafa);
  flex-shrink: 0;
  gap: 8px;
}

/* --- Balance badge --- */
.balance-badge {
  background: rgba(255, 255, 255, 0.2) !important;
  color: white !important;
  border: 1px solid rgba(255, 255, 255, 0.3) !important;
  font-size: 11px !important;
}

/* --- Token usage per message --- */
.chat-tokens-used {
  font-size: 10px;
  color: #999;
  margin-top: 2px;
  padding-left: 4px;
}

/* --- Insufficient balance banner --- */
.insufficient-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  background: #fff3cd;
  border-top: 1px solid #ffc107;
  font-size: 12px;
  color: #856404;
  flex-shrink: 0;
}

.chat-input {
  display: flex;
  align-items: flex-end;
  padding: 12px;
  border-top: 1px solid var(--n-border-color, #e0e0e0);
  gap: 0;
  flex-shrink: 0;
}

/* Slide animation */
.slide-enter-active,
.slide-leave-active {
  transition: all 0.3s ease;
}
.slide-enter-from,
.slide-leave-to {
  opacity: 0;
  transform: translateY(20px) scale(0.95);
}

/* Mobile responsive */
@media (max-width: 480px) {
  .chat-panel {
    width: calc(100vw - 16px) !important;
    height: calc(100vh - 120px);
    right: 8px;
    bottom: 80px;
  }
  .chat-history-sidebar {
    width: 160px;
    min-width: 160px;
  }
}
</style>
