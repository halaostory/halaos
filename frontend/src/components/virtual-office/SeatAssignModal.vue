<template>
  <n-modal
    :show="show"
    preset="card"
    :title="t('virtualOffice.assignToSeat')"
    style="max-width: 420px"
    :mask-closable="true"
    @update:show="emit('update:show', $event)"
  >
    <n-spin v-if="loadingEmployees" size="small" style="display: flex; justify-content: center; padding: 24px 0" />
    <template v-else>
      <div v-if="employeeOptions.length === 0" style="text-align: center; padding: 16px 0; color: #999">
        {{ t('virtualOffice.noUnassigned') }}
      </div>
      <template v-else>
        <n-select
          v-model:value="selectedEmployeeId"
          :options="employeeOptions"
          :placeholder="t('virtualOffice.selectEmployee')"
          filterable
          style="margin-bottom: 16px"
        />
        <n-button
          type="primary"
          block
          :disabled="!selectedEmployeeId"
          :loading="assigning"
          @click="assignSeat"
        >
          {{ t('virtualOffice.assignToSeat') }}
        </n-button>
      </template>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { NModal, NSelect, NButton, NSpin, useMessage } from 'naive-ui'
import { virtualOfficeAPI } from '../../api/client'

const props = defineProps<{
  show: boolean
  seatPosition: { floor: number; zone: string; seat_x: number; seat_y: number } | null
}>()

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'assigned'): void
}>()

const { t } = useI18n()
const message = useMessage()

const loadingEmployees = ref(false)
const employeeOptions = ref<{ label: string; value: number }[]>([])
const selectedEmployeeId = ref<number | null>(null)
const assigning = ref(false)

watch(() => props.show, async (visible) => {
  if (!visible) {
    selectedEmployeeId.value = null
    employeeOptions.value = []
    return
  }
  loadingEmployees.value = true
  try {
    const res = await virtualOfficeAPI.listUnassigned() as { data?: { id: number; first_name: string; last_name: string }[] }
    const employees = (res.data || res) as { id: number; first_name: string; last_name: string }[]
    employeeOptions.value = employees.map((emp) => ({
      label: `${emp.first_name} ${emp.last_name}`,
      value: emp.id,
    }))
  } catch {
    message.error(t('common.failed'))
  } finally {
    loadingEmployees.value = false
  }
})

async function assignSeat() {
  if (!selectedEmployeeId.value || !props.seatPosition) return
  assigning.value = true
  try {
    await virtualOfficeAPI.assignSeat({
      employee_id: selectedEmployeeId.value,
      floor: props.seatPosition.floor,
      zone: props.seatPosition.zone,
      seat_x: props.seatPosition.seat_x,
      seat_y: props.seatPosition.seat_y,
    })
    message.success(t('virtualOffice.seatAssigned'))
    emit('assigned')
    emit('update:show', false)
  } catch {
    message.error(t('common.failed'))
  } finally {
    assigning.value = false
  }
}
</script>
