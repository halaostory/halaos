<script setup lang="ts">
import { ref, computed, nextTick, watch, onMounted } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import {
  NavBar,
  ActionSheet,
  Popup,
  Cell,
  CellGroup,
  Empty,
  Tag,
  SwipeCell,
  Button,
  showToast,
  showConfirmDialog,
} from "vant";
import AiMessageBubble from "../components/ai/AiMessageBubble.vue";
import AiChatInput from "../components/ai/AiChatInput.vue";
import AiSuggestionChips from "../components/ai/AiSuggestionChips.vue";
import { useAiChat } from "../composables/useAiChat";
import { useAiContext, getSuggestions } from "../composables/useAiContext";
import { aiAPI, billingAPI } from "../api/client";
import type { Agent, ChatSession, TokenBalance, ApiResponse } from "../types";

const { t } = useI18n();
const router = useRouter();
const chat = useAiChat();
const { context } = useAiContext();

const messagesEl = ref<HTMLElement | null>(null);
const showAgentPicker = ref(false);
const showSessions = ref(false);
const agents = ref<Agent[]>([]);
const sessions = ref<ChatSession[]>([]);
const tokenBalance = ref<number | null>(null);

const suggestions = computed(() =>
  chat.messages.value.length === 0
    ? getSuggestions(context.value.section)
    : [],
);

const agentActions = computed(() =>
  agents.value.map((a) => ({
    name: `${a.icon || "🤖"} ${a.name}`,
    subname: a.description,
    callback: () => {
      chat.setAgent(a.slug);
      chat.newSession();
      showAgentPicker.value = false;
    },
  })),
);

const currentAgentName = computed(() => {
  const a = agents.value.find((a) => a.slug === chat.currentAgent.value);
  return a ? a.name : t("ai.assistant");
});

const balanceText = computed(() => {
  if (tokenBalance.value === null) return "";
  if (tokenBalance.value >= 1000000) {
    return `${(tokenBalance.value / 1000000).toFixed(1)}M`;
  }
  if (tokenBalance.value >= 1000) {
    return `${(tokenBalance.value / 1000).toFixed(0)}K`;
  }
  return String(tokenBalance.value);
});

async function loadAgents() {
  try {
    const res = (await aiAPI.listAgents()) as ApiResponse<Agent[]>;
    agents.value = res.data ?? (res as unknown as Agent[]);
  } catch {
    // fallback
  }
}

async function loadBalance() {
  try {
    const res = (await billingAPI.getBalance()) as ApiResponse<TokenBalance>;
    const data = res.data ?? (res as unknown as TokenBalance);
    tokenBalance.value = data.balance;
  } catch {
    // ignore
  }
}

async function loadSessions() {
  try {
    const res = (await aiAPI.listSessions()) as ApiResponse<ChatSession[]>;
    sessions.value = res.data ?? (res as unknown as ChatSession[]);
  } catch {
    // ignore
  }
}

async function deleteSession(sid: string) {
  try {
    await showConfirmDialog({
      title: t("common.confirm"),
      message: t("ai.deleteSessionConfirm"),
    });
    await aiAPI.deleteSession(sid);
    sessions.value = sessions.value.filter((s) => s.id !== sid);
    // If deleting current session, start new
    if (chat.sessionId.value === sid) {
      chat.newSession();
    }
  } catch {
    // user cancelled or error
  }
}

function openSession(s: ChatSession) {
  chat.setAgent(s.agent_slug);
  chat.loadSession(s.id);
  showSessions.value = false;
}

function onSend(text: string) {
  chat.sendMessage(text);
}

function onSuggestionSelect(text: string) {
  chat.sendMessage(text);
}

// Auto-scroll on new messages
watch(
  () => chat.messages.value.length,
  async () => {
    await nextTick();
    if (messagesEl.value) {
      messagesEl.value.scrollTop = messagesEl.value.scrollHeight;
    }
  },
);

// Also scroll during streaming
watch(
  () => chat.messages.value[chat.messages.value.length - 1]?.content,
  async () => {
    await nextTick();
    if (messagesEl.value) {
      messagesEl.value.scrollTop = messagesEl.value.scrollHeight;
    }
  },
);

watch(
  () => chat.error.value,
  (err) => {
    if (err) showToast({ message: err, type: "fail" });
  },
);

// Refresh balance after each message
watch(
  () => chat.streaming.value,
  (streaming) => {
    if (!streaming) loadBalance();
  },
);

onMounted(() => {
  loadAgents();
  loadBalance();
  // Auto-send from query param (e.g., from AiQuickAsk)
  const q = router.currentRoute.value.query.q;
  if (typeof q === "string" && q.trim()) {
    chat.sendMessage(q.trim());
    router.replace({ name: "ai-chat" });
  }
});
</script>

<template>
  <div class="ai-chat-page">
    <NavBar
      :title="currentAgentName"
      left-arrow
      @click-left="router.back()"
    >
      <template #right>
        <div class="nav-actions">
          <Tag v-if="balanceText" plain type="primary" size="medium" class="balance-tag">
            {{ balanceText }}
          </Tag>
          <van-icon
            name="clock-o"
            size="20"
            @click="loadSessions(); showSessions = true;"
          />
          <van-icon
            name="apps-o"
            size="20"
            @click="showAgentPicker = true"
          />
          <van-icon name="add-o" size="20" @click="chat.newSession()" />
        </div>
      </template>
    </NavBar>

    <!-- Messages Area -->
    <div ref="messagesEl" class="chat-messages">
      <!-- Empty state with suggestions -->
      <template v-if="chat.messages.value.length === 0">
        <div class="empty-state">
          <Empty
            image="search"
            :description="`Ask ${currentAgentName} anything`"
          />
        </div>
      </template>

      <!-- Message list -->
      <AiMessageBubble
        v-for="(msg, i) in chat.messages.value"
        :key="i"
        :message="msg"
        @confirm="chat.confirmDraft(i)"
        @reject="chat.rejectDraft(i)"
      />

      <!-- Streaming indicator -->
      <div v-if="chat.streaming.value" class="typing-indicator">
        <span></span><span></span><span></span>
      </div>
    </div>

    <!-- Suggestion chips (only when empty) -->
    <AiSuggestionChips
      :suggestions="suggestions"
      @select="onSuggestionSelect"
    />

    <!-- Input -->
    <AiChatInput :disabled="chat.streaming.value" @send="onSend" />

    <!-- Agent Picker -->
    <ActionSheet
      v-model:show="showAgentPicker"
      :actions="agentActions"
      :cancel-text="t('common.cancel')"
    />

    <!-- Session History -->
    <Popup
      v-model:show="showSessions"
      position="right"
      :style="{ width: '80%', height: '100%' }"
    >
      <div class="sessions-panel">
        <NavBar :title="t('ai.chatHistory')" left-arrow @click-left="showSessions = false" />
        <CellGroup v-if="sessions.length > 0">
          <SwipeCell v-for="s in sessions" :key="s.id">
            <Cell
              :title="s.title || t('ai.untitled')"
              :label="s.updated_at?.slice(0, 10)"
              is-link
              @click="openSession(s)"
            >
              <template #right-icon>
                <Tag plain type="primary">{{ s.agent_slug }}</Tag>
              </template>
            </Cell>
            <template #right>
              <Button
                square
                type="danger"
                icon="delete-o"
                class="swipe-delete"
                @click="deleteSession(s.id)"
              />
            </template>
          </SwipeCell>
        </CellGroup>
        <Empty v-else :description="t('ai.noHistory')" />
      </div>
    </Popup>
  </div>
</template>

<style scoped>
.ai-chat-page {
  display: flex;
  flex-direction: column;
  height: 100vh;
  height: 100dvh;
  background: var(--app-bg);
}

.nav-actions {
  display: flex;
  gap: 12px;
  align-items: center;
}

.balance-tag {
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}

.chat-messages {
  flex: 1;
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
  padding: 8px 0;
}

.empty-state {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 200px;
}

.typing-indicator {
  display: flex;
  gap: 4px;
  padding: 12px 24px;
}

.typing-indicator span {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--text-secondary);
  animation: typing 1.4s infinite;
}

.typing-indicator span:nth-child(2) {
  animation-delay: 0.2s;
}

.typing-indicator span:nth-child(3) {
  animation-delay: 0.4s;
}

@keyframes typing {
  0%,
  60%,
  100% {
    opacity: 0.3;
    transform: scale(0.8);
  }
  30% {
    opacity: 1;
    transform: scale(1);
  }
}

.sessions-panel {
  height: 100%;
  overflow-y: auto;
  background: var(--app-bg);
}

.swipe-delete {
  height: 100%;
}
</style>
