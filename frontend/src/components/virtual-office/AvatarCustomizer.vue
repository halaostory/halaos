<template>
  <n-modal
    v-model:show="showModal"
    preset="card"
    :title="t('virtualOffice.chooseAvatar')"
    style="width: 480px; max-width: 95vw"
    :mask-closable="true"
  >
    <!-- Avatar Type Section -->
    <div style="margin-bottom: 20px">
      <div style="font-weight: 600; margin-bottom: 8px">{{ t('virtualOffice.people') }}</div>
      <div class="avatar-grid">
        <div
          v-for="avatar in peopleAvatars"
          :key="avatar.type"
          class="avatar-item"
          :class="{ selected: selectedType === avatar.type }"
          @click="selectedType = avatar.type"
        >
          <div class="avatar-circle" :class="{ selected: selectedType === avatar.type }">
            <span class="avatar-emoji">{{ avatar.emoji }}</span>
          </div>
          <span class="avatar-label">{{ avatar.label }}</span>
        </div>
      </div>

      <div style="font-weight: 600; margin-bottom: 8px; margin-top: 16px">{{ t('virtualOffice.animals') }}</div>
      <div class="avatar-grid">
        <div
          v-for="avatar in animalAvatars"
          :key="avatar.type"
          class="avatar-item"
          :class="{ selected: selectedType === avatar.type }"
          @click="selectedType = avatar.type"
        >
          <div class="avatar-circle" :class="{ selected: selectedType === avatar.type }">
            <span class="avatar-emoji">{{ avatar.emoji }}</span>
          </div>
          <span class="avatar-label">{{ avatar.label }}</span>
        </div>
      </div>
    </div>

    <!-- Color Section -->
    <div>
      <div style="font-weight: 600; margin-bottom: 8px">{{ t('virtualOffice.chooseColor') }}</div>
      <div class="color-grid">
        <div
          v-for="color in presetColors"
          :key="color"
          class="color-swatch"
          :class="{ selected: selectedColor === color }"
          :style="{ backgroundColor: color }"
          @click="selectedColor = color"
        />
      </div>
      <n-input
        v-model:value="selectedColor"
        size="small"
        :placeholder="t('virtualOffice.colorHex')"
        style="margin-top: 10px; max-width: 180px"
      />
    </div>

    <template #footer>
      <div style="display: flex; justify-content: flex-end; gap: 8px">
        <n-button size="small" @click="showModal = false">
          {{ t('common.cancel') }}
        </n-button>
        <n-button size="small" type="primary" :loading="saving" @click="handleSave">
          {{ t('common.save') }}
        </n-button>
      </div>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { NModal, NButton, NInput, useMessage } from 'naive-ui'
import { virtualOfficeAPI } from '../../api/client'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'saved'): void
}>()

const { t } = useI18n()
const message = useMessage()

const showModal = computed({
  get: () => props.show,
  set: (val: boolean) => emit('update:show', val),
})

const selectedType = ref('person_1')
const selectedColor = ref('#4A90D9')
const saving = ref(false)

const peopleAvatars = [
  { type: 'person_1', emoji: '\u{1F464}', label: 'Person 1' },
  { type: 'person_2', emoji: '\u{1F464}', label: 'Person 2' },
  { type: 'person_3', emoji: '\u{1F464}', label: 'Person 3' },
  { type: 'person_4', emoji: '\u{1F464}', label: 'Person 4' },
  { type: 'person_5', emoji: '\u{1F464}', label: 'Person 5' },
  { type: 'person_6', emoji: '\u{1F464}', label: 'Person 6' },
]

const animalAvatars = [
  { type: 'cat', emoji: '\u{1F431}', label: 'Cat' },
  { type: 'dog', emoji: '\u{1F436}', label: 'Dog' },
  { type: 'rabbit', emoji: '\u{1F430}', label: 'Rabbit' },
  { type: 'bear', emoji: '\u{1F43B}', label: 'Bear' },
  { type: 'penguin', emoji: '\u{1F427}', label: 'Penguin' },
  { type: 'shiba', emoji: '\u{1F415}', label: 'Shiba' },
]

const presetColors = [
  '#4A90D9', '#E53935', '#43A047', '#FB8C00',
  '#8E24AA', '#00ACC1', '#F4511E', '#6D4C41',
  '#1E88E5', '#D81B60', '#00897B', '#FFB300',
  '#5E35B1', '#039BE5', '#C0CA33', '#546E7A',
]

async function handleSave() {
  saving.value = true
  try {
    await virtualOfficeAPI.updateMyAvatar({
      avatar_type: selectedType.value,
      avatar_color: selectedColor.value,
    })
    message.success(t('virtualOffice.avatarSaved'))
    emit('saved')
    showModal.value = false
  } catch {
    message.error(t('common.failed'))
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.avatar-grid {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  gap: 12px;
}

.avatar-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  cursor: pointer;
  gap: 4px;
}

.avatar-circle {
  width: 48px;
  height: 48px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f5f5f5;
  border: 2px solid transparent;
  transition: border-color 0.2s;
}

.avatar-circle.selected {
  border-color: var(--n-color, #18a058);
  border-width: 3px;
}

.avatar-emoji {
  font-size: 22px;
}

.avatar-label {
  font-size: 11px;
  color: #666;
  text-align: center;
}

.color-grid {
  display: grid;
  grid-template-columns: repeat(8, 28px);
  gap: 8px;
}

.color-swatch {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  cursor: pointer;
  border: 2px solid transparent;
  transition: border-color 0.2s, box-shadow 0.2s;
}

.color-swatch:hover {
  transform: scale(1.1);
}

.color-swatch.selected {
  border-color: #333;
  box-shadow: 0 0 0 2px #fff, 0 0 0 4px #333;
}
</style>
