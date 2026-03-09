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
} from "@vicons/ionicons5";
import { integrationAPI } from "../api/client";

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

function openConnect(providerKey: string) {
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

onMounted(loadConnections);
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
                v-if="providerStatus.has(p.key)"
                :type="statusMap[providerStatus.get(p.key)!.status] || 'default'"
                size="small"
              >
                {{ providerStatus.get(p.key)!.status }}
              </NTag>
              <NTag v-else size="small">{{ t("integration.notConnected") }}</NTag>
            </NSpace>
            <template #action>
              <NSpace justify="center" :size="8">
                <template v-if="providerStatus.has(p.key)">
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
