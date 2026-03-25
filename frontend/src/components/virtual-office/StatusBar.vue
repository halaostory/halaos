<template>
  <div class="status-bar">
    <!-- Emoji Picker -->
    <n-popover trigger="click" v-model:show="showEmojiPicker" placement="bottom-start">
      <template #trigger>
        <n-button size="small" quaternary style="min-width: 36px; font-size: 18px">
          {{ customEmoji || '\u{1F60A}' }}
        </n-button>
      </template>
      <div style="width: 276px">
        <div style="font-size: 12px; color: #999; margin-bottom: 6px">
          {{ t('virtualOffice.customEmoji') }}
        </div>
        <div class="emoji-grid">
          <span
            v-for="emoji in emojiList"
            :key="emoji"
            class="emoji-cell"
            :class="{ selected: customEmoji === emoji }"
            @click="toggleEmoji(emoji)"
          >
            {{ emoji }}
          </span>
        </div>
        <a
          v-if="customEmoji"
          class="clear-emoji-link"
          @click="customEmoji = ''; showEmojiPicker = false"
        >
          {{ t('virtualOffice.clearEmoji') }}
        </a>
      </div>
    </n-popover>

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

    <!-- Avatar Customizer Button -->
    <n-button size="small" quaternary @click="showAvatarModal = true">
      {{ t('virtualOffice.chooseAvatar') }}
    </n-button>

    <AvatarCustomizer
      v-model:show="showAvatarModal"
      @saved="avatarSaved"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { NInput, NSelect, NButton, NPopover, useMessage } from 'naive-ui'
import { virtualOfficeAPI } from '../../api/client'
import AvatarCustomizer from './AvatarCustomizer.vue'

const props = defineProps<{ meetingRooms?: { zone_id: string; label: string }[] }>()
const { t } = useI18n()
const message = useMessage()

const customStatus = ref('')
const manualStatus = ref<string | null>(null)
const meetingRoomZone = ref<string | null>(null)
const saving = ref(false)
const customEmoji = ref('')
const showEmojiPicker = ref(false)
const showAvatarModal = ref(false)

const emojiList = [
  '\u{1F60A}', '\u{1F602}', '\u{1F917}', '\u{1F60E}', '\u{1F914}', '\u{1F634}', '\u{1F389}', '\u{1F525}',
  '\u{1F4A1}', '\u{2B50}', '\u{1F4AA}', '\u{1F44D}', '\u{2764}\u{FE0F}', '\u{1F3AF}', '\u{1F680}', '\u{2615}',
  '\u{1F3A7}', '\u{1F91D}', '\u{1F4DD}', '\u{2705}', '\u{1F512}', '\u{1F31F}', '\u{1F4AC}', '\u{1F4CA}',
  '\u{1F3D6}\u{FE0F}', '\u{1F3AE}', '\u{1F3A8}', '\u{1F4F1}', '\u{1F4BB}', '\u{1F3E0}', '\u{23F0}', '\u{1F514}',
]

function toggleEmoji(emoji: string) {
  if (customEmoji.value === emoji) {
    customEmoji.value = ''
  } else {
    customEmoji.value = emoji
  }
  showEmojiPicker.value = false
}

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

function avatarSaved() {
  emit('updated')
}

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
      custom_emoji: customEmoji.value || null,
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

.emoji-grid {
  display: grid;
  grid-template-columns: repeat(8, 32px);
  gap: 4px;
}

.emoji-cell {
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
  cursor: pointer;
  border-radius: 6px;
  transition: background 0.15s;
}

.emoji-cell:hover {
  background: #f0f0f0;
}

.emoji-cell.selected {
  background: #e3f2fd;
}

.clear-emoji-link {
  display: inline-block;
  margin-top: 8px;
  font-size: 12px;
  color: #999;
  cursor: pointer;
  text-decoration: underline;
}

.clear-emoji-link:hover {
  color: #666;
}
</style>
