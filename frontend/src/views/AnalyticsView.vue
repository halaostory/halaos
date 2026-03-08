<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { useI18n } from "vue-i18n";
import {
  NCard,
  NGrid,
  NGi,
  NStatistic,
  NSpace,
  NDataTable,
  NSpin,
  NButton,
  NDatePicker,
  type DataTableColumns,
} from "naive-ui";
import VChart from "vue-echarts";
import { use } from "echarts/core";
import { CanvasRenderer } from "echarts/renderers";
import { BarChart, LineChart, PieChart } from "echarts/charts";
import {
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  GridComponent,
} from "echarts/components";
import { analyticsAPI } from "../api/client";

use([
  CanvasRenderer,
  BarChart,
  LineChart,
  PieChart,
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  GridComponent,
]);

const { t } = useI18n();
const loading = ref(true);

// Date filter
const now = new Date();
const defaultStart = new Date(now.getFullYear(), now.getMonth() - 11, 1);
const dateRange = ref<[number, number]>([defaultStart.getTime(), now.getTime()]);

function formatDate(ts: number): string {
  const d = new Date(ts);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

// Summary
const summary = ref<Record<string, unknown>>({});

// Chart data
const headcountData = ref<Record<string, unknown>[]>([]);
const turnoverData = ref<Record<string, unknown>[]>([]);
const deptCosts = ref<Record<string, unknown>[]>([]);
const attendancePatterns = ref<Record<string, unknown>[]>([]);
const employmentTypes = ref<Record<string, unknown>[]>([]);
const leaveUtil = ref<Record<string, unknown>[]>([]);

function php(v: unknown): string {
  return Number(v || 0).toLocaleString("en-PH", {
    style: "currency",
    currency: "PHP",
    maximumFractionDigits: 0,
  });
}

const dayNames = computed(() => [
  t("analytics.sun"),
  t("analytics.mon"),
  t("analytics.tue"),
  t("analytics.wed"),
  t("analytics.thu"),
  t("analytics.fri"),
  t("analytics.sat"),
]);

// ECharts options
const headcountOption = computed(() => ({
  tooltip: { trigger: "axis" as const },
  legend: {},
  grid: { left: "3%", right: "4%", bottom: "3%", containLabel: true },
  xAxis: {
    type: "category" as const,
    data: headcountData.value.map((d) => d.month as string),
  },
  yAxis: { type: "value" as const },
  series: [
    {
      name: t("analytics.hires"),
      type: "bar" as const,
      data: headcountData.value.map((d) => Number(d.total_count)),
      itemStyle: { color: "#36a2eb" },
    },
  ],
}));

const turnoverOption = computed(() => ({
  tooltip: { trigger: "axis" as const },
  legend: {},
  grid: { left: "3%", right: "4%", bottom: "3%", containLabel: true },
  xAxis: {
    type: "category" as const,
    data: turnoverData.value.map((d) => d.month as string),
  },
  yAxis: { type: "value" as const },
  series: [
    {
      name: t("analytics.separations"),
      type: "line" as const,
      data: turnoverData.value.map((d) => Number(d.separations)),
      itemStyle: { color: "#ff6384" },
    },
    {
      name: t("analytics.activeEmployees"),
      type: "line" as const,
      data: turnoverData.value.map((d) => Number(d.active_count)),
      itemStyle: { color: "#4bc0c0" },
    },
  ],
}));

const deptCostOption = computed(() => ({
  tooltip: { trigger: "axis" as const },
  grid: { left: "3%", right: "4%", bottom: "3%", containLabel: true },
  xAxis: {
    type: "category" as const,
    data: deptCosts.value.map((d) => d.department_name as string),
    axisLabel: { rotate: 30 },
  },
  yAxis: { type: "value" as const },
  series: [
    {
      type: "bar" as const,
      data: deptCosts.value.map((d) => Number(d.total_salary_cost)),
      itemStyle: { color: "#ffce56" },
    },
  ],
}));

const attendanceOption = computed(() => ({
  tooltip: { trigger: "axis" as const },
  legend: {},
  grid: { left: "3%", right: "4%", bottom: "3%", containLabel: true },
  xAxis: {
    type: "category" as const,
    data: attendancePatterns.value.map(
      (d) => dayNames.value[Number(d.day_of_week)] || String(d.day_of_week),
    ),
  },
  yAxis: [
    { type: "value" as const, name: t("analytics.avgHours") },
    { type: "value" as const, name: t("analytics.avgLateMin"), position: "right" as const },
  ],
  series: [
    {
      name: t("analytics.avgHours"),
      type: "bar" as const,
      data: attendancePatterns.value.map((d) => Number(d.avg_hours)),
      itemStyle: { color: "#36a2eb" },
    },
    {
      name: t("analytics.avgLateMin"),
      type: "line" as const,
      yAxisIndex: 1,
      data: attendancePatterns.value.map((d) => Number(d.avg_late_minutes)),
      itemStyle: { color: "#ff6384" },
    },
  ],
}));

const employmentTypeOption = computed(() => ({
  tooltip: { trigger: "item" as const },
  legend: { orient: "vertical" as const, left: "left" },
  series: [
    {
      type: "pie" as const,
      radius: "60%",
      data: employmentTypes.value.map((d) => ({
        name: d.employment_type as string,
        value: Number(d.count),
      })),
    },
  ],
}));

const leaveColumns: DataTableColumns = [
  { title: t("analytics.leaveType"), key: "leave_type" },
  { title: t("analytics.requests"), key: "total_requests", width: 100 },
  {
    title: t("analytics.daysUsed"),
    key: "total_days_used",
    width: 120,
    render(row) {
      return Number(row.total_days_used || 0).toFixed(1);
    },
  },
];

const deptCostColumns: DataTableColumns = [
  { title: t("analytics.department"), key: "department_name" },
  { title: t("analytics.employees"), key: "employee_count", width: 100 },
  {
    title: t("analytics.totalCost"),
    key: "total_salary_cost",
    width: 160,
    render(row) {
      return php(row.total_salary_cost);
    },
  },
];

async function loadData() {
  loading.value = true;
  const startDate = formatDate(dateRange.value[0]);
  const dateParams = { start_date: startDate };

  try {
    const [sumRes, hcRes, toRes, dcRes, apRes, etRes, luRes] = await Promise.allSettled([
      analyticsAPI.getSummary(),
      analyticsAPI.getHeadcountTrend(dateParams),
      analyticsAPI.getTurnover(dateParams),
      analyticsAPI.getDepartmentCosts(),
      analyticsAPI.getAttendancePatterns(dateParams),
      analyticsAPI.getEmploymentTypes(),
      analyticsAPI.getLeaveUtilization(),
    ]);

    const extract = (r: PromiseSettledResult<unknown>) => {
      if (r.status !== "fulfilled") return null;
      const v = r.value as { data?: unknown };
      return v.data ?? v;
    };

    summary.value = (extract(sumRes) as Record<string, unknown>) || {};
    headcountData.value = (extract(hcRes) as Record<string, unknown>[]) || [];
    turnoverData.value = (extract(toRes) as Record<string, unknown>[]) || [];
    deptCosts.value = (extract(dcRes) as Record<string, unknown>[]) || [];
    attendancePatterns.value = (extract(apRes) as Record<string, unknown>[]) || [];
    employmentTypes.value = (extract(etRes) as Record<string, unknown>[]) || [];
    leaveUtil.value = (extract(luRes) as Record<string, unknown>[]) || [];
  } finally {
    loading.value = false;
  }
}

function handleExport() {
  analyticsAPI.exportCSV();
}

onMounted(loadData);
</script>

<template>
  <NSpin :show="loading">
    <NSpace vertical :size="16">
      <div style="display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 12px">
        <h2 style="margin: 0">{{ t("analytics.title") }}</h2>
        <NSpace :size="8" align="center">
          <NDatePicker
            v-model:value="dateRange"
            type="daterange"
            clearable
            size="small"
            style="width: 280px"
            @update:value="loadData"
          />
          <NButton size="small" @click="handleExport">
            CSV
          </NButton>
        </NSpace>
      </div>

      <!-- Summary Cards -->
      <NGrid :cols="5" :x-gap="12" :y-gap="12" responsive="screen" :item-responsive="true">
        <NGi span="5 m:1">
          <NCard size="small">
            <NStatistic :label="t('analytics.activeEmployees')" :value="Number(summary.active_employees || 0)" />
          </NCard>
        </NGi>
        <NGi span="5 m:1">
          <NCard size="small">
            <NStatistic :label="t('analytics.separatedEmployees')" :value="Number(summary.separated_employees || 0)" />
          </NCard>
        </NGi>
        <NGi span="5 m:1">
          <NCard size="small">
            <NStatistic :label="t('analytics.newHires')" :value="Number(summary.new_hires_this_month || 0)" />
          </NCard>
        </NGi>
        <NGi span="5 m:1">
          <NCard size="small">
            <NStatistic :label="t('analytics.probationary')" :value="Number(summary.probationary_count || 0)" />
          </NCard>
        </NGi>
        <NGi span="5 m:1">
          <NCard size="small">
            <NStatistic :label="t('analytics.avgTenure')" :value="Number(summary.avg_tenure_years || 0)" />
          </NCard>
        </NGi>
      </NGrid>

      <!-- Charts Row 1 -->
      <NGrid :cols="2" :x-gap="16" :y-gap="16" responsive="screen" :item-responsive="true">
        <NGi span="2 m:1">
          <NCard :title="t('analytics.headcountTrend')" size="small">
            <VChart v-if="headcountData.length" :option="headcountOption" style="height: 300px" autoresize />
            <p v-else style="color: #999; text-align: center; padding: 40px 0">No data</p>
          </NCard>
        </NGi>
        <NGi span="2 m:1">
          <NCard :title="t('analytics.turnoverRate')" size="small">
            <VChart v-if="turnoverData.length" :option="turnoverOption" style="height: 300px" autoresize />
            <p v-else style="color: #999; text-align: center; padding: 40px 0">No data</p>
          </NCard>
        </NGi>
      </NGrid>

      <!-- Charts Row 2 -->
      <NGrid :cols="2" :x-gap="16" :y-gap="16" responsive="screen" :item-responsive="true">
        <NGi span="2 m:1">
          <NCard :title="t('analytics.departmentCosts')" size="small">
            <VChart v-if="deptCosts.length" :option="deptCostOption" style="height: 300px" autoresize />
            <NDataTable v-if="deptCosts.length" :columns="deptCostColumns" :data="deptCosts" size="small" :pagination="false" style="margin-top: 12px" />
          </NCard>
        </NGi>
        <NGi span="2 m:1">
          <NCard :title="t('analytics.attendancePatterns')" size="small">
            <VChart v-if="attendancePatterns.length" :option="attendanceOption" style="height: 300px" autoresize />
            <p v-else style="color: #999; text-align: center; padding: 40px 0">No data</p>
          </NCard>
        </NGi>
      </NGrid>

      <!-- Charts Row 3 -->
      <NGrid :cols="2" :x-gap="16" :y-gap="16" responsive="screen" :item-responsive="true">
        <NGi span="2 m:1">
          <NCard :title="t('analytics.employmentTypes')" size="small">
            <VChart v-if="employmentTypes.length" :option="employmentTypeOption" style="height: 300px" autoresize />
            <p v-else style="color: #999; text-align: center; padding: 40px 0">No data</p>
          </NCard>
        </NGi>
        <NGi span="2 m:1">
          <NCard :title="t('analytics.leaveUtilization')" size="small">
            <NDataTable :columns="leaveColumns" :data="leaveUtil" size="small" :pagination="false" />
          </NCard>
        </NGi>
      </NGrid>
    </NSpace>
  </NSpin>
</template>
