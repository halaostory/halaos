<script setup lang="ts">
import { ref, nextTick, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { NButton, NInput, NSpin } from 'naive-ui'
import { aiAPI } from '../api/client'

const { t, locale } = useI18n()

const isOpen = ref(false)
const message = ref('')
const loading = ref(false)
const sessionId = ref<string | undefined>()
const messagesContainer = ref<HTMLElement>()

interface ChatMessage {
  role: 'user' | 'assistant' | 'system'
  content: string
  tools?: string[]
}

const messages = ref<ChatMessage[]>([
  {
    role: 'system',
    content: locale.value === 'zh'
      ? '你好！我是 AigoNHR AI 助手。我可以帮你查询假期余额、薪资信息、考勤状况、菲律宾劳工法规等。请问有什么可以帮你的？'
      : 'Hello! I\'m the AigoNHR AI Assistant. I can help you check leave balances, payroll info, attendance, and Philippine labor regulations. How can I help you?'
  }
])

watch(locale, (newLocale) => {
  if (messages.value.length === 1 && messages.value[0].role === 'system') {
    messages.value[0].content = newLocale === 'zh'
      ? '你好！我是 AigoNHR AI 助手。我可以帮你查询假期余额、薪资信息、考勤状况、菲律宾劳工法规等。请问有什么可以帮你的？'
      : 'Hello! I\'m the AigoNHR AI Assistant. I can help you check leave balances, payroll info, attendance, and Philippine labor regulations. How can I help you?'
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

  messages.value.push({ role: 'user', content: text })
  message.value = ''
  loading.value = true
  scrollToBottom()

  // Add empty assistant message for streaming
  const assistantMsg: ChatMessage = { role: 'assistant', content: '', tools: [] }
  messages.value.push(assistantMsg)

  try {
    const stream = aiAPI.streamChat(text, sessionId.value)
    for await (const chunk of stream) {
      switch (chunk.type) {
        case 'text':
          assistantMsg.content += chunk.text || ''
          scrollToBottom()
          break
        case 'tool':
          if (chunk.name) {
            assistantMsg.tools = assistantMsg.tools || []
            assistantMsg.tools.push(chunk.name)
          }
          break
        case 'error':
          assistantMsg.content += `\n\n⚠️ ${chunk.message || 'An error occurred'}`
          break
        case 'done':
          break
      }
    }
  } catch (e: unknown) {
    const err = e as Error
    assistantMsg.content = `⚠️ ${err.message || 'Failed to get AI response'}`
  } finally {
    loading.value = false
    scrollToBottom()
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
}
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
    <div v-if="isOpen" class="chat-panel">
      <div class="chat-header">
        <span class="chat-title">AigoNHR AI</span>
        <NButton text size="small" @click="isOpen = false">✕</NButton>
      </div>

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
        </div>

        <div v-if="loading" class="chat-msg chat-msg-assistant">
          <div class="chat-bubble">
            <NSpin size="small" />
            <span style="margin-left: 8px; color: #999;">{{ t('common.loading') }}</span>
          </div>
        </div>
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
  width: 400px;
  height: 540px;
  background: var(--n-color, #fff);
  border: 1px solid var(--n-border-color, #e0e0e0);
  border-radius: 12px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.12);
  display: flex;
  flex-direction: column;
  z-index: 999;
  overflow: hidden;
}

.chat-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-bottom: 1px solid var(--n-border-color, #e0e0e0);
  background: var(--n-color-primary, #18a058);
  color: white;
}
.chat-title {
  font-weight: 600;
  font-size: 15px;
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

.chat-input {
  display: flex;
  align-items: flex-end;
  padding: 12px;
  border-top: 1px solid var(--n-border-color, #e0e0e0);
  gap: 0;
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
    width: calc(100vw - 16px);
    height: calc(100vh - 120px);
    right: 8px;
    bottom: 80px;
  }
}
</style>
