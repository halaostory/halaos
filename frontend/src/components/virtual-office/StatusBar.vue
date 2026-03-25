<template>
  <div class="status-bar">
    <n-input
      v-model:value="customStatus"
      :placeholder="t('virtualOffice.statusPlaceholder')"
      size="small"
      style="flex: 1; max-width: 300px"
      @keyup.enter="saveStatus"
    />
    <n-select
      v-model:value="manualStatus"
      size="small"
      :options="statusOptions"
      style="width: 140px"
      clearable
      :placeholder="t('virtualOffice.setStatus')"
    />
    <n-select
      v-if="manualStatus === 'in_meeting'"
      v-model:value="meetingRoomZone"
      size="small"
      :options="meetingRoomOptions"
      style="width: 160px"
      :placeholder="t('virtualOffice.meetingRoom')"
    />
    <n-button size="small" type="primary" @click="saveStatus" :loading="saving">
      {{ t('virtualOffice.setStatus') }}
    </n-button>
    <n-button v-if="manualStatus" size="small" quaternary @click="clearStatus">
      {{ t('virtualOffice.clearStatus') }}
    </n-button>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { NInput, NSelect, NButton, useMessage } from 'naive-ui'
import { virtualOfficeAPI } from '../../api/client'

const props = defineProps<{ meetingRooms?: { zone_id: string; label: string }[] }>()
const { t } = useI18n()
const message = useMessage()

const customStatus = ref('')
const manualStatus = ref<string | null>(null)
const meetingRoomZone = ref<string | null>(null)
const saving = ref(false)

const statusOptions = computed(() => [
  { label: t('virtualOffice.focused'), value: 'focused' },
  { label: t('virtualOffice.inMeetingStatus'), value: 'in_meeting' },
  { label: t('virtualOffice.onBreak'), value: 'on_break' },
  { label: t('virtualOffice.away'), value: 'away' },
])

const meetingRoomOptions = computed(() =>
  (props.meetingRooms ?? []).map(r => ({ label: r.label, value: r.zone_id }))
)

const emit = defineEmits<{ (e: 'updated'): void }>()

async function saveStatus() {
  if (manualStatus.value === 'in_meeting' && !meetingRoomZone.value) {
    message.warning(t('virtualOffice.meetingRoom') + ' is required')
    return
  }
  saving.value = true
  try {
    await virtualOfficeAPI.updateMyStatus({
      custom_status: customStatus.value || null,
      manual_status: manualStatus.value,
      meeting_room_zone: manualStatus.value === 'in_meeting' ? meetingRoomZone.value : null,
    })
    message.success(t('virtualOffice.statusSaved'))
    emit('updated')
  } catch {
    message.error(t('common.failed'))
  } finally {
    saving.value = false
  }
}

async function clearStatus() {
  saving.value = true
  try {
    await virtualOfficeAPI.updateMyStatus({ manual_status: null, meeting_room_zone: null })
    manualStatus.value = null
    meetingRoomZone.value = null
    message.success(t('virtualOffice.statusSaved'))
    emit('updated')
  } catch {
    message.error(t('common.failed'))
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.status-bar {
  display: flex;
  gap: 8px;
  align-items: center;
  padding: 10px 16px;
  background: #fff;
  border-radius: 10px;
  border: 1px solid #f0f0f0;
  box-shadow: 0 1px 3px rgba(0,0,0,0.04);
}
</style>
