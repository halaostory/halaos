<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { NSpin, NResult } from 'naive-ui'
import { publicAPI } from '../../api/client'
import SignDocumentPanel from '../../components/lifecycle/SignDocumentPanel.vue'

const route = useRoute()
const { t } = useI18n()
const token = String(route.params.token)
const doc = ref<{ title: string; body: string; company_name: string; signer_name: string; status: string; signed_at?: string } | null>(null)
const gone = ref('')
const loading = ref(true)

async function load() {
  try {
    doc.value = ((await publicAPI.doc(token)) as { data: typeof doc.value }).data
  } catch (e: unknown) {
    gone.value = (e as { data?: { error?: { message?: string } } }).data?.error?.message || t('portal.invalidLink')
  } finally {
    loading.value = false
  }
}
async function submit(payload: Record<string, unknown>) {
  await publicAPI.sign(token, payload)
  await load()
}
onMounted(load)
</script>

<template>
  <div class="public-wrap">
    <NSpin :show="loading">
      <NResult v-if="gone" status="warning" :title="gone" />
      <template v-else-if="doc">
        <div class="brand">{{ doc.company_name }}</div>
        <SignDocumentPanel :doc="doc" :submit="submit" />
      </template>
    </NSpin>
  </div>
</template>

<style scoped>
.public-wrap { max-width: 820px; margin: 0 auto; padding: 32px 16px; min-height: 100vh; }
.brand { font-size: 22px; font-weight: 700; margin-bottom: 16px; }
</style>
