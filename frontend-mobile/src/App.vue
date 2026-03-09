<script setup lang="ts">
import { computed } from "vue";
import { ConfigProvider } from "vant";
import { useThemeStore } from "./stores/theme";

const theme = useThemeStore();

const themeVars = computed(() =>
  theme.isDark
    ? {
        background: "#1a1a2e",
        background2: "#16213e",
        textColor: "#e0e0e0",
      }
    : undefined,
);
</script>

<template>
  <ConfigProvider :theme-vars="themeVars" :class="{ dark: theme.isDark }">
    <router-view />
  </ConfigProvider>
</template>
