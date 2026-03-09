<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import {
  Cell,
  CellGroup,
  Switch,
  Button,
  Popup,
  Form,
  Field,
  showToast,
  showConfirmDialog,
} from "vant";
import AiQuickAsk from "../components/ai/AiQuickAsk.vue";
import { useAuthStore } from "../stores/auth";
import { useThemeStore } from "../stores/theme";
import { authAPI, selfServiceAPI, botAPI } from "../api/client";
import type { ApiResponse } from "../types";

const { t, locale } = useI18n();
const router = useRouter();
const auth = useAuthStore();
const theme = useThemeStore();

interface EmployeeInfo {
  employee_id?: string;
  department_name?: string;
  position_title?: string;
}

const employeeInfo = ref<EmployeeInfo | null>(null);
const showPasswordPopup = ref(false);
const currentPassword = ref("");
const newPassword = ref("");
const confirmPassword = ref("");
const passwordLoading = ref(false);

// Telegram Bot Link
const showBotPopup = ref(false);
const botLinked = ref(false);
const botUsername = ref("");
const botLinkCode = ref("");
const botLoading = ref(false);

async function loadBotStatus() {
  try {
    const res = (await botAPI.getLinkStatus()) as ApiResponse<{
      linked: boolean;
      platform_username?: string;
    }>;
    const data = res.data ?? (res as unknown as { linked: boolean; platform_username?: string });
    botLinked.value = !!data.linked;
    botUsername.value = data.platform_username || "";
  } catch {
    // not linked
  }
}

async function generateLinkCode() {
  botLoading.value = true;
  try {
    const res = (await botAPI.getLinkCode()) as ApiResponse<{ code: string }>;
    const data = res.data ?? (res as unknown as { code: string });
    botLinkCode.value = data.code || "";
    showBotPopup.value = true;
  } catch {
    showToast({ message: t("common.failed"), type: "fail" });
  } finally {
    botLoading.value = false;
  }
}

async function unlinkTelegram() {
  botLoading.value = true;
  try {
    await botAPI.unlinkPlatform("telegram");
    botLinked.value = false;
    botUsername.value = "";
    botLinkCode.value = "";
    showToast({ message: t("profile.telegramUnlinked"), type: "success" });
  } catch {
    showToast({ message: t("common.failed"), type: "fail" });
  } finally {
    botLoading.value = false;
  }
}

async function loadInfo() {
  try {
    const res = (await selfServiceAPI.getMyInfo()) as ApiResponse<EmployeeInfo>;
    employeeInfo.value = res.data ?? (res as unknown as EmployeeInfo);
  } catch {
    // ignore
  }
}

function switchLanguage() {
  const next = locale.value === "en" ? "zh" : "en";
  locale.value = next;
  localStorage.setItem("locale", next);
}

async function onChangePassword() {
  if (newPassword.value !== confirmPassword.value) {
    showToast({ message: t("profile.passwordMismatch"), type: "fail" });
    return;
  }
  passwordLoading.value = true;
  try {
    await authAPI.changePassword({
      current_password: currentPassword.value,
      new_password: newPassword.value,
    });
    showToast({ message: t("profile.passwordChanged"), type: "success" });
    showPasswordPopup.value = false;
    currentPassword.value = "";
    newPassword.value = "";
    confirmPassword.value = "";
  } catch {
    showToast({ message: t("profile.passwordFailed"), type: "fail" });
  } finally {
    passwordLoading.value = false;
  }
}

async function onLogout() {
  try {
    await showConfirmDialog({
      title: t("profile.logout"),
      message: t("profile.logoutConfirm"),
    });
    auth.logout();
    router.replace({ name: "login" });
  } catch {
    // cancelled
  }
}

onMounted(() => {
  loadInfo();
  loadBotStatus();
});
</script>

<template>
  <div class="profile-page">
    <!-- User Header -->
    <div class="profile-header">
      <div class="profile-avatar">
        {{ auth.user?.first_name?.charAt(0) || "?" }}
      </div>
      <div class="profile-name">{{ auth.fullName }}</div>
      <div class="profile-role">{{ auth.user?.role }}</div>
    </div>

    <!-- Personal Info -->
    <CellGroup inset :title="t('profile.personalInfo')">
      <Cell :title="t('profile.email')" :value="auth.user?.email" />
      <Cell :title="t('profile.role')" :value="auth.user?.role" />
      <Cell
        v-if="employeeInfo?.department_name"
        :title="t('profile.department')"
        :value="employeeInfo.department_name"
      />
      <Cell
        v-if="employeeInfo?.position_title"
        :title="t('profile.position')"
        :value="employeeInfo.position_title"
      />
      <Cell
        v-if="employeeInfo?.employee_id"
        :title="t('profile.employeeId')"
        :value="employeeInfo.employee_id"
      />
    </CellGroup>

    <!-- AI Quick Ask -->
    <AiQuickAsk
      :questions="[
        'How do I change my password?',
        'What are my benefits?',
        'Show company policies',
      ]"
    />

    <!-- Telegram Bot -->
    <CellGroup inset :title="t('profile.telegramBot')">
      <Cell
        v-if="botLinked"
        :title="t('profile.telegramConnected')"
        :value="'@' + botUsername"
        is-link
        @click="unlinkTelegram"
      >
        <template #right-icon>
          <span style="color: var(--van-danger-color); font-size: 12px;">{{ t('profile.telegramDisconnect') }}</span>
        </template>
      </Cell>
      <Cell
        v-else
        :title="t('profile.telegramConnect')"
        is-link
        :loading="botLoading"
        @click="generateLinkCode"
      />
    </CellGroup>

    <!-- Settings -->
    <CellGroup inset :title="t('profile.settings')">
      <Cell
        :title="t('profile.changePassword')"
        is-link
        @click="showPasswordPopup = true"
      />
      <Cell :title="t('profile.language')" is-link @click="switchLanguage">
        <template #right-icon>
          <span class="lang-value">{{ locale === "en" ? "English" : "中文" }}</span>
        </template>
      </Cell>
      <Cell :title="t('profile.darkMode')" center>
        <template #right-icon>
          <Switch :model-value="theme.isDark" size="22" @update:model-value="theme.toggle" />
        </template>
      </Cell>
      <Cell
        :title="t('profile.notifications')"
        is-link
        @click="router.push({ name: 'notifications' })"
      />
    </CellGroup>

    <!-- Logout -->
    <div class="logout-section">
      <Button round block type="danger" plain @click="onLogout">
        {{ t("profile.logout") }}
      </Button>
    </div>

    <!-- Change Password Popup -->
    <Popup
      v-model:show="showPasswordPopup"
      position="bottom"
      round
      :style="{ height: '50%' }"
      closeable
    >
      <div class="password-popup">
        <h3 class="popup-title">{{ t("profile.changePassword") }}</h3>
        <Form @submit="onChangePassword">
          <CellGroup inset>
            <Field
              v-model="currentPassword"
              :label="t('profile.currentPassword')"
              type="password"
              :rules="[{ required: true }]"
            />
            <Field
              v-model="newPassword"
              :label="t('profile.newPassword')"
              type="password"
              :rules="[{ required: true }]"
            />
            <Field
              v-model="confirmPassword"
              :label="t('profile.confirmPassword')"
              type="password"
              :rules="[{ required: true }]"
            />
          </CellGroup>
          <div class="form-actions">
            <Button
              round
              block
              type="primary"
              native-type="submit"
              :loading="passwordLoading"
            >
              {{ t("common.confirm") }}
            </Button>
          </div>
        </Form>
      </div>
    </Popup>
    <!-- Bot Link Code Popup -->
    <Popup
      v-model:show="showBotPopup"
      position="bottom"
      round
      :style="{ height: '40%' }"
      closeable
    >
      <div class="password-popup">
        <h3 class="popup-title">{{ t("profile.telegramConnect") }}</h3>
        <p style="text-align: center; color: var(--text-secondary); margin-bottom: 16px;">
          {{ t("profile.telegramCodeInstructions") }}
        </p>
        <div style="text-align: center; font-size: 32px; font-weight: 700; letter-spacing: 6px; font-family: monospace; padding: 16px 0;">
          {{ botLinkCode }}
        </div>
        <p style="text-align: center; font-size: 12px; color: var(--text-secondary);">
          {{ t("profile.telegramCodeHint") }}
        </p>
      </div>
    </Popup>
  </div>
</template>

<style scoped>
.profile-page {
  padding-bottom: 24px;
}

.profile-header {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 32px 16px 24px;
  background: linear-gradient(180deg, #e8f4fd 0%, var(--app-bg) 100%);
}

.profile-avatar {
  width: 64px;
  height: 64px;
  border-radius: 50%;
  background: var(--brand-color);
  color: #fff;
  font-size: 28px;
  font-weight: 600;
  display: flex;
  align-items: center;
  justify-content: center;
}

.profile-name {
  font-size: 20px;
  font-weight: 600;
  margin-top: 12px;
}

.profile-role {
  font-size: 13px;
  color: var(--text-secondary);
  margin-top: 4px;
  text-transform: capitalize;
}

.lang-value {
  font-size: 14px;
  color: var(--text-secondary);
  margin-right: 4px;
}

.logout-section {
  padding: 24px 16px;
}

.password-popup {
  padding: 16px;
}

.popup-title {
  text-align: center;
  font-size: 16px;
  margin-bottom: 16px;
}

.form-actions {
  padding: 20px 16px;
}
</style>
