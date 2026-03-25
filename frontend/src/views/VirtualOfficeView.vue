<template>
  <div class="vo-standalone">
    <!-- Top bar -->
    <div class="vo-topbar">
      <div class="vo-topbar-left">
        <n-button text @click="goBack">
          <template #icon><n-icon :component="ArrowBackOutline" /></template>
          {{ t('common.back') }}
        </n-button>
        <span class="vo-topbar-title">{{ t('virtualOffice.title') }}</span>
      </div>
      <div class="vo-topbar-right">
        <n-avatar :size="28" round>{{ authStore.fullName?.charAt(0) || 'U' }}</n-avatar>
        <span class="vo-topbar-user">{{ authStore.fullName }}</span>
      </div>
    </div>

    <!-- Content -->
    <div class="vo-content">
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
        <!-- Filter bar -->
        <OfficeFilterBar :seats="snapshot?.seats ?? []" @filter="onFilter" />

        <!-- Stats bar -->
        <OfficeStats :stats="snapshot?.stats ?? {}" />

        <!-- Admin setup toggle -->
        <n-collapse v-if="isAdmin" style="margin: 8px 0">
          <n-collapse-item :title="t('virtualOffice.setup')">
            <OfficeSetup :current-template="config?.template" @saved="loadData" />
          </n-collapse-item>
        </n-collapse>

        <!-- Canvas + sidebar -->
        <div class="vo-main">
          <div class="vo-canvas-wrap">
            <OfficeCanvas
              ref="officeCanvasRef"
              :template="template"
              :seats="snapshot?.seats ?? []"
              :is-admin="isAdmin"
              @select="selectedSeat = $event"
              @empty-seat-click="onEmptySeatClick"
            />
          </div>
          <div class="vo-sidebar">
            <MiniMap
              v-if="template"
              :template-width="template.width"
              :template-height="template.height"
              :tile-size="template.tileSize"
              :seats="snapshot?.seats ?? []"
            />
            <SeatInfoCard v-if="selectedSeat" :seat="selectedSeat" :is-admin="isAdmin" @remove-seat="onRemoveSeat" style="margin-top: 12px" />
          </div>
        </div>

        <!-- Status bar -->
        <StatusBar :meeting-rooms="snapshot?.meeting_rooms as any" @updated="fetchSnapshot" />

        <!-- Seat assignment modal -->
        <SeatAssignModal v-model:show="showAssignModal" :seat-position="assignSeatPosition" @assigned="fetchSnapshot" />
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { NButton, NIcon, NAvatar, NResult, NSpin, NCollapse, NCollapseItem, useMessage } from 'naive-ui'
import { ArrowBackOutline } from '@vicons/ionicons5'
import { useAuthStore } from '../stores/auth'
import { virtualOfficeAPI } from '../api/client'
import OfficeCanvas from '../components/virtual-office/OfficeCanvas.vue'
import OfficeStats from '../components/virtual-office/OfficeStats.vue'
import OfficeSetup from '../components/virtual-office/OfficeSetup.vue'
import StatusBar from '../components/virtual-office/StatusBar.vue'
import SeatInfoCard from '../components/virtual-office/SeatInfoCard.vue'
import SeatAssignModal from '../components/virtual-office/SeatAssignModal.vue'
import OfficeFilterBar from '../components/virtual-office/OfficeFilterBar.vue'
import MiniMap from '../components/virtual-office/MiniMap.vue'
import type { SeatData } from '../components/virtual-office/SpriteManager'
import type { OfficeTemplate } from '../components/virtual-office/OfficeRenderer'

const router = useRouter()
const { t } = useI18n()
const authStore = useAuthStore()

const loading = ref(true)
const config = ref<{ template: string } | null>(null)
const snapshot = ref<{ template: string; stats: Record<string, number>; seats: SeatData[]; meeting_rooms: unknown[] } | null>(null)
const template = ref<OfficeTemplate | null>(null)
const selectedSeat = ref<SeatData | null>(null)
const officeCanvasRef = ref<InstanceType<typeof OfficeCanvas> | null>(null)
const showAssignModal = ref(false)
const assignSeatPosition = ref<{ floor: number; zone: string; seat_x: number; seat_y: number } | null>(null)

const isAdmin = computed(() => authStore.isAdmin)
const message = useMessage()

let pollTimer: ReturnType<typeof setInterval> | null = null

function goBack() {
  router.push({ name: 'dashboard' })
}

async function loadData() {
  loading.value = true
  try {
    const cfgRes = await virtualOfficeAPI.getConfig() as { data?: { template: string } }
    config.value = (cfgRes.data || cfgRes) as { template: string }

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
    const res = await virtualOfficeAPI.getSnapshot() as { data?: typeof snapshot.value }
    snapshot.value = (res.data || res) as typeof snapshot.value
  } catch {
    // Silent fail — will retry on next poll
  }
}

function onEmptySeatClick(position: { floor: number; zone: string; seat_x: number; seat_y: number }) {
  assignSeatPosition.value = position
  showAssignModal.value = true
}

function onFilter(matchIds: number[] | null) {
  officeCanvasRef.value?.setFilter(matchIds)
}

async function onRemoveSeat(employeeId: number) {
  try {
    await virtualOfficeAPI.removeSeat(employeeId)
    message.success(t('common.success'))
    selectedSeat.value = null
    await fetchSnapshot()
  } catch {
    message.error(t('common.failed'))
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

<style scoped>
.vo-standalone {
  min-height: 100vh;
  background: #FAFAFA;
  display: flex;
  flex-direction: column;
}
.vo-topbar {
  height: 52px;
  background: #fff;
  border-bottom: 1px solid #f0f0f0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  flex-shrink: 0;
}
.vo-topbar-left {
  display: flex;
  align-items: center;
  gap: 16px;
}
.vo-topbar-title {
  font-size: 16px;
  font-weight: 600;
  color: #333;
}
.vo-topbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
}
.vo-topbar-user {
  font-size: 13px;
  color: #666;
}
.vo-content {
  flex: 1;
  padding: 16px 20px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.vo-main {
  display: flex;
  gap: 16px;
  flex: 1;
}
.vo-canvas-wrap {
  flex: 1;
  background: #fff;
  border-radius: 12px;
  border: 1px solid #f0f0f0;
  box-shadow: 0 2px 8px rgba(0,0,0,0.04);
  padding: 12px;
  overflow: hidden;
}
.vo-sidebar {
  width: 190px;
  flex-shrink: 0;
}
</style>
