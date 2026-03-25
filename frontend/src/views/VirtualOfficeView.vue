<template>
  <n-space vertical :size="12">
    <n-page-header :title="t('virtualOffice.title')" />

    <!-- No config yet: show setup -->
    <template v-if="!config && !loading">
      <n-result status="info" :title="t('virtualOffice.noOffice')" :description="t('virtualOffice.setupFirst')">
        <template #footer>
          <OfficeSetup @saved="loadData" />
        </template>
      </n-result>
    </template>

    <!-- Loading -->
    <n-spin v-else-if="loading" size="large" style="display: flex; justify-content: center; padding: 80px 0" />

    <!-- Office view -->
    <template v-else>
      <!-- Stats bar -->
      <OfficeStats :stats="snapshot?.stats ?? {}" />

      <!-- Admin setup toggle -->
      <n-collapse v-if="isAdmin" style="margin-bottom: 8px">
        <n-collapse-item :title="t('virtualOffice.setup')">
          <OfficeSetup :current-template="config?.template" @saved="loadData" />
        </n-collapse-item>
      </n-collapse>

      <!-- Canvas + sidebar -->
      <div style="display: flex; gap: 12px">
        <div style="flex: 1; position: relative">
          <OfficeCanvas
            :template="template"
            :seats="snapshot?.seats ?? []"
            @select="selectedSeat = $event"
          />
        </div>
        <div style="width: 170px">
          <MiniMap
            v-if="template"
            :template-width="template.width"
            :template-height="template.height"
            :tile-size="template.tileSize"
            :seats="snapshot?.seats ?? []"
          />
          <SeatInfoCard v-if="selectedSeat" :seat="selectedSeat" style="margin-top: 12px" />
        </div>
      </div>

      <!-- Status bar -->
      <StatusBar :meeting-rooms="snapshot?.meeting_rooms as any" @updated="fetchSnapshot" />
    </template>
  </n-space>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '../stores/auth'
import { virtualOfficeAPI } from '../api/client'
import OfficeCanvas from '../components/virtual-office/OfficeCanvas.vue'
import OfficeStats from '../components/virtual-office/OfficeStats.vue'
import OfficeSetup from '../components/virtual-office/OfficeSetup.vue'
import StatusBar from '../components/virtual-office/StatusBar.vue'
import SeatInfoCard from '../components/virtual-office/SeatInfoCard.vue'
import MiniMap from '../components/virtual-office/MiniMap.vue'
import type { SeatData } from '../components/virtual-office/SpriteManager'
import type { OfficeTemplate } from '../components/virtual-office/OfficeRenderer'

const { t } = useI18n()
const authStore = useAuthStore()

const loading = ref(true)
const config = ref<{ template: string } | null>(null)
const snapshot = ref<{ template: string; stats: Record<string, number>; seats: SeatData[]; meeting_rooms: unknown[] } | null>(null)
const template = ref<OfficeTemplate | null>(null)
const selectedSeat = ref<SeatData | null>(null)

const isAdmin = computed(() => authStore.isAdmin)

let pollTimer: ReturnType<typeof setInterval> | null = null

async function loadData() {
  loading.value = true
  try {
    const cfgRes = await virtualOfficeAPI.getConfig()
    config.value = cfgRes as { template: string }

    // Load template JSON (validate against allowlist before dynamic import)
    const allowedTemplates = ['small', 'medium', 'large']
    if (!allowedTemplates.includes(config.value.template)) {
      config.value = null
      return
    }
    const tmplModule = await import(`../assets/virtual-office/templates/${config.value.template}.json`)
    template.value = tmplModule.default as OfficeTemplate

    await fetchSnapshot()
  } catch {
    config.value = null
  } finally {
    loading.value = false
  }
}

async function fetchSnapshot() {
  try {
    const res = await virtualOfficeAPI.getSnapshot()
    snapshot.value = res as typeof snapshot.value
  } catch {
    // Silent fail — will retry on next poll
  }
}

onMounted(async () => {
  await loadData()
  pollTimer = setInterval(fetchSnapshot, 30000)
})

onBeforeUnmount(() => {
  if (pollTimer) clearInterval(pollTimer)
})
</script>
