<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NCard, NGrid, NGi, NStatistic, NSkeleton, NEmpty, NTag,
} from 'naive-ui'
import { orgIntelligenceAPI, flightRiskAPI, burnoutRiskAPI } from '../api/client'
import OrgHealthScore from '../components/org-intelligence/OrgHealthScore.vue'
import ScoreTrendCharts from '../components/org-intelligence/ScoreTrendCharts.vue'
import RiskDistributionWidget from '../components/org-intelligence/RiskDistributionWidget.vue'
import DepartmentComparisonWidget from '../components/org-intelligence/DepartmentComparisonWidget.vue'
import TopConcernsWidget from '../components/org-intelligence/TopConcernsWidget.vue'
import CrossCorrelationInsights from '../components/org-intelligence/CrossCorrelationInsights.vue'
import ExecutiveBriefingWidget from '../components/org-intelligence/ExecutiveBriefingWidget.vue'

const { t } = useI18n()

const loading = ref(true)

// Overview data
const overview = ref<any>(null)
const orgHealthScore = ref(0)

// Trends data
const trends = ref<any[]>([])

// Risk distribution
const riskTiers = ref<any[]>([])
const deptRisks = ref<any[]>([])

// Correlations
const correlations = ref<any>(null)

// Briefing
const briefing = ref<any>(null)

// Top concerns
const topFlightRisk = ref<any[]>([])
const topBurnout = ref<any[]>([])

// Team health (for department comparison)
const teamHealth = ref<any[]>([])

function numVal(v: any): number {
  if (typeof v === 'number') return v
  if (typeof v === 'string') return parseFloat(v) || 0
  if (v && typeof v === 'object' && 'Float64' in v) return v.Float64 || 0
  return 0
}

const currentSnapshot = computed(() => overview.value?.current)
const deltas = computed(() => overview.value?.deltas)

function formatDelta(val: number): string {
  if (!val || val === 0) return ''
  const sign = val > 0 ? '+' : ''
  return `${sign}${val.toFixed(1)}`
}

async function loadData() {
  loading.value = true
  try {
    const results = await Promise.allSettled([
      orgIntelligenceAPI.getOverview(),
      orgIntelligenceAPI.getTrends({ weeks: '12' }),
      orgIntelligenceAPI.getRiskDistribution(),
      orgIntelligenceAPI.getCorrelations(),
      orgIntelligenceAPI.getExecutiveBriefing(),
      flightRiskAPI.getTopRisks(),
      burnoutRiskAPI.getTopRisks(),
    ])

    // Overview
    if (results[0].status === 'fulfilled') {
      const data = (results[0].value as any)?.data ?? results[0].value
      overview.value = data
      orgHealthScore.value = data?.org_health_score ?? 0
      // Extract team health from current snapshot departments
    }

    // Trends
    if (results[1].status === 'fulfilled') {
      const data = (results[1].value as any)?.data ?? results[1].value
      trends.value = Array.isArray(data) ? data : []
    }

    // Risk distribution
    if (results[2].status === 'fulfilled') {
      const data = (results[2].value as any)?.data ?? results[2].value
      riskTiers.value = data?.tiers ?? []
      deptRisks.value = data?.departments ?? []
    }

    // Correlations
    if (results[3].status === 'fulfilled') {
      const data = (results[3].value as any)?.data ?? results[3].value
      correlations.value = data
    }

    // Briefing
    if (results[4].status === 'fulfilled') {
      const data = (results[4].value as any)?.data ?? results[4].value
      briefing.value = data
    }

    // Flight risk
    if (results[5].status === 'fulfilled') {
      const data = (results[5].value as any)?.data ?? results[5].value
      topFlightRisk.value = Array.isArray(data) ? data.map((e: any) => ({
        employee_id: e.employee_id,
        name: `${e.first_name} ${e.last_name}`,
        employee_no: e.employee_no,
        department: e.department,
        risk_score: e.risk_score,
      })) : []
    }

    // Burnout
    if (results[6].status === 'fulfilled') {
      const data = (results[6].value as any)?.data ?? results[6].value
      topBurnout.value = Array.isArray(data) ? data.map((e: any) => ({
        employee_id: e.employee_id,
        name: `${e.first_name} ${e.last_name}`,
        employee_no: e.employee_no,
        department: e.department,
        burnout_score: e.burnout_score,
      })) : []
    }

    // Load team health for department comparison
    try {
      const res = await orgIntelligenceAPI.getCorrelations()
      const data = (res as any)?.data ?? res
      if (data?.dept_risk_averages) {
        // Build team health from available data
        const depts = new Map<string, any>()
        for (const d of (data.dept_risk_averages || [])) {
          depts.set(d.department_name, { department_name: d.department_name, health_score: 50 })
        }
        teamHealth.value = Array.from(depts.values())
      }
    } catch { /* ok */ }
  } catch (e) {
    console.error('Failed to load org intelligence', e)
  } finally {
    loading.value = false
  }
}

async function refreshBriefing() {
  try {
    const res = await orgIntelligenceAPI.getExecutiveBriefing()
    briefing.value = (res as any)?.data ?? res
  } catch { /* ok */ }
}

onMounted(loadData)
</script>

<template>
  <div style="max-width: 1200px; margin: 0 auto;">
    <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 24px;">
      <div>
        <h2 style="margin: 0;">{{ t('orgIntelligence.title') }}</h2>
        <p style="margin: 4px 0 0; opacity: 0.7;">{{ t('orgIntelligence.subtitle') }}</p>
      </div>
    </div>

    <!-- Loading skeleton -->
    <div v-if="loading" style="display: flex; flex-direction: column; gap: 16px;">
      <NGrid :cols="4" :x-gap="16" responsive="screen" :item-responsive="true">
        <NGi v-for="i in 4" :key="i" span="4 m:2 l:1">
          <NCard><NSkeleton text :repeat="3" /></NCard>
        </NGi>
      </NGrid>
      <NCard><NSkeleton text :repeat="6" /></NCard>
    </div>

    <template v-else>
      <!-- Row 1: Key Metrics -->
      <NGrid :cols="4" :x-gap="16" :y-gap="16" responsive="screen" :item-responsive="true" style="margin-bottom: 24px;">
        <NGi span="4 m:2 l:1">
          <NCard>
            <OrgHealthScore :score="orgHealthScore" />
          </NCard>
        </NGi>
        <NGi span="4 m:2 l:1">
          <NCard>
            <NStatistic :label="t('orgIntelligence.flightRiskAvg')" :value="numVal(currentSnapshot?.avg_flight_risk)" />
            <NTag v-if="deltas?.avg_flight_risk" :type="deltas.avg_flight_risk > 0 ? 'error' : 'success'" size="small" :bordered="false" style="margin-top: 4px;">
              {{ formatDelta(deltas.avg_flight_risk) }}
            </NTag>
          </NCard>
        </NGi>
        <NGi span="4 m:2 l:1">
          <NCard>
            <NStatistic :label="t('orgIntelligence.burnoutAvg')" :value="numVal(currentSnapshot?.avg_burnout)" />
            <NTag v-if="deltas?.avg_burnout" :type="deltas.avg_burnout > 0 ? 'error' : 'success'" size="small" :bordered="false" style="margin-top: 4px;">
              {{ formatDelta(deltas.avg_burnout) }}
            </NTag>
          </NCard>
        </NGi>
        <NGi span="4 m:2 l:1">
          <NCard>
            <NStatistic :label="t('orgIntelligence.teamHealthAvg')" :value="numVal(currentSnapshot?.avg_team_health)" />
            <NTag v-if="deltas?.avg_team_health" :type="deltas.avg_team_health > 0 ? 'success' : 'error'" size="small" :bordered="false" style="margin-top: 4px;">
              {{ formatDelta(deltas.avg_team_health) }}
            </NTag>
          </NCard>
        </NGi>
      </NGrid>

      <!-- Row 2: Executive Briefing -->
      <div style="margin-bottom: 24px;">
        <ExecutiveBriefingWidget :briefing="briefing" @refresh="refreshBriefing" />
      </div>

      <!-- Row 3: Trends + Risk Distribution -->
      <NGrid :cols="2" :x-gap="16" :y-gap="16" responsive="screen" :item-responsive="true" style="margin-bottom: 24px;">
        <NGi span="2 l:1">
          <NCard :title="t('orgIntelligence.scoreTrends')">
            <NEmpty v-if="trends.length === 0" :description="t('orgIntelligence.noTrends')" style="padding: 40px 0;" />
            <ScoreTrendCharts v-else :trends="trends" />
          </NCard>
        </NGi>
        <NGi span="2 l:1">
          <NCard :title="t('orgIntelligence.riskDistribution')">
            <NEmpty v-if="riskTiers.length === 0" :description="t('orgIntelligence.noData')" style="padding: 40px 0;" />
            <RiskDistributionWidget v-else :tiers="riskTiers" />
          </NCard>
        </NGi>
      </NGrid>

      <!-- Row 4: Department Comparison + Top Concerns -->
      <NGrid :cols="2" :x-gap="16" :y-gap="16" responsive="screen" :item-responsive="true" style="margin-bottom: 24px;">
        <NGi span="2 l:1">
          <NCard :title="t('orgIntelligence.deptComparison')">
            <NEmpty v-if="!correlations?.dept_risk_averages?.length" :description="t('orgIntelligence.noData')" style="padding: 40px 0;" />
            <DepartmentComparisonWidget
              v-else
              :departments="teamHealth"
              :dept-risks="correlations?.dept_risk_averages"
              :dept-burnout="correlations?.dept_burnout_averages"
            />
          </NCard>
        </NGi>
        <NGi span="2 l:1">
          <NCard :title="t('orgIntelligence.topConcerns')">
            <TopConcernsWidget :flight-risk="topFlightRisk" :burnout="topBurnout" />
          </NCard>
        </NGi>
      </NGrid>

      <!-- Row 5: Cross-Correlation Insights -->
      <NCard :title="t('orgIntelligence.correlations')" style="margin-bottom: 24px;">
        <CrossCorrelationInsights
          :high-burnout-and-high-risk="correlations?.high_burnout_and_high_risk ?? 0"
          :dept-risk-averages="correlations?.dept_risk_averages ?? []"
          :dept-burnout-averages="correlations?.dept_burnout_averages ?? []"
        />
      </NCard>
    </template>
  </div>
</template>

<style scoped>
.delta-positive {
  color: #18a058;
}
.delta-negative {
  color: #d03050;
}
</style>
