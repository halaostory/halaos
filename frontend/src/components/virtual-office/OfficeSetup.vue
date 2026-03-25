<template>
  <n-card :title="t('virtualOffice.setup')">
    <n-space vertical>
      <n-radio-group v-model:value="selectedTemplate">
        <n-space>
          <n-radio value="small">{{ t('virtualOffice.small') }}</n-radio>
          <n-radio value="medium">{{ t('virtualOffice.medium') }}</n-radio>
          <n-radio value="large">{{ t('virtualOffice.large') }}</n-radio>
        </n-space>
      </n-radio-group>
      <n-space>
        <n-button type="primary" @click="saveConfig" :loading="saving">
          {{ t('virtualOffice.saveConfig') }}
        </n-button>
        <n-button @click="autoAssign" :loading="assigning">
          {{ t('virtualOffice.autoAssign') }}
        </n-button>
      </n-space>
    </n-space>
  </n-card>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage } from 'naive-ui'
import { virtualOfficeAPI } from '../../api/client'

const props = defineProps<{ currentTemplate?: string }>()
const emit = defineEmits<{ (e: 'saved'): void }>()
const { t } = useI18n()
const message = useMessage()

const selectedTemplate = ref(props.currentTemplate ?? 'small')
const saving = ref(false)
const assigning = ref(false)

async function saveConfig() {
  saving.value = true
  try {
    await virtualOfficeAPI.updateConfig({ template: selectedTemplate.value })
    message.success(t('common.saved'))
    emit('saved')
  } catch {
    message.error(t('common.failed'))
  } finally {
    saving.value = false
  }
}

async function autoAssign() {
  assigning.value = true
  try {
    const res = await virtualOfficeAPI.autoAssign()
    const data = res as { assigned: number }
    message.success(t('virtualOffice.autoAssignSuccess', { count: data.assigned }))
    emit('saved')
  } catch {
    message.error(t('common.failed'))
  } finally {
    assigning.value = false
  }
}
</script>
