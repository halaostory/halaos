<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NCard, NButton, NSpace, NInput, NDataTable, NTag, NModal,
  NForm, NFormItem, NSelect, NDynamicTags, useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { knowledgeAPI } from '../api/client'

const { t } = useI18n()
const message = useMessage()

interface Article {
  id: number
  category: string
  topic: string
  title: string
  content: string
  tags: string[]
  source: string | null
  is_active: boolean
  created_at: string
}

const articles = ref<Article[]>([])
const selectedCategory = ref('')
const searchQuery = ref('')
const loading = ref(false)
const showModal = ref(false)
const editingId = ref<number | null>(null)
const showDetail = ref(false)
const detailArticle = ref<Article | null>(null)

const form = ref({
  category: '',
  topic: '',
  title: '',
  content: '',
  tags: [] as string[],
  source: '',
})

const categoryOptions = [
  { label: t('knowledge.labor_law'), value: 'labor_law' },
  { label: t('knowledge.compliance'), value: 'compliance' },
  { label: t('knowledge.benefits'), value: 'benefits' },
  { label: t('knowledge.payroll'), value: 'payroll' },
  { label: t('knowledge.leave'), value: 'leave' },
  { label: t('knowledge.hr_policy'), value: 'hr_policy' },
]

const filterOptions = computed(() => [
  { label: t('knowledge.allCategories'), value: '' },
  ...categoryOptions,
])

const columns: DataTableColumns<Article> = [
  {
    title: t('knowledge.category'),
    key: 'category',
    width: 120,
    render: (row) => {
      const colors: Record<string, string> = {
        labor_law: 'error', compliance: 'warning', benefits: 'success',
        payroll: 'info', leave: 'success', hr_policy: 'default',
      }
      const key = `knowledge.${row.category}` as const
      return h(NTag, { size: 'small', type: (colors[row.category] || 'default') as any }, { default: () => t(key) })
    },
  },
  { title: t('knowledge.topic'), key: 'topic', width: 150 },
  { title: 'Title', key: 'title', ellipsis: { tooltip: true } },
  {
    title: t('knowledge.source'),
    key: 'source',
    width: 180,
    render: (row) => row.source || '-',
  },
  {
    title: t('common.actions'),
    key: 'actions',
    width: 180,
    render: (row) => h(NSpace, { size: 4 }, {
      default: () => [
        h(NButton, { size: 'small', onClick: () => viewArticle(row) }, { default: () => t('common.view') }),
        h(NButton, { size: 'small', type: 'info', onClick: () => editArticle(row) }, { default: () => t('common.edit') }),
        h(NButton, { size: 'small', type: 'error', onClick: () => deleteArticle(row.id) }, { default: () => t('common.delete') }),
      ],
    }),
  },
]

import { h } from 'vue'

async function loadArticles() {
  loading.value = true
  try {
    const params: Record<string, string> = {}
    if (selectedCategory.value) params.category = selectedCategory.value
    const res = await knowledgeAPI.list(params)
    const data = (res as any)?.data ?? res
    articles.value = Array.isArray(data) ? data : []
  } catch {
    articles.value = []
  } finally {
    loading.value = false
  }
}

async function searchArticles() {
  if (!searchQuery.value.trim()) {
    loadArticles()
    return
  }
  loading.value = true
  try {
    const res = await knowledgeAPI.search(searchQuery.value)
    const data = (res as any)?.data ?? res
    articles.value = Array.isArray(data) ? data : []
  } catch {
    articles.value = []
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingId.value = null
  form.value = { category: 'labor_law', topic: '', title: '', content: '', tags: [], source: '' }
  showModal.value = true
}

function editArticle(article: Article) {
  editingId.value = article.id
  form.value = {
    category: article.category,
    topic: article.topic,
    title: article.title,
    content: article.content,
    tags: article.tags || [],
    source: article.source || '',
  }
  showModal.value = true
}

function viewArticle(article: Article) {
  detailArticle.value = article
  showDetail.value = true
}

async function saveArticle() {
  const data = {
    ...form.value,
    source: form.value.source || null,
  }

  try {
    if (editingId.value) {
      await knowledgeAPI.update(editingId.value, data)
      message.success(t('knowledge.updated'))
    } else {
      await knowledgeAPI.create(data)
      message.success(t('knowledge.created'))
    }
    showModal.value = false
    loadArticles()
  } catch {
    message.error(t('common.failed'))
  }
}

async function deleteArticle(id: number) {
  try {
    await knowledgeAPI.delete(id)
    message.success(t('knowledge.deleted'))
    loadArticles()
  } catch {
    message.error(t('common.failed'))
  }
}

onMounted(loadArticles)
</script>

<template>
  <NCard :title="t('knowledge.title')">
    <template #header-extra>
      <NButton type="primary" @click="openCreate">{{ t('knowledge.addArticle') }}</NButton>
    </template>

    <NSpace vertical :size="16">
      <NSpace :size="12">
        <NSelect
          v-model:value="selectedCategory"
          :options="filterOptions"
          style="width: 200px;"
          @update:value="loadArticles"
        />
        <NInput
          v-model:value="searchQuery"
          :placeholder="t('knowledge.search')"
          clearable
          style="width: 300px;"
          @keyup.enter="searchArticles"
          @clear="loadArticles"
        />
        <NButton @click="searchArticles">{{ t('common.search') }}</NButton>
      </NSpace>

      <NDataTable
        :columns="columns"
        :data="articles"
        :loading="loading"
        :row-key="(row: Article) => row.id"
        :pagination="{ pageSize: 20 }"
      />
    </NSpace>
  </NCard>

  <!-- Create/Edit Modal -->
  <NModal
    v-model:show="showModal"
    preset="card"
    :title="editingId ? t('knowledge.editArticle') : t('knowledge.addArticle')"
    style="width: 700px;"
  >
    <NForm label-placement="top">
      <NFormItem :label="t('knowledge.category')">
        <NSelect v-model:value="form.category" :options="categoryOptions" />
      </NFormItem>
      <NFormItem :label="t('knowledge.topic')">
        <NInput v-model:value="form.topic" />
      </NFormItem>
      <NFormItem label="Title">
        <NInput v-model:value="form.title" />
      </NFormItem>
      <NFormItem :label="t('knowledge.content')">
        <NInput v-model:value="form.content" type="textarea" :rows="10" />
      </NFormItem>
      <NFormItem :label="t('knowledge.tags')">
        <NDynamicTags v-model:value="form.tags" />
      </NFormItem>
      <NFormItem :label="t('knowledge.source')">
        <NInput v-model:value="form.source" :placeholder="t('knowledge.sourcePlaceholder')" />
      </NFormItem>
    </NForm>
    <template #footer>
      <NSpace justify="end">
        <NButton @click="showModal = false">{{ t('common.cancel') }}</NButton>
        <NButton type="primary" @click="saveArticle">{{ t('common.save') }}</NButton>
      </NSpace>
    </template>
  </NModal>

  <!-- Detail Modal -->
  <NModal
    v-model:show="showDetail"
    preset="card"
    :title="detailArticle?.title || ''"
    style="width: 700px;"
  >
    <template v-if="detailArticle">
      <NSpace vertical :size="12">
        <NSpace :size="8">
          <NTag size="small">{{ t(`knowledge.${detailArticle.category}`) }}</NTag>
          <NTag v-if="detailArticle.source" size="small" type="info">{{ detailArticle.source }}</NTag>
        </NSpace>
        <div style="white-space: pre-wrap; line-height: 1.7;">{{ detailArticle.content }}</div>
        <NSpace v-if="detailArticle.tags?.length" :size="4">
          <NTag v-for="tag in detailArticle.tags" :key="tag" size="small" round>{{ tag }}</NTag>
        </NSpace>
      </NSpace>
    </template>
  </NModal>
</template>
