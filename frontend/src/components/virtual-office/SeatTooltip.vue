<template>
  <div
    v-if="seat"
    class="seat-tooltip"
    :style="{ left: position.x + 'px', top: position.y + 'px' }"
  >
    <div class="tooltip-row">
      <div class="tooltip-avatar" :style="{ backgroundColor: seat.avatar_color }">
        {{ seat.name.charAt(0) }}
      </div>
      <div class="tooltip-info">
        <div class="tooltip-name">{{ seat.name }}</div>
        <div class="tooltip-dept">{{ seat.department }}</div>
      </div>
      <div class="tooltip-status">
        <span class="status-dot" :style="{ backgroundColor: statusColor }" />
        <span class="status-label">{{ statusLabel }}</span>
      </div>
    </div>
    <div class="tooltip-arrow" />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SeatData } from './SpriteManager'

const props = defineProps<{
  seat: SeatData | null
  position: { x: number; y: number }
}>()
const { t } = useI18n()

const STATUS_COLORS: Record<string, string> = {
  working: '#4CAF50',
  overtime: '#FF9800',
  focused: '#2196F3',
  in_meeting: '#9C27B0',
  on_break: '#FF9800',
  away: '#9E9E9E',
  on_leave: '#E53935',
  offline: '#BDBDBD',
}

const statusColor = computed(() => STATUS_COLORS[props.seat?.status ?? 'offline'] ?? '#BDBDBD')

const statusLabel = computed(() => {
  const key = props.seat?.status ?? 'offline'
  const map: Record<string, string> = {
    working: t('virtualOffice.working'),
    overtime: t('virtualOffice.overtime'),
    focused: t('virtualOffice.focused'),
    in_meeting: t('virtualOffice.inMeetingStatus'),
    on_break: t('virtualOffice.onBreak'),
    away: t('virtualOffice.away'),
    on_leave: t('virtualOffice.onLeave'),
    offline: t('virtualOffice.offline'),
  }
  return map[key] ?? key
})
</script>

<style scoped>
.seat-tooltip {
  position: absolute;
  pointer-events: none;
  z-index: 100;
  background: #fff;
  border-radius: 8px;
  padding: 8px 12px;
  box-shadow: 0 4px 12px rgba(0,0,0,0.15);
  border: 1px solid #e8e8e8;
  transform: translate(-50%, -100%);
  margin-top: -12px;
  white-space: nowrap;
}
.tooltip-row {
  display: flex;
  align-items: center;
  gap: 8px;
}
.tooltip-avatar {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-weight: 700;
  font-size: 13px;
  flex-shrink: 0;
}
.tooltip-info {
  min-width: 0;
}
.tooltip-name {
  font-weight: 600;
  font-size: 13px;
  color: #333;
}
.tooltip-dept {
  font-size: 11px;
  color: #999;
}
.tooltip-status {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-left: 8px;
}
.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}
.status-label {
  font-size: 11px;
  color: #666;
}
.tooltip-arrow {
  position: absolute;
  bottom: -6px;
  left: 50%;
  transform: translateX(-50%) rotate(45deg);
  width: 10px;
  height: 10px;
  background: #fff;
  border-right: 1px solid #e8e8e8;
  border-bottom: 1px solid #e8e8e8;
}
</style>
