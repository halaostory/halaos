import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 3001,
    proxy: {
      "/api": {
        target: "http://127.0.0.1:8080",
        changeOrigin: true,
      },
    },
  },
  build: {
    target: "es2020",
    cssCodeSplit: true,
    rollupOptions: {
      output: {
        manualChunks: {
          "naive-ui": ["naive-ui"],
          vendor: ["vue", "vue-router", "pinia", "vue-i18n"],
          charts: ["echarts", "vue-echarts"],
          utils: ["date-fns", "ofetch", "markdown-it"],
        },
      },
    },
  },
});
