<template>
  <div class="office-filter-bar">
    <n-input
      v-model:value="search"
      size="small"
      :placeholder="t('virtualOffice.searchEmployee')"
      clearable
      style="width: 200px"
      @update:value="onFilterChange"
    />
    <n-select
      v-model:value="department"
      size="small"
      :options="departmentOptions"
      clearable
      :placeholder="t('virtualOffice.filterDepartment')"
      style="width: 160px"
      @update:value="onFilterChange"
    />
    <n-select
      v-model:value="status"
      size="small"
      :options="statusOptions"
      clearable
      :placeholder="t('virtualOffice.filterStatus')"
      style="width: 140px"
      @update:value="onFilterChange"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { NInput, NSelect } from 'naive-ui'
import type { SeatData } from './SpriteManager'

const props = defineProps<{ seats: SeatData[] }>()
const { t } = useI18n()

const search = ref('')
const department = ref<string | null>(null)
const status = ref<string | null>(null)

const emit = defineEmits<{ (e: 'filter', matchIds: number[] | null): void }>()

const departmentOptions = computed(() => {
  const deps = new Set(props.seats.map(s => s.department).filter(Boolean))
  return Array.from(deps).sort().map(d => ({ label: d, value: d }))
})

const statusOptions = computed(() => [
  { label: t('virtualOffice.working'), value: 'working' },
  { label: t('virtualOffice.overtime'), value: 'overtime' },
  { label: t('virtualOffice.focused'), value: 'focused' },
  { label: t('virtualOffice.inMeetingStatus'), value: 'in_meeting' },
  { label: t('virtualOffice.onBreak'), value: 'on_break' },
  { label: t('virtualOffice.away'), value: 'away' },
  { label: t('virtualOffice.onLeave'), value: 'on_leave' },
  { label: t('virtualOffice.offline'), value: 'offline' },
])

function onFilterChange() {
  const hasFilter = search.value || department.value || status.value
  if (!hasFilter) {
    emit('filter', null)
    return
  }
  const q = search.value.toLowerCase()
  const matchIds = props.seats
    .filter(s => {
      if (q && !s.name.toLowerCase().includes(q)) return false
      if (department.value && s.department !== department.value) return false
      if (status.value && s.status !== status.value) return false
      return true
    })
    .map(s => s.employee_id)
  emit('filter', matchIds)
}
</script>

<style scoped>
.office-filter-bar {
  display: flex;
  gap: 8px;
  align-items: center;
  padding: 8px 12px;
  background: #fff;
  border-radius: 8px;
  border: 1px solid #f0f0f0;
}
</style>
