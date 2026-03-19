<script setup lang="ts">
import { ref, computed, onMounted, h } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import {
  NCard,
  NSpace,
  NButton,
  NTag,
  NDataTable,
  NGrid,
  NGi,
  NModal,
  NForm,
  NFormItem,
  NInput,
  NSelect,
  NSpin,
  NIcon,
  NSwitch,
  useMessage,
  type DataTableColumns,
} from "naive-ui";
import {
  LogoSlack,
  LogoGithub,
  ChatbubblesOutline,
  CloudOutline,
  WalletOutline,
  DocumentTextOutline,
  CalculatorOutline,
} from "@vicons/ionicons5";
import { integrationAPI, botAPI } from "../api/client";

const { t } = useI18n();
const message = useMessage();
const router = useRouter();
const loading = ref(true);

interface Connection {
  id: string;
  provider: string;
  display_name: string;
  status: string;
  auth_type: string;
  last_used_at: string | null;
  error_count: number;
  created_at: string;
}

const connections = ref<Connection[]>([]);

const providers = [
  { key: "aistarlight", name: "AIStarlight Accounting", icon: CalculatorOutline, color: "#4f46e5" },
  { key: "slack", name: "Slack", icon: LogoSlack, color: "#4A154B" },
  { key: "google", name: "Google Workspace", icon: CloudOutline, color: "#4285F4" },
  { key: "github", name: "GitHub", icon: LogoGithub, color: "#24292e" },
  { key: "telegram", name: "Telegram", icon: ChatbubblesOutline, color: "#0088cc" },
  { key: "notion", name: "Notion", icon: DocumentTextOutline, color: "#000000" },
  { key: "xero", name: "Xero", icon: WalletOutline, color: "#13B5EA" },
  { key: "wise", name: "Wise", icon: WalletOutline, color: "#9FE870" },
];

const providerStatus = computed(() => {
  const map = new Map<string, Connection>();
  for (const c of connections.value) {
    if (!map.has(c.provider) || c.status === "active") {
      map.set(c.provider, c);
    }
  }
  return map;
});

const authTypeOptions = [
  { label: "OAuth 2.0", value: "oauth2" },
  { label: "API Key", value: "api_key" },
  { label: "Bot Token", value: "bot_token" },
  { label: "Service Account", value: "service_account" },
];

// Connection form
const showModal = ref(false);
const formLoading = ref(false);
const form = ref({
  provider: "",
  display_name: "",
  auth_type: "api_key",
  credentials: "",
});

// Telegram bot config (separate from generic integrations)
const showTelegramModal = ref(false);
const telegramLoading = ref(false);
const telegramForm = ref({
  bot_token: "",
  bot_username: "",
  is_active: false,
});
const telegramConnected = ref(false);

async function loadTelegramConfig() {
  try {
    const res = (await botAPI.listBotConfigs()) as { data?: Array<Record<string, unknown>> };
    const configs = res.data || (Array.isArray(res) ? res : []);
    const tg = (configs as Array<Record<string, unknown>>).find((c) => c.platform === "telegram");
    if (tg && tg.bot_token) {
      telegramForm.value.bot_token = (tg.bot_token as string) || "";
      telegramForm.value.bot_username = (tg.bot_username as string) || "";
      telegramForm.value.is_active = !!tg.is_active;
      telegramConnected.value = true;
    }
  } catch {
    // no config yet
  }
}

async function saveTelegramConfig() {
  if (!telegramForm.value.bot_token) {
    message.warning(t("common.fillAllFields"));
    return;
  }
  telegramLoading.value = true;
  try {
    await botAPI.saveBotConfig({
      platform: "telegram",
      bot_token: telegramForm.value.bot_token,
      bot_username: telegramForm.value.bot_username,
      is_active: telegramForm.value.is_active,
    });
    message.success(t("integration.connectionCreated"));
    showTelegramModal.value = false;
    telegramConnected.value = true;
  } catch {
    message.error(t("common.failed"));
  } finally {
    telegramLoading.value = false;
  }
}

async function disconnectTelegram() {
  try {
    await botAPI.saveBotConfig({
      platform: "telegram",
      bot_token: "",
      bot_username: "",
      is_active: false,
    });
    telegramConnected.value = false;
    telegramForm.value = { bot_token: "", bot_username: "", is_active: false };
    message.success(t("integration.disconnected"));
  } catch {
    message.error(t("common.failed"));
  }
}

// AIStarlight accounting link
const showAccountingModal = ref(false);
const accountingLoading = ref(false);
const accountingConnected = ref(false);
const accountingLink = ref<{
  id?: number;
  remote_company_id?: string;
  api_endpoint?: string;
  jurisdiction?: string;
  status?: string;
  webhook_secret?: string;
  last_synced_at?: string;
}>({});
const accountingForm = ref({
  remote_company_id: "",
  api_endpoint: "",
  jurisdiction: "PH",
});
const accountingSyncStatus = ref<{
  outbox?: { pending: number; sent: number; failed: number; dead: number };
}>({});

const jurisdictionOptions = [
  { label: "Philippines (PH)", value: "PH" },
  { label: "Sri Lanka (LK)", value: "LK" },
  { label: "Singapore (SG)", value: "SG" },
];

async function loadAccountingLink() {
  try {
    const res = (await integrationAPI.getAccountingLink()) as { data?: Record<string, unknown> };
    const data = res.data || res;
    if (data && (data as Record<string, unknown>).connected) {
      accountingConnected.value = true;
      accountingLink.value = data as typeof accountingLink.value;
    }
  } catch {
    // not connected
  }
  try {
    const res = (await integrationAPI.getAccountingSyncStatus()) as { data?: Record<string, unknown> };
    const data = res.data || res;
    accountingSyncStatus.value = data as typeof accountingSyncStatus.value;
  } catch {
    // ignore
  }
}

async function saveAccountingLink() {
  if (!accountingForm.value.remote_company_id || !accountingForm.value.api_endpoint) {
    message.warning(t("common.fillAllFields"));
    return;
  }
  accountingLoading.value = true;
  try {
    const res = (await integrationAPI.createAccountingLink({
      remote_company_id: accountingForm.value.remote_company_id,
      api_endpoint: accountingForm.value.api_endpoint,
      jurisdiction: accountingForm.value.jurisdiction,
    })) as { data?: Record<string, unknown> };
    const data = res.data || res;
    message.success(t("integration.connectionCreated"));
    showAccountingModal.value = false;
    accountingConnected.value = true;
    accountingLink.value = data as typeof accountingLink.value;
    // Show webhook secret for user to copy
    if ((data as Record<string, unknown>).webhook_secret) {
      message.info(`Webhook Secret: ${(data as Record<string, unknown>).webhook_secret}`, { duration: 15000 });
    }
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } };
    message.error(err.data?.error?.message || t("common.failed"));
  } finally {
    accountingLoading.value = false;
  }
}

async function disconnectAccounting() {
  if (!accountingLink.value.id) return;
  try {
    await integrationAPI.deleteAccountingLink(accountingLink.value.id);
    accountingConnected.value = false;
    accountingLink.value = {};
    message.success(t("integration.disconnected"));
  } catch {
    message.error(t("common.failed"));
  }
}

function openConnect(providerKey: string) {
  if (providerKey === "telegram") {
    showTelegramModal.value = true;
    return;
  }
  if (providerKey === "aistarlight") {
    showAccountingModal.value = true;
    return;
  }
  form.value = {
    provider: providerKey,
    display_name: providers.find((p) => p.key === providerKey)?.name || providerKey,
    auth_type: "api_key",
    credentials: "",
  };
  showModal.value = true;
}

async function submitConnection() {
  if (!form.value.display_name || !form.value.credentials) {
    message.warning(t("common.fillAllFields"));
    return;
  }
  formLoading.value = true;
  try {
    let creds: Record<string, unknown>;
    try {
      creds = JSON.parse(form.value.credentials);
    } catch {
      creds = { token: form.value.credentials };
    }
    await integrationAPI.createConnection({
      provider: form.value.provider,
      display_name: form.value.display_name,
      auth_type: form.value.auth_type,
      credentials: creds,
    });
    message.success(t("integration.connectionCreated"));
    showModal.value = false;
    loadConnections();
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } };
    message.error(err.data?.error?.message || t("common.failed"));
  } finally {
    formLoading.value = false;
  }
}

async function testConnection(id: string) {
  try {
    await integrationAPI.testConnection(id);
    message.success(t("integration.testSuccess"));
  } catch {
    message.error(t("integration.testFailed"));
  }
}

async function deleteConnection(id: string) {
  try {
    await integrationAPI.deleteConnection(id);
    message.success(t("common.delete"));
    loadConnections();
  } catch {
    message.error(t("common.deleteFailed"));
  }
}

const statusMap: Record<string, "success" | "warning" | "error" | "default"> = {
  active: "success",
  inactive: "default",
  error: "error",
  pending: "warning",
};

const columns: DataTableColumns<Connection> = [
  {
    title: () => t("integration.provider"),
    key: "provider",
    width: 120,
    render: (row) => row.provider.charAt(0).toUpperCase() + row.provider.slice(1),
  },
  {
    title: () => t("integration.displayName"),
    key: "display_name",
    ellipsis: { tooltip: true },
  },
  {
    title: () => t("common.status"),
    key: "status",
    width: 100,
    render: (row) =>
      h(NTag, { type: statusMap[row.status] || "default", size: "small" }, { default: () => row.status }),
  },
  {
    title: () => t("integration.authType"),
    key: "auth_type",
    width: 120,
  },
  {
    title: () => t("integration.lastUsed"),
    key: "last_used_at",
    width: 160,
    render: (row) => (row.last_used_at ? new Date(row.last_used_at).toLocaleDateString() : "-"),
  },
  {
    title: () => t("common.actions"),
    key: "actions",
    width: 200,
    render: (row) =>
      h(NSpace, { size: 4 }, {
        default: () => [
          h(NButton, { size: "small", onClick: () => testConnection(row.id) }, { default: () => t("integration.test") }),
          h(NButton, { size: "small", onClick: () => router.push({ name: "integration-detail", params: { id: row.id } }) }, { default: () => t("common.edit") }),
          h(NButton, { size: "small", type: "error", onClick: () => deleteConnection(row.id) }, { default: () => t("common.delete") }),
        ],
      }),
  },
];

async function loadConnections() {
  loading.value = true;
  try {
    const res = (await integrationAPI.listConnections()) as { data?: Connection[] };
    connections.value = res.data || (Array.isArray(res) ? (res as Connection[]) : []);
  } catch {
    connections.value = [];
  } finally {
    loading.value = false;
  }
}

onMounted(() => {
  loadConnections();
  loadTelegramConfig();
  loadAccountingLink();
});
</script>

<template>
  <NSpin :show="loading">
    <NSpace vertical :size="24">
      <NSpace justify="space-between" align="center">
        <div>
          <h2>{{ t("integration.title") }}</h2>
          <p style="color: var(--n-text-color3); margin-top: 4px;">{{ t("integration.subtitle") }}</p>
        </div>
        <NButton type="primary" @click="router.push({ name: 'provisioning-jobs' })">
          {{ t("integration.viewJobs") }}
        </NButton>
      </NSpace>

      <!-- Provider Cards -->
      <NGrid :cols="4" :x-gap="16" :y-gap="16" responsive="screen" :item-responsive="true">
        <NGi v-for="p in providers" :key="p.key" span="4 s:2 m:1">
          <NCard hoverable size="small" style="text-align: center;">
            <NSpace vertical align="center" :size="8">
              <NIcon :size="32" :color="p.color" :component="p.icon" />
              <strong>{{ p.name }}</strong>
              <NTag
                v-if="p.key === 'aistarlight' && accountingConnected"
                type="success"
                size="small"
              >
                active
              </NTag>
              <NTag
                v-else-if="p.key === 'telegram' && telegramConnected"
                :type="telegramForm.is_active ? 'success' : 'default'"
                size="small"
              >
                {{ telegramForm.is_active ? 'active' : 'inactive' }}
              </NTag>
              <NTag
                v-else-if="!['telegram', 'aistarlight'].includes(p.key) && providerStatus.has(p.key)"
                :type="statusMap[providerStatus.get(p.key)!.status] || 'default'"
                size="small"
              >
                {{ providerStatus.get(p.key)!.status }}
              </NTag>
              <NTag v-else size="small">{{ t("integration.notConnected") }}</NTag>
            </NSpace>
            <template #action>
              <NSpace justify="center" :size="8">
                <!-- AIStarlight: accounting integration -->
                <template v-if="p.key === 'aistarlight'">
                  <template v-if="accountingConnected">
                    <NButton size="small" @click="showAccountingModal = true">
                      {{ t("nav.settings") }}
                    </NButton>
                    <NButton size="small" type="error" quaternary @click="disconnectAccounting">
                      {{ t("integration.disconnect") }}
                    </NButton>
                  </template>
                  <NButton v-else size="small" type="primary" @click="openConnect('aistarlight')">
                    {{ t("integration.connect") }}
                  </NButton>
                </template>
                <!-- Telegram: special handling via bot_configs -->
                <template v-else-if="p.key === 'telegram'">
                  <template v-if="telegramConnected">
                    <NButton size="small" @click="showTelegramModal = true">
                      {{ t("nav.settings") }}
                    </NButton>
                    <NButton size="small" type="error" quaternary @click="disconnectTelegram">
                      {{ t("integration.disconnect") }}
                    </NButton>
                  </template>
                  <NButton v-else size="small" type="primary" @click="openConnect('telegram')">
                    {{ t("integration.connect") }}
                  </NButton>
                </template>
                <!-- Other providers: generic integration_connections -->
                <template v-else-if="providerStatus.has(p.key)">
                  <NButton
                    size="small"
                    @click="router.push({ name: 'integration-detail', params: { id: providerStatus.get(p.key)!.id } })"
                  >
                    {{ t("nav.settings") }}
                  </NButton>
                  <NButton
                    size="small"
                    type="error"
                    quaternary
                    @click="deleteConnection(providerStatus.get(p.key)!.id)"
                  >
                    {{ t("integration.disconnect") }}
                  </NButton>
                </template>
                <NButton v-else size="small" type="primary" @click="openConnect(p.key)">
                  {{ t("integration.connect") }}
                </NButton>
              </NSpace>
            </template>
          </NCard>
        </NGi>
      </NGrid>

      <!-- Connections Table -->
      <NCard :title="t('integration.activeConnections')" v-if="connections.length">
        <NDataTable :columns="columns" :data="connections" :row-key="(row: Connection) => row.id" size="small" />
      </NCard>
    </NSpace>

    <!-- Telegram Bot Config Modal -->
    <NModal v-model:show="showTelegramModal" preset="card" title="Telegram Bot" style="width: 480px; max-width: 95vw;">
      <NForm label-placement="left" label-width="120">
        <NFormItem label="Bot Token" required>
          <NInput v-model:value="telegramForm.bot_token" type="password" show-password-on="click" placeholder="123456:ABC-DEF..." />
        </NFormItem>
        <NFormItem label="Bot Username">
          <NInput v-model:value="telegramForm.bot_username" placeholder="@your_bot" />
        </NFormItem>
        <NFormItem label="Active">
          <NSpace align="center" :size="8">
            <NSwitch v-model:value="telegramForm.is_active" />
            <span v-if="telegramForm.is_active" style="color: #18a058; font-size: 12px;">Bot will start on next server restart</span>
          </NSpace>
        </NFormItem>
        <p style="color: #999; font-size: 12px; margin-bottom: 12px;">
          Get your bot token from <a href="https://t.me/BotFather" target="_blank" style="color: #0088cc;">@BotFather</a> on Telegram.
          After saving, the bot needs a server restart to take effect.
        </p>
        <NSpace>
          <NButton type="primary" :loading="telegramLoading" @click="saveTelegramConfig">{{ t("common.save") }}</NButton>
          <NButton @click="showTelegramModal = false">{{ t("common.cancel") }}</NButton>
        </NSpace>
      </NForm>
    </NModal>

    <!-- AIStarlight Accounting Modal -->
    <NModal v-model:show="showAccountingModal" preset="card" title="AIStarlight Accounting" style="width: 520px; max-width: 95vw;">
      <template v-if="accountingConnected">
        <NSpace vertical :size="12">
          <div>
            <strong>Status:</strong> <NTag type="success" size="small">Connected</NTag>
          </div>
          <div><strong>Jurisdiction:</strong> {{ accountingLink.jurisdiction }}</div>
          <div><strong>API Endpoint:</strong> {{ accountingLink.api_endpoint }}</div>
          <div><strong>Remote Company ID:</strong> {{ accountingLink.remote_company_id }}</div>
          <div v-if="accountingLink.last_synced_at">
            <strong>Last Synced:</strong> {{ new Date(accountingLink.last_synced_at).toLocaleString() }}
          </div>
          <div v-if="accountingSyncStatus.outbox">
            <strong>Outbox:</strong>
            <NSpace :size="8" style="margin-top: 4px;">
              <NTag size="small">Pending: {{ accountingSyncStatus.outbox.pending }}</NTag>
              <NTag size="small" type="success">Sent: {{ accountingSyncStatus.outbox.sent }}</NTag>
              <NTag v-if="accountingSyncStatus.outbox.failed" size="small" type="warning">Failed: {{ accountingSyncStatus.outbox.failed }}</NTag>
              <NTag v-if="accountingSyncStatus.outbox.dead" size="small" type="error">Dead: {{ accountingSyncStatus.outbox.dead }}</NTag>
            </NSpace>
          </div>
        </NSpace>
      </template>
      <template v-else>
        <p style="color: #666; margin-bottom: 16px;">
          Connect to AIStarlight to automatically sync payroll data to accounting journal entries and tax forms.
        </p>
        <NForm label-placement="left" label-width="140">
          <NFormItem label="Remote Company ID" required>
            <NInput v-model:value="accountingForm.remote_company_id" placeholder="UUID of company in AIStarlight" />
          </NFormItem>
          <NFormItem label="API Endpoint" required>
            <NInput v-model:value="accountingForm.api_endpoint" placeholder="https://tax.clawpapa.win" />
          </NFormItem>
          <NFormItem label="Jurisdiction" required>
            <NSelect v-model:value="accountingForm.jurisdiction" :options="jurisdictionOptions" />
          </NFormItem>
          <NSpace>
            <NButton type="primary" :loading="accountingLoading" @click="saveAccountingLink">{{ t("integration.connect") }}</NButton>
            <NButton @click="showAccountingModal = false">{{ t("common.cancel") }}</NButton>
          </NSpace>
        </NForm>
        <p style="color: #999; font-size: 12px; margin-top: 12px;">
          After connecting, a webhook secret will be generated. Copy it to configure the integration source in AIStarlight.
        </p>
      </template>
    </NModal>

    <!-- Connect Modal -->
    <NModal v-model:show="showModal" preset="card" :title="t('integration.connect')" style="width: 480px; max-width: 95vw;">
      <NForm label-placement="left" label-width="120">
        <NFormItem :label="t('integration.provider')">
          <NInput :value="form.provider" disabled />
        </NFormItem>
        <NFormItem :label="t('integration.displayName')" required>
          <NInput v-model:value="form.display_name" />
        </NFormItem>
        <NFormItem :label="t('integration.authType')">
          <NSelect v-model:value="form.auth_type" :options="authTypeOptions" />
        </NFormItem>
        <NFormItem :label="t('integration.credentials')" required>
          <NInput
            v-model:value="form.credentials"
            type="textarea"
            :rows="3"
            :placeholder="t('integration.credentialsHint')"
          />
        </NFormItem>
        <NSpace>
          <NButton type="primary" :loading="formLoading" @click="submitConnection">{{ t("common.save") }}</NButton>
          <NButton @click="showModal = false">{{ t("common.cancel") }}</NButton>
        </NSpace>
      </NForm>
    </NModal>
  </NSpin>
</template>
