<script setup lang="ts">
import { ref, h, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NCard, NGrid, NGi, NStatistic, NDataTable, NTag, NButton,
  NModal, NProgress, NSpin, useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { billingAPI } from '../api/client'

const { t } = useI18n()
const message = useMessage()
const loading = ref(true)

// --- Types ---

interface TokenBalance {
  balance: number
  total_purchased: number
  total_granted: number
  total_consumed: number
}

interface DailyUsage {
  date: string
  tokens_used: number
  request_count: number
}

interface AgentUsage {
  agent_name: string
  agent_slug: string
  request_count: number
  tokens_used: number
}

interface TokenPackage {
  id: number
  name: string
  tokens: number
  price: number
  currency: string
  is_active: boolean
}

interface Transaction {
  id: number
  type: string
  amount: number
  balance_after: number
  agent_slug: string | null
  description: string
  created_at: string
}

interface TransactionsResponse {
  items: Transaction[]
  total: number
}

// --- State ---

const balance = ref<TokenBalance>({
  balance: 0,
  total_purchased: 0,
  total_granted: 0,
  total_consumed: 0,
})

const dailyUsage = ref<DailyUsage[]>([])
const agentUsage = ref<AgentUsage[]>([])
const packages = ref<TokenPackage[]>([])
const transactions = ref<Transaction[]>([])
const transactionTotal = ref(0)
const transactionPage = ref(1)
const transactionLimit = 20

// Purchase modal
const showPurchaseModal = ref(false)
const purchasingPackage = ref<TokenPackage | null>(null)
const purchaseLoading = ref(false)

// --- Computed ---

const maxDailyTokens = computed(() => {
  if (dailyUsage.value.length === 0) return 1
  return Math.max(...dailyUsage.value.map(d => d.tokens_used), 1)
})

const balancePercentage = computed(() => {
  const total = balance.value.total_purchased + balance.value.total_granted
  if (total === 0) return 0
  return Math.round((balance.value.balance / total) * 100)
})

// --- Table Columns ---

const dailyUsageColumns = computed<DataTableColumns<DailyUsage>>(() => [
  {
    title: t('billing.date'),
    key: 'date',
    width: 120,
    render: (row) => row.date,
  },
  {
    title: t('billing.tokensUsed'),
    key: 'tokens_used',
    render: (row) => h('div', { style: 'display: flex; align-items: center; gap: 8px;' }, [
      h(NProgress, {
        type: 'line',
        percentage: Math.round((row.tokens_used / maxDailyTokens.value) * 100),
        showIndicator: false,
        railColor: 'rgba(0,0,0,0.05)',
        color: '#18a058',
        style: 'flex: 1;',
      }),
      h('span', { style: 'min-width: 70px; text-align: right; font-size: 13px;' },
        row.tokens_used.toLocaleString()),
    ]),
  },
  {
    title: t('billing.requests'),
    key: 'request_count',
    width: 100,
    render: (row) => row.request_count.toLocaleString(),
  },
])

const agentUsageColumns = computed<DataTableColumns<AgentUsage>>(() => [
  { title: t('billing.agentName'), key: 'agent_name' },
  {
    title: t('billing.requests'),
    key: 'request_count',
    width: 120,
    render: (row) => row.request_count.toLocaleString(),
  },
  {
    title: t('billing.tokensUsed'),
    key: 'tokens_used',
    width: 140,
    render: (row) => row.tokens_used.toLocaleString(),
  },
])

const transactionTypeMap: Record<string, { type: string; label: string }> = {
  purchase: { type: 'success', label: 'billing.typePurchase' },
  consumption: { type: 'error', label: 'billing.typeUsage' },
  free_grant: { type: 'info', label: 'billing.typeFreeGrant' },
  refund: { type: 'warning', label: 'billing.typeRefund' },
}

const transactionColumns = computed<DataTableColumns<Transaction>>(() => [
  {
    title: t('common.type'),
    key: 'type',
    width: 120,
    render: (row) => {
      const mapped = transactionTypeMap[row.type] || { type: 'default', label: row.type }
      return h(NTag, { size: 'small', type: mapped.type as any }, () => t(mapped.label))
    },
  },
  {
    title: t('billing.amount'),
    key: 'amount',
    width: 120,
    render: (row) => {
      const isPositive = row.amount > 0
      return h('span', {
        style: `font-weight: 600; color: ${isPositive ? '#18a058' : '#d03050'};`,
      }, `${isPositive ? '+' : ''}${row.amount.toLocaleString()}`)
    },
  },
  {
    title: t('billing.balanceAfter'),
    key: 'balance_after',
    width: 120,
    render: (row) => row.balance_after.toLocaleString(),
  },
  {
    title: t('billing.agent'),
    key: 'agent_slug',
    width: 120,
    render: (row) => row.agent_slug || '-',
  },
  {
    title: t('billing.description'),
    key: 'description',
    ellipsis: { tooltip: true },
  },
  {
    title: t('billing.dateCol'),
    key: 'created_at',
    width: 160,
    render: (row) => {
      try {
        return new Date(row.created_at).toLocaleString()
      } catch {
        return row.created_at
      }
    },
  },
])

const transactionPagination = computed(() => ({
  page: transactionPage.value,
  pageSize: transactionLimit,
  itemCount: transactionTotal.value,
  showSizePicker: false,
}))

// --- Data Fetching ---

function extractData<T>(res: unknown): T {
  const r = res as { data?: T }
  return (r.data ?? res) as T
}

async function fetchBalance() {
  try {
    const res = await billingAPI.getBalance()
    const data = extractData<TokenBalance>(res)
    balance.value = {
      balance: data.balance || 0,
      total_purchased: data.total_purchased || 0,
      total_granted: data.total_granted || 0,
      total_consumed: data.total_consumed || 0,
    }
  } catch {
    // balance may not be available yet
  }
}

async function fetchDailyUsage() {
  try {
    const res = await billingAPI.dailyUsage()
    const data = extractData<DailyUsage[]>(res)
    dailyUsage.value = Array.isArray(data) ? data : []
  } catch {
    dailyUsage.value = []
  }
}

async function fetchAgentUsage() {
  try {
    const res = await billingAPI.usageByAgent()
    const data = extractData<AgentUsage[]>(res)
    agentUsage.value = Array.isArray(data) ? data : []
  } catch {
    agentUsage.value = []
  }
}

async function fetchPackages() {
  try {
    const res = await billingAPI.listPackages()
    const data = extractData<TokenPackage[]>(res)
    packages.value = Array.isArray(data) ? data.filter(p => p.is_active) : []
  } catch {
    packages.value = []
  }
}

async function fetchTransactions() {
  try {
    const offset = (transactionPage.value - 1) * transactionLimit
    const res = await billingAPI.listTransactions({
      limit: String(transactionLimit),
      offset: String(offset),
    })
    const data = extractData<TransactionsResponse>(res)
    transactions.value = data?.items || []
    transactionTotal.value = data?.total || 0
  } catch {
    transactions.value = []
  }
}

async function fetchAll() {
  loading.value = true
  try {
    await Promise.allSettled([
      fetchBalance(),
      fetchDailyUsage(),
      fetchAgentUsage(),
      fetchPackages(),
      fetchTransactions(),
    ])
  } finally {
    loading.value = false
  }
}

// --- Purchase ---

function openPurchase(pkg: TokenPackage) {
  purchasingPackage.value = pkg
  showPurchaseModal.value = true
}

async function confirmPurchase() {
  if (!purchasingPackage.value) return
  purchaseLoading.value = true
  try {
    await billingAPI.purchaseTokens(purchasingPackage.value.id)
    message.success(t('billing.purchaseSuccess'))
    showPurchaseModal.value = false
    purchasingPackage.value = null
    await Promise.allSettled([fetchBalance(), fetchTransactions()])
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } }
    const msg = err.data?.error?.message || t('common.failed')
    if (msg.toLowerCase().includes('insufficient')) {
      message.error(t('billing.insufficientBalance'))
    } else {
      message.error(msg)
    }
  } finally {
    purchaseLoading.value = false
  }
}

function handleTransactionPageChange(page: number) {
  transactionPage.value = page
  fetchTransactions()
}

// --- Helpers ---

function formatTokens(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return n.toLocaleString()
}

function formatPrice(price: number, currency: string): string {
  if (currency === 'PHP') return `₱${price.toLocaleString()}`
  return `${currency} ${price.toLocaleString()}`
}

// --- Mount ---

onMounted(fetchAll)
</script>

<template>
  <NSpin :show="loading">
    <div>
      <h2 style="margin-bottom: 24px;">{{ t('billing.title') }}</h2>

      <!-- Token Balance Card -->
      <NCard style="margin-bottom: 24px;">
        <div style="text-align: center; margin-bottom: 16px;">
          <div style="font-size: 14px; color: #999; margin-bottom: 4px;">
            {{ t('billing.balance') }}
          </div>
          <div style="font-size: 42px; font-weight: 700; line-height: 1.2;">
            {{ formatTokens(balance.balance) }}
          </div>
          <div style="font-size: 13px; color: #999;">tokens</div>
        </div>
        <NProgress
          type="line"
          :percentage="balancePercentage"
          :show-indicator="true"
          indicator-placement="inside"
          :height="24"
          :border-radius="12"
          color="#18a058"
          rail-color="rgba(0,0,0,0.06)"
          style="margin-bottom: 16px;"
        />
        <NGrid :cols="3" :x-gap="16" responsive="screen">
          <NGi>
            <NStatistic :label="t('billing.purchased')" :value="formatTokens(balance.total_purchased)" />
          </NGi>
          <NGi>
            <NStatistic :label="t('billing.granted')" :value="formatTokens(balance.total_granted)" />
          </NGi>
          <NGi>
            <NStatistic :label="t('billing.consumed')" :value="formatTokens(balance.total_consumed)" />
          </NGi>
        </NGrid>
      </NCard>

      <!-- Daily Usage -->
      <NCard :title="t('billing.dailyUsage')" style="margin-bottom: 24px;" size="small">
        <NDataTable
          :columns="dailyUsageColumns"
          :data="dailyUsage"
          :bordered="false"
          :pagination="false"
          size="small"
          :max-height="320"
        />
      </NCard>

      <!-- Agent Usage -->
      <NCard :title="t('billing.agentUsage')" style="margin-bottom: 24px;" size="small">
        <NDataTable
          :columns="agentUsageColumns"
          :data="agentUsage"
          :bordered="false"
          :pagination="false"
          size="small"
        />
      </NCard>

      <!-- Token Packages -->
      <NCard :title="t('billing.packages')" style="margin-bottom: 24px;" size="small">
        <NGrid cols="1 s:2 m:3 l:4" :x-gap="16" :y-gap="16" responsive="screen" :item-responsive="true">
          <NGi v-for="pkg in packages" :key="pkg.id">
            <NCard hoverable style="text-align: center;">
              <div style="font-size: 16px; font-weight: 600; margin-bottom: 8px;">
                {{ pkg.name }}
              </div>
              <div style="font-size: 28px; font-weight: 700; color: #18a058; margin-bottom: 4px;">
                {{ formatTokens(pkg.tokens) }}
              </div>
              <div style="font-size: 13px; color: #999; margin-bottom: 12px;">tokens</div>
              <div style="font-size: 20px; font-weight: 600; margin-bottom: 16px;">
                {{ formatPrice(pkg.price, pkg.currency || 'PHP') }}
              </div>
              <NButton type="primary" block @click="openPurchase(pkg)">
                {{ t('billing.buy') }}
              </NButton>
            </NCard>
          </NGi>
        </NGrid>
        <div v-if="packages.length === 0" style="text-align: center; padding: 24px; color: #999;">
          {{ t('common.noData') }}
        </div>
      </NCard>

      <!-- Transaction History -->
      <NCard :title="t('billing.transactions')" size="small">
        <NDataTable
          :columns="transactionColumns"
          :data="transactions"
          :bordered="false"
          :pagination="transactionPagination"
          :remote="true"
          size="small"
          @update:page="handleTransactionPageChange"
        />
      </NCard>

      <!-- Purchase Confirmation Modal -->
      <NModal
        v-model:show="showPurchaseModal"
        preset="dialog"
        :title="t('billing.buyConfirm')"
        :positive-text="t('common.confirm')"
        :negative-text="t('common.cancel')"
        :loading="purchaseLoading"
        @positive-click="confirmPurchase"
      >
        <template v-if="purchasingPackage">
          <div style="padding: 12px 0;">
            <p style="margin-bottom: 8px;">
              <strong>{{ purchasingPackage.name }}</strong>
            </p>
            <p style="margin-bottom: 4px;">
              Tokens: <strong>{{ purchasingPackage.tokens.toLocaleString() }}</strong>
            </p>
            <p>
              Price: <strong>{{ formatPrice(purchasingPackage.price, purchasingPackage.currency || 'PHP') }}</strong>
            </p>
          </div>
        </template>
      </NModal>
    </div>
  </NSpin>
</template>
