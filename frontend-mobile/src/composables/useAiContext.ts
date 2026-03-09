import { ref, watch } from "vue";
import { useRoute } from "vue-router";

export interface AiPageContext {
  route: string;
  section: string;
  entityType?: string;
  entityId?: number;
}

export function useAiContext() {
  const context = ref<AiPageContext>({ route: "/", section: "home" });
  const route = useRoute();

  function parseRoute(path: string): AiPageContext {
    const segments = path.split("/").filter(Boolean);
    const section = segments[0] || "home";
    const ctx: AiPageContext = { route: path, section };

    if (segments.length >= 2 && !isNaN(Number(segments[1]))) {
      ctx.entityType = section.replace(/s$/, "");
      ctx.entityId = Number(segments[1]);
    }

    return ctx;
  }

  watch(
    () => route.fullPath,
    (path) => {
      context.value = parseRoute(path);
    },
    { immediate: true },
  );

  return { context };
}

/** Contextual suggestion chips based on current page section */
export function getSuggestions(section: string): string[] {
  const map: Record<string, string[]> = {
    home: [
      "How many leave days do I have?",
      "Show my attendance today",
      "Any pending notifications?",
    ],
    attendance: [
      "What time did I clock in?",
      "Show my attendance this week",
      "Am I late today?",
    ],
    leave: [
      "What's my leave balance?",
      "Help me apply for leave",
      "Check my pending leave requests",
    ],
    payslips: [
      "Show my latest payslip",
      "Why is my pay different this month?",
      "What's my year-to-date income?",
    ],
    profile: [
      "What's the password policy?",
      "How do I update my info?",
      "Show company policies",
    ],
  };
  return map[section] || map.home;
}
