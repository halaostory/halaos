<script setup lang="ts">
import { computed } from 'vue'
import { NConfigProvider, NMessageProvider, NDialogProvider, darkTheme, enUS, zhCN, type GlobalThemeOverrides, type NLocale } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { useThemeStore } from './stores/theme'

const themeStore = useThemeStore()
const theme = computed(() => themeStore.isDark ? darkTheme : null)

// Friendlier empty states than Naive UI's bare "No Data", in the UI language.
const { locale: i18nLocale } = useI18n()
const naiveLocale = computed<NLocale>(() => {
  const base = i18nLocale.value === 'zh' ? zhCN : enUS
  return { ...base, Empty: { ...base.Empty, description: i18nLocale.value === 'zh' ? '这里还没有内容' : 'Nothing here yet' } }
})

// One calm, Warp-like look for every page: softer radii, quieter table
// headers, round status pills and roomier rows.
const themeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: '#2563eb',
    primaryColorHover: '#3b82f6',
    primaryColorPressed: '#1d4ed8',
    primaryColorSuppl: '#60a5fa',
    borderRadius: '8px',
    borderRadiusSmall: '6px',
    fontFamily: "Inter, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif",
    fontSize: '14px',
  },
  Card: {
    borderRadius: '12px',
    titleFontSizeMedium: '15px',
    titleFontWeight: '600',
    paddingMedium: '18px 20px 20px',
  },
  DataTable: {
    thFontWeight: '500',
    thPaddingMedium: '10px 14px',
    tdPaddingMedium: '12px 14px',
    borderRadius: '12px',
  },
  Tag: {
    borderRadius: '999px',
  },
  Button: {
    borderRadiusMedium: '8px',
    borderRadiusSmall: '7px',
    fontWeight: '500',
  },
  Tabs: {
    tabFontWeightActive: '600',
  },
  Input: {
    borderRadius: '8px',
  },
}
</script>

<template>
  <NConfigProvider :theme="theme" :theme-overrides="themeOverrides" :locale="naiveLocale">
    <NMessageProvider>
      <NDialogProvider>
        <router-view />
      </NDialogProvider>
    </NMessageProvider>
  </NConfigProvider>
</template>
