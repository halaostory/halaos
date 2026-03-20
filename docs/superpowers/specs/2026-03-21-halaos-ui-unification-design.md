# HalaOS UI Unification: HR & Finance Full Alignment

## Goal

Eliminate visual and functional fragmentation between HR (`hr.halaos.com`) and Finance (`finance.halaos.com`) so both products feel like one cohesive platform. Users switching between HR and Finance should experience zero friction — same login flow, same shell, same theme, same language options, seamless SSO navigation.

## Baseline

HR (AIGoNHR) is the reference implementation. Finance (AIStarlight) aligns to HR patterns. Finance-only features that improve both products (jurisdiction selector, multi-tenant switcher, responsive CSS, PWA) are adopted into HR as well.

## Scope

Six workstreams, ordered by user-visible impact:

1. Login page unification
2. Registration flow unification (Magic Link)
3. Dashboard shell unification
4. Theme & CSS variable unification
5. Auth & routing unification (SSO, tokens, i18n)
6. Internationalization (vue-i18n for Finance)

Internal page views are **out of scope** — each product keeps its domain-specific pages. Only the shared shell (login, register, sidebar, header, theme, auth) is unified.

---

## 1. Login Page

### Target State (identical on both products)

- **Background**: Light gradient (`linear-gradient(135deg, #f8fafc 0%, #eef2ff 50%, #f8fafc 100%)`) — HR pattern
- **Logo**: Gradient circle with "H" letter + "HalaOS" text — HR pattern. Logo icon gradient: `linear-gradient(135deg, #2563eb, #1d4ed8)` (blue, matching brand primary — replaces HR's current indigo `#4f46e5`)
- **Product switcher**: Two buttons (HR / Finance), active one has blue gradient (`#2563eb → #1d4ed8`). Links use SSO token for seamless cross-product navigation.
- **Jurisdiction selector**: PH / SG / LK country buttons — adopted from Finance into both products. (ID/Indonesia deferred to future phase when backend tax rules are ready.)
- **Form**: NaiveUI `NForm` + `NFormItem` + `NInput` with validation rules (`required`, `email` type)
- **Labels**: All text via `t()` i18n calls (no hardcoded strings)
- **Submit**: `NButton type="primary" block` with loading state
- **Footer**: "Need an account?" with `router-link` to `/register`
- **Responsive**: Media query at 768px for mobile card/jurisdiction sizing
- **Testing**: `data-testid` attributes on all interactive elements

### Changes Required

**Finance (aistarlight)**:
- Replace dark gradient background with HR's light gradient
- Replace plain `<h1>HalaOS</h1>` with HR's logo component (gradient "H" icon + text)
- Remove inline registration toggle — replace with router-link to `/register`
- Add NaiveUI form validation rules (required email, required password)
- Replace all hardcoded English strings with `t()` i18n calls
- Change product switcher link from `window.open` to SSO token URL
- Add `data-testid` attributes (already present, keep them)
- Change email input `type="text"` to `type="email"`

**HR (aigonhr)**:
- Add jurisdiction selector (PH/SG/LK buttons) below product switcher
- Change product switcher colors from indigo (`#4f46e5`) to blue (`#2563eb`)
- Add `data-testid` attributes to all interactive elements
- Store selected jurisdiction in registration payload

---

## 2. Registration Flow

### Target State (Magic Link — both products)

Step 1 — Register page (`/register`):
- Email-only input field
- Optional referral code (from query param `?ref=`)
- Jurisdiction selector (PH/SG/LK) — user picks country at registration
- "Get Started" button → sends magic link email
- On success: show confirmation ("Check your email")

Step 2 — Email verification:
- User clicks magic link in email
- Redirected to `/verify-email?token=xxx`
- Backend verifies token, creates account
- Frontend shows success, redirects to setup or login

### Changes Required

**Finance (aistarlight)**:
- Create new `RegisterView.vue` matching HR's pattern
- Create `VerifyEmailView.vue` for email verification callback
- Add `/register` and `/verify-email` routes
- Backend: add magic link registration API endpoints (send-verification, verify-email)
- Remove inline registration from `LoginView.vue`

**HR (aigonhr)**:
- Add jurisdiction selector to `RegisterView.vue`
- Pass jurisdiction in registration payload
- Backend: store jurisdiction on company record

---

## 3. Dashboard Shell

### Target State (both products use identical shell structure)

```
NLayout (has-sider, min-height: 100vh)
├── NLayoutSider (width: 260, collapsed-width: 64, show-trigger)
│   ├── Logo: "HalaOS" + jurisdiction badge (e.g., "PH")
│   ├── Company switcher (NSelect, shown when >1 company)
│   ├── Company name (shown when single company)
│   └── NMenu (grouped, single-line labels, role-filtered)
└── NLayout
    ├── NLayoutHeader (56px, bordered)
    │   └── NSpace: [Notifications] [Locale EN/中] [Theme ☀/🌙] [User dropdown]
    └── NLayoutContent (padding: 24px)
        └── router-view
```

### Menu Style

Single-line labels (Finance pattern). HR's two-line labels (title + description) are removed — menu descriptions add visual noise without adding navigation value.

### Header Components (left to right)

1. **Notifications**: `NPopover` + `NBadge` with unread count, mark-read, list with `NThing`
2. **Locale toggle**: `NButton` showing "中" or "EN", toggles between zh/en
3. **Theme toggle**: `NSwitch` with sun/moon icons
4. **User dropdown**: `NDropdown` with `NAvatar` + name, options: Profile, Logout

### Cross-Product Navigation

Both sidebar menus include a "Connected Apps" section:
- HR sidebar: "Accounting & Tax" → SSO jump to Finance
- Finance sidebar: "HR & Payroll" → SSO jump to HR

SSO mechanism: request short-lived token from backend → open `https://{other-product}/sso?token={token}` in new tab (`_blank`). New tab preserves user's current work context while allowing them to use the other product.

Backend endpoints for SSO:
- HR→Finance (already exists): `GET /api/v1/integrations/accounting/sso-token` returns signed JWT with `CrossAppClaims` (user_id, email, company_id, role). Finance's `/sso` route calls `auth.loginWithSSO(token)`.
- Finance→HR (new): `GET /api/v1/integrations/hr/sso-token` returns signed JWT using shared `INTEGRATION_JWT_SECRET`. HR's new `/sso` route validates and logs in.
- Both use the same JWT structure: `{user_id, email, company_id, role, jurisdiction, exp}` signed with `INTEGRATION_JWT_SECRET`.

### Changes Required

**Finance (aistarlight)**:
- Add locale toggle button (EN/中) to header
- Change cross-app link from `window.open` to SSO token flow (request token, then `window.open` with token URL)
- Backend: add `GET /api/v1/integrations/hr/sso-token` endpoint

**HR (aigonhr)**:
- Add jurisdiction badge next to "HalaOS" logo
- Add company switcher (NSelect) below logo
- Change menu items from two-line (title+desc) to single-line labels
- Add sidebar collapse state to theme/UI store (persist across sessions)
- Add `/sso` route and `SSOCallbackView.vue` for incoming SSO from Finance (use CSS variables for styling: `var(--bg-app)`, `var(--brand-primary)`, not hardcoded colors)
- Backend: add SSO token validation endpoint and login-by-SSO-token handler

---

## 4. Theme & CSS Variables

### Target State

Both products use the same 19 CSS variables (Finance's comprehensive set):

```css
:root, [data-theme="light"] {
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
}

html.dark, [data-theme="dark"] {
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
}
```

### Dark Mode Toggle

Both products use dual selectors: `data-theme` attribute + `html.dark` class (backward compat).

### NaiveUI Theme Overrides

Both products use identical `themeOverrides`:
```ts
primaryColor: '#2563eb',
primaryColorHover: '#3b82f6',
primaryColorPressed: '#1d4ed8',
primaryColorSuppl: '#60a5fa',
```

### Responsive CSS

Finance's 179-line `responsive.css` utility file is adopted into HR. Covers:
- Table horizontal scroll on mobile
- Grid column adjustments (1→2→3+ columns)
- Modal/dialog max-width for mobile
- Touch target sizing (44x44px minimum)
- Safe area insets for notched devices
- Input font-size 16px to prevent iOS zoom

### PWA

HR adopts VitePWA plugin matching Finance's configuration (theme color updated to `#2563eb`).

### Changes Required

**HR (aigonhr)**:
- Replace 6 CSS variables in `style.css` with the 19-variable set
- Add `html.dark` class toggle alongside `data-theme` attribute
- Copy/adapt Finance's `responsive.css` into HR
- Add VitePWA plugin to `vite.config.ts`
- Update existing scoped styles that reference old variable names (e.g., `--bg` → `--bg-surface`, `--text` → `--text-primary`, `--border` → `--border-default`)

**Finance (aistarlight)**:
- No changes needed (Finance already has the target state)

---

## 5. Auth & Routing

### Token Naming

Standardize on `access_token` / `refresh_token` (Finance pattern, more explicit):

| Key | Current HR | Current Finance | Target |
|-----|-----------|-----------------|--------|
| Access token | `token` | `access_token` | `access_token` |
| Refresh token | `refresh_token` | `refresh_token` | `refresh_token` (no change) |

### HTTP Client

Keep each product's current client (HR: ofetch, Finance: axios). Unifying HTTP clients is high effort, low user-visible impact. Both implement the same pattern (auth header injection, 401 refresh, retry).

### Auth Store Alignment

Both auth stores expose the same interface:

```ts
interface AuthStore {
  // State
  user: User | null
  accessToken: string
  companies: Company[]
  userLoading: boolean

  // Computed
  isAuthenticated: boolean
  currentRole: string
  jurisdiction: string
  fullName: string
  isAdmin: boolean

  // Actions
  login(data: { email: string; password: string }): Promise<void>
  register(data: RegisterData): Promise<void>
  loginWithSSO(ssoToken: string): Promise<void>
  logout(): Promise<void>
  fetchUser(): Promise<void>
  fetchCompanies(): Promise<void>
  switchCompany(tenantId: string): Promise<void>
}
```

### Route Structure

Standardize on root-level paths (Finance pattern): `/upload` not `/dashboard/upload`.

HR currently uses `/dashboard` prefix with public marketing pages at `/`. Migration strategy:

```
/ (PublicLayout — guest only, redirect to /home for authenticated users)
├── /home → HomePage
├── /features → FeaturesPage
├── /pricing → PricingPage
├── /about, /contact, /blog, /tools, etc.
/login (standalone — redirect to / if authenticated)
/register (standalone)
/verify-email (standalone)
/sso (standalone)
/ (DashboardLayout — authenticated only)
├── / → Dashboard (name: "dashboard")
├── /employees → Employees
├── /attendance → Attendance
├── ...all other app routes
```

Route resolution for `/`:
- `router.beforeEach`: if authenticated → render DashboardLayout children. If guest → render PublicLayout children.
- Implementation: two route entries for `/` with different `meta.requiresAuth` — the auth guard redirects guests to public homepage and authenticated users bypass public routes.
- Add redirects from old `/dashboard/*` paths: `{ path: '/dashboard/:pathMatch(.*)*', redirect: to => to.path.replace('/dashboard', '') }`

### SSO Routes

Both products add `/sso` route with `SSOCallbackView.vue`:
- Accepts `?token=xxx` query param
- Calls `auth.loginWithSSO(token)`
- Redirects to dashboard on success, login on failure

### Logout

Both products call server-side token revocation (Finance pattern) before clearing local state.

### Changes Required

**HR (aigonhr)**:
- Rename `token` → `access_token` in localStorage and auth store
- Add `jurisdiction`, `companies`, `userLoading`, `switchCompany`, `fetchCompanies` to auth store
- Add `loginWithSSO` method to auth store
- Add `/sso` route + `SSOCallbackView.vue`
- Add server-side logout call
- Move routes from `/dashboard/...` to `/...` prefix
- Add redirect routes for backward compat
- Backend: add SSO token endpoint, add jurisdiction to company model

**Finance (aistarlight)**:
- Add SSO token generation endpoint for Finance→HR navigation
- Backend: add magic link registration endpoints

---

## 6. Internationalization

### Target State

Both products use `vue-i18n` with matching translation key structure.

### Shared Key Namespace

Shell-related keys share the same namespace across both products:

```ts
// auth.*
auth.login, auth.register, auth.email, auth.password, auth.loginTitle,
auth.noAccount, auth.loginFailed, auth.fieldRequired

// nav.*
nav.dashboard, nav.settings, nav.notifications, nav.logout, nav.profile

// common.*
common.save, common.cancel, common.delete, common.loading, common.error
```

Product-specific keys use their own namespace:
- HR: `hr.employees`, `hr.payroll`, `hr.attendance`, etc.
- Finance: `fin.upload`, `fin.transactions`, `fin.reports`, etc.

### Changes Required

**Finance (aistarlight)**:
- Install `vue-i18n`
- Create `src/locales/en.ts` and `src/locales/zh.ts`
- Set up i18n plugin in `main.ts`
- Add `useI18n()` to all shell components (LoginView, RegisterView, DashboardLayout)
- Replace all hardcoded strings with `t()` calls (shell components only in Phase 1; internal pages in Phase 2)
- Add locale toggle to header (matching HR)
- Persist locale choice in localStorage

**HR (aigonhr)**:
- Audit and convert remaining hardcoded English strings to `t()` calls in shell components (RegisterView.vue has "Check your email", "Get Started", "Didn't receive it?", "Join 100+ companies" etc. that are not using i18n despite importing `useI18n`)

---

## Out of Scope (Future Phases)

- Internal page view unification (each product keeps domain-specific pages)
- Finance i18n for internal pages (Phase 2)
- Shared npm component library extraction
- Unified backend auth service
- Shared notification service
- Command palette for Finance
- NPS survey for Finance
- HR's setup wizard for Finance
- Indonesia (ID) jurisdiction support (requires backend tax rule implementation)
- `fullName` computed property alignment (HR uses `first_name + last_name`, Finance uses `full_name` string — keep both until user model is unified)
