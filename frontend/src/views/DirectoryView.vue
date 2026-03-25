<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  NInput, NSelect, NSpace, NCard, NGrid, NGi, NAvatar,
  NTag, NSpin, NRadioGroup, NRadioButton,
} from 'naive-ui'
import { directoryAPI, companyAPI } from '../api/client'
import EmptyState from '../components/EmptyState.vue'

const { t } = useI18n()
const router = useRouter()

const loading = ref(false)
const viewMode = ref<'list' | 'orgchart'>('list')
const search = ref('')
const departmentId = ref<string | null>(null)

interface DirectoryEmployee {
  id: number
  employee_no: string
  first_name: string
  last_name: string
  display_name: string | null
  email: string | null
  phone: string | null
  status: string
  employment_type: string
  manager_id: number | null
  department_name: string
  position_title: string
  avatar_url: string | null
}

interface OrgNode {
  id: number
  first_name: string
  last_name: string
  display_name: string | null
  manager_id: number | null
  department_name: string
  position_title: string
  avatar_url: string | null
  children: OrgNode[]
}

interface FlatOrgItem {
  node: OrgNode
  depth: number
  childCount: number
  isLast: boolean[]
}

const employees = ref<DirectoryEmployee[]>([])
const orgData = ref<OrgNode[]>([])
const collapsed = ref<Set<number>>(new Set())
const departments = ref<{ label: string; value: string }[]>([])

const departmentOptions = computed(() => [
  { label: t('directory.allDepartments'), value: '' },
  ...departments.value,
])

function toggleCollapse(id: number) {
  const s = new Set(collapsed.value)
  if (s.has(id)) s.delete(id)
  else s.add(id)
  collapsed.value = s
}

// Flatten the org tree for rendering with tree lines
const flatOrgList = computed<FlatOrgItem[]>(() => {
  const result: FlatOrgItem[] = []
  function walk(nodes: OrgNode[], depth: number, isLast: boolean[]) {
    for (let i = 0; i < nodes.length; i++) {
      const node = nodes[i]
      const lastFlags = [...isLast, i === nodes.length - 1]
      result.push({ node, depth, childCount: node.children.length, isLast: lastFlags })
      if (node.children.length > 0 && !collapsed.value.has(node.id)) {
        walk(node.children, depth + 1, lastFlags)
      }
    }
  }
  walk(orgData.value, 0, [])
  return result
})

let searchTimer: ReturnType<typeof setTimeout> | null = null

function debouncedFetch() {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(fetchDirectory, 300)
}

async function fetchDirectory() {
  loading.value = true
  try {
    const params: Record<string, string> = {}
    if (search.value.trim()) params.search = search.value.trim()
    if (departmentId.value) params.department_id = departmentId.value
    const res = await directoryAPI.list(params)
    const data = (res as any)?.data ?? res
    employees.value = Array.isArray(data) ? data : []
  } catch {
    employees.value = []
  } finally {
    loading.value = false
  }
}

async function fetchOrgChart() {
  loading.value = true
  try {
    const res = await directoryAPI.orgChart()
    const data = (res as any)?.data ?? res
    const flat: OrgNode[] = Array.isArray(data) ? data.map((e: any) => ({ ...e, children: [] as OrgNode[] })) : []

    // Build tree
    const map = new Map<number, OrgNode>()
    flat.forEach(n => map.set(n.id, n))

    const roots: OrgNode[] = []
    flat.forEach(n => {
      if (n.manager_id && map.has(n.manager_id)) {
        map.get(n.manager_id)!.children.push(n)
      } else {
        roots.push(n)
      }
    })
    orgData.value = roots
  } catch {
    orgData.value = []
  } finally {
    loading.value = false
  }
}

async function fetchDepartments() {
  try {
    const res = await companyAPI.listDepartments()
    const data = (res as any)?.data ?? res
    departments.value = (Array.isArray(data) ? data : []).map((d: any) => ({
      label: d.name,
      value: String(d.id),
    }))
  } catch (e) { console.error('Failed to load departments', e) }
}

function switchView(mode: 'list' | 'orgchart') {
  viewMode.value = mode
  if (mode === 'orgchart') {
    fetchOrgChart()
  } else {
    fetchDirectory()
  }
}

function getName(emp: { first_name: string; last_name: string; display_name?: string | null }) {
  return emp.display_name || `${emp.first_name} ${emp.last_name}`
}

function getInitial(emp: { first_name: string; last_name: string }) {
  return (emp.first_name.charAt(0) + emp.last_name.charAt(0)).toUpperCase()
}

function goToProfile(id: number) {
  router.push({ name: 'employee-detail', params: { id } })
}

onMounted(() => {
  fetchDirectory()
  fetchDepartments()
})
</script>

<template>
  <div>
    <NSpace justify="space-between" align="center" style="margin-bottom: 16px;">
      <h2 style="margin: 0;">{{ t('directory.title') }}</h2>
      <NRadioGroup :value="viewMode" @update:value="switchView" size="small">
        <NRadioButton value="list">{{ t('directory.listView') }}</NRadioButton>
        <NRadioButton value="orgchart">{{ t('directory.orgChart') }}</NRadioButton>
      </NRadioGroup>
    </NSpace>

    <!-- Search & Filter (list view) -->
    <NSpace v-if="viewMode === 'list'" style="margin-bottom: 16px;" :size="12">
      <NInput
        v-model:value="search"
        :placeholder="t('directory.search')"
        clearable
        style="width: 360px;"
        @update:value="debouncedFetch"
      />
      <NSelect
        :value="departmentId || ''"
        :options="departmentOptions"
        style="width: 200px;"
        @update:value="(v: string) => { departmentId = v || null; fetchDirectory() }"
      />
    </NSpace>

    <NSpin :show="loading">
      <!-- List View -->
      <template v-if="viewMode === 'list'">
        <EmptyState
          v-if="employees.length === 0 && !loading"
          icon="👥"
          :title="t('emptyState.directory.title')"
          :description="t('emptyState.directory.desc')"
        />
        <NGrid v-else :cols="4" :x-gap="16" :y-gap="16" responsive="screen" :item-responsive="true">
          <NGi v-for="emp in employees" :key="emp.id" span="0:4 600:2 900:1">
            <NCard hoverable style="cursor: pointer;" @click="goToProfile(emp.id)">
              <template #header>
                <NSpace align="center" :size="12">
                  <NAvatar :size="48" round :style="{ backgroundColor: '#18a058', fontSize: '16px' }">
                    {{ getInitial(emp) }}
                  </NAvatar>
                  <div>
                    <div style="font-weight: 600; font-size: 15px;">{{ getName(emp) }}</div>
                    <div style="font-size: 12px; color: #999;">{{ emp.employee_no }}</div>
                  </div>
                </NSpace>
              </template>
              <div style="display: flex; flex-direction: column; gap: 6px; font-size: 13px;">
                <div v-if="emp.position_title">
                  <NTag size="small" type="info">{{ emp.position_title }}</NTag>
                </div>
                <div v-if="emp.department_name" style="color: #666;">
                  {{ emp.department_name }}
                </div>
                <div v-if="emp.email" style="color: #666;">
                  {{ emp.email }}
                </div>
                <div v-if="emp.phone" style="color: #666;">
                  {{ emp.phone }}
                </div>
              </div>
            </NCard>
          </NGi>
        </NGrid>
      </template>

      <!-- Org Chart View -->
      <template v-else>
        <EmptyState
          v-if="flatOrgList.length === 0 && !loading"
          icon="👥"
          :title="t('emptyState.directory.title')"
          :description="t('emptyState.directory.desc')"
        />
        <div v-else class="org-chart">
          <div
            v-for="item in flatOrgList"
            :key="item.node.id"
            class="org-row"
          >
            <div class="org-tree-lines">
              <template v-for="d in item.depth" :key="d">
                <span
                  class="org-line-segment"
                  :class="{ 'org-line-pipe': !item.isLast[d], 'org-line-space': item.isLast[d] }"
                />
              </template>
            </div>
            <div class="org-node" @click="goToProfile(item.node.id)">
              <span
                v-if="item.childCount > 0"
                class="org-toggle"
                @click.stop="toggleCollapse(item.node.id)"
              >
                {{ collapsed.has(item.node.id) ? '+' : '-' }}
              </span>
              <NAvatar :size="36" round :style="{ backgroundColor: '#18a058', fontSize: '14px', flexShrink: 0 }">
                {{ getInitial(item.node) }}
              </NAvatar>
              <div style="margin-left: 10px; flex: 1; min-width: 0;">
                <div style="font-weight: 600; font-size: 14px;">{{ getName(item.node) }}</div>
                <NSpace :size="4" style="margin-top: 2px;">
                  <NTag v-if="item.node.position_title" size="tiny" type="info">{{ item.node.position_title }}</NTag>
                  <NTag v-if="item.node.department_name" size="tiny">{{ item.node.department_name }}</NTag>
                </NSpace>
              </div>
              <span v-if="item.childCount > 0" style="font-size: 12px; color: #999; white-space: nowrap;">
                {{ item.childCount }} {{ item.childCount > 1 ? 'reports' : 'report' }}
              </span>
            </div>
          </div>
        </div>
      </template>
    </NSpin>
  </div>
</template>

<style scoped>
.org-chart {
  padding: 8px 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.org-row {
  display: flex;
  align-items: stretch;
}
.org-tree-lines {
  display: flex;
  flex-shrink: 0;
}
.org-line-segment {
  display: inline-block;
  width: 28px;
  border-left: 1px solid #ccc;
}
.org-line-segment.org-line-space {
  border-left: none;
}
.org-node {
  display: flex;
  align-items: center;
  padding: 8px 14px;
  border: 1px solid var(--n-border-color, #e0e0e0);
  border-radius: 8px;
  cursor: pointer;
  transition: background 0.2s;
  background: var(--n-color, #fff);
  flex: 1;
  min-width: 0;
}
.org-node:hover {
  background: var(--n-color-hover, #f5f5f5);
}
.org-toggle {
  width: 20px;
  height: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
  font-weight: bold;
  color: #666;
  border: 1px solid #ddd;
  border-radius: 4px;
  margin-right: 8px;
  cursor: pointer;
  flex-shrink: 0;
  user-select: none;
}
.org-toggle:hover {
  background: #e8e8e8;
}
</style>
