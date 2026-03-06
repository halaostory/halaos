<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { NResult, NButton } from 'naive-ui'
import { useRouter } from 'vue-router'

const { t } = useI18n()
const router = useRouter()
</script>

<template>
  <div style="display: flex; justify-content: center; align-items: center; min-height: 80vh;">
    <NResult status="404" :title="t('common.notFound')" :description="t('common.pageNotFound')">
      <template #footer>
        <NButton type="primary" @click="router.push({ name: 'dashboard' })">{{ t('common.backHome') }}</NButton>
      </template>
    </NResult>
  </div>
</template>
