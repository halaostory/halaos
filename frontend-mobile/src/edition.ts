// Product edition (modules) for the mobile app. Mirrors internal/edition.
import { reactive } from "vue";

export const edition = reactive<{ productName: string; modules: string[]; aiBilling: boolean }>({
  productName: "HalaPeople",
  modules: [],
  aiBilling: false,
});

let loading: Promise<void> | null = null;

export function loadEdition(): Promise<void> {
  if (!loading) {
    loading = fetch((import.meta.env.VITE_API_URL || "/api") + "/v1/edition", { signal: AbortSignal.timeout(4000) })
      .then((r) => (r.ok ? r.json() : null))
      .then((j) => {
        if (j?.data) {
          edition.productName = j.data.product_name || edition.productName;
          edition.modules = j.data.modules || [];
          edition.aiBilling = !!j.data.ai_billing;
        }
      })
      .catch(() => undefined)
      .finally(() => {
        document.title = edition.productName;
      });
  }
  return loading;
}

/** Whether a module ships (true until the edition is known). */
export function hasModule(m: string): boolean {
  return !edition.modules.length || edition.modules.includes(m);
}
