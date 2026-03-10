import { computed } from "vue";
import { useAuthStore } from "../stores/auth";

const localeMap: Record<string, string> = {
  PHP: "en-PH",
  LKR: "en-LK",
  SGD: "en-SG",
  IDR: "id-ID",
};

export function useCurrency() {
  const auth = useAuthStore();
  const currency = computed(() => auth.user?.company_currency || "PHP");

  function formatCurrency(value: unknown): string {
    const num = Number(value || 0);
    const cur = currency.value;
    return num.toLocaleString(localeMap[cur] || "en-US", {
      style: "currency",
      currency: cur,
    });
  }

  return { currency, formatCurrency };
}
