<template>
  <n-card v-if="seat" size="small" :bordered="true" style="width: 260px">
    <div style="display: flex; align-items: center; gap: 12px; margin-bottom: 8px">
      <div :style="{ width: '40px', height: '40px', borderRadius: '50%', backgroundColor: seat.avatar_color, display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#fff', fontWeight: 'bold', fontSize: '16px' }">
        {{ seat.name.charAt(0) }}
      </div>
      <div>
        <div style="font-weight: 600">{{ seat.name }}</div>
        <div style="font-size: 12px; color: #999">{{ seat.position }}</div>
      </div>
    </div>
    <n-descriptions :column="1" label-placement="left" size="small">
      <n-descriptions-item :label="t('common.department')">{{ seat.department }}</n-descriptions-item>
      <n-descriptions-item :label="t('common.status')">
        <n-tag :type="statusType" size="small">{{ statusLabel }}</n-tag>
      </n-descriptions-item>
      <n-descriptions-item v-if="seat.custom_status" :label="t('virtualOffice.setStatus')">
        {{ seat.custom_emoji ?? '' }} {{ seat.custom_status }}
      </n-descriptions-item>
      <n-descriptions-item v-if="seat.clock_in_at" :label="t('virtualOffice.clockedIn')">
        {{ new Date(seat.clock_in_at).toLocaleTimeString() }}
      </n-descriptions-item>
      <n-descriptions-item v-if="seat.leave_type" :label="t('virtualOffice.leaveType')">
        {{ seat.leave_type }}
      </n-descriptions-item>
    </n-descriptions>
  </n-card>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SeatData } from './SpriteManager'

const props = defineProps<{ seat: SeatData | null }>()
const { t } = useI18n()

const statusType = computed(() => {
  switch (props.seat?.status) {
    case 'working': return 'success'
    case 'overtime': return 'warning'
    case 'focused': return 'info'
    case 'in_meeting': return 'info'
    case 'on_leave': return 'error'
    case 'offline': return 'default'
    default: return 'default'
  }
})

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
