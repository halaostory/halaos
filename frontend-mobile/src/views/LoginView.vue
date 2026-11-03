<script setup lang="ts">
import { ref } from "vue";
import { useRouter, useRoute } from "vue-router";
import { useI18n } from "vue-i18n";
import { Form, Field, Button, CellGroup, Cell, showToast } from "vant";
import { useAuthStore } from "../stores/auth";
import { isNative, serverURL, setServerURL } from "../platform";

const { t } = useI18n();
const router = useRouter();
const route = useRoute();
const auth = useAuthStore();

const email = ref("");
const password = ref("");
const loading = ref(false);

const mfaToken = ref("");
const companies = ref<{ id: number; name: string }[]>([]);

// Native app: which HalaPeople server to sign in to (cloud by default, or
// the company's own server when self-hosted).
const showServer = ref(isNative && !import.meta.env.VITE_API_URL && !serverURL());
const server = ref(serverURL());
function saveServer() {
  setServerURL(server.value);
  window.location.reload(); // API client picks up the new server
}
const mfaCode = ref("");

async function onMfa() {
  loading.value = true;
  try {
    await auth.verifyMfa(mfaToken.value, mfaCode.value);
    router.replace((route.query.redirect as string) || "/");
  } catch {
    showToast({ message: t("login.mfaFailed"), type: "fail" });
  } finally {
    loading.value = false;
  }
}

async function onSubmit(companyId?: number) {
  loading.value = true;
  try {
    const r = await auth.login(email.value, password.value, companyId);
    if (r.companies) {
      companies.value = r.companies;
      return;
    }
    companies.value = [];
    if (r.mfaToken) {
      mfaToken.value = r.mfaToken;
      return;
    }
    const redirect = (route.query.redirect as string) || "/";
    router.replace(redirect);
  } catch {
    showToast({ message: t("login.failed"), type: "fail" });
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <div class="login-page">
    <div class="login-header">
      <h1 class="login-title">{{ t("login.title") }}</h1>
      <p class="login-subtitle">{{ t("login.subtitle") }}</p>
    </div>

    <Form v-if="mfaToken" @submit="onMfa" class="login-form">
      <p class="mfa-help">{{ t("login.mfaHelp") }}</p>
      <CellGroup inset>
        <Field v-model="mfaCode" name="code" :label="t('login.mfaCode')" placeholder="123456" inputmode="numeric" autocomplete="one-time-code" :rules="[{ required: true }]" />
      </CellGroup>
      <div class="login-actions">
        <Button round block type="primary" native-type="submit" :loading="loading" size="large">{{ t("login.mfaVerify") }}</Button>
      </div>
    </Form>
    <div v-else-if="companies.length" class="login-form">
      <p class="mfa-help">{{ t("login.chooseCompany") }}</p>
      <CellGroup inset>
        <Cell v-for="co in companies" :key="co.id" :title="co.name" is-link :clickable="!loading" @click="onSubmit(co.id)" />
      </CellGroup>
      <div class="login-actions">
        <Button round block plain size="large" @click="companies = []">{{ t("login.back") }}</Button>
      </div>
    </div>
    <Form v-else-if="showServer" @submit="saveServer" class="login-form">
      <p class="mfa-help">{{ t("login.serverHelp") }}</p>
      <CellGroup inset>
        <Field v-model="server" name="server" :label="t('login.server')" placeholder="people.yourcompany.com" type="url" autocomplete="url" :rules="[{ required: true }]" />
      </CellGroup>
      <div class="login-actions">
        <Button round block type="primary" native-type="submit" size="large">{{ t("login.serverSave") }}</Button>
      </div>
    </Form>
    <Form v-else @submit="onSubmit()" class="login-form">
      <CellGroup inset>
        <Field
          v-model="email"
          name="email"
          :label="t('login.email')"
          :placeholder="t('login.emailPlaceholder')"
          type="email"
          autocomplete="email"
          :rules="[{ required: true }]"
        />
        <Field
          v-model="password"
          name="password"
          :label="t('login.password')"
          :placeholder="t('login.passwordPlaceholder')"
          type="password"
          autocomplete="current-password"
          :rules="[{ required: true }]"
        />
      </CellGroup>

      <div class="login-actions">
        <Button
          round
          block
          type="primary"
          native-type="submit"
          :loading="loading"
          size="large"
        >
          {{ t("login.submit") }}
        </Button>
        <Button v-if="isNative" round block plain size="small" style="margin-top: 12px" @click="showServer = true">
          {{ serverURL() ? t("login.serverCurrent", { url: serverURL() }) : t("login.serverChange") }}
        </Button>
      </div>
    </Form>
  </div>
</template>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  justify-content: center;
  padding: 0 16px;
  background: var(--app-bg);
}

.login-header {
  text-align: center;
  margin-bottom: 40px;
}

.login-title {
  font-size: 28px;
  font-weight: 700;
  color: var(--brand-color);
}

.login-subtitle {
  font-size: 14px;
  color: var(--text-secondary);
  margin-top: 8px;
}

.login-form {
  max-width: 400px;
  margin: 0 auto;
  width: 100%;
}

.login-actions {
  padding: 24px 16px;
}
.mfa-help { padding: 0 16px; font-size: 14px; color: var(--van-text-color-2); }
</style>
