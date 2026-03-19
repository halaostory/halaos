<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NCard, NSpace, NButton, NInput, NGrid, NGi,
  NStatistic, NTag, NDataTable, NEmpty, NAlert,
  useMessage, type DataTableColumns,
} from 'naive-ui'
import { referralAPI } from '../api/client'

const { t } = useI18n()
const message = useMessage()

const loading = ref(true)
const referralCode = ref('')
const referralLink = ref('')
const totalReferrals = ref(0)
const activeReferrals = ref(0)

interface Referral {
  id: number
  referred_company_name: string
  status: string
  created_at: string
}

const referrals = ref<Referral[]>([])

const pendingCount = computed(() => totalReferrals.value - activeReferrals.value)

const columns = computed<DataTableColumns<Referral>>(() => [
  {
    title: t('referral.companyName'),
    key: 'referred_company_name',
  },
  {
    title: t('referral.status'),
    key: 'status',
    render(row) {
      return row.status === 'activated'
        ? { type: NTag, props: { type: 'success', size: 'small' }, children: () => t('referral.activated') }
        : { type: NTag, props: { type: 'warning', size: 'small' }, children: () => t('referral.pending') }
    },
  },
  {
    title: t('referral.date'),
    key: 'created_at',
    render(row) {
      return new Date(row.created_at).toLocaleDateString()
    },
  },
])

async function loadData() {
  loading.value = true
  try {
    const [codeRes, statsRes, listRes] = await Promise.all([
      referralAPI.getCode(),
      referralAPI.getStats(),
      referralAPI.listReferrals(),
    ])
    const codeData = (codeRes as any)?.data ?? codeRes
    referralCode.value = codeData.referral_code || ''
    referralLink.value = codeData.referral_link || ''

    const statsData = (statsRes as any)?.data ?? statsRes
    totalReferrals.value = statsData.total_referrals || 0
    activeReferrals.value = statsData.active_referrals || 0

    const listData = (listRes as any)?.data ?? listRes
    referrals.value = Array.isArray(listData) ? listData : []
  } catch {
    // Stats may be empty for new companies
  } finally {
    loading.value = false
  }
}

function copyToClipboard(text: string) {
  navigator.clipboard.writeText(text)
  message.success(t('referral.copied'))
}

onMounted(loadData)
</script>

<template>
  <div style="max-width: 900px; margin: 0 auto;">
    <h2 style="margin-bottom: 4px;">{{ t('referral.title') }}</h2>
    <p style="color: #64748b; margin-bottom: 24px;">{{ t('referral.subtitle') }}</p>

    <!-- How It Works -->
    <NAlert type="info" style="margin-bottom: 24px;">
      <strong>{{ t('referral.howItWorks') }}</strong>
      <ol style="margin: 8px 0 0; padding-left: 20px;">
        <li>{{ t('referral.step1') }}</li>
        <li>{{ t('referral.step2') }}</li>
        <li>{{ t('referral.step3') }}</li>
      </ol>
    </NAlert>

    <!-- Referral Code & Link -->
    <NCard :title="t('referral.yourLink')" style="margin-bottom: 24px;">
      <NSpace vertical :size="16">
        <div>
          <label style="font-size: 13px; color: #64748b; display: block; margin-bottom: 4px;">{{ t('referral.yourCode') }}</label>
          <NSpace>
            <NInput :value="referralCode" readonly style="width: 200px; font-weight: 700; font-size: 18px; letter-spacing: 2px;" />
            <NButton @click="copyToClipboard(referralCode)">{{ t('referral.copyCode') }}</NButton>
          </NSpace>
        </div>
        <div>
          <label style="font-size: 13px; color: #64748b; display: block; margin-bottom: 4px;">{{ t('referral.yourLink') }}</label>
          <NSpace>
            <NInput :value="referralLink" readonly style="min-width: 380px;" />
            <NButton type="primary" @click="copyToClipboard(referralLink)">{{ t('referral.copyLink') }}</NButton>
          </NSpace>
        </div>
      </NSpace>
    </NCard>

    <!-- Stats -->
    <NGrid :cols="3" :x-gap="16" :y-gap="16" style="margin-bottom: 24px;">
      <NGi>
        <NCard>
          <NStatistic :label="t('referral.totalReferrals')" :value="totalReferrals" />
        </NCard>
      </NGi>
      <NGi>
        <NCard>
          <NStatistic :label="t('referral.activeReferrals')" :value="activeReferrals" />
        </NCard>
      </NGi>
      <NGi>
        <NCard>
          <NStatistic :label="t('referral.pendingReferrals')" :value="pendingCount" />
        </NCard>
      </NGi>
    </NGrid>

    <!-- Referral History -->
    <NCard :title="t('referral.history')">
      <NDataTable
        v-if="referrals.length > 0"
        :columns="columns"
        :data="referrals"
        :bordered="false"
        :loading="loading"
      />
      <NEmpty v-else :description="t('referral.noReferrals')" style="padding: 40px 0;" />
    </NCard>
  </div>
</template>
