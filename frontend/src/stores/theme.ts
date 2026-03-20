import { ref, computed, watch } from 'vue'
import { defineStore } from 'pinia'

export const useThemeStore = defineStore('theme', () => {
  const sidebarCollapsed = ref(false)
  const mode = ref<'light' | 'dark'>(
    (localStorage.getItem('theme') as 'light' | 'dark') || 'light'
  )

  const isDark = computed(() => mode.value === 'dark')

  function toggle() {
    mode.value = mode.value === 'dark' ? 'light' : 'dark'
  }

  watch(mode, (val) => {
    localStorage.setItem('theme', val)
    document.documentElement.setAttribute('data-theme', val)
    if (val === 'dark') {
      document.documentElement.classList.add('dark')
    } else {
      document.documentElement.classList.remove('dark')
    }
  }, { immediate: true })

  return { sidebarCollapsed, mode, isDark, toggle }
})
