<template>
  <div class="pricing-page">
    <!-- Hero -->
    <section class="price-hero">
      <div class="container">
        <div class="hero-badge">Simple, transparent pricing</div>
        <h1>Start free, scale as you grow</h1>
        <p>HalaOS is free forever for core HR & payroll. Upgrade only when you need advanced features.</p>
        <div class="toggle-row">
          <span :class="{ active: !annual }" @click="annual = false">Monthly</span>
          <button class="toggle-btn" :class="{ on: annual }" @click="annual = !annual" aria-label="Toggle billing">
            <span class="toggle-thumb" />
          </button>
          <span :class="{ active: annual }" @click="annual = true">Annual <span class="save-tag">Save 20%</span></span>
        </div>
      </div>
    </section>

    <!-- Pricing Cards -->
    <section class="pricing-section">
      <div class="container">
        <div class="pricing-cards">
          <!-- Free Tier -->
          <div class="pricing-card">
            <div class="card-header">
              <h3>Free</h3>
              <p class="card-sub">For small teams getting started</p>
            </div>
            <div class="card-price">
              <span class="price-amount">$0</span>
              <span class="price-period">forever</span>
            </div>
            <router-link to="/register" class="btn-outline btn-block">Get Started Free</router-link>
            <div class="card-features">
              <p class="features-label">Everything to run payroll:</p>
              <ul>
                <li v-for="f in freeTierFeatures" :key="f">{{ f }}</li>
              </ul>
            </div>
          </div>

          <!-- Pro Tier -->
          <div class="pricing-card popular">
            <div class="popular-badge">Most Popular</div>
            <div class="card-header">
              <h3>Pro</h3>
              <p class="card-sub">For growing companies that need more</p>
            </div>
            <div class="card-price">
              <span class="price-amount">${{ annual ? '2.40' : '3' }}</span>
              <span class="price-period">/ employee / month</span>
            </div>
            <div class="local-price">
              {{ annual ? '~P120' : '~P149' }}/emp/mo (PH) &middot; {{ annual ? '~S$5.60' : '~S$7' }}/emp/mo (SG)
            </div>
            <router-link to="/register" class="btn-primary btn-block">Start 14-day Free Trial</router-link>
            <div class="card-features">
              <p class="features-label">Everything in Free, plus:</p>
              <ul>
                <li v-for="f in proTierFeatures" :key="f">{{ f }}</li>
              </ul>
            </div>
          </div>

          <!-- Enterprise Tier -->
          <div class="pricing-card">
            <div class="card-header">
              <h3>Enterprise</h3>
              <p class="card-sub">For large organizations with custom needs</p>
            </div>
            <div class="card-price">
              <span class="price-amount">Custom</span>
              <span class="price-period">tailored pricing</span>
            </div>
            <router-link to="/contact" class="btn-outline btn-block">Contact Sales</router-link>
            <div class="card-features">
              <p class="features-label">Everything in Pro, plus:</p>
              <ul>
                <li v-for="f in enterpriseTierFeatures" :key="f">{{ f }}</li>
              </ul>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- Feature Comparison -->
    <section class="comparison">
      <div class="container">
        <h2 class="section-title">Feature comparison</h2>
        <div class="comparison-table-wrap">
          <table class="comparison-table">
            <thead>
              <tr>
                <th>Feature</th>
                <th>Free</th>
                <th class="highlight-col">Pro</th>
                <th>Enterprise</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in comparisonRows" :key="row.feature">
                <td>{{ row.feature }}</td>
                <td>{{ row.free }}</td>
                <td class="highlight-col">{{ row.pro }}</td>
                <td>{{ row.enterprise }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </section>

    <!-- Competitors -->
    <section class="competitors">
      <div class="container">
        <h2 class="section-title">How HalaOS compares</h2>
        <p class="section-sub">See why businesses choose HalaOS over other HR platforms.</p>
        <div class="competitor-grid">
          <div v-for="c in competitors" :key="c.name" class="competitor-card">
            <h4>HalaOS vs {{ c.name }}</h4>
            <ul>
              <li v-for="point in c.points" :key="point">{{ point }}</li>
            </ul>
          </div>
        </div>
      </div>
    </section>

    <!-- FAQ -->
    <section class="faq">
      <div class="container">
        <h2 class="section-title">Frequently asked questions</h2>
        <div class="faq-list">
          <details v-for="q in faqs" :key="q.q">
            <summary>{{ q.q }}</summary>
            <p>{{ q.a }}</p>
          </details>
        </div>
      </div>
    </section>

    <!-- CTA -->
    <section class="price-cta">
      <div class="container">
        <h2>Start using HalaOS today</h2>
        <p>No credit card required. Set up in 2 minutes.</p>
        <router-link to="/register" class="btn-primary btn-lg">Create Free Account</router-link>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useHead } from '@unhead/vue'

useHead({
  title: 'Pricing - HalaOS | Free HR & Payroll Software',
  meta: [
    { name: 'description', content: 'HalaOS is free for core HR & payroll. Pro tier starts at $3/employee/month. Compare plans for businesses in the Philippines, Singapore, Sri Lanka, Indonesia, or the US.' },
    { property: 'og:title', content: 'Pricing - HalaOS | Free HR & Payroll Software' },
    { property: 'og:description', content: 'Free HR, payroll & tax compliance for Southeast Asian and US businesses. Pro features from $3/employee/month.' },
    { property: 'og:url', content: 'https://halaos.com/pricing' },
  ],
})

const annual = ref(false)

const freeTierFeatures = [
  'Unlimited employees',
  'Full HR management & 201 files',
  'Complete payroll processing',
  'Auto tax calculations (BIR, CPF, EPF, IRS)',
  'Leave & attendance management',
  'Digital payslips',
  'Basic AI assistant',
  'CLI access (read + basic operations)',
  'Lark, Slack & Telegram bots',
  'CSV/Excel data export',
  'Email support',
  '1-year data history',
]

const proTierFeatures = [
  'Auto e-file tax forms (BIR, IRAS)',
  'Full AI agents & analytics',
  'Bank & QuickBooks integrations',
  'Advanced workforce reports',
  'Custom approval workflows',
  'White-label PDF exports',
  'Full API access (CRUD) & CLI',
  'MCP server for Claude Code & AI IDEs',
  'Priority email support',
  '5-year data history',
  'Performance reviews & KPIs',
]

const enterpriseTierFeatures = [
  'Dedicated account manager',
  'Custom integrations & webhooks',
  'Full white-label branding',
  'Unlimited data retention',
  'SSO / SAML authentication',
  'SLA guarantee (99.9% uptime)',
  'Audit support & compliance consulting',
  'Custom AI agent workflows',
  'Custom MCP tools & bot workflows',
  'On-premise deployment option',
]

const comparisonRows = [
  { feature: 'Employees', free: 'Unlimited', pro: 'Unlimited', enterprise: 'Unlimited' },
  { feature: 'Core HR & 201 Files', free: 'Full', pro: 'Full', enterprise: 'Full' },
  { feature: 'Payroll Processing', free: 'Full', pro: 'Full', enterprise: 'Full' },
  { feature: 'Tax Calculation', free: 'Auto', pro: 'Auto', enterprise: 'Auto' },
  { feature: 'Tax Filing', free: 'Generate forms', pro: 'Auto e-file', enterprise: 'Auto + support' },
  { feature: 'Accounting Integration', free: 'Basic GL', pro: 'Full + reports', enterprise: 'Full + audit' },
  { feature: 'AI Features', free: 'Basic chat', pro: 'All AI agents', enterprise: 'Custom agents' },
  { feature: 'Support', free: 'Email', pro: 'Priority email', enterprise: 'Dedicated CSM' },
  { feature: 'PDF Exports', free: 'HalaOS branding', pro: 'White-label', enterprise: 'Full white-label' },
  { feature: 'API Access', free: 'Read-only', pro: 'Full CRUD', enterprise: 'Full + webhooks' },
  { feature: 'Data History', free: '1 year', pro: '5 years', enterprise: 'Unlimited' },
  { feature: 'Integrations', free: 'CSV export', pro: 'Banks, Slack, QB', enterprise: 'Custom' },
  { feature: 'CLI', free: 'Basic', pro: 'Full', enterprise: 'Full + custom' },
  { feature: 'MCP (AI Tools)', free: 'Read-only', pro: 'Full access', enterprise: 'Custom tools' },
  { feature: 'Chat Bots', free: 'Lark, Slack, TG', pro: 'Lark, Slack, TG', enterprise: 'Custom bots' },
]

const competitors = [
  {
    name: 'Sprout Solutions',
    points: [
      'HalaOS Free tier = $0; Sprout starts ~P50-150/emp/mo',
      'Built-in accounting & tax filing; Sprout HR-only',
      'AI-powered insights included free',
      'Multi-country support (PH, SG, LK, ID)',
    ],
  },
  {
    name: 'Swingvy',
    points: [
      'HalaOS Free tier = $0; Swingvy ~S$5/emp/mo',
      'Full payroll + tax; Swingvy limited accounting',
      'BIR & IRAS compliance built-in',
      'No employee minimums or setup fees',
    ],
  },
  {
    name: 'JuanTax',
    points: [
      'HalaOS combines HR + payroll + tax in one platform',
      'JuanTax is tax-filing only, no HR/payroll',
      'Auto-generate all BIR forms from payroll data',
      'No need for separate HR software + tax software',
    ],
  },
]

const faqs = [
  { q: 'Is the Free tier really free forever?', a: 'Yes. The Free tier includes full HR management, payroll processing, and tax calculations at no cost, with no trial period or credit card required. We believe every company deserves modern HR technology.' },
  { q: 'When should I upgrade to Pro?', a: 'The Free tier handles day-to-day HR and payroll. Upgrade to Pro when you need auto e-filing of tax forms, advanced integrations (banks, QuickBooks), full AI analytics, or longer data retention. Most companies upgrade around 20-30 employees.' },
  { q: 'How does the per-employee pricing work?', a: 'You pay per active employee per month. Inactive or terminated employees are not counted. The price adjusts automatically as you add or remove employees.' },
  { q: 'Which countries are supported?', a: 'HalaOS supports Philippines (BIR, SSS, PhilHealth, Pag-IBIG), Singapore (IRAS, CPF, MOM), Sri Lanka (EPF/ETF), and Indonesia payroll & tax regulations.' },
  { q: 'Is my data secure?', a: 'We use AES-256 encryption, TLS 1.3, role-based access control, and complete audit trails. Data is hosted on secure cloud infrastructure with daily backups.' },
  { q: 'Can I export my data anytime?', a: 'Yes. All plans include data export in Excel, CSV, or PDF format. Your data belongs to you, always.' },
  { q: 'Do you offer discounts for annual billing?', a: 'Yes, annual billing saves 20% on Pro and Enterprise plans.' },
  { q: 'Is HalaOS PSG-approved in Singapore?', a: 'We are currently applying for PSG (Productivity Solutions Grant) approval. Once approved, Singapore SMEs can claim up to 50% co-funding on Pro and Enterprise plans.' },
  { q: 'What are CLI and MCP?', a: 'The HalaOS CLI lets you manage employees, run payroll, and generate tax forms from your terminal. MCP (Model Context Protocol) lets AI tools like Claude Code and Cursor connect directly to your HR data, so AI can help you manage HR tasks. Both are included in all plans.' },
  { q: 'Which messaging bots are supported?', a: 'HalaOS integrates with Lark, Slack, and Telegram. Employees can clock in/out, check leave balances, and submit requests directly from chat. Managers can approve requests without leaving their messaging app.' },
]
</script>

<style scoped>
.container {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 24px;
}

/* Hero */
.price-hero {
  padding: 100px 0 40px;
  text-align: center;
  background: linear-gradient(180deg, #f8fafc 0%, #fff 100%);
}
.hero-badge {
  display: inline-block;
  background: #eef2ff;
  color: #4f46e5;
  font-size: 13px;
  font-weight: 600;
  padding: 6px 16px;
  border-radius: 20px;
  margin-bottom: 20px;
}
.price-hero h1 {
  font-size: 48px;
  font-weight: 800;
  color: #0f172a;
  margin: 0 0 16px;
  letter-spacing: -1px;
}
.price-hero > .container > p {
  font-size: 18px;
  color: #475569;
  max-width: 560px;
  margin: 0 auto 32px;
}
.toggle-row {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  font-size: 15px;
  color: #64748b;
}
.toggle-row span.active { color: #0f172a; font-weight: 600; }
.toggle-row span { cursor: pointer; }
.toggle-btn {
  width: 48px;
  height: 26px;
  background: #e2e8f0;
  border: none;
  border-radius: 13px;
  position: relative;
  cursor: pointer;
  transition: background 0.2s;
}
.toggle-btn.on { background: #4f46e5; }
.toggle-thumb {
  position: absolute;
  top: 3px;
  left: 3px;
  width: 20px;
  height: 20px;
  background: #fff;
  border-radius: 50%;
  transition: left 0.2s;
}
.toggle-btn.on .toggle-thumb { left: 25px; }
.save-tag {
  background: #dcfce7;
  color: #16a34a;
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 10px;
  margin-left: 4px;
}

/* Pricing Cards */
.pricing-section {
  padding: 40px 0 80px;
}
.pricing-cards {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 24px;
  align-items: start;
}
.pricing-card {
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 16px;
  padding: 32px;
  position: relative;
}
.pricing-card.popular {
  border: 2px solid #4f46e5;
  box-shadow: 0 4px 24px rgba(79, 70, 229, 0.1);
}
.popular-badge {
  position: absolute;
  top: -12px;
  left: 50%;
  transform: translateX(-50%);
  background: #4f46e5;
  color: #fff;
  font-size: 12px;
  font-weight: 600;
  padding: 4px 16px;
  border-radius: 12px;
}
.card-header h3 {
  font-size: 22px;
  font-weight: 700;
  color: #0f172a;
  margin: 0 0 4px;
}
.card-sub {
  font-size: 14px;
  color: #64748b;
  margin: 0 0 24px;
}
.card-price {
  margin-bottom: 4px;
}
.price-amount {
  font-size: 48px;
  font-weight: 800;
  color: #0f172a;
  letter-spacing: -1px;
}
.price-period {
  font-size: 15px;
  color: #64748b;
  margin-left: 4px;
}
.local-price {
  font-size: 13px;
  color: #94a3b8;
  margin-bottom: 20px;
}
.card-features {
  margin-top: 28px;
}
.features-label {
  font-size: 13px;
  font-weight: 600;
  color: #64748b;
  text-transform: uppercase;
  letter-spacing: 0.03em;
  margin: 0 0 12px;
}
.card-features ul {
  list-style: none;
  padding: 0;
  margin: 0;
}
.card-features li {
  padding: 6px 0;
  font-size: 14px;
  color: #334155;
}
.card-features li::before {
  content: '\2713';
  color: #4f46e5;
  font-weight: 700;
  margin-right: 8px;
}

/* Buttons */
.btn-primary {
  display: inline-block;
  background: #4f46e5;
  color: #fff;
  font-weight: 600;
  font-size: 15px;
  padding: 12px 28px;
  border-radius: 8px;
  text-decoration: none;
  text-align: center;
  transition: background 0.2s;
}
.btn-primary:hover { background: #4338ca; }
.btn-outline {
  display: inline-block;
  border: 1.5px solid #e2e8f0;
  color: #334155;
  font-weight: 600;
  font-size: 15px;
  padding: 11px 28px;
  border-radius: 8px;
  text-decoration: none;
  text-align: center;
  transition: all 0.2s;
}
.btn-outline:hover { border-color: #4f46e5; color: #4f46e5; }
.btn-block { display: block; width: 100%; }
.btn-lg { padding: 14px 36px; font-size: 16px; }

/* Feature Comparison */
.comparison {
  padding: 80px 0;
  background: #f8fafc;
}
.section-title {
  text-align: center;
  font-size: 36px;
  font-weight: 800;
  color: #0f172a;
  margin-bottom: 12px;
}
.section-sub {
  text-align: center;
  color: #64748b;
  font-size: 17px;
  margin-bottom: 48px;
}
.comparison-table-wrap {
  overflow-x: auto;
  margin-top: 32px;
}
.comparison-table {
  width: 100%;
  border-collapse: collapse;
  background: #fff;
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 1px 3px rgba(0,0,0,0.06);
}
.comparison-table th,
.comparison-table td {
  padding: 14px 20px;
  text-align: left;
  font-size: 14px;
  border-bottom: 1px solid #f1f5f9;
}
.comparison-table th {
  background: #f8fafc;
  font-weight: 600;
  color: #0f172a;
  font-size: 15px;
}
.comparison-table td:first-child {
  font-weight: 500;
  color: #334155;
}
.comparison-table td {
  color: #475569;
}
.highlight-col {
  background: #faf5ff;
}

/* Competitors */
.competitors {
  padding: 80px 0;
}
.competitor-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 24px;
}
.competitor-card {
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  padding: 28px;
}
.competitor-card h4 {
  font-size: 16px;
  font-weight: 700;
  color: #0f172a;
  margin: 0 0 16px;
}
.competitor-card ul {
  list-style: none;
  padding: 0;
  margin: 0;
}
.competitor-card li {
  padding: 6px 0;
  font-size: 13px;
  color: #475569;
  line-height: 1.5;
}
.competitor-card li::before {
  content: '\2713';
  color: #16a34a;
  font-weight: 700;
  margin-right: 8px;
}

/* FAQ */
.faq {
  padding: 80px 0;
  background: #f8fafc;
}
.faq-list {
  max-width: 720px;
  margin: 32px auto 0;
}
.faq-list details {
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  margin-bottom: 12px;
  overflow: hidden;
  background: #fff;
}
.faq-list summary {
  padding: 16px 20px;
  font-weight: 600;
  font-size: 15px;
  color: #0f172a;
  cursor: pointer;
  list-style: none;
}
.faq-list summary::-webkit-details-marker { display: none; }
.faq-list details[open] summary {
  border-bottom: 1px solid #f1f5f9;
}
.faq-list details p {
  padding: 16px 20px;
  margin: 0;
  color: #475569;
  font-size: 14px;
  line-height: 1.7;
}

/* CTA */
.price-cta {
  padding: 80px 0;
  text-align: center;
  background: linear-gradient(135deg, #4f46e5, #7c3aed);
  color: #fff;
}
.price-cta h2 {
  font-size: 36px;
  font-weight: 800;
  margin: 0 0 12px;
}
.price-cta p {
  font-size: 17px;
  opacity: 0.9;
  margin-bottom: 32px;
}
.price-cta .btn-primary {
  background: #fff;
  color: #4f46e5;
}
.price-cta .btn-primary:hover { background: #f1f5f9; }

/* Responsive */
@media (max-width: 768px) {
  .price-hero h1 { font-size: 32px; }
  .pricing-cards { grid-template-columns: 1fr; }
  .competitor-grid { grid-template-columns: 1fr; }
  .section-title { font-size: 28px; }
  .price-cta h2 { font-size: 28px; }
  .price-amount { font-size: 36px; }
}
</style>
