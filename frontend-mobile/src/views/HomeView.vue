<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import {
  PullRefresh,
  Cell,
  CellGroup,
  Tag,
  Skeleton,
  Badge,
} from "vant";
import { useAuthStore } from "../stores/auth";
import { attendanceAPI, leaveAPI, notificationAPI } from "../api/client";
import AiQuickAsk from "../components/ai/AiQuickAsk.vue";
import { getSuggestions } from "../composables/useAiContext";
import { format } from "date-fns";
import type { AttendanceSummary, LeaveBalance, ApiResponse } from "../types";

const { t } = useI18n();
const router = useRouter();
const auth = useAuthStore();

const refreshing = ref(false);
const loading = ref(true);
const clockSummary = ref<AttendanceSummary | null>(null);
const leaveBalances = ref<LeaveBalance[]>([]);
const unreadCount = ref(0);

const greeting = computed(() => {
  const hour = new Date().getHours();
  let period = "morning";
  if (hour >= 12 && hour < 18) period = "afternoon";
  else if (hour >= 18) period = "evening";
  return t("home.greeting", {
    period: t(`home.${period}`),
    name: auth.user?.first_name || "",
  });
});

const clockStatusText = computed(() => {
  if (!clockSummary.value) return t("home.notClockedIn");
  switch (clockSummary.value.status) {
    case "clocked_in":
      return t("home.clockedIn");
    case "clocked_out":
      return t("home.clockedOut");
    default:
      return t("home.notClockedIn");
  }
});

const clockStatusType = computed(() => {
  if (!clockSummary.value) return "default";
  switch (clockSummary.value.status) {
    case "clocked_in":
      return "success";
    case "clocked_out":
      return "primary";
    default:
      return "default";
  }
});

const aiSuggestions = computed(() => {
  const base = getSuggestions("home");
  // Add contextual suggestions based on current state
  if (clockSummary.value?.status !== "clocked_in" && clockSummary.value?.status !== "clocked_out") {
    return [t("ai.remindClockIn"), ...base];
  }
  return base;
});

async function loadData() {
  const [summaryRes, balanceRes, countRes] = await Promise.allSettled([
    attendanceAPI.getSummary(),
    leaveAPI.getBalances(),
    notificationAPI.unreadCount(),
  ]);
  if (summaryRes.status === "fulfilled") {
    const sr = summaryRes.value as ApiResponse<AttendanceSummary>;
    clockSummary.value = sr.data ?? (sr as unknown as AttendanceSummary);
  }
  if (balanceRes.status === "fulfilled") {
    const br = balanceRes.value as ApiResponse<LeaveBalance[]>;
    leaveBalances.value = br.data ?? (br as unknown as LeaveBalance[]);
  }
  if (countRes.status === "fulfilled") {
    const cr = countRes.value as ApiResponse<{ count: number }>;
    unreadCount.value = cr.data?.count ?? 0;
  }
  loading.value = false;
}

async function onRefresh() {
  await loadData();
  refreshing.value = false;
}

function formatTime(dt: string | null) {
  if (!dt) return "--";
  return format(new Date(dt), "HH:mm");
}

onMounted(loadData);
</script>

<template>
  <PullRefresh v-model="refreshing" @refresh="onRefresh">
    <div class="home-page">
      <div class="home-greeting">{{ greeting }}</div>

      <!-- Clock Status Card -->
      <Skeleton :loading="loading" :row="3" class="home-skeleton">
        <CellGroup inset :title="t('home.clockStatus')">
          <Cell
            :title="clockStatusText"
            is-link
            @click="router.push({ name: 'attendance' })"
          >
            <template #label>
              <div v-if="clockSummary?.today_clock_in" class="clock-times">
                <span>{{
                  t("home.clockInTime", {
                    time: formatTime(clockSummary.today_clock_in),
                  })
                }}</span>
                <span v-if="clockSummary?.today_clock_out">
                  {{
                    t("home.clockOutTime", {
                      time: formatTime(clockSummary.today_clock_out),
                    })
                  }}
                </span>
              </div>
            </template>
            <template #right-icon>
              <Tag :type="clockStatusType" size="medium">{{
                clockStatusText
              }}</Tag>
            </template>
          </Cell>
        </CellGroup>
      </Skeleton>

      <!-- Leave Balance -->
      <Skeleton :loading="loading" :row="2" class="home-skeleton">
        <CellGroup
          v-if="leaveBalances.length > 0"
          inset
          :title="t('home.leaveBalance')"
        >
          <div class="leave-scroll">
            <div
              v-for="b in leaveBalances"
              :key="b.leave_type_id"
              class="leave-card"
            >
              <div class="leave-card-name">{{ b.leave_type_name }}</div>
              <div class="leave-card-days">{{ b.remaining }}</div>
              <div class="leave-card-label">
                {{ t("home.remaining", { n: b.remaining }) }}
              </div>
            </div>
          </div>
        </CellGroup>
      </Skeleton>

      <AiQuickAsk :questions="aiSuggestions" />

      <!-- Quick Actions -->
      <CellGroup inset :title="t('home.quickActions')">
        <Cell
          :title="t('home.applyLeave')"
          is-link
          icon="calendar-o"
          @click="router.push({ name: 'leave' })"
        />
        <Cell
          :title="t('home.viewPayslips')"
          is-link
          icon="bill-o"
          @click="router.push({ name: 'payslips' })"
        />
        <Cell
          :title="t('home.notifications')"
          is-link
          icon="bell-o"
          @click="router.push({ name: 'notifications' })"
        >
          <template #right-icon>
            <Badge :content="unreadCount > 0 ? unreadCount : undefined" dot>
              <van-icon name="arrow" />
            </Badge>
          </template>
        </Cell>
        <Cell
          :title="t('home.myInfo')"
          is-link
          icon="user-o"
          @click="router.push({ name: 'profile' })"
        />
        <Cell
          :title="t('ai.assistant')"
          is-link
          icon="chat-o"
          @click="router.push({ name: 'ai-chat' })"
        >
          <template #right-icon>
            <Tag type="primary" plain size="medium">AI</Tag>
          </template>
        </Cell>
      </CellGroup>
    </div>
  </PullRefresh>
</template>

<style scoped>
.home-page {
  padding: 16px 0 16px;
}

.home-greeting {
  font-size: 20px;
  font-weight: 600;
  padding: 8px 16px 16px;
  color: var(--text-primary);
}

.home-skeleton {
  padding: 0 16px;
  margin-bottom: 12px;
}

.clock-times {
  display: flex;
  gap: 16px;
  font-size: 12px;
  color: var(--text-secondary);
  margin-top: 4px;
}

.leave-scroll {
  display: flex;
  overflow-x: auto;
  padding: 12px;
  gap: 12px;
  -webkit-overflow-scrolling: touch;
}

.leave-scroll::-webkit-scrollbar {
  display: none;
}

.leave-card {
  min-width: 100px;
  padding: 12px;
  border-radius: 8px;
  background: linear-gradient(135deg, #e8f4fd, #d0ecff);
  text-align: center;
  flex-shrink: 0;
}

.leave-card-name {
  font-size: 12px;
  color: var(--text-secondary);
  margin-bottom: 4px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.leave-card-days {
  font-size: 24px;
  font-weight: 700;
  color: var(--brand-color);
}

.leave-card-label {
  font-size: 11px;
  color: var(--text-secondary);
  margin-top: 2px;
}
</style>
