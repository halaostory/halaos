<script setup lang="ts">
import { ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { Tabbar, TabbarItem } from "vant";
import AiFloatingBubble from "./ai/AiFloatingBubble.vue";

const { t } = useI18n();
const route = useRoute();
const router = useRouter();

const tabRoutes = ["home", "attendance", "leave", "payslips", "profile"];

const active = ref(0);

watch(
  () => route.name,
  (name) => {
    const idx = tabRoutes.indexOf(name as string);
    if (idx >= 0) active.value = idx;
  },
  { immediate: true },
);

function onTabChange(index: number | string) {
  const idx = typeof index === "string" ? parseInt(index) : index;
  router.push({ name: tabRoutes[idx] });
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
      <TabbarItem icon="home-o">{{ t("tab.home") }}</TabbarItem>
      <TabbarItem icon="clock-o">{{ t("tab.attendance") }}</TabbarItem>
      <TabbarItem icon="calendar-o">{{ t("tab.leave") }}</TabbarItem>
      <TabbarItem icon="bill-o">{{ t("tab.payslips") }}</TabbarItem>
      <TabbarItem icon="user-o">{{ t("tab.profile") }}</TabbarItem>
    </Tabbar>
    <AiFloatingBubble v-if="route.name !== 'ai-chat'" />
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
