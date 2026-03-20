import { createApp } from "vue";
import { createPinia } from "pinia";
import { createI18n } from "vue-i18n";
import { createHead } from "@unhead/vue/client";
import App from "./App.vue";
import router from "./router";
import en from "./i18n/en";
import zh from "./i18n/zh";
import "./style.css";
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
app.mount("#app");
