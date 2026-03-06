<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { useI18n } from "vue-i18n";
import {
  NCard,
  NList,
  NListItem,
  NThing,
  NSpace,
  NButton,
  NTag,
  NEmpty,
  NSelect,
  NBadge,
  NPagination,
  NTime,
  useMessage,
} from "naive-ui";
import { notificationAPI } from "../api/client";

const { t } = useI18n();
const message = useMessage();

interface Notification {
  id: number;
  title: string;
  message: string;
  category: string;
  entity_type: string | null;
  entity_id: number | null;
  is_read: boolean;
  created_at: string;
}

const notifications = ref<Notification[]>([]);
const unreadCount = ref(0);
const loading = ref(false);
const selectedCategory = ref<string>("all");
const currentPage = ref(1);
const pageSize = ref(20);

const categoryColor: Record<string, string> = {
  info: "default",
  leave: "success",
  payroll: "warning",
  performance: "info",
  onboarding: "info",
  loan: "warning",
  approval: "error",
  attendance: "default",
};

const categoryOptions = computed(() => [
  { label: t("notification.all"), value: "all" },
  { label: t("notification.info"), value: "info" },
  { label: t("notification.leave"), value: "leave" },
  { label: t("notification.payroll"), value: "payroll" },
  { label: t("notification.performance"), value: "performance" },
  { label: t("notification.onboarding"), value: "onboarding" },
  { label: t("notification.loan"), value: "loan" },
  { label: t("notification.approval"), value: "approval" },
  { label: t("notification.attendance"), value: "attendance" },
]);

const filteredNotifications = computed(() => {
  if (selectedCategory.value === "all") {
    return notifications.value;
  }
  return notifications.value.filter(
    (n) => n.category === selectedCategory.value
  );
});

const totalFiltered = computed(() => filteredNotifications.value.length);

const paginatedNotifications = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value;
  return filteredNotifications.value.slice(start, start + pageSize.value);
});

async function fetchNotifications() {
  loading.value = true;
  try {
    const res = await notificationAPI.list();
    const data = (res as any)?.data ?? res;
    notifications.value = Array.isArray(data) ? data : [];
  } catch {
    message.error("Failed to load notifications");
  } finally {
    loading.value = false;
  }
}

async function fetchUnreadCount() {
  try {
    const res = await notificationAPI.unreadCount();
    const data = (res as any)?.data ?? res;
    unreadCount.value = data?.count ?? 0;
  } catch {
    /* ignore */
  }
}

async function handleMarkRead(id: number) {
  try {
    await notificationAPI.markRead(id);
    const n = notifications.value.find((x) => x.id === id);
    if (n && !n.is_read) {
      notifications.value = notifications.value.map((item) =>
        item.id === id ? { ...item, is_read: true } : item
      );
      unreadCount.value = Math.max(0, unreadCount.value - 1);
    }
    message.success(t("notification.markRead"));
  } catch {
    message.error("Failed to mark as read");
  }
}

async function handleMarkAllRead() {
  try {
    await notificationAPI.markAllRead();
    notifications.value = notifications.value.map((n) => ({
      ...n,
      is_read: true,
    }));
    unreadCount.value = 0;
    message.success(t("notification.allRead"));
  } catch {
    message.error("Failed to mark all as read");
  }
}

async function handleDelete(id: number) {
  try {
    const n = notifications.value.find((x) => x.id === id);
    if (n && !n.is_read) {
      unreadCount.value = Math.max(0, unreadCount.value - 1);
    }
    notifications.value = notifications.value.filter((x) => x.id !== id);
    await notificationAPI.delete(id);
    message.success(t("notification.deleted"));
  } catch {
    message.error("Failed to delete notification");
    await fetchNotifications();
    await fetchUnreadCount();
  }
}

function handleCategoryChange() {
  currentPage.value = 1;
}

onMounted(() => {
  fetchNotifications();
  fetchUnreadCount();
});
</script>

<template>
  <div>
    <div
      style="
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 20px;
      "
    >
      <NSpace align="center" :size="12">
        <h2 style="margin: 0">{{ t("notification.title") }}</h2>
        <NBadge :value="unreadCount" :max="99" :show="unreadCount > 0" />
      </NSpace>
      <NSpace>
        <NButton
          v-if="unreadCount > 0"
          size="small"
          @click="handleMarkAllRead"
        >
          {{ t("notification.markAllRead") }}
        </NButton>
      </NSpace>
    </div>

    <NCard style="margin-bottom: 16px">
      <NSpace align="center" :size="12">
        <span>{{ t("notification.category") }}:</span>
        <NSelect
          v-model:value="selectedCategory"
          :options="categoryOptions"
          style="width: 180px"
          @update:value="handleCategoryChange"
        />
      </NSpace>
    </NCard>

    <NEmpty
      v-if="paginatedNotifications.length === 0 && !loading"
      :description="t('notification.noNotifications')"
      style="padding: 48px 0"
    />

    <NList v-else hoverable>
      <NListItem
        v-for="n in paginatedNotifications"
        :key="n.id"
        :style="{
          opacity: n.is_read ? 0.6 : 1,
          background: n.is_read ? 'transparent' : 'var(--n-color-hover)',
        }"
      >
        <NThing
          :title="n.title"
          :description="n.message"
          content-style="margin-top: 4px;"
        >
          <template #header-extra>
            <NSpace :size="8">
              <NButton
                v-if="!n.is_read"
                text
                size="small"
                type="primary"
                @click.stop="handleMarkRead(n.id)"
              >
                {{ t("notification.markRead") }}
              </NButton>
              <NButton
                text
                size="small"
                type="error"
                @click.stop="handleDelete(n.id)"
              >
                {{ t("common.delete") }}
              </NButton>
            </NSpace>
          </template>
          <template #footer>
            <NSpace :size="8" align="center">
              <NTag
                size="small"
                :type="(categoryColor[n.category] as any) || 'default'"
              >
                {{ t(`notification.${n.category}`) || n.category }}
              </NTag>
              <NTime :time="new Date(n.created_at)" type="relative" />
            </NSpace>
          </template>
        </NThing>
      </NListItem>
    </NList>

    <div
      v-if="totalFiltered > pageSize"
      style="display: flex; justify-content: center; margin-top: 20px"
    >
      <NPagination
        v-model:page="currentPage"
        :page-size="pageSize"
        :item-count="totalFiltered"
        show-quick-jumper
      />
    </div>
  </div>
</template>
