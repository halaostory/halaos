import { createApp } from "vue";
import { createPinia } from "pinia";
import { createI18n } from "vue-i18n";
import App from "./App.vue";
import router from "./router";
import { loadEdition } from "./edition";
import en from "./i18n/en";
import zh from "./i18n/zh";
import "vant/lib/index.css";
import "./style.css";

const i18n = createI18n({
  legacy: false,
  locale: localStorage.getItem("locale") || "en",
  fallbackLocale: "en",
  messages: { en, zh },
});

const app = createApp(App);
app.use(createPinia());
app.use(router);
app.use(i18n);
loadEdition().finally(() => app.mount("#app"));

// Installable app: register the service worker in production builds.
if ("serviceWorker" in navigator && import.meta.env.PROD) {
  window.addEventListener("load", () => {
    navigator.serviceWorker.register("/m/sw.js", { scope: "/m/" }).catch(() => undefined);
  });
}
