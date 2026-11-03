<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { Tabbar, TabbarItem } from "vant";
import AiFloatingBubble from "./ai/AiFloatingBubble.vue";
import { hasModule } from "../edition";

const { t } = useI18n();
const route = useRoute();
const router = useRouter();

// Warp-style employee app: home, (clock), time off, pay, expenses, me.
const allTabs = [
  { name: "home", icon: "home-o", label: "tab.home", module: "" },
  { name: "attendance", icon: "clock-o", label: "tab.attendance", module: "attendance" },
  { name: "leave", icon: "calendar-o", label: "tab.leave", module: "time_off" },
  { name: "payslips", icon: "bill-o", label: "tab.payslips", module: "payroll" },
  { name: "expenses", icon: "balance-list-o", label: "tab.expenses", module: "expenses" },
  { name: "profile", icon: "user-o", label: "tab.profile", module: "" },
];
const tabs = computed(() => allTabs.filter((t) => !t.module || hasModule(t.module)));
const tabRoutes = computed(() => tabs.value.map((t) => t.name));

const active = ref(0);

watch(
  () => route.name,
  (name) => {
    const idx = tabRoutes.value.indexOf(name as string);
    if (idx >= 0) active.value = idx;
  },
  { immediate: true },
);

function onTabChange(index: number | string) {
  const idx = typeof index === "string" ? parseInt(index) : index;
  router.push({ name: tabRoutes.value[idx] });
}
</script>

<template>
  <div class="mobile-layout">
    <div class="mobile-content">
      <router-view />
    </div>
    <Tabbar
      v-model="active"
      :placeholder="true"
      safe-area-inset-bottom
      @change="onTabChange"
    >
      <TabbarItem v-for="tab in tabs" :key="tab.name" :icon="tab.icon">{{ t(tab.label) }}</TabbarItem>
    </Tabbar>
    <AiFloatingBubble v-if="route.name !== 'ai-chat' && hasModule('ai')" />
  </div>
</template>

<style scoped>
.mobile-layout {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.mobile-content {
  flex: 1;
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
}
</style>
