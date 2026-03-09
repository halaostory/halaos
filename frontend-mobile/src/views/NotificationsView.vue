<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import {
  NavBar,
  PullRefresh,
  List,
  SwipeCell,
  Cell,
  Button,
  Tag,
  Empty,
  showToast,
} from "vant";
import AiQuickAsk from "../components/ai/AiQuickAsk.vue";
import { notificationAPI } from "../api/client";
import { format } from "date-fns";
import type { Notification, ApiResponse } from "../types";

const { t } = useI18n();
const router = useRouter();

const notifications = ref<Notification[]>([]);
const loading = ref(false);
const refreshing = ref(false);
const finished = ref(false);

function typeColor(type: string): "success" | "warning" | "danger" | "primary" {
  switch (type) {
    case "leave":
      return "success";
    case "payroll":
      return "warning";
    case "approval":
      return "danger";
    default:
      return "primary";
  }
}

function typeLabel(type: string) {
  const key = `notifications.${type}` as const;
  return t(key) || type;
}

async function loadNotifications() {
  loading.value = true;
  try {
    const res = (await notificationAPI.list()) as ApiResponse<Notification[]>;
    const items = res.data ?? (res as unknown as Notification[]);
    notifications.value = Array.isArray(items) ? items : [];
    finished.value = true;
  } catch {
    finished.value = true;
  } finally {
    loading.value = false;
  }
}

async function onRefresh() {
  await loadNotifications();
  refreshing.value = false;
}

async function markRead(id: number) {
  try {
    await notificationAPI.markRead(id);
    notifications.value = notifications.value.map((n) =>
      n.id === id ? { ...n, is_read: true } : n,
    );
  } catch {
    showToast({ message: t("common.failed"), type: "fail" });
  }
}

async function markAllRead() {
  try {
    await notificationAPI.markAllRead();
    notifications.value = notifications.value.map((n) => ({
      ...n,
      is_read: true,
    }));
    showToast({ message: t("common.success"), type: "success" });
  } catch {
    showToast({ message: t("common.failed"), type: "fail" });
  }
}

function formatTime(dt: string) {
  return format(new Date(dt), "MM/dd HH:mm");
}

onMounted(loadNotifications);
</script>

<template>
  <div class="notifications-page">
    <NavBar
      :title="t('notifications.title')"
      left-arrow
      @click-left="router.back()"
    >
      <template #right>
        <span class="mark-all" @click="markAllRead">
          {{ t("notifications.markAllRead") }}
        </span>
      </template>
    </NavBar>

    <!-- AI Quick Ask -->
    <AiQuickAsk
      :questions="[
        'Summarize my notifications',
        'Any pending approvals?',
        'What needs my attention?',
      ]"
    />

    <PullRefresh v-model="refreshing" @refresh="onRefresh">
      <List
        v-model:loading="loading"
        :finished="finished"
        @load="loadNotifications"
      >
        <template v-if="notifications.length === 0 && !loading">
          <Empty :description="t('notifications.noNotifications')" />
        </template>

        <SwipeCell v-for="n in notifications" :key="n.id" :disabled="n.is_read">
          <Cell :class="{ 'notification-unread': !n.is_read }">
            <template #title>
              <div class="notification-header">
                <Tag :type="typeColor(n.type)" size="medium">
                  {{ typeLabel(n.type) }}
                </Tag>
                <span class="notification-time">{{ formatTime(n.created_at) }}</span>
              </div>
            </template>
            <template #label>
              <div class="notification-title">{{ n.title }}</div>
              <div class="notification-message">{{ n.message }}</div>
            </template>
          </Cell>
          <template #right>
            <Button
              v-if="!n.is_read"
              square
              type="primary"
              :text="t('notifications.markRead')"
              class="swipe-btn"
              @click="markRead(n.id)"
            />
          </template>
        </SwipeCell>
      </List>
    </PullRefresh>
  </div>
</template>

<style scoped>
.notifications-page {
  min-height: 100%;
  background: var(--app-bg);
}

.mark-all {
  font-size: 13px;
  color: var(--brand-color);
  cursor: pointer;
}

.notification-unread {
  background: #f0f7ff;
}

.notification-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 4px;
}

.notification-time {
  font-size: 12px;
  color: var(--text-secondary);
}

.notification-title {
  font-weight: 500;
  font-size: 14px;
  margin-bottom: 2px;
}

.notification-message {
  font-size: 13px;
  color: var(--text-secondary);
  line-height: 1.4;
}

.swipe-btn {
  height: 100%;
}
</style>
