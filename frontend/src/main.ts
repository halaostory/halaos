import { createApp } from "vue";
import { createPinia } from "pinia";
import { createI18n } from "vue-i18n";
import { createHead } from "@unhead/vue/client";
import App from "./App.vue";
import router from "./router";
import en from "./i18n/en";
import zh from "./i18n/zh";
import "./style.css";
import { loadEdition, edition } from "./edition";
import "./assets/responsive.css";

const i18n = createI18n({
  legacy: false,
  locale: localStorage.getItem("locale") || "en",
  fallbackLocale: "en",
  messages: { en, zh },
});

const head = createHead();

const app = createApp(App);
app.use(createPinia());
app.use(router);
app.use(i18n);
app.use(head);
// Re-brand UI copy for products built from this codebase (e.g. HalaPeople).
function rebrand(node: unknown, name: string): unknown {
  if (typeof node === "string") return node.replace(/HalaOS/g, name);
  if (node && typeof node === "object") {
    const out: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(node as Record<string, unknown>)) out[k] = rebrand(v, name);
    return out;
  }
  return node;
}

loadEdition().finally(() => {
  const name = edition.config.product_name;
  if (name && name !== "HalaOS") {
    for (const loc of ["en", "zh"] as const) {
      i18n.global.setLocaleMessage(loc, rebrand(i18n.global.getLocaleMessage(loc), name) as typeof en);
    }
  }
  app.mount("#app");
});
