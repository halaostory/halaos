<script setup lang="ts">
import { ref, h, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NCard, NDataTable, NTag, NButton, NSpace, NModal, NForm, NFormItem,
  NSelect, NInput, useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { userAPI } from '../api/client'

const { t } = useI18n()
const message = useMessage()

interface UserRow {
  id: number
  email: string
  first_name: string
  last_name: string
  role: string
  status: string
  avatar_url: string | null
  last_login_at: string | null
  created_at: string
}

const users = ref<UserRow[]>([])
const loading = ref(false)

// Role modal
const showRoleModal = ref(false)
const roleTarget = ref<UserRow | null>(null)
const newRole = ref('')

// Status modal
const showStatusModal = ref(false)
const statusTarget = ref<UserRow | null>(null)
const newStatus = ref('')

// Password modal
const showPasswordModal = ref(false)
const passwordTarget = ref<UserRow | null>(null)
const newPassword = ref('')

const roleOptions = computed(() => [
  { label: t('userMgmt.admin'), value: 'admin' },
  { label: t('userMgmt.manager'), value: 'manager' },
  { label: t('userMgmt.employee'), value: 'employee' },
])

const statusOptions = computed(() => [
  { label: t('userMgmt.active'), value: 'active' },
  { label: t('userMgmt.inactive'), value: 'inactive' },
  { label: t('userMgmt.suspended'), value: 'suspended' },
])

const statusColor: Record<string, 'success' | 'default' | 'error'> = {
  active: 'success',
  inactive: 'default',
  suspended: 'error',
}

const roleColor: Record<string, 'info' | 'warning' | 'default'> = {
  admin: 'warning',
  manager: 'info',
  employee: 'default',
  super_admin: 'error' as any,
}

const columns = computed<DataTableColumns<UserRow>>(() => [
  {
    title: t('userMgmt.name'),
    key: 'name',
    render: (row) => h('span', {}, `${row.first_name} ${row.last_name}`),
  },
  { title: t('userMgmt.email'), key: 'email' },
  {
    title: t('userMgmt.role'),
    key: 'role',
    width: 120,
    render: (row) => h(NTag, { type: roleColor[row.role] || 'default', size: 'small' }, () => row.role),
  },
  {
    title: t('userMgmt.status'),
    key: 'status',
    width: 100,
    render: (row) => h(NTag, { type: statusColor[row.status] || 'default', size: 'small' }, () => t(`userMgmt.${row.status}`)),
  },
  {
    title: t('userMgmt.lastLogin'),
    key: 'last_login_at',
    width: 160,
    render: (row) => {
      if (!row.last_login_at) return h('span', { style: 'color: #999' }, 'Never')
      return h('span', {}, new Date(row.last_login_at).toLocaleString())
    },
  },
  {
    title: t('common.actions'),
    key: 'actions',
    width: 280,
    render: (row) => h(NSpace, { size: 4 }, () => [
      h(NButton, { size: 'small', onClick: () => openRoleModal(row) }, () => t('userMgmt.changeRole')),
      h(NButton, { size: 'small', onClick: () => openStatusModal(row) }, () => t('userMgmt.changeStatus')),
      h(NButton, { size: 'small', type: 'warning', onClick: () => openPasswordModal(row) }, () => t('userMgmt.resetPassword')),
    ]),
  },
])

async function loadUsers() {
  loading.value = true
  try {
    const res = await userAPI.list() as { data?: { users: UserRow[]; total: number } }
    const data = (res.data || res) as { users: UserRow[]; total: number }
    users.value = data.users || []
  } catch { message.error(t('common.loadFailed')) }
  finally { loading.value = false }
}

onMounted(loadUsers)

function openRoleModal(user: UserRow) {
  roleTarget.value = user
  newRole.value = user.role
  showRoleModal.value = true
}

async function handleChangeRole() {
  if (!roleTarget.value) return
  try {
    await userAPI.updateRole(roleTarget.value.id, newRole.value)
    message.success(t('userMgmt.roleUpdated'))
    showRoleModal.value = false
    await loadUsers()
  } catch { message.error(t('common.failed')) }
}

function openStatusModal(user: UserRow) {
  statusTarget.value = user
  newStatus.value = user.status
  showStatusModal.value = true
}

async function handleChangeStatus() {
  if (!statusTarget.value) return
  try {
    await userAPI.updateStatus(statusTarget.value.id, newStatus.value)
    message.success(t('userMgmt.statusUpdated'))
    showStatusModal.value = false
    await loadUsers()
  } catch { message.error(t('common.failed')) }
}

function openPasswordModal(user: UserRow) {
  passwordTarget.value = user
  newPassword.value = ''
  showPasswordModal.value = true
}

async function handleResetPassword() {
  if (!passwordTarget.value || newPassword.value.length < 8) return
  try {
    await userAPI.resetPassword(passwordTarget.value.id, newPassword.value)
    message.success(t('userMgmt.passwordReset'))
    showPasswordModal.value = false
  } catch { message.error(t('common.failed')) }
}
</script>

<template>
  <NCard :title="t('userMgmt.title')">
    <NDataTable
      :columns="columns"
      :data="users"
      :loading="loading"
      :bordered="false"
      :row-key="(row: UserRow) => row.id"
    />

    <!-- Change Role Modal -->
    <NModal v-model:show="showRoleModal" preset="card" :title="t('userMgmt.changeRole')" style="max-width: 400px; width: 95vw;">
      <p>{{ roleTarget?.first_name }} {{ roleTarget?.last_name }} ({{ roleTarget?.email }})</p>
      <NForm @submit.prevent="handleChangeRole">
        <NFormItem :label="t('userMgmt.role')">
          <NSelect v-model:value="newRole" :options="roleOptions" />
        </NFormItem>
        <NButton type="primary" attr-type="submit">{{ t('common.save') }}</NButton>
      </NForm>
    </NModal>

    <!-- Change Status Modal -->
    <NModal v-model:show="showStatusModal" preset="card" :title="t('userMgmt.changeStatus')" style="max-width: 400px; width: 95vw;">
      <p>{{ statusTarget?.first_name }} {{ statusTarget?.last_name }} ({{ statusTarget?.email }})</p>
      <NForm @submit.prevent="handleChangeStatus">
        <NFormItem :label="t('userMgmt.status')">
          <NSelect v-model:value="newStatus" :options="statusOptions" />
        </NFormItem>
        <NButton type="primary" attr-type="submit">{{ t('common.save') }}</NButton>
      </NForm>
    </NModal>

    <!-- Reset Password Modal -->
    <NModal v-model:show="showPasswordModal" preset="card" :title="t('userMgmt.resetPassword')" style="max-width: 400px; width: 95vw;">
      <p>{{ passwordTarget?.first_name }} {{ passwordTarget?.last_name }} ({{ passwordTarget?.email }})</p>
      <NForm @submit.prevent="handleResetPassword">
        <NFormItem :label="t('userMgmt.newPassword')">
          <NInput v-model:value="newPassword" type="password" show-password-on="click" placeholder="Min 8 characters" />
        </NFormItem>
        <NButton type="warning" attr-type="submit" :disabled="newPassword.length < 8">{{ t('userMgmt.resetPassword') }}</NButton>
      </NForm>
    </NModal>
  </NCard>
</template>
