import { ref, readonly } from "vue";
import { aiAPI } from "../api/client";
import { useAiContext } from "./useAiContext";
import type {
  ChatMessage,
  StreamChunk,
  DraftConfirmation,
  ApiResponse,
} from "../types";

export function useAiChat() {
  const messages = ref<ChatMessage[]>([]);
  const streaming = ref(false);
  const sessionId = ref<string | null>(null);
  const currentAgent = ref("general");
  const error = ref<string | null>(null);
  const { context } = useAiContext();

  async function sendMessage(text: string) {
    if (!text.trim() || streaming.value) return;

    error.value = null;

    // Add user message
    const userMsg: ChatMessage = { role: "user", content: text };
    messages.value = [...messages.value, userMsg];

    // Add placeholder assistant message
    const assistantMsg: ChatMessage = { role: "assistant", content: "" };
    messages.value = [...messages.value, assistantMsg];

    streaming.value = true;
    let fullText = "";

    try {
      const pageCtx = {
        section: context.value.section,
        action: context.value.route,
      };
      const stream = aiAPI.streamChat(
        text,
        sessionId.value ?? undefined,
        currentAgent.value,
        pageCtx,
      );

      for await (const chunk of stream) {
        const c = chunk as StreamChunk;

        if (c.type === "text" && c.text) {
          fullText += c.text;
          const updated = [...messages.value];
          updated[updated.length - 1] = {
            ...updated[updated.length - 1],
            content: fullText,
          };
          messages.value = updated;
        } else if (c.type === "confirmation" && c.data) {
          // Draft confirmation — attach to current assistant message
          const draft: DraftConfirmation = c.data;
          fullText += `\n${draft.message}`;
          const updated = [...messages.value];
          updated[updated.length - 1] = {
            ...updated[updated.length - 1],
            content: fullText,
            draft,
          };
          messages.value = updated;
        } else if (c.type === "done") {
          if (c.session_id) sessionId.value = c.session_id;
          const updated = [...messages.value];
          updated[updated.length - 1] = {
            ...updated[updated.length - 1],
            id: c.message_id,
            tokens_used: c.tokens_used,
          };
          messages.value = updated;
        } else if (c.type === "error") {
          error.value = c.message || "AI error";
          if (c.code === 402) {
            error.value = "Insufficient token balance";
          }
          if (!fullText) {
            messages.value = messages.value.slice(0, -1);
          }
        } else if (c.type === "tool") {
          const toolNote = `\n> *Using: ${c.name}...*\n`;
          fullText += toolNote;
          const updated = [...messages.value];
          updated[updated.length - 1] = {
            ...updated[updated.length - 1],
            content: fullText,
          };
          messages.value = updated;
        }
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : "Connection error";
      if (!fullText) {
        messages.value = messages.value.slice(0, -1);
      }
    } finally {
      streaming.value = false;
    }
  }

  async function confirmDraft(msgIndex: number) {
    const msg = messages.value[msgIndex];
    if (!msg?.draft) return;

    try {
      await aiAPI.confirmDraft(msg.draft.draft_id);
      // Update the message to reflect confirmed status
      const updated = [...messages.value];
      updated[msgIndex] = {
        ...updated[msgIndex],
        draft: { ...msg.draft, status: "confirmed" },
      };
      messages.value = updated;
    } catch {
      error.value = "Failed to confirm action";
    }
  }

  async function rejectDraft(msgIndex: number) {
    const msg = messages.value[msgIndex];
    if (!msg?.draft) return;

    try {
      await aiAPI.rejectDraft(msg.draft.draft_id);
      const updated = [...messages.value];
      updated[msgIndex] = {
        ...updated[msgIndex],
        draft: { ...msg.draft, status: "rejected" },
      };
      messages.value = updated;
    } catch {
      error.value = "Failed to reject action";
    }
  }

  async function loadSession(sid: string) {
    try {
      const res = (await aiAPI.getSessionMessages(sid)) as ApiResponse<
        ChatMessage[]
      >;
      const items = res.data ?? (res as unknown as ChatMessage[]);
      messages.value = Array.isArray(items) ? items : [];
      sessionId.value = sid;
    } catch {
      error.value = "Failed to load session";
    }
  }

  function newSession() {
    messages.value = [];
    sessionId.value = null;
    error.value = null;
  }

  function setAgent(slug: string) {
    currentAgent.value = slug;
  }

  return {
    messages: readonly(messages),
    streaming: readonly(streaming),
    sessionId: readonly(sessionId),
    currentAgent: readonly(currentAgent),
    error: readonly(error),
    sendMessage,
    confirmDraft,
    rejectDraft,
    loadSession,
    newSession,
    setAgent,
  };
}
