<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from "vue";
import { useI18n } from "vue-i18n";
import { List, Cell, CellGroup, Tag, showToast, showLoadingToast, closeToast } from "vant";
import { attendanceAPI, geofenceAPI } from "../api/client";
import AiQuickAsk from "../components/ai/AiQuickAsk.vue";
import EmptyState from "../components/EmptyState.vue";
import { useGeolocation } from "../composables/useGeolocation";
import { format } from "date-fns";
import type {
  AttendanceSummary,
  AttendanceRecord,
  GeofenceSettings,
  ApiResponse,
} from "../types";

const { t } = useI18n();

const summary = ref<AttendanceSummary | null>(null);
const records = ref<AttendanceRecord[]>([]);
const page = ref(1);
const finished = ref(false);
const listLoading = ref(false);
const clockLoading = ref(false);
const geofenceEnabled = ref(false);
const currentTime = ref(format(new Date(), "HH:mm:ss"));

const geo = useGeolocation();

// Update clock every second
let timer: ReturnType<typeof setInterval>;
onMounted(() => {
  timer = setInterval(() => {
    currentTime.value = format(new Date(), "HH:mm:ss");
  }, 1000);
  loadSummary();
  loadGeofenceSettings();
});

onUnmounted(() => {
  clearInterval(timer);
});

const isClockIn = computed(
  () => !summary.value || summary.value.status === "not_clocked_in",
);

const clockBtnText = computed(() =>
  isClockIn.value ? t("attendance.clockIn") : t("attendance.clockOut"),
);

const clockBtnColor = computed(() =>
  isClockIn.value ? "#1989fa" : "#07c160",
);

const geoStatusText = computed(() => {
  if (!geofenceEnabled.value) return t("attendance.locationNotRequired");
  switch (geo.status.value) {
    case "acquiring":
      return t("attendance.locating");
    case "acquired":
      return t("attendance.locationAcquired");
    case "denied":
      return t("attendance.locationDenied");
    case "error":
      return t("attendance.locationError");
    default:
      return t("attendance.location");
  }
});

async function loadGeofenceSettings() {
  try {
    const res = (await geofenceAPI.getSettings()) as ApiResponse<GeofenceSettings>;
    const settings = res.data ?? (res as unknown as GeofenceSettings);
    geofenceEnabled.value = settings.geofence_enabled;
  } catch {
    // Default to no geofence
  }
}

async function loadSummary() {
  try {
    const res = (await attendanceAPI.getSummary()) as ApiResponse<AttendanceSummary>;
    summary.value = res.data ?? (res as unknown as AttendanceSummary);
  } catch {
    // ignore
  }
}

async function onClock() {
  clockLoading.value = true;

  try {
    // Get location if geofence is enabled
    if (geofenceEnabled.value && geo.status.value !== "acquired") {
      await geo.acquire();
      if (geo.status.value === "denied" || geo.status.value === "error") {
        showToast({ message: geoStatusText.value, type: "fail" });
        clockLoading.value = false;
        return;
      }
    }

    const payload: Record<string, string> = { source: "mobile_web" };
    if (geo.lat.value != null) payload.lat = String(geo.lat.value);
    if (geo.lng.value != null) payload.lng = String(geo.lng.value);

    showLoadingToast({ message: t("common.loading"), forbidClick: true });

    if (isClockIn.value) {
      await attendanceAPI.clockIn(payload);
      showToast({ message: t("attendance.clockInSuccess"), type: "success" });
    } else {
      await attendanceAPI.clockOut(payload);
      showToast({ message: t("attendance.clockOutSuccess"), type: "success" });
    }

    closeToast();
    await loadSummary();
    // Reset list to reload
    records.value = [];
    page.value = 1;
    finished.value = false;
  } catch {
    closeToast();
    showToast({ message: t("attendance.clockFailed"), type: "fail" });
  } finally {
    clockLoading.value = false;
  }
}

async function loadRecords() {
  listLoading.value = true;
  try {
    const res = (await attendanceAPI.listRecords({
      page: String(page.value),
      limit: "20",
    })) as ApiResponse<AttendanceRecord[]>;
    const items = res.data ?? (res as unknown as AttendanceRecord[]);
    if (Array.isArray(items)) {
      records.value = [...records.value, ...items];
      if (items.length < 20) finished.value = true;
      else page.value++;
    } else {
      finished.value = true;
    }
  } catch {
    finished.value = true;
  } finally {
    listLoading.value = false;
  }
}

function formatRecordTime(dt: string | null) {
  if (!dt) return "--";
  return format(new Date(dt), "HH:mm");
}

function formatRecordDate(dt: string) {
  return format(new Date(dt), "yyyy-MM-dd");
}
</script>

<template>
  <div class="attendance-page">
    <!-- Clock Section -->
    <div class="clock-section">
      <div class="current-time">{{ currentTime }}</div>
      <div class="geo-status">{{ geoStatusText }}</div>

      <button
        class="clock-btn clock-btn-pulse"
        :style="{ background: clockBtnColor }"
        :disabled="clockLoading"
        @click="onClock"
      >
        {{ clockBtnText }}
      </button>

      <div v-if="summary?.today_clock_in" class="today-summary">
        <span>{{ t("attendance.clockIn") }}: {{ formatRecordTime(summary.today_clock_in) }}</span>
        <span v-if="summary?.today_clock_out">
          {{ t("attendance.clockOut") }}: {{ formatRecordTime(summary.today_clock_out) }}
        </span>
      </div>
    </div>

    <AiQuickAsk :questions="[
      'Am I late today?',
      'Show my attendance this week',
      'Generate my attendance report',
    ]" />

    <!-- Records List -->
    <CellGroup inset :title="t('attendance.records')">
      <List
        v-model:loading="listLoading"
        :finished="finished"
        :finished-text="records.length > 0 ? '' : ''"
        @load="loadRecords"
      >
        <EmptyState
          v-if="records.length === 0 && finished"
          icon="⏰"
          :title="t('emptyState.attendance.title')"
          :description="t('emptyState.attendance.desc')"
        />
        <Cell v-for="r in records" :key="r.id" :label="formatRecordDate(r.clock_in)">
          <template #title>
            <div class="record-row">
              <span>{{ formatRecordTime(r.clock_in) }}</span>
              <span class="record-sep">-</span>
              <span>{{ formatRecordTime(r.clock_out) }}</span>
              <Tag v-if="r.source" plain type="primary" class="record-source">
                {{ r.source }}
              </Tag>
            </div>
          </template>
        </Cell>
      </List>
    </CellGroup>
  </div>
</template>

<style scoped>
.attendance-page {
  padding-bottom: 16px;
}

.clock-section {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 32px 16px 24px;
  background: linear-gradient(180deg, #e8f4fd 0%, var(--app-bg) 100%);
}

.current-time {
  font-size: 36px;
  font-weight: 700;
  color: var(--text-primary);
  font-variant-numeric: tabular-nums;
}

.geo-status {
  font-size: 13px;
  color: var(--text-secondary);
  margin: 8px 0 20px;
}

.clock-btn {
  width: 120px;
  height: 120px;
  border-radius: 50%;
  border: none;
  color: #fff;
  font-size: 18px;
  font-weight: 600;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  -webkit-tap-highlight-color: transparent;
}

.clock-btn:disabled {
  opacity: 0.6;
}

.today-summary {
  display: flex;
  gap: 16px;
  margin-top: 16px;
  font-size: 13px;
  color: var(--text-secondary);
}

.record-row {
  display: flex;
  align-items: center;
  gap: 4px;
}

.record-sep {
  color: var(--text-secondary);
}

.record-source {
  margin-left: 8px;
}
</style>
