<script setup lang="ts">
import { ref } from "vue";
import { useRouter, useRoute } from "vue-router";
import { useI18n } from "vue-i18n";
import { Form, Field, Button, CellGroup, showToast } from "vant";
import { useAuthStore } from "../stores/auth";

const { t } = useI18n();
const router = useRouter();
const route = useRoute();
const auth = useAuthStore();

const email = ref("");
const password = ref("");
const loading = ref(false);

async function onSubmit() {
  loading.value = true;
  try {
    await auth.login(email.value, password.value);
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

    <Form @submit="onSubmit" class="login-form">
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
</style>
