<template>
  <div class="features-page">
    <!-- Hero -->
    <section class="feat-hero">
      <div class="container">
        <h1>Built for modern HR teams</h1>
        <p>Every tool you need to manage employees, run payroll, and stay compliant — all in one place.</p>
      </div>
    </section>

    <!-- Feature Sections -->
    <section v-for="(cat, idx) in categories" :key="cat.id" :id="cat.id" class="feat-section" :class="{ alt: idx % 2 === 1 }">
      <div class="container">
        <div class="feat-row" :class="{ reverse: idx % 2 === 1 }">
          <div class="feat-text">
            <div class="feat-label">{{ cat.label }}</div>
            <h2>{{ cat.title }}</h2>
            <p>{{ cat.desc }}</p>
            <ul class="feat-list">
              <li v-for="item in cat.items" :key="item">{{ item }}</li>
            </ul>
          </div>
          <div class="feat-visual">
            <div class="feat-mockup">
              <div class="mockup-header">
                <span class="mockup-dot" style="background:#ef4444"></span>
                <span class="mockup-dot" style="background:#eab308"></span>
                <span class="mockup-dot" style="background:#22c55e"></span>
                <span class="mockup-title">{{ cat.label }}</span>
              </div>
              <div class="mockup-body">
                <div v-for="(line, li) in cat.mockupLines" :key="li" class="mockup-line" :style="{ width: line + '%' }"></div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- Integrations -->
    <section class="integrations" id="integrations">
      <div class="container">
        <h2 class="section-title">Integrations &amp; Compatibility</h2>
        <p class="section-sub">Works with the tools your team already uses.</p>
        <div class="int-grid">
          <div v-for="int_item in integrations" :key="int_item.name" class="int-card">
            <span class="int-icon">{{ int_item.icon }}</span>
            <span class="int-name">{{ int_item.name }}</span>
          </div>
        </div>
      </div>
    </section>

    <!-- Security -->
    <section class="security" id="security">
      <div class="container">
        <h2 class="section-title">Enterprise-grade security</h2>
        <p class="section-sub">Your data is protected with industry-leading security standards.</p>
        <div class="sec-grid">
          <div v-for="s in securityFeatures" :key="s.title" class="sec-card">
            <span class="sec-icon">{{ s.icon }}</span>
            <h3>{{ s.title }}</h3>
            <p>{{ s.desc }}</p>
          </div>
        </div>
      </div>
    </section>

    <!-- CTA -->
    <section class="feat-cta">
      <div class="container">
        <h2>Ready to streamline your HR?</h2>
        <p>Start your free trial today. No credit card required.</p>
        <router-link to="/register" class="btn-primary btn-lg">Get Started Free</router-link>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { useHead } from '@unhead/vue'

useHead({
  title: 'Features - HalaOS',
  meta: [
    { name: 'description', content: 'Explore HalaOS features: HR management, automated payroll, tax compliance, AI-powered insights, and more. Built for businesses in Southeast Asia.' },
    { property: 'og:title', content: 'Features - HalaOS' },
    { property: 'og:description', content: 'Every tool you need to manage employees, run payroll, and stay compliant — all in one place.' },
    { property: 'og:url', content: 'https://halaos.com/features' },
  ],
})

const categories = [
  {
    id: 'hr',
    label: 'HR Management',
    title: 'Complete employee lifecycle',
    desc: 'Manage everything from onboarding to offboarding with digital 201 files, org charts, and self-service portals.',
    items: [
      'Employee profiles & 201 file management',
      'Onboarding checklists & document collection',
      'Org chart & directory',
      'Self-service portal for employees',
      'Disciplinary & grievance tracking',
      'Contract milestone alerts',
    ],
    mockupLines: [90, 75, 85, 60, 80, 70],
  },
  {
    id: 'payroll',
    label: 'Payroll & Compensation',
    title: 'Payroll that runs itself',
    desc: 'Automated payroll processing with local tax calculations, statutory deductions, and one-click payslip generation.',
    items: [
      'Multi-frequency payroll (weekly, bi-monthly, monthly)',
      'Auto-calculated SSS, PhilHealth, Pag-IBIG, tax',
      'Overtime, holiday pay, night differential',
      'Loan management & salary deductions',
      'Digital payslips & bank file generation',
      'Final pay computation',
    ],
    mockupLines: [85, 70, 95, 55, 80, 65],
  },
  {
    id: 'tax',
    label: 'Tax & Compliance',
    title: 'Stay compliant, effortlessly',
    desc: 'Automatically generate and manage tax filings across multiple jurisdictions with built-in compliance checks.',
    items: [
      'BIR 2316, 1601C, 2550M/Q auto-generation',
      'EPF/ETF filings for Sri Lanka',
      'IRAS compliance for Singapore',
      'Compliance calendar & reminders',
      'Audit trail for all transactions',
      'Multi-jurisdiction support',
    ],
    mockupLines: [80, 60, 90, 75, 85, 55],
  },
  {
    id: 'time',
    label: 'Time & Attendance',
    title: 'Track time, anywhere',
    desc: 'Flexible attendance tracking with geofencing, mobile clock-in, and automated schedule management.',
    items: [
      'GPS-enabled clock in/out',
      'Geofence zones for office locations',
      'Flexible shift scheduling',
      'Leave management & calendar',
      'Overtime request & approval',
      'Daily Time Record (DTR) reports',
    ],
    mockupLines: [70, 85, 60, 90, 75, 80],
  },
  {
    id: 'ai',
    label: 'AI Intelligence',
    title: 'AI that works for HR',
    desc: 'Leverage AI-powered agents to automate routine tasks, surface insights, and answer employee questions instantly.',
    items: [
      'Natural language HR queries',
      'Automated compliance monitoring',
      'Predictive attrition analytics',
      'Smart document classification',
      'AI-assisted performance reviews',
      'Knowledge base with RAG search',
    ],
    mockupLines: [95, 80, 70, 85, 65, 90],
  },
  {
    id: 'analytics',
    label: 'Analytics & Reporting',
    title: 'Data-driven decisions',
    desc: 'Real-time dashboards and custom reports to understand your workforce and make informed decisions.',
    items: [
      'Workforce headcount & demographics',
      'Payroll cost analysis',
      'Attendance & leave trends',
      'Turnover & retention metrics',
      'Custom report builder',
      'Export to Excel/PDF',
    ],
    mockupLines: [85, 75, 90, 65, 80, 70],
  },
]

const integrations = [
  { icon: '🏦', name: 'Banks' },
  { icon: '📧', name: 'Email' },
  { icon: '💬', name: 'Slack' },
  { icon: '📊', name: 'Excel' },
  { icon: '☁️', name: 'Google Workspace' },
  { icon: '🔗', name: 'REST API' },
  { icon: '📱', name: 'Mobile App' },
  { icon: '🧮', name: 'Accounting' },
]

const securityFeatures = [
  { icon: '🔒', title: 'Data Encryption', desc: 'AES-256 encryption at rest and TLS 1.3 in transit for all data.' },
  { icon: '🛡️', title: 'Role-Based Access', desc: 'Granular permissions with role-based access control for every feature.' },
  { icon: '📋', title: 'Audit Logging', desc: 'Complete audit trail of every action taken in the system.' },
  { icon: '🔐', title: 'SSO Support', desc: 'Single sign-on integration with enterprise identity providers.' },
]
</script>

<style scoped>
.container {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 24px;
}

.feat-hero {
  padding: 100px 0 60px;
  text-align: center;
  background: linear-gradient(180deg, #f8fafc 0%, #fff 100%);
}
.feat-hero h1 {
  font-size: 48px;
  font-weight: 800;
  color: #0f172a;
  margin: 0 0 16px;
  letter-spacing: -1px;
}
.feat-hero p {
  font-size: 18px;
  color: #475569;
  max-width: 600px;
  margin: 0 auto;
}

.feat-section {
  padding: 80px 0;
}
.feat-section.alt {
  background: #f8fafc;
}
.feat-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 48px;
  align-items: center;
}
.feat-row.reverse {
  direction: rtl;
}
.feat-row.reverse > * {
  direction: ltr;
}
.feat-label {
  display: inline-block;
  background: #eef2ff;
  color: #4f46e5;
  font-size: 12px;
  font-weight: 600;
  padding: 4px 12px;
  border-radius: 12px;
  margin-bottom: 12px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
.feat-text h2 {
  font-size: 32px;
  font-weight: 800;
  color: #0f172a;
  margin: 0 0 12px;
}
.feat-text p {
  color: #475569;
  line-height: 1.7;
  margin-bottom: 20px;
}
.feat-list {
  list-style: none;
  padding: 0;
  margin: 0;
}
.feat-list li {
  padding: 6px 0;
  color: #334155;
  font-size: 14px;
}
.feat-list li::before {
  content: '✓';
  color: #4f46e5;
  font-weight: 700;
  margin-right: 10px;
}

.feat-mockup {
  background: #0f172a;
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.12);
}
.mockup-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 12px 16px;
  background: #1e293b;
}
.mockup-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}
.mockup-title {
  margin-left: 8px;
  color: #94a3b8;
  font-size: 12px;
}
.mockup-body {
  padding: 24px 16px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.mockup-line {
  height: 10px;
  background: #334155;
  border-radius: 4px;
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

.integrations {
  padding: 80px 0;
}
.int-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
}
.int-card {
  display: flex;
  align-items: center;
  gap: 10px;
  background: #fff;
  border: 1px solid #f1f5f9;
  border-radius: 10px;
  padding: 16px 20px;
  font-size: 15px;
  font-weight: 500;
  color: #334155;
}
.int-icon { font-size: 24px; }

.security {
  padding: 80px 0;
  background: #f8fafc;
}
.sec-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 24px;
}
.sec-card {
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  padding: 24px;
  text-align: center;
}
.sec-icon { font-size: 32px; display: block; margin-bottom: 12px; }
.sec-card h3 {
  font-size: 16px;
  font-weight: 700;
  color: #0f172a;
  margin: 0 0 8px;
}
.sec-card p {
  font-size: 13px;
  color: #64748b;
  line-height: 1.5;
  margin: 0;
}

.feat-cta {
  padding: 80px 0;
  text-align: center;
  background: linear-gradient(135deg, #4f46e5, #7c3aed);
  color: #fff;
}
.feat-cta h2 {
  font-size: 36px;
  font-weight: 800;
  margin: 0 0 12px;
}
.feat-cta p {
  font-size: 17px;
  opacity: 0.9;
  margin-bottom: 32px;
}
.btn-primary {
  display: inline-block;
  background: #fff;
  color: #4f46e5;
  font-weight: 600;
  font-size: 15px;
  padding: 12px 28px;
  border-radius: 8px;
  text-decoration: none;
  transition: background 0.2s;
}
.btn-primary:hover { background: #f1f5f9; }
.btn-lg { padding: 14px 36px; font-size: 16px; }

@media (max-width: 768px) {
  .feat-hero h1 { font-size: 32px; }
  .feat-row, .feat-row.reverse { grid-template-columns: 1fr; direction: ltr; }
  .int-grid { grid-template-columns: repeat(2, 1fr); }
  .sec-grid { grid-template-columns: 1fr 1fr; }
  .section-title { font-size: 28px; }
}
</style>
