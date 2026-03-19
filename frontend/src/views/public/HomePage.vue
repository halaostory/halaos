<template>
  <div class="home">
    <!-- Hero -->
    <section class="hero">
      <div class="container">
        <div class="hero-content">
          <div class="hero-badge">Free HR & Payroll for Southeast Asia</div>
          <h1>Stop paying for <span class="gradient-text">HR software</span></h1>
          <p class="hero-sub">
            HalaOS gives you full HR management, automated payroll, and BIR/IRAS tax compliance
            in one platform — completely free. Built for businesses in the Philippines and Singapore.
          </p>
          <div class="hero-actions">
            <router-link to="/register" class="btn-primary">Get Started Free</router-link>
            <router-link to="/tools" class="btn-outline">Try Free Calculators</router-link>
          </div>
          <p class="hero-note">No credit card required &middot; Set up in 2 minutes &middot; Free forever</p>
        </div>
      </div>
    </section>

    <!-- Social Proof Numbers -->
    <section class="social-proof">
      <div class="container">
        <div class="proof-row">
          <div v-for="stat in stats" :key="stat.label" class="proof-item">
            <span class="proof-num">{{ stat.num }}</span>
            <span class="proof-label">{{ stat.label }}</span>
          </div>
        </div>
      </div>
    </section>

    <!-- Pain Points -->
    <section class="pain-points">
      <div class="container">
        <h2 class="section-title">Sound familiar?</h2>
        <div class="pain-grid">
          <div v-for="pain in pains" :key="pain.title" class="pain-card">
            <div class="pain-icon">{{ pain.icon }}</div>
            <h3>{{ pain.title }}</h3>
            <p>{{ pain.desc }}</p>
            <p class="pain-solution">{{ pain.solution }}</p>
          </div>
        </div>
      </div>
    </section>

    <!-- Countries -->
    <section class="countries-section">
      <div class="container">
        <h2 class="section-title">Built for Southeast Asian compliance</h2>
        <p class="section-sub">Pre-configured for local tax rules, statutory deductions, and government filing requirements.</p>
        <div class="countries-grid">
          <div v-for="c in countryDetails" :key="c.code" class="country-card">
            <div class="country-flag">{{ c.flag }}</div>
            <h3>{{ c.name }}</h3>
            <ul>
              <li v-for="item in c.items" :key="item">{{ item }}</li>
            </ul>
          </div>
        </div>
      </div>
    </section>

    <!-- Features -->
    <section class="features" id="features-preview">
      <div class="container">
        <h2 class="section-title">Everything you need to run HR</h2>
        <p class="section-sub">From hiring to retirement, HalaOS covers the complete employee lifecycle.</p>
        <div class="features-grid">
          <div v-for="f in features" :key="f.title" class="feature-card">
            <div class="feature-icon">{{ f.icon }}</div>
            <h3>{{ f.title }}</h3>
            <p>{{ f.desc }}</p>
          </div>
        </div>
      </div>
    </section>

    <!-- How It Works -->
    <section class="how-it-works">
      <div class="container">
        <h2 class="section-title">Get started in 3 simple steps</h2>
        <div class="steps-row">
          <div v-for="(step, i) in steps" :key="i" class="step-card">
            <div class="step-num">{{ i + 1 }}</div>
            <h3>{{ step.title }}</h3>
            <p>{{ step.desc }}</p>
          </div>
        </div>
      </div>
    </section>

    <!-- AI Section -->
    <section class="ai-section">
      <div class="container">
        <div class="ai-content">
          <div class="ai-text">
            <h2>Powered by <span class="gradient-text">AI Intelligence</span></h2>
            <p>
              HalaOS uses AI to automate repetitive HR tasks,
              provide predictive workforce analytics, and ensure compliance
              with local regulations across multiple jurisdictions.
            </p>
            <ul class="ai-features">
              <li v-for="af in aiFeatures" :key="af">{{ af }}</li>
            </ul>
          </div>
          <div class="ai-visual">
            <div class="ai-card">
              <div class="ai-card-header">AI Assistant</div>
              <div class="ai-card-body">
                <div class="ai-msg ai-msg-user">Compute 13th month pay for all employees</div>
                <div class="ai-msg ai-msg-bot">Computed 13th month pay for 45 employees. Total: P1,234,567. 3 employees have pro-rated amounts due to mid-year hire dates.</div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- Comparison -->
    <section class="comparison-section">
      <div class="container">
        <h2 class="section-title">Why choose HalaOS?</h2>
        <p class="section-sub">The only platform that combines HR, payroll, and tax compliance — for free.</p>
        <div class="compare-table-wrap">
          <table class="compare-table">
            <thead>
              <tr>
                <th></th>
                <th class="highlight-col">HalaOS</th>
                <th>Sprout</th>
                <th>Swingvy</th>
                <th>JuanTax</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in compareRows" :key="row.feature">
                <td class="feature-name">{{ row.feature }}</td>
                <td class="highlight-col">{{ row.halaos }}</td>
                <td>{{ row.sprout }}</td>
                <td>{{ row.swingvy }}</td>
                <td>{{ row.juantax }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </section>

    <!-- Free Tools CTA -->
    <section class="tools-cta">
      <div class="container">
        <div class="tools-cta-content">
          <div class="tools-cta-text">
            <h2>Free HR & Tax Calculators</h2>
            <p>Try our free tools — no signup required. Compute SSS contributions, income tax, 13th month pay, and more.</p>
            <router-link to="/tools" class="btn-primary">Open Free Tools</router-link>
          </div>
          <div class="tools-cta-list">
            <router-link v-for="tool in toolLinks" :key="tool.to" :to="tool.to" class="tool-link">
              <span class="tool-icon">{{ tool.icon }}</span>
              <span>{{ tool.label }}</span>
            </router-link>
          </div>
        </div>
      </div>
    </section>

    <!-- Free Banner -->
    <section class="free-banner">
      <div class="container">
        <div class="free-card">
          <div class="free-price">$0</div>
          <h2>Free forever</h2>
          <p>Full HR, payroll, and tax compliance — no limits, no catch. Upgrade to Pro only when you need advanced features.</p>
          <ul class="free-perks">
            <li>Unlimited employees</li>
            <li>Full payroll with tax calculations</li>
            <li>BIR &amp; IRAS form generation</li>
            <li>AI-powered insights</li>
            <li>Leave &amp; attendance tracking</li>
          </ul>
          <div class="free-actions">
            <router-link to="/register" class="btn-primary btn-lg">Get Started Free</router-link>
            <router-link to="/pricing" class="btn-text">See all plans</router-link>
          </div>
        </div>
      </div>
    </section>

    <!-- CTA -->
    <section class="cta-section">
      <div class="container">
        <h2>Join businesses across Southeast Asia</h2>
        <p>Stop overpaying for HR software. Start with HalaOS for free today.</p>
        <router-link to="/register" class="btn-primary btn-lg">Create Free Account</router-link>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { useHead } from '@unhead/vue'

useHead({
  title: 'HalaOS - Free HR, Payroll & Tax Software for Philippines & Singapore',
  meta: [
    { name: 'description', content: 'Free HR management, payroll processing, and BIR/IRAS tax compliance for businesses in the Philippines and Singapore. No credit card, no limits. Get started in 2 minutes.' },
    { property: 'og:title', content: 'HalaOS - Free HR, Payroll & Tax Software for Philippines & Singapore' },
    { property: 'og:description', content: 'Free HR, payroll & tax compliance for Southeast Asian businesses. BIR forms, CPF, SSS auto-calculations. Start in 2 minutes.' },
    { property: 'og:url', content: 'https://halaos.com/' },
    { name: 'keywords', content: 'free HR software philippines, free payroll software philippines, BIR compliance software, IRAS filing software, CPF calculator, SSS contribution calculator, HR software singapore, payroll philippines free' },
  ],
})

const stats = [
  { num: '4', label: 'Countries Supported' },
  { num: '100%', label: 'Free Core Features' },
  { num: '50+', label: 'BIR/Tax Forms Automated' },
  { num: '2 min', label: 'Setup Time' },
]

const pains = [
  {
    icon: '😩',
    title: 'Spending hours on manual payroll?',
    desc: 'Computing SSS, PhilHealth, Pag-IBIG, and withholding tax for each employee in Excel is error-prone and time-consuming.',
    solution: 'HalaOS auto-calculates everything in one click.',
  },
  {
    icon: '😰',
    title: 'Stressed about BIR deadlines?',
    desc: 'Filing 2550M monthly, 1601C, 2316 year-end — miss a deadline and face penalties.',
    solution: 'HalaOS auto-generates all BIR forms from your payroll data.',
  },
  {
    icon: '💸',
    title: 'Paying too much for HR software?',
    desc: 'Sprout Solutions, GreatDay, Swingvy — they all charge per employee. It adds up fast.',
    solution: 'HalaOS gives you everything for free. Really.',
  },
  {
    icon: '🔀',
    title: 'Juggling multiple tools?',
    desc: 'One system for HR, another for payroll, another for tax filing, another for accounting.',
    solution: 'HalaOS unifies HR + Payroll + Tax + Accounting in one platform.',
  },
]

const countryDetails = [
  {
    code: 'PH', flag: '🇵🇭', name: 'Philippines',
    items: [
      'BIR Forms: 2550M, 2550Q, 1701, 1702, 2316, 1601C, 0619E, SAWT',
      'SSS, PhilHealth, Pag-IBIG auto-calculation',
      '13th Month Pay & separation pay computation',
      'DOLE compliance reports',
    ],
  },
  {
    code: 'SG', flag: '🇸🇬', name: 'Singapore',
    items: [
      'IRAS tax filing (IR8A, IR8S, Appendix 8A)',
      'CPF contribution auto-calculation',
      'MOM Employment Act compliance',
      'SDL and FWL computation',
    ],
  },
  {
    code: 'LK', flag: '🇱🇰', name: 'Sri Lanka',
    items: [
      'EPF & ETF auto-calculation',
      'APIT tax compliance',
      'Gratuity computation',
      'Labour law compliance',
    ],
  },
  {
    code: 'ID', flag: '🇮🇩', name: 'Indonesia',
    items: [
      'PPh 21 income tax calculation',
      'BPJS Ketenagakerjaan & Kesehatan',
      'THR (Tunjangan Hari Raya) computation',
      'UMR/UMK minimum wage compliance',
    ],
  },
]

const features = [
  { icon: '👥', title: 'HR Management', desc: 'Complete employee lifecycle — from onboarding and 201 files to offboarding and final pay.' },
  { icon: '💰', title: 'Payroll Processing', desc: 'One-click payroll with auto tax calculations, statutory deductions, and digital payslips.' },
  { icon: '📊', title: 'Tax & Compliance', desc: 'Auto-generate BIR forms, IRAS filings, EPF/ETF, and stay compliant with local regulations.' },
  { icon: '🤖', title: 'AI-Powered Insights', desc: 'Natural language HR queries, automated compliance monitoring, and predictive analytics.' },
  { icon: '📅', title: 'Time & Attendance', desc: 'GPS clock-in, geofencing, shift scheduling, overtime tracking, and DTR reports.' },
  { icon: '🎯', title: 'Performance & Goals', desc: 'KPI tracking, performance reviews, OKRs, training management, and career development.' },
]

const steps = [
  { title: 'Sign Up Free', desc: 'Create your account in 30 seconds. No credit card, no commitment.' },
  { title: 'Setup Your Company', desc: 'Add departments, import employees via CSV, and configure leave policies.' },
  { title: 'Run Payroll', desc: 'Process payroll in one click. Tax forms are auto-generated for filing.' },
]

const aiFeatures = [
  'Natural language HR queries — "Show me employees with expiring contracts"',
  'Auto-compute 13th month pay, final pay, separation pay',
  'BIR compliance monitoring & deadline reminders',
  'Workforce analytics & flight risk detection',
  'AI-assisted onboarding workflows',
]

const compareRows = [
  { feature: 'Starting price', halaos: 'Free', sprout: '~P50/emp/mo', swingvy: '~S$5/emp/mo', juantax: 'Free (tax only)' },
  { feature: 'HR Management', halaos: 'Full', sprout: 'Full', swingvy: 'Full', juantax: 'None' },
  { feature: 'Payroll', halaos: 'Full', sprout: 'Full', swingvy: 'Basic', juantax: 'None' },
  { feature: 'Tax Filing (BIR)', halaos: 'Auto-generate', sprout: 'Yes', swingvy: 'No', juantax: 'Yes' },
  { feature: 'Tax Filing (IRAS)', halaos: 'Yes', sprout: 'No', swingvy: 'Yes', juantax: 'No' },
  { feature: 'Accounting', halaos: 'Included', sprout: 'No', swingvy: 'No', juantax: 'Basic' },
  { feature: 'AI Features', halaos: 'Included', sprout: 'No', swingvy: 'No', juantax: 'No' },
  { feature: 'Multi-country', halaos: '4 countries', sprout: 'PH only', swingvy: 'SG/MY', juantax: 'PH only' },
  { feature: 'Employee limit', halaos: 'Unlimited', sprout: 'Plan-based', swingvy: 'Plan-based', juantax: 'N/A' },
]

const toolLinks = [
  { icon: '🧮', label: 'PH Income Tax Calculator', to: '/tools/tax-calculator' },
  { icon: '🏦', label: 'SSS Contribution Calculator', to: '/tools/sss-calculator' },
  { icon: '🎄', label: '13th Month Pay Calculator', to: '/tools/13th-month-calculator' },
]
</script>

<style scoped>
.container {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 24px;
}

/* Hero */
.hero {
  padding: 100px 0 60px;
  text-align: center;
  background: linear-gradient(180deg, #f8fafc 0%, #fff 100%);
}
.hero-content {
  max-width: 740px;
  margin: 0 auto;
}
.hero-badge {
  display: inline-block;
  background: #dcfce7;
  color: #16a34a;
  font-size: 13px;
  font-weight: 600;
  padding: 6px 16px;
  border-radius: 20px;
  margin-bottom: 24px;
}
.hero h1 {
  font-size: 52px;
  font-weight: 800;
  line-height: 1.1;
  color: #0f172a;
  margin: 0 0 20px;
  letter-spacing: -1px;
}
.gradient-text {
  background: linear-gradient(135deg, #4f46e5, #7c3aed);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}
.hero-sub {
  font-size: 18px;
  color: #475569;
  line-height: 1.7;
  margin-bottom: 32px;
}
.hero-actions {
  display: flex;
  gap: 12px;
  justify-content: center;
}
.hero-note {
  margin-top: 16px;
  font-size: 13px;
  color: #94a3b8;
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
  transition: all 0.2s;
}
.btn-outline:hover { border-color: #4f46e5; color: #4f46e5; }
.btn-lg { padding: 14px 36px; font-size: 16px; }
.btn-text {
  color: #64748b;
  text-decoration: none;
  font-size: 14px;
  font-weight: 500;
  margin-left: 16px;
}
.btn-text:hover { color: #4f46e5; }

/* Social Proof */
.social-proof {
  padding: 40px 0;
  border-top: 1px solid #f1f5f9;
  border-bottom: 1px solid #f1f5f9;
}
.proof-row {
  display: flex;
  justify-content: center;
  gap: 64px;
  flex-wrap: wrap;
}
.proof-item {
  text-align: center;
}
.proof-num {
  display: block;
  font-size: 32px;
  font-weight: 800;
  color: #4f46e5;
  letter-spacing: -0.5px;
}
.proof-label {
  font-size: 14px;
  color: #64748b;
  margin-top: 4px;
}

/* Pain Points */
.pain-points {
  padding: 80px 0;
}
.pain-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 24px;
  margin-top: 48px;
}
.pain-card {
  background: #fff;
  border: 1px solid #f1f5f9;
  border-radius: 12px;
  padding: 28px;
  transition: box-shadow 0.2s;
}
.pain-card:hover { box-shadow: 0 4px 16px rgba(0,0,0,0.06); }
.pain-icon { font-size: 28px; margin-bottom: 12px; }
.pain-card h3 {
  font-size: 17px;
  font-weight: 700;
  color: #0f172a;
  margin: 0 0 8px;
}
.pain-card > p {
  font-size: 14px;
  color: #64748b;
  line-height: 1.6;
  margin: 0;
}
.pain-solution {
  font-size: 14px;
  font-weight: 600;
  color: #4f46e5;
  margin-top: 12px;
}

/* Countries */
.countries-section {
  padding: 80px 0;
  background: #f8fafc;
}
.countries-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 24px;
}
.country-card {
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  padding: 24px;
}
.country-flag { font-size: 36px; margin-bottom: 8px; }
.country-card h3 {
  font-size: 18px;
  font-weight: 700;
  color: #0f172a;
  margin: 0 0 12px;
}
.country-card ul {
  list-style: none;
  padding: 0;
  margin: 0;
}
.country-card li {
  padding: 4px 0;
  font-size: 13px;
  color: #475569;
  line-height: 1.5;
}
.country-card li::before {
  content: '\2713';
  color: #4f46e5;
  font-weight: 700;
  margin-right: 6px;
}

/* Section Titles */
.section-title {
  text-align: center;
  font-size: 36px;
  font-weight: 800;
  color: #0f172a;
  margin-bottom: 12px;
  letter-spacing: -0.5px;
}
.section-sub {
  text-align: center;
  color: #64748b;
  font-size: 17px;
  margin-bottom: 48px;
}

/* Features */
.features { padding: 80px 0; }
.features-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 28px;
}
.feature-card {
  background: #fff;
  border: 1px solid #f1f5f9;
  border-radius: 12px;
  padding: 28px;
  transition: box-shadow 0.2s;
}
.feature-card:hover { box-shadow: 0 4px 16px rgba(0,0,0,0.06); }
.feature-icon { font-size: 32px; margin-bottom: 12px; }
.feature-card h3 {
  font-size: 18px;
  font-weight: 700;
  color: #0f172a;
  margin: 0 0 8px;
}
.feature-card p {
  font-size: 14px;
  color: #64748b;
  line-height: 1.6;
  margin: 0;
}

/* How It Works */
.how-it-works {
  padding: 80px 0;
  background: #f8fafc;
}
.steps-row {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 32px;
  margin-top: 48px;
}
.step-card {
  text-align: center;
  padding: 32px 24px;
}
.step-num {
  width: 48px;
  height: 48px;
  background: linear-gradient(135deg, #4f46e5, #7c3aed);
  color: #fff;
  font-size: 20px;
  font-weight: 700;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 0 auto 16px;
}
.step-card h3 {
  font-size: 18px;
  font-weight: 700;
  color: #0f172a;
  margin: 0 0 8px;
}
.step-card p {
  font-size: 14px;
  color: #64748b;
  line-height: 1.6;
  margin: 0;
}

/* AI Section */
.ai-section { padding: 80px 0; }
.ai-content {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 48px;
  align-items: center;
}
.ai-text h2 {
  font-size: 36px;
  font-weight: 800;
  color: #0f172a;
  margin: 0 0 16px;
  letter-spacing: -0.5px;
}
.ai-text p {
  color: #475569;
  line-height: 1.7;
  margin-bottom: 24px;
}
.ai-features {
  list-style: none;
  padding: 0;
  margin: 0;
}
.ai-features li {
  padding: 8px 0;
  color: #334155;
  font-size: 14px;
}
.ai-features li::before {
  content: '\2713';
  color: #4f46e5;
  font-weight: 700;
  margin-right: 10px;
}
.ai-card {
  background: #0f172a;
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 8px 32px rgba(0,0,0,0.12);
}
.ai-card-header {
  background: #1e293b;
  padding: 12px 16px;
  color: #94a3b8;
  font-size: 13px;
  font-weight: 600;
}
.ai-card-body {
  padding: 20px 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.ai-msg {
  padding: 10px 14px;
  border-radius: 8px;
  font-size: 13px;
  line-height: 1.5;
  max-width: 85%;
}
.ai-msg-user {
  background: #4f46e5;
  color: #fff;
  align-self: flex-end;
}
.ai-msg-bot {
  background: #1e293b;
  color: #e2e8f0;
  align-self: flex-start;
}

/* Comparison Table */
.comparison-section {
  padding: 80px 0;
  background: #f8fafc;
}
.compare-table-wrap {
  overflow-x: auto;
}
.compare-table {
  width: 100%;
  border-collapse: collapse;
  background: #fff;
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 1px 3px rgba(0,0,0,0.06);
}
.compare-table th,
.compare-table td {
  padding: 14px 20px;
  text-align: center;
  font-size: 14px;
  border-bottom: 1px solid #f1f5f9;
}
.compare-table th {
  background: #f8fafc;
  font-weight: 600;
  color: #0f172a;
  font-size: 15px;
}
.compare-table td.feature-name {
  text-align: left;
  font-weight: 500;
  color: #334155;
}
.compare-table td { color: #475569; }
.highlight-col { background: #eef2ff; }

/* Tools CTA */
.tools-cta {
  padding: 80px 0;
}
.tools-cta-content {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 48px;
  align-items: center;
}
.tools-cta-text h2 {
  font-size: 32px;
  font-weight: 800;
  color: #0f172a;
  margin: 0 0 12px;
}
.tools-cta-text p {
  color: #475569;
  line-height: 1.7;
  margin-bottom: 24px;
}
.tools-cta-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.tool-link {
  display: flex;
  align-items: center;
  gap: 12px;
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 10px;
  padding: 16px 20px;
  text-decoration: none;
  color: #334155;
  font-weight: 500;
  font-size: 15px;
  transition: all 0.2s;
}
.tool-link:hover {
  border-color: #4f46e5;
  box-shadow: 0 2px 8px rgba(79,70,229,0.1);
}
.tool-icon { font-size: 24px; }

/* Free Banner */
.free-banner {
  padding: 80px 0;
  background: #f8fafc;
}
.free-card {
  max-width: 580px;
  margin: 0 auto;
  text-align: center;
  background: #fff;
  border: 2px solid #4f46e5;
  border-radius: 16px;
  padding: 48px 40px;
  box-shadow: 0 4px 24px rgba(79,70,229,0.1);
}
.free-price {
  font-size: 56px;
  font-weight: 800;
  color: #4f46e5;
  line-height: 1;
  margin-bottom: 8px;
}
.free-card h2 {
  font-size: 28px;
  font-weight: 800;
  color: #0f172a;
  margin: 0 0 12px;
}
.free-card > p {
  color: #64748b;
  font-size: 15px;
  line-height: 1.7;
  margin-bottom: 24px;
}
.free-perks {
  list-style: none;
  padding: 0;
  margin: 0 0 32px;
  display: inline-block;
  text-align: left;
}
.free-perks li {
  padding: 6px 0;
  color: #334155;
  font-size: 15px;
}
.free-perks li::before {
  content: '\2713';
  color: #4f46e5;
  font-weight: 700;
  margin-right: 10px;
}
.free-actions {
  display: flex;
  align-items: center;
  justify-content: center;
}

/* CTA */
.cta-section {
  padding: 80px 0;
  text-align: center;
  background: linear-gradient(135deg, #4f46e5, #7c3aed);
  color: #fff;
}
.cta-section h2 {
  font-size: 36px;
  font-weight: 800;
  margin: 0 0 12px;
}
.cta-section p {
  font-size: 17px;
  opacity: 0.9;
  margin-bottom: 32px;
}
.cta-section .btn-primary {
  background: #fff;
  color: #4f46e5;
}
.cta-section .btn-primary:hover { background: #f1f5f9; }

/* Responsive */
@media (max-width: 768px) {
  .hero h1 { font-size: 36px; }
  .hero-sub { font-size: 16px; }
  .hero-actions { flex-direction: column; align-items: center; }
  .proof-row { gap: 32px; }
  .pain-grid,
  .features-grid,
  .steps-row {
    grid-template-columns: 1fr;
  }
  .countries-grid { grid-template-columns: 1fr 1fr; }
  .ai-content,
  .tools-cta-content {
    grid-template-columns: 1fr;
  }
  .section-title { font-size: 28px; }
  .ai-text h2 { font-size: 28px; }
  .cta-section h2 { font-size: 28px; }
  .tools-cta-text h2 { font-size: 26px; }
}

@media (max-width: 480px) {
  .countries-grid { grid-template-columns: 1fr; }
  .proof-row { gap: 24px; }
}
</style>
