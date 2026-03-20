# HalaOS UI Unification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Unify the HR and Finance frontends so login, registration, dashboard shell, theme, auth, and i18n are visually and functionally identical across both products.

**Architecture:** Two separate Vue3+NaiveUI codebases (HR at `/Users/anna/Documents/aigonhr/frontend`, Finance at `/Users/anna/Documents/aistarlight/frontend`) are modified independently to converge on the same patterns. HR is the reference for most patterns; Finance contributes jurisdiction, multi-tenant, responsive CSS, and SSO. No shared component library — each project gets its own copy of unified code.

**Tech Stack:** Vue 3, TypeScript, NaiveUI, Pinia, vue-i18n, vue-router, ofetch (HR), axios (Finance)

**Spec:** `docs/superpowers/specs/2026-03-21-halaos-ui-unification-design.md`

---

## File Map

### HR Project (`/Users/anna/Documents/aigonhr/frontend/src/`)

| File | Action | Purpose |
|------|--------|---------|
| `style.css` | Modify | Expand 6 → 19 CSS variables, add backward-compat aliases |
| `stores/theme.ts` | Modify | Add `html.dark` class toggle, add `sidebarCollapsed` state |
| `assets/responsive.css` | Create | Copy Finance's responsive utilities |
| `main.ts` | Modify | Import responsive.css |
| `views/LoginView.vue` | Modify | Add jurisdiction selector, change colors to blue, add data-testid |
| `views/RegisterView.vue` | Modify | Add jurisdiction selector, convert hardcoded strings to i18n |
| `components/DashboardLayout.vue` | Modify | Single-line menu, jurisdiction badge, company switcher, collapse state |
| `stores/auth.ts` | Modify | Rename token→access_token, add jurisdiction/companies/SSO/switchCompany |
| `api/client.ts` | Modify | Update token key name, add SSO and company-switching endpoints |
| `views/SSOCallbackView.vue` | Create | SSO token login callback page |
| `router/index.ts` | Modify | Move /dashboard→/, add /sso route, add redirects |
| `i18n/en.ts` | Modify | Add missing auth/nav keys for RegisterView |
| `i18n/zh.ts` | Modify | Add missing auth/nav keys for RegisterView |

### Finance Project (`/Users/anna/Documents/aistarlight/frontend/src/`)

| File | Action | Purpose |
|------|--------|---------|
| `locales/en.ts` | Create | English translations (shell keys) |
| `locales/zh.ts` | Create | Chinese translations (shell keys) |
| `main.ts` | Modify | Add vue-i18n setup |
| `views/LoginView.vue` | Modify | Light background, logo, remove inline register, add validation, i18n |
| `views/RegisterView.vue` | Create | Magic link registration flow |
| `views/VerifyEmailView.vue` | Create | Email verification callback |
| `views/SSOCallbackView.vue` | Modify | Use CSS variables instead of hardcoded colors |
| `components/layout/DashboardLayout.vue` | Modify | Add locale toggle, i18n shell strings, SSO cross-app nav |
| `stores/auth.ts` | Modify | Align login() signature to object param |
| `api/auth.ts` | Modify | Add SSO token generation endpoint, magic link endpoints |
| `router/index.ts` | Modify | Add /register, /verify-email routes |

---

## Task Order & Dependencies

```
Task 1 (HR Theme/CSS) ──────────────┐
Task 2 (HR Responsive CSS) ─────────┤
Task 3 (Finance i18n Setup) ────────┤
                                     ├── Task 4 (HR Login) ──── Task 6 (HR Register)
                                     ├── Task 5 (Finance Login) ── Task 7 (Finance Register)
                                     ├── Task 8 (HR Dashboard Shell)
                                     ├── Task 9 (Finance Dashboard Shell)
Task 10 (HR Auth Store) ────────────── Task 11 (HR Routes + SSO)
Task 12 (Finance Auth/SSO) ─────────── Task 13 (Build Verification)
```

Tasks 1-3 are independent foundations. Tasks 4-9 depend on foundations. Tasks 10-12 are auth/routing. Task 13 is final verification.

---

### Task 1: HR — Expand CSS Variables & Theme Store

**Files:**
- Modify: `/Users/anna/Documents/aigonhr/frontend/src/style.css`
- Modify: `/Users/anna/Documents/aigonhr/frontend/src/stores/theme.ts`

- [ ] **Step 1: Replace style.css with 19-variable set**

Replace the entire content of `style.css` with:

```css
/* HalaOS Unified Design Tokens */
:root,
[data-theme="light"] {
  --bg-app: #f8fafc;
  --bg-surface: #ffffff;
  --bg-surface-alt: #f9fafb;
  --bg-surface-hover: #f3f4f6;
  --bg-input: #ffffff;
  --text-primary: #111111;
  --text-secondary: #555555;
  --text-muted: #9ca3af;
  --border-default: #e5e7eb;
  --border-input: #d1d5db;
  --bg-accent: #eff6ff;
  --brand-primary: #2563eb;
  --brand-primary-hover: #1d4ed8;

  /* Backward compat aliases (old HR variable names) */
  --primary: #2563eb;
  --bg: #ffffff;
  --bg-secondary: #f8fafc;
  --text: #111111;
  --text-secondary-old: #555555;
  --border: #e5e7eb;
}

html.dark,
[data-theme="dark"] {
  --bg-app: #0f1117;
  --bg-surface: #1c1c27;
  --bg-surface-alt: #252535;
  --bg-surface-hover: #2e2e42;
  --bg-input: #252535;
  --text-primary: #f0f0f5;
  --text-secondary: #a0a0b8;
  --text-muted: #6b7280;
  --border-default: #2e2e42;
  --border-input: #3a3a54;
  --bg-accent: #1e2a4a;
  --brand-primary: #3b82f6;
  --brand-primary-hover: #60a5fa;

  /* Backward compat aliases */
  --primary: #3b82f6;
  --bg: #1c1c27;
  --bg-secondary: #0f1117;
  --text: #f0f0f5;
  --border: #2e2e42;
}

* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  background: var(--bg-app);
  color: var(--text-primary);
}

#app {
  min-height: 100vh;
}
```

- [ ] **Step 2: Update theme store to add html.dark class + sidebarCollapsed**

Replace `/Users/anna/Documents/aigonhr/frontend/src/stores/theme.ts` with:

```ts
import { ref, computed, watch } from 'vue'
import { defineStore } from 'pinia'

export const useThemeStore = defineStore('theme', () => {
  const sidebarCollapsed = ref(false)
  const mode = ref<'light' | 'dark'>(
    (localStorage.getItem('theme') as 'light' | 'dark') || 'light'
  )

  const isDark = computed(() => mode.value === 'dark')

  function toggle() {
    mode.value = mode.value === 'dark' ? 'light' : 'dark'
  }

  watch(mode, (val) => {
    localStorage.setItem('theme', val)
    document.documentElement.setAttribute('data-theme', val)
    if (val === 'dark') {
      document.documentElement.classList.add('dark')
    } else {
      document.documentElement.classList.remove('dark')
    }
  }, { immediate: true })

  return { sidebarCollapsed, mode, isDark, toggle }
})
```

- [ ] **Step 3: Verify build**

```bash
cd /Users/anna/Documents/aigonhr/frontend && npx vue-tsc -b --noEmit 2>&1 | head -20; npx vite build 2>&1 | tail -5
```

Expected: Build succeeds. If scoped styles reference old variable names (`--bg`, `--text`, `--border`), the backward compat aliases ensure they still work.

- [ ] **Step 4: Commit**

```bash
cd /Users/anna/Documents/aigonhr && git add frontend/src/style.css frontend/src/stores/theme.ts
git commit -m "feat: expand CSS variables to 19-token design system and add html.dark toggle"
```

---

### Task 2: HR — Add Responsive CSS

**Files:**
- Create: `/Users/anna/Documents/aigonhr/frontend/src/assets/responsive.css`
- Modify: `/Users/anna/Documents/aigonhr/frontend/src/main.ts`

- [ ] **Step 1: Copy Finance's responsive.css to HR**

```bash
cp /Users/anna/Documents/aistarlight/frontend/src/assets/responsive.css /Users/anna/Documents/aigonhr/frontend/src/assets/responsive.css
```

- [ ] **Step 2: Add import to main.ts**

In `/Users/anna/Documents/aigonhr/frontend/src/main.ts`, add after the `import "./style.css"` line:

```ts
import "./assets/responsive.css";
```

- [ ] **Step 3: Verify build**

```bash
cd /Users/anna/Documents/aigonhr/frontend && npx vite build 2>&1 | tail -5
```

- [ ] **Step 4: Commit**

```bash
cd /Users/anna/Documents/aigonhr && git add frontend/src/assets/responsive.css frontend/src/main.ts
git commit -m "feat: add responsive CSS utilities from Finance"
```

---

### Task 3: Finance — Install vue-i18n & Create Locale Files

**Files:**
- Modify: `/Users/anna/Documents/aistarlight/frontend/package.json` (via npm install)
- Create: `/Users/anna/Documents/aistarlight/frontend/src/locales/en.ts`
- Create: `/Users/anna/Documents/aistarlight/frontend/src/locales/zh.ts`
- Modify: `/Users/anna/Documents/aistarlight/frontend/src/main.ts`

- [ ] **Step 1: Install vue-i18n**

```bash
cd /Users/anna/Documents/aistarlight/frontend && npm install vue-i18n
```

- [ ] **Step 2: Create English locale file**

Create `/Users/anna/Documents/aistarlight/frontend/src/locales/en.ts`:

```ts
export default {
  auth: {
    loginTitle: 'Sign In',
    registerTitle: 'Get Started',
    email: 'Email',
    password: 'Password',
    fullName: 'Full Name',
    companyName: 'Company Name',
    login: 'Sign In',
    register: 'Register',
    createAccount: 'Create Account',
    noAccount: "Need an account?",
    hasAccount: 'Already have an account?',
    loginFailed: 'Something went wrong',
    fieldRequired: 'This field is required',
    emailPlaceholder: 'you@company.com',
    passwordPlaceholder: 'Enter password',
    selectCountry: 'Select Your Country',
    getStarted: 'Get Started — It\'s Free',
    checkEmail: 'Check your email',
    magicLinkSent: 'We sent you a magic link to {email}. Click the link to complete your registration.',
    magicLinkExpiry: 'Link expires in 24 hours',
    didntReceive: 'Didn\'t receive it?',
    resend: 'Resend',
    emailSent: 'Email sent!',
  },
  nav: {
    dashboard: 'Dashboard',
    settings: 'Settings',
    notifications: 'Notifications',
    markAllRead: 'Mark all read',
    noNotifications: 'No notifications',
    logout: 'Logout',
    profile: 'Profile',
    locale: 'Language',
  },
  product: {
    hr: 'HR',
    finance: 'Finance',
    subtitle: 'Smart Tax Filing System',
  },
  common: {
    save: 'Save',
    cancel: 'Cancel',
    delete: 'Delete',
    loading: 'Loading...',
    error: 'Error',
    success: 'Success',
  },
}
```

- [ ] **Step 3: Create Chinese locale file**

Create `/Users/anna/Documents/aistarlight/frontend/src/locales/zh.ts`:

```ts
export default {
  auth: {
    loginTitle: '登录',
    registerTitle: '开始使用',
    email: '邮箱',
    password: '密码',
    fullName: '姓名',
    companyName: '公司名称',
    login: '登录',
    register: '注册',
    createAccount: '创建账户',
    noAccount: '还没有账户？',
    hasAccount: '已有账户？',
    loginFailed: '出错了',
    fieldRequired: '此字段为必填',
    emailPlaceholder: 'you@company.com',
    passwordPlaceholder: '输入密码',
    selectCountry: '选择国家',
    getStarted: '免费开始',
    checkEmail: '请查看邮箱',
    magicLinkSent: '我们已向 {email} 发送了验证链接。点击链接完成注册。',
    magicLinkExpiry: '链接24小时内有效',
    didntReceive: '没有收到？',
    resend: '重新发送',
    emailSent: '邮件已发送！',
  },
  nav: {
    dashboard: '仪表盘',
    settings: '设置',
    notifications: '通知',
    markAllRead: '全部已读',
    noNotifications: '暂无通知',
    logout: '退出登录',
    profile: '个人资料',
    locale: '语言',
  },
  product: {
    hr: '人事',
    finance: '财务',
    subtitle: '智能税务申报系统',
  },
  common: {
    save: '保存',
    cancel: '取消',
    delete: '删除',
    loading: '加载中...',
    error: '错误',
    success: '成功',
  },
}
```

- [ ] **Step 4: Update main.ts to add i18n**

Replace `/Users/anna/Documents/aistarlight/frontend/src/main.ts` with:

```ts
import { createPinia } from "pinia";
import { createApp } from "vue";
import { createI18n } from "vue-i18n";
import App from "./App.vue";
import { router } from "./router";
import "./assets/responsive.css";
import en from "./locales/en";
import zh from "./locales/zh";

const meta = document.createElement("meta");
meta.name = "naive-ui-style";
document.head.appendChild(meta);

const i18n = createI18n({
  legacy: false,
  locale: localStorage.getItem("locale") || "en",
  fallbackLocale: "en",
  messages: { en, zh },
});

const app = createApp(App);
app.use(createPinia());
app.use(router);
app.use(i18n);
app.mount("#app");
```

- [ ] **Step 5: Verify build**

```bash
cd /Users/anna/Documents/aistarlight/frontend && npx vite build 2>&1 | tail -5
```

- [ ] **Step 6: Commit**

```bash
cd /Users/anna/Documents/aistarlight && git add frontend/src/locales/ frontend/src/main.ts frontend/package.json frontend/package-lock.json
git commit -m "feat: add vue-i18n with English and Chinese locale files"
```

---

### Task 4: HR — Unify Login Page

**Files:**
- Modify: `/Users/anna/Documents/aigonhr/frontend/src/views/LoginView.vue`

- [ ] **Step 1: Read current LoginView**

Read the full file at `/Users/anna/Documents/aigonhr/frontend/src/views/LoginView.vue` to understand current structure.

- [ ] **Step 2: Rewrite LoginView with jurisdiction selector and blue colors**

Key changes to make:
- Add `selectedJurisdiction` ref defaulting to `'PH'`
- Add jurisdiction selector UI (PH/SG/LK buttons) between product switcher and form
- Change logo icon gradient from `#4f46e5, #7c3aed` to `#2563eb, #1d4ed8`
- Change product switcher active gradient from `#4f46e5, #7c3aed` to `#2563eb, #1d4ed8`
- Change all indigo/purple color references to blue (`#2563eb`)
- Add `data-testid` attributes to email input, password input, submit button, jurisdiction buttons
- Keep i18n `t()` calls (already present)
- Keep form validation (already present)
- Keep product switcher (already present, just recolor)
- Product switcher Finance link should stay as `https://finance.halaos.com/login` for now (SSO updated in Task 11)

Template for jurisdiction selector (add between product switcher and NForm):
```html
<div class="jurisdiction-selector">
  <p class="jurisdiction-label">{{ t('auth.selectCountry') }}</p>
  <div class="jurisdiction-options">
    <button
      v-for="j in jurisdictions"
      :key="j.code"
      type="button"
      class="jurisdiction-btn"
      :class="{ active: selectedJurisdiction === j.code }"
      @click="selectedJurisdiction = j.code"
      :data-testid="'jurisdiction-' + j.code.toLowerCase()"
    >
      <span class="flag">{{ j.code }}</span>
      <span class="country-name">{{ j.name }}</span>
    </button>
  </div>
</div>
```

Script addition:
```ts
const selectedJurisdiction = ref('PH')
const jurisdictions = [
  { code: 'PH', name: 'Philippines' },
  { code: 'SG', name: 'Singapore' },
  { code: 'LK', name: 'Sri Lanka' },
]
```

Add jurisdiction CSS (matching Finance's existing styling):
```css
.jurisdiction-selector { margin-bottom: 24px; }
.jurisdiction-label { text-align: center; font-size: 14px; color: var(--text-secondary); margin-bottom: 10px; }
.jurisdiction-options { display: flex; gap: 12px; justify-content: center; }
.jurisdiction-btn {
  display: flex; flex-direction: column; align-items: center;
  padding: 12px 24px; border: 2px solid var(--border-default); border-radius: 12px;
  background: var(--bg-surface); cursor: pointer; transition: all 0.2s; min-width: 100px;
}
.jurisdiction-btn:hover { border-color: #93c5fd; background: #eff6ff; }
.jurisdiction-btn.active { border-color: #2563eb; background: #eff6ff; box-shadow: 0 0 0 1px #2563eb; }
.jurisdiction-btn .flag { font-size: 24px; font-weight: 700; color: #1e3a5f; margin-bottom: 4px; }
.jurisdiction-btn .country-name { font-size: 12px; color: var(--text-secondary); }
```

Add i18n key `auth.selectCountry` to HR's `i18n/en.ts` (value: `"Select Your Country"`) and `i18n/zh.ts` (value: `"选择国家"`).

- [ ] **Step 3: Verify build**

```bash
cd /Users/anna/Documents/aigonhr/frontend && npx vite build 2>&1 | tail -5
```

- [ ] **Step 4: Commit**

```bash
cd /Users/anna/Documents/aigonhr && git add frontend/src/views/LoginView.vue frontend/src/i18n/en.ts frontend/src/i18n/zh.ts
git commit -m "feat: add jurisdiction selector and unify login colors to blue"
```

---

### Task 5: Finance — Unify Login Page

**Files:**
- Modify: `/Users/anna/Documents/aistarlight/frontend/src/views/LoginView.vue`

- [ ] **Step 1: Read current LoginView**

Read `/Users/anna/Documents/aistarlight/frontend/src/views/LoginView.vue` fully.

- [ ] **Step 2: Rewrite LoginView to match HR pattern**

Key changes:
- Replace dark gradient background with light: `linear-gradient(135deg, #f8fafc 0%, #eef2ff 50%, #f8fafc 100%)`
- Replace `<h1>HalaOS</h1>` with HR's logo component:
  ```html
  <router-link to="/" class="brand-logo">
    <span class="logo-icon">H</span>
    <span class="logo-text">HalaOS</span>
  </router-link>
  ```
- Remove `<p class="subtitle">` (HR doesn't have it)
- Remove inline register toggle (the `isRegister` ref and all v-if="isRegister" fields)
- Remove fullName, companyName, selectedJurisdiction refs (registration moves to RegisterView)
- Add `useI18n` import and `const { t } = useI18n()`
- Replace all hardcoded strings with `t()` calls
- Add NaiveUI form validation rules:
  ```ts
  const formRef = ref<FormInst | null>(null)
  const rules: FormRules = {
    email: [{ required: true, message: t('auth.fieldRequired'), trigger: ['blur', 'input'] }],
    password: [{ required: true, message: t('auth.fieldRequired'), trigger: ['blur', 'input'] }],
  }
  ```
- Change email input `type="text"` to `type="email"`
- Keep jurisdiction selector (already present, but now only shown on login page for display/preference)
- Replace footer toggle with: `<router-link to="/register">{{ t('auth.register') }}</router-link>`
- Keep `data-testid` attributes
- Update box-shadow from heavy (0.3 opacity) to subtle (0.06 opacity) matching HR
- Keep product switcher, but update Finance link target (stays as-is for now, SSO in Task 12)

Logo CSS:
```css
.brand-logo {
  display: flex; align-items: center; justify-content: center; gap: 8px;
  text-decoration: none; margin-bottom: 8px;
}
.logo-icon {
  display: flex; align-items: center; justify-content: center;
  width: 36px; height: 36px; border-radius: 8px;
  background: linear-gradient(135deg, #2563eb, #1d4ed8);
  color: #fff; font-size: 20px; font-weight: 800;
}
.logo-text { font-size: 22px; font-weight: 700; color: var(--text-primary); }
```

- [ ] **Step 3: Verify build**

```bash
cd /Users/anna/Documents/aistarlight/frontend && npx vite build 2>&1 | tail -5
```

- [ ] **Step 4: Commit**

```bash
cd /Users/anna/Documents/aistarlight && git add frontend/src/views/LoginView.vue
git commit -m "feat: unify login page to match HR pattern with i18n and validation"
```

---

### Task 6: HR — Update Register Page

**Files:**
- Modify: `/Users/anna/Documents/aigonhr/frontend/src/views/RegisterView.vue`
- Modify: `/Users/anna/Documents/aigonhr/frontend/src/i18n/en.ts`
- Modify: `/Users/anna/Documents/aigonhr/frontend/src/i18n/zh.ts`

- [ ] **Step 1: Read current RegisterView**

Read `/Users/anna/Documents/aigonhr/frontend/src/views/RegisterView.vue` fully.

- [ ] **Step 2: Add jurisdiction selector and convert hardcoded strings to i18n**

Key changes:
- Add `selectedJurisdiction` ref defaulting to `'PH'`
- Add jurisdiction selector UI (same component as LoginView Task 4)
- Pass `jurisdiction: selectedJurisdiction.value` in the register call payload
- Convert all hardcoded English strings to `t()` calls:
  - "Check your email" → `t('auth.checkEmail')`
  - "We sent you a magic link" → `t('auth.magicLinkSent', { email: emailInput.value })`
  - "Get Started — It's Free" → `t('auth.getStarted')`
  - "Didn't receive it?" → `t('auth.didntReceive')`
  - "Resend" → `t('auth.resend')`
  - "Join 100+ companies" → `t('auth.joinCompanies')`
  - "Already have an account?" → `t('auth.hasAccount')`
- Change logo gradient colors from indigo to blue (same as Task 4)

Add new i18n keys to `en.ts`:
```ts
auth: {
  // ... existing keys ...
  selectCountry: 'Select Your Country',
  joinCompanies: 'Join 100+ companies using HalaOS',
}
```

Add corresponding keys to `zh.ts`:
```ts
auth: {
  // ... existing keys ...
  selectCountry: '选择国家',
  joinCompanies: '加入100+使用HalaOS的公司',
}
```

- [ ] **Step 3: Verify build**

```bash
cd /Users/anna/Documents/aigonhr/frontend && npx vite build 2>&1 | tail -5
```

- [ ] **Step 4: Commit**

```bash
cd /Users/anna/Documents/aigonhr && git add frontend/src/views/RegisterView.vue frontend/src/i18n/en.ts frontend/src/i18n/zh.ts
git commit -m "feat: add jurisdiction to register page and convert strings to i18n"
```

---

### Task 7: Finance — Create Register & Verify Email Views

**Files:**
- Create: `/Users/anna/Documents/aistarlight/frontend/src/views/RegisterView.vue`
- Create: `/Users/anna/Documents/aistarlight/frontend/src/views/VerifyEmailView.vue`
- Modify: `/Users/anna/Documents/aistarlight/frontend/src/router/index.ts`
- Modify: `/Users/anna/Documents/aistarlight/frontend/src/api/auth.ts`

- [ ] **Step 1: Create RegisterView.vue**

Model after HR's RegisterView pattern. Key elements:
- Same light gradient background as unified LoginView (Task 5)
- Same logo component
- Email-only input field with validation
- Jurisdiction selector (PH/SG/LK)
- Optional referral code from `route.query.ref`
- "Get Started" button → calls `authApi.register({ email, jurisdiction, referral_code })`
- On success: show "Check your email" confirmation with resend button
- All strings via `t()` i18n calls
- Product switcher (same as login)

```vue
<script setup lang="ts">
import { ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { NForm, NFormItem, NInput, NButton, NResult, useMessage, type FormInst, type FormRules } from 'naive-ui'
import { authApi } from '../api/auth'

const { t } = useI18n()
const route = useRoute()
const message = useMessage()

const formRef = ref<FormInst | null>(null)
const emailInput = ref('')
const selectedJurisdiction = ref('PH')
const loading = ref(false)
const emailSent = ref(false)
const referralCode = ref((route.query.ref as string) || '')

const jurisdictions = [
  { code: 'PH', name: 'Philippines' },
  { code: 'SG', name: 'Singapore' },
  { code: 'LK', name: 'Sri Lanka' },
]

const rules: FormRules = {
  email: [{ required: true, type: 'email', message: t('auth.fieldRequired'), trigger: ['blur'] }],
}

async function handleRegister() {
  try {
    await formRef.value?.validate()
  } catch { return }
  loading.value = true
  try {
    await authApi.register({
      email: emailInput.value,
      password: '',
      jurisdiction: selectedJurisdiction.value,
      referral_code: referralCode.value || undefined,
    })
    emailSent.value = true
  } catch (e: unknown) {
    const err = e as { response?: { data?: { error?: string } } }
    message.error(err.response?.data?.error || t('auth.loginFailed'))
  } finally {
    loading.value = false
  }
}

async function handleResend() {
  try {
    await authApi.resendVerification(emailInput.value)
    message.success(t('auth.emailSent'))
  } catch {
    message.error(t('auth.loginFailed'))
  }
}
</script>
```

Template: match HR's two-step flow (email form → success confirmation).

- [ ] **Step 2: Create VerifyEmailView.vue**

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { NResult, NSpin } from 'naive-ui'
import { authApi } from '../api/auth'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()
const loading = ref(true)
const error = ref('')

onMounted(async () => {
  const token = route.query.token as string
  if (!token) {
    error.value = 'No verification token'
    loading.value = false
    return
  }
  try {
    await authApi.verifyEmail(token)
    setTimeout(() => router.push('/login'), 2000)
  } catch (e: unknown) {
    const err = e as { response?: { data?: { error?: string } } }
    error.value = err.response?.data?.error || 'Verification failed'
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="verify-page">
    <div class="verify-card">
      <NSpin v-if="loading" size="large" />
      <NResult v-else-if="!error" status="success" :title="t('common.success')" description="Redirecting to login..." />
      <NResult v-else status="error" title="Verification Failed" :description="error" />
    </div>
  </div>
</template>
```

- [ ] **Step 3: Add auth API endpoints**

In `/Users/anna/Documents/aistarlight/frontend/src/api/auth.ts`, add:

```ts
resendVerification: (email: string) => client.post('/auth/resend-verification', { email }),
verifyEmail: (token: string) => client.post('/auth/verify-email', { token }),
```

Update the `RegisterData` interface to include optional `referral_code` and `jurisdiction`:
```ts
export interface RegisterData {
  email: string
  password: string
  full_name?: string
  company_name?: string
  jurisdiction?: string
  referral_code?: string
}
```

- [ ] **Step 4: Add routes**

In `/Users/anna/Documents/aistarlight/frontend/src/router/index.ts`, add after the `/sso` route:

```ts
{
  path: "/register",
  name: "register",
  component: () => import("../views/RegisterView.vue"),
  meta: { title: "Register" },
},
{
  path: "/verify-email",
  name: "verify-email",
  component: () => import("../views/VerifyEmailView.vue"),
  meta: { title: "Verify Email" },
},
```

- [ ] **Step 5: Verify build**

```bash
cd /Users/anna/Documents/aistarlight/frontend && npx vite build 2>&1 | tail -5
```

- [ ] **Step 6: Commit**

```bash
cd /Users/anna/Documents/aistarlight && git add frontend/src/views/RegisterView.vue frontend/src/views/VerifyEmailView.vue frontend/src/api/auth.ts frontend/src/router/index.ts
git commit -m "feat: add magic link registration and email verification views"
```

---

### Task 8: HR — Unify Dashboard Shell

**Files:**
- Modify: `/Users/anna/Documents/aigonhr/frontend/src/components/DashboardLayout.vue`

- [ ] **Step 1: Read current DashboardLayout**

Read `/Users/anna/Documents/aigonhr/frontend/src/components/DashboardLayout.vue` fully.

- [ ] **Step 2: Apply shell unification changes**

Key changes:

1. **Logo area**: Add jurisdiction badge after "HalaOS" text:
```html
<div style="padding: 16px 20px; font-size: 18px; font-weight: 700; display: flex; align-items: center; gap: 8px;">
  HalaOS
  <span style="font-size: 11px; padding: 2px 8px; border-radius: 4px; background: rgba(37, 99, 235, 0.15); color: #3b82f6;">
    {{ auth.user?.company_country || 'PH' }}
  </span>
</div>
```

2. **Company switcher**: Add below logo (before NMenu):
```html
<div v-if="companyOptions.length > 1 && !themeStore.sidebarCollapsed" style="padding: 0 12px 8px;">
  <NSelect :value="currentCompanyId" :options="companyOptions" size="small" @update:value="handleSwitchCompany" />
</div>
<div v-else-if="auth.user && !themeStore.sidebarCollapsed" style="padding: 0 20px 8px; font-size: 13px; color: var(--text-muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis;">
  {{ auth.companyName }}
</div>
```

Add computed properties:
```ts
const companyOptions = computed(() =>
  (auth.companies || []).map((c: { company_name: string; id: number }) => ({ label: c.company_name, value: c.id }))
)
const currentCompanyId = computed(() => auth.user?.company_id)
```

3. **Menu style**: Change from two-line `renderMenuLabel(titleKey, descKey)` to single-line:
```ts
function mi(titleKey: string, key: string, icon: Component, minRole?: string): MenuOption | null {
  if (minRole && !hasAccess(minRole)) return null
  return { label: t(titleKey), key, icon: renderIcon(icon) }
}
```

Update all `mi()` calls to remove the description key parameter. For example:
```ts
// Before: mi('nav.dashboard', 'nav.dashboardDesc', 'dashboard', HomeOutline)
// After:  mi('nav.dashboard', 'dashboard', HomeOutline)
```

4. **Sidebar collapse**: Use `themeStore.sidebarCollapsed` instead of local state:
```html
<NLayoutSider
  :collapsed="themeStore.sidebarCollapsed"
  @update:collapsed="(val: boolean) => themeStore.sidebarCollapsed = val"
  ...
>
```

5. **Header locale toggle**: Already present, keep it.

6. **Cross-app accounting link**: Keep existing SSO flow (HR→Finance already works).

- [ ] **Step 3: Verify build**

```bash
cd /Users/anna/Documents/aigonhr/frontend && npx vite build 2>&1 | tail -5
```

- [ ] **Step 4: Commit**

```bash
cd /Users/anna/Documents/aigonhr && git add frontend/src/components/DashboardLayout.vue
git commit -m "feat: unify dashboard shell with jurisdiction badge, company switcher, single-line menu"
```

---

### Task 9: Finance — Unify Dashboard Shell

**Files:**
- Modify: `/Users/anna/Documents/aistarlight/frontend/src/components/layout/DashboardLayout.vue`

- [ ] **Step 1: Read current DashboardLayout**

Read `/Users/anna/Documents/aistarlight/frontend/src/components/layout/DashboardLayout.vue` fully.

- [ ] **Step 2: Apply shell unification changes**

Key changes:

1. **Add locale toggle** to header (between notifications and theme toggle):
```ts
import { useI18n } from 'vue-i18n'
const { locale } = useI18n()

function toggleLocale() {
  locale.value = locale.value === 'en' ? 'zh' : 'en'
  localStorage.setItem('locale', locale.value)
}
```

Template (add before theme toggle NSwitch):
```html
<NButton quaternary size="small" @click="toggleLocale" style="font-weight: 600; min-width: 36px;">
  {{ locale === 'en' ? '中' : 'EN' }}
</NButton>
```

2. **i18n for shell strings**: Replace hardcoded strings:
- "Notifications" → `{{ t('nav.notifications') }}`
- "Mark all read" → `{{ t('nav.markAllRead') }}`
- "No notifications" → `{{ t('nav.noNotifications') }}`
- "Profile" → label uses `t('nav.profile')`
- "Logout" → label uses `t('nav.logout')`

Update userMenuOptions to use i18n:
```ts
const { t, locale } = useI18n()
const userMenuOptions = computed(() => [
  { label: t('nav.profile'), key: 'profile', icon: renderIcon(PersonOutline) },
  { type: 'divider', key: 'd' },
  { label: t('nav.logout'), key: 'logout', icon: renderIcon(LogOutOutline) },
])
```

3. **Cross-app HR link**: Change from `window.open` to SSO token flow:
```ts
async function openHR() {
  try {
    const res = await integrationApi.getHRSSOToken()
    const token = res.data.data.token
    window.open(`https://hr.halaos.com/sso?token=${token}`, '_blank')
  } catch {
    window.open('https://hr.halaos.com', '_blank')
  }
}
```

Update menu click handler:
```ts
if (key === 'cross-hr') { openHR(); return }
```

Add import for integration API (create if needed):
```ts
// In api/integration.ts, add:
getHRSSOToken: () => client.get('/integrations/hr/sso-token'),
```

- [ ] **Step 3: Verify build**

```bash
cd /Users/anna/Documents/aistarlight/frontend && npx vite build 2>&1 | tail -5
```

- [ ] **Step 4: Commit**

```bash
cd /Users/anna/Documents/aistarlight && git add frontend/src/components/layout/DashboardLayout.vue frontend/src/api/integration.ts
git commit -m "feat: add locale toggle, i18n shell strings, and SSO cross-app navigation"
```

---

### Task 10: HR — Auth Store Alignment

**Files:**
- Modify: `/Users/anna/Documents/aigonhr/frontend/src/stores/auth.ts`
- Modify: `/Users/anna/Documents/aigonhr/frontend/src/api/client.ts`

- [ ] **Step 1: Read current auth store and client**

Read both files fully.

- [ ] **Step 2: Update auth store**

Key changes:

1. **Rename token**: `token` → `accessToken`, localStorage key `token` → `access_token`
2. **Add new state**: `companies`, `userLoading`, `jurisdiction` computed
3. **Add new methods**: `loginWithSSO()`, `fetchCompanies()`, `switchCompany()`
4. **Update logout**: Add server-side revocation call
5. **Change login signature**: From `login(email, password)` to `login(data: { email: string; password: string })`

```ts
// New state additions
const companies = ref<Array<{ id: number; company_name: string; role: string }>>([])
const userLoading = ref(false)

// New computed
const jurisdiction = computed(() => user.value?.company_country || localStorage.getItem('jurisdiction') || 'PH')

// New methods
async function loginWithSSO(ssoToken: string) {
  const raw = await authAPI.ssoLogin(ssoToken)
  const res = extractAuthData(raw)
  setTokens(res.token, res.refresh_token)
  setUser(res.user)
}

async function fetchCompanies() {
  try {
    const raw = await companyAPI.list()
    companies.value = (raw.data ?? raw) as Array<{ id: number; company_name: string; role: string }>
  } catch {
    companies.value = []
  }
}

async function switchCompany(companyId: number) {
  const raw = await authAPI.switchCompany(companyId)
  const res = extractAuthData(raw)
  setTokens(res.token, res.refresh_token)
  setUser(res.user)
  await fetchCompanies()
}
```

Updated `setTokens`:
```ts
function setTokens(access: string, refresh: string) {
  accessToken.value = access
  refreshToken.value = refresh
  localStorage.setItem('access_token', access)
  localStorage.setItem('refresh_token', refresh)
}
```

Updated `logout`:
```ts
async function logout() {
  const refresh = localStorage.getItem('refresh_token')
  if (refresh) {
    try { await authAPI.logout(refresh) } catch { /* best effort */ }
  }
  accessToken.value = ''
  refreshToken.value = ''
  user.value = null
  companies.value = []
  localStorage.removeItem('access_token')
  localStorage.removeItem('refresh_token')
  localStorage.removeItem('jurisdiction')
}
```

- [ ] **Step 3: Update API client token key**

In `client.ts`, change all references from `localStorage.getItem('token')` to `localStorage.getItem('access_token')` and `localStorage.setItem('token', ...)` to `localStorage.setItem('access_token', ...)`.

Add new auth API methods:
```ts
ssoLogin: (token: string) => apiFetch('/v1/auth/sso', { method: 'POST', body: { token } }),
switchCompany: (companyId: number) => apiFetch('/v1/auth/switch-company', { method: 'POST', body: { company_id: companyId } }),
logout: (refreshToken: string) => apiFetch('/v1/auth/logout', { method: 'POST', body: { refresh_token: refreshToken } }),
```

- [ ] **Step 4: Verify build**

```bash
cd /Users/anna/Documents/aigonhr/frontend && npx vite build 2>&1 | tail -5
```

- [ ] **Step 5: Commit**

```bash
cd /Users/anna/Documents/aigonhr && git add frontend/src/stores/auth.ts frontend/src/api/client.ts
git commit -m "feat: align auth store with unified interface (access_token, SSO, companies, logout)"
```

---

### Task 11: HR — Route Migration & SSO Callback

**Files:**
- Modify: `/Users/anna/Documents/aigonhr/frontend/src/router/index.ts`
- Create: `/Users/anna/Documents/aigonhr/frontend/src/views/SSOCallbackView.vue`

- [ ] **Step 1: Read current router**

Read `/Users/anna/Documents/aigonhr/frontend/src/router/index.ts` fully.

- [ ] **Step 2: Create SSOCallbackView**

Create `/Users/anna/Documents/aigonhr/frontend/src/views/SSOCallbackView.vue`:

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { NSpin } from 'naive-ui'
import { useAuthStore } from '../stores/auth'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()
const auth = useAuthStore()
const error = ref('')
const loading = ref(true)

onMounted(async () => {
  const token = route.query.token as string
  if (!token) {
    error.value = 'No SSO token provided'
    loading.value = false
    return
  }
  try {
    await auth.loginWithSSO(token)
    router.replace('/')
  } catch {
    error.value = 'SSO login failed'
    loading.value = false
  }
})
</script>

<template>
  <div class="sso-page">
    <div class="sso-card">
      <template v-if="loading">
        <NSpin size="large" />
        <p style="margin-top: 16px; color: var(--text-muted);">{{ t('common.loading') }}</p>
      </template>
      <template v-else>
        <p style="color: var(--text-primary); font-size: 16px;">{{ error }}</p>
        <router-link to="/login" style="color: var(--brand-primary); margin-top: 12px; display: inline-block;">
          {{ t('auth.login') }}
        </router-link>
      </template>
    </div>
  </div>
</template>

<style scoped>
.sso-page {
  min-height: 100vh; display: flex; align-items: center; justify-content: center;
  background: var(--bg-app);
}
.sso-card {
  text-align: center; padding: 48px;
  background: var(--bg-surface); border-radius: 16px;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.06);
}
</style>
```

- [ ] **Step 3: Update router — move /dashboard to / and add /sso**

Key changes:
1. Move all routes from `path: '/dashboard'` children to `path: '/'` children under DashboardLayout
2. Add `/sso` route at top level
3. Add redirect catch-all for old `/dashboard/*` paths
4. Update `router.beforeEach` to redirect authenticated users from `/` (public) to dashboard

Add `/sso` route:
```ts
{
  path: "/sso",
  name: "sso",
  component: () => import("../views/SSOCallbackView.vue"),
  meta: { title: "SSO Login" },
},
```

Add redirect for old paths:
```ts
{
  path: "/dashboard/:pathMatch(.*)*",
  redirect: (to) => to.path.replace('/dashboard', '') || '/',
},
```

Move DashboardLayout from `path: '/dashboard'` to `path: '/'` with `meta: { requiresAuth: true }`. All child paths stay the same (they were already relative).

Update auth guard: when authenticated user visits a public marketing route (home, features, pricing), allow it. Only redirect login/register to dashboard.

- [ ] **Step 4: Verify build**

```bash
cd /Users/anna/Documents/aigonhr/frontend && npx vite build 2>&1 | tail -5
```

- [ ] **Step 5: Commit**

```bash
cd /Users/anna/Documents/aigonhr && git add frontend/src/views/SSOCallbackView.vue frontend/src/router/index.ts
git commit -m "feat: migrate routes from /dashboard to root, add SSO callback"
```

---

### Task 12: Finance — Auth Store & SSO Token Generation

**Files:**
- Modify: `/Users/anna/Documents/aistarlight/frontend/src/stores/auth.ts`
- Modify: `/Users/anna/Documents/aistarlight/frontend/src/views/SSOCallbackView.vue`
- Modify: `/Users/anna/Documents/aistarlight/frontend/src/api/integration.ts`

- [ ] **Step 1: Read current files**

Read auth store, SSOCallbackView, and check if integration.ts exists.

- [ ] **Step 2: Update SSOCallbackView to use CSS variables**

Replace hardcoded colors in SSOCallbackView:
- Background `#0f0f23` → `var(--bg-app)`
- Spinner color `#6366f1` → `var(--brand-primary)`
- Text colors → `var(--text-primary)`, `var(--text-muted)`
- Link color `#6366f1` → `var(--brand-primary)`

- [ ] **Step 3: Add SSO token generation endpoint**

In `/Users/anna/Documents/aistarlight/frontend/src/api/integration.ts`, ensure this endpoint exists:
```ts
getHRSSOToken: () => client.get('/integrations/hr/sso-token'),
```

- [ ] **Step 4: Verify build**

```bash
cd /Users/anna/Documents/aistarlight/frontend && npx vite build 2>&1 | tail -5
```

- [ ] **Step 5: Commit**

```bash
cd /Users/anna/Documents/aistarlight && git add frontend/src/views/SSOCallbackView.vue frontend/src/api/integration.ts frontend/src/stores/auth.ts
git commit -m "feat: update SSO callback styling and add HR SSO token endpoint"
```

---

### Task 13: Build Verification & Deploy

**Files:** None (verification only)

- [ ] **Step 1: Build both projects**

```bash
cd /Users/anna/Documents/aigonhr/frontend && npx vite build 2>&1 | tail -10
cd /Users/anna/Documents/aistarlight/frontend && npx vite build 2>&1 | tail -10
```

Both should succeed with no errors.

- [ ] **Step 2: Push both repos**

```bash
cd /Users/anna/Documents/aigonhr && git push
cd /Users/anna/Documents/aistarlight && git push
```

- [ ] **Step 3: Deploy HR**

```bash
ssh aigonhr "cd /home/ubuntu/aigonhr/frontend && sudo rm -rf dist"
cd /Users/anna/Documents/aigonhr/frontend && tar czf /tmp/aigonhr-dist.tar.gz dist/
scp /tmp/aigonhr-dist.tar.gz aigonhr:/home/ubuntu/aigonhr/frontend/
ssh aigonhr "cd /home/ubuntu/aigonhr/frontend && tar xzf aigonhr-dist.tar.gz && cd /home/ubuntu/aigonhr && docker compose -f docker-compose.deploy.yml up -d --build frontend"
```

- [ ] **Step 4: Deploy Finance**

```bash
cd /Users/anna/Documents/aistarlight/frontend && tar czf /tmp/aistarlight-dist.tar.gz dist/
scp /tmp/aistarlight-dist.tar.gz aistarlight-gce:/tmp/
ssh aistarlight-gce "sudo -u anna bash -c 'cd /home/anna/aistarlight-go && sudo docker compose -f docker-compose.prod.yml up -d --build frontend'"
```

Note: Finance frontend is built locally (server has insufficient memory for vue-tsc). Upload dist tarball and rebuild the nginx container.

- [ ] **Step 5: Verify both sites**

```bash
curl -s -o /dev/null -w "%{http_code}" https://hr.halaos.com/login
curl -s -o /dev/null -w "%{http_code}" https://tax.clawpapa.win/login
```

Both should return 200.

- [ ] **Step 6: Final commit tag**

```bash
cd /Users/anna/Documents/aigonhr && git tag halaos-ui-unification-v1
cd /Users/anna/Documents/aistarlight && git tag halaos-ui-unification-v1
```

---

## Backend Tasks (Out of Scope for This Plan)

The following backend changes are required to fully support the unified frontend. These should be planned separately:

**HR Backend (aigonhr-go):**
- `POST /v1/auth/sso` — validate SSO token from Finance, create session
- `POST /v1/auth/switch-company` — switch active company for multi-tenant
- `POST /v1/auth/logout` — revoke refresh token server-side
- `GET /v1/companies` — list companies user belongs to
- Add `jurisdiction` field to companies table and API responses

**Finance Backend (aistarlight-go):**
- `GET /api/v1/integrations/hr/sso-token` — generate signed JWT for HR navigation
- `POST /api/v1/auth/register` — support magic link flow (email-only, send verification)
- `POST /api/v1/auth/resend-verification` — resend magic link
- `POST /api/v1/auth/verify-email` — verify email token, create account

Until these backend endpoints exist, the following frontend features will gracefully degrade:
- SSO cross-app navigation: falls back to opening the other product's URL without SSO token
- Magic link registration on Finance: not functional until backend supports it
- Company switching on HR: not functional until backend supports it
- Server-side logout on HR: silently fails (local cleanup still works)
