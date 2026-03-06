<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NCard, NSpace, NSelect, NDataTable, NEmpty, NTag, NTime, NPagination,
  type DataTableColumns,
} from 'naive-ui'
import { auditAPI } from '../api/client'

const { t } = useI18n()

interface ActivityLog {
  id: number
  company_id: number
  user_id: number
  action: string
  entity_type: string
  entity_id: string | null
  description: string
  ip_address: string | null
  user_agent: string | null
  metadata: Record<string, unknown>
  created_at: string
  user_email: string
  first_name: string
  last_name: string
}

const logs = ref<ActivityLog[]>([])
const total = ref(0)
const loading = ref(false)
const page = ref(1)
const pageSize = ref(20)
const actionFilter = ref<string | null>(null)
const entityFilter = ref<string | null>(null)

const actionOptions = computed(() => [
  { label: t('audit.allActions'), value: '' },
  { label: t('audit.create'), value: 'create' },
  { label: t('audit.update'), value: 'update' },
  { label: t('audit.delete'), value: 'delete' },
  { label: t('audit.approve'), value: 'approve' },
  { label: t('audit.reject'), value: 'reject' },
  { label: t('audit.login'), value: 'login' },
  { label: t('audit.logout'), value: 'logout' },
])

const entityOptions = computed(() => [
  { label: t('audit.allEntities'), value: '' },
  { label: t('audit.employee'), value: 'employee' },
  { label: t('audit.leave_request'), value: 'leave_request' },
  { label: t('audit.overtime_request'), value: 'overtime_request' },
  { label: t('audit.payroll'), value: 'payroll' },
  { label: t('audit.loan'), value: 'loan' },
  { label: t('audit.attendance'), value: 'attendance' },
  { label: t('audit.schedule'), value: 'schedule' },
])

const actionColor: Record<string, string> = {
  create: 'success',
  update: 'info',
  delete: 'error',
  approve: 'success',
  reject: 'warning',
  login: 'default',
  logout: 'default',
}

const columns = computed<DataTableColumns<ActivityLog>>(() => [
  {
    title: t('audit.timestamp'),
    key: 'created_at',
    width: 170,
    render: (row) => h(NTime, { time: new Date(row.created_at), format: 'yyyy-MM-dd HH:mm:ss' }),
  },
  {
    title: t('audit.user'),
    key: 'user_email',
    width: 200,
    render: (row) => h('div', [
      h('div', { style: 'font-weight: 600;' }, `${row.last_name}, ${row.first_name}`),
      h('div', { style: 'font-size: 12px; color: #999;' }, row.user_email),
    ]),
  },
  {
    title: t('audit.action'),
    key: 'action',
    width: 110,
    render: (row) => h(NTag, { size: 'small', type: (actionColor[row.action] as any) || 'default' }, { default: () => row.action }),
  },
  {
    title: t('audit.entityType'),
    key: 'entity_type',
    width: 130,
    render: (row) => h(NTag, { size: 'small', bordered: false }, { default: () => row.entity_type }),
  },
  {
    title: t('audit.entityId'),
    key: 'entity_id',
    width: 100,
  },
  {
    title: t('audit.description'),
    key: 'description',
    ellipsis: { tooltip: true },
  },
  {
    title: t('audit.ipAddress'),
    key: 'ip_address',
    width: 140,
  },
])

async function loadData() {
  loading.value = true
  try {
    const params: Record<string, string> = {
      page: String(page.value),
      page_size: String(pageSize.value),
    }
    if (actionFilter.value) params.action = actionFilter.value
    if (entityFilter.value) params.entity_type = entityFilter.value

    const res = await auditAPI.list(params)
    const data = (res as any)?.data ?? res
    logs.value = Array.isArray(data?.data) ? data.data : []
    total.value = data?.total ?? 0
  } catch { /* ignore */ }
  loading.value = false
}

function handleActionFilter(val: string) {
  actionFilter.value = val || null
  page.value = 1
  loadData()
}

function handleEntityFilter(val: string) {
  entityFilter.value = val || null
  page.value = 1
  loadData()
}

function handlePageChange(p: number) {
  page.value = p
  loadData()
}

function handlePageSizeChange(size: number) {
  pageSize.value = size
  page.value = 1
  loadData()
}

onMounted(loadData)
</script>

<template>
  <NCard :title="t('audit.title')">
    <NSpace vertical :size="16">
      <NSpace :size="12" align="center">
        <NSelect
          :value="actionFilter ?? ''"
          :options="actionOptions"
          style="width: 180px;"
          @update:value="handleActionFilter"
        />
        <NSelect
          :value="entityFilter ?? ''"
          :options="entityOptions"
          style="width: 200px;"
          @update:value="handleEntityFilter"
        />
      </NSpace>

      <NDataTable
        v-if="logs.length"
        :columns="columns"
        :data="logs"
        :loading="loading"
        :row-key="(row: any) => row.id"
        :scroll-x="1100"
        size="small"
      />
      <NEmpty v-else-if="!loading" :description="t('audit.noLogs')" />

      <NSpace justify="end" v-if="total > pageSize">
        <NPagination
          :page="page"
          :page-size="pageSize"
          :item-count="total"
          :page-sizes="[20, 50, 100]"
          show-size-picker
          @update:page="handlePageChange"
          @update:page-size="handlePageSizeChange"
        />
      </NSpace>
    </NSpace>
  </NCard>
</template>
