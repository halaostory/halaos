<template>
  <div class="home">
    <!-- Hero -->
    <section class="hero">
      <div class="container">
        <div class="hero-content">
          <div class="hero-badge">Trusted by 500+ companies across Asia</div>
          <h1>One Platform for <span class="gradient-text">HR, Payroll &amp; Tax</span></h1>
          <p class="hero-sub">
            HalaOS unifies employee management, payroll processing, and tax compliance
            into a single intelligent platform. Built for businesses in the Philippines,
            Sri Lanka, Singapore, and beyond.
          </p>
          <div class="hero-actions">
            <router-link to="/register" class="btn-primary">Get Started Free</router-link>
            <router-link to="/features" class="btn-outline">Explore Features</router-link>
          </div>
          <p class="hero-note">100% Free &middot; No credit card required &middot; Unlimited features</p>
        </div>
      </div>
    </section>

    <!-- Logos -->
    <section class="logos-bar">
      <div class="container">
        <p class="logos-label">Powering HR operations across Southeast Asia</p>
        <div class="logos-row">
          <span v-for="c in countries" :key="c.code" class="logo-item">
            <span class="flag">{{ c.flag }}</span>
            <span>{{ c.name }}</span>
          </span>
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
              HalaOS leverages advanced AI to automate repetitive HR tasks,
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
                <div class="ai-msg ai-msg-user">Show me employees with expiring contracts this month</div>
                <div class="ai-msg ai-msg-bot">Found 5 employees with contracts expiring in March 2026. 3 are recommended for renewal based on performance scores.</div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- Free Banner -->
    <section class="free-banner">
      <div class="container">
        <div class="free-card">
          <div class="free-price">$0</div>
          <h2>Completely Free</h2>
          <p>All features, unlimited employees, no hidden fees. HalaOS is free for everyone.</p>
          <ul class="free-perks">
            <li>Full HR &amp; Payroll management</li>
            <li>Tax filing automation</li>
            <li>AI-powered insights</li>
            <li>Unlimited employees</li>
          </ul>
          <router-link to="/register" class="btn-primary btn-lg">Get Started Free</router-link>
        </div>
      </div>
    </section>

    <!-- CTA -->
    <section class="cta-section">
      <div class="container">
        <h2>Start managing your workforce today</h2>
        <p>Join hundreds of companies using HalaOS to streamline HR operations.</p>
        <router-link to="/register" class="btn-primary btn-lg">Get Started Free</router-link>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { useHead } from '@unhead/vue'

useHead({
  title: 'HalaOS - Unified HR, Payroll & Tax Platform',
  meta: [
    { name: 'description', content: 'HalaOS unifies employee management, payroll processing, and tax compliance into a single intelligent platform for businesses across Asia.' },
    { property: 'og:title', content: 'HalaOS - Unified HR, Payroll & Tax Platform' },
    { property: 'og:description', content: 'One platform for HR management, payroll processing, and tax compliance across Southeast Asia.' },
    { property: 'og:url', content: 'https://halaos.com/' },
  ],
})

const countries = [
  { code: 'PH', flag: '🇵🇭', name: 'Philippines' },
  { code: 'LK', flag: '🇱🇰', name: 'Sri Lanka' },
  { code: 'SG', flag: '🇸🇬', name: 'Singapore' },
  { code: 'ID', flag: '🇮🇩', name: 'Indonesia' },
  { code: 'MY', flag: '🇲🇾', name: 'Malaysia' },
]

const features = [
  { icon: '👥', title: 'HR Management', desc: 'Complete employee lifecycle — from onboarding and 201 files to offboarding and final pay.' },
  { icon: '💰', title: 'Payroll Processing', desc: 'Automated payroll with local tax calculations, statutory deductions, and payslip generation.' },
  { icon: '📊', title: 'Tax & Compliance', desc: 'Auto-generate BIR forms, EPF/ETF filings, and ensure compliance with local regulations.' },
  { icon: '🤖', title: 'AI-Powered Insights', desc: 'Intelligent workforce analytics, natural language queries, and automated recommendations.' },
  { icon: '📅', title: 'Time & Attendance', desc: 'Flexible scheduling, geo-fenced clock-in, overtime tracking, and DTR reports.' },
  { icon: '🎯', title: 'Performance & Goals', desc: 'Set KPIs, conduct reviews, track OKRs, and align individual goals with company objectives.' },
]

const steps = [
  { title: 'Sign Up', desc: 'Create your account in 30 seconds. No credit card needed.' },
  { title: 'Setup Your Company', desc: 'Add departments, import employees, and configure your policies.' },
  { title: 'Run Payroll', desc: 'Process payroll in one click with auto-calculated taxes and deductions.' },
]

const aiFeatures = [
  'Natural language HR queries',
  'Automated compliance monitoring',
  'Predictive workforce analytics',
  'Smart document processing',
  'AI-assisted onboarding workflows',
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
  padding: 100px 0 80px;
  text-align: center;
  background: linear-gradient(180deg, #f8fafc 0%, #fff 100%);
}
.hero-content {
  max-width: 720px;
  margin: 0 auto;
}
.hero-badge {
  display: inline-block;
  background: #eef2ff;
  color: #4f46e5;
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

/* Logos Bar */
.logos-bar {
  padding: 40px 0;
  border-top: 1px solid #f1f5f9;
  border-bottom: 1px solid #f1f5f9;
  text-align: center;
}
.logos-label {
  font-size: 13px;
  color: #94a3b8;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin-bottom: 20px;
}
.logos-row {
  display: flex;
  justify-content: center;
  gap: 40px;
  flex-wrap: wrap;
}
.logo-item {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #475569;
  font-weight: 500;
  font-size: 15px;
}
.flag { font-size: 24px; }

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
.features {
  padding: 80px 0;
}
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
.feature-card:hover {
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.06);
}
.feature-icon {
  font-size: 32px;
  margin-bottom: 12px;
}
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
.ai-section {
  padding: 80px 0;
}
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
  font-size: 15px;
}
.ai-features li::before {
  content: '✓';
  color: #4f46e5;
  font-weight: 700;
  margin-right: 10px;
}
.ai-card {
  background: #0f172a;
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.12);
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

/* Free Banner */
.free-banner {
  padding: 80px 0;
  background: #f8fafc;
}
.free-card {
  max-width: 560px;
  margin: 0 auto;
  text-align: center;
  background: #fff;
  border: 2px solid #4f46e5;
  border-radius: 16px;
  padding: 48px 40px;
  box-shadow: 0 4px 24px rgba(79, 70, 229, 0.1);
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
  font-size: 16px;
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
  content: '✓';
  color: #4f46e5;
  font-weight: 700;
  margin-right: 10px;
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
.cta-section .btn-primary:hover {
  background: #f1f5f9;
}

/* Responsive */
@media (max-width: 768px) {
  .hero h1 { font-size: 36px; }
  .hero-sub { font-size: 16px; }
  .hero-actions { flex-direction: column; align-items: center; }
  .features-grid,
  .steps-row,
  .pricing-cards {
    grid-template-columns: 1fr;
  }
  .ai-content {
    grid-template-columns: 1fr;
  }
  .section-title { font-size: 28px; }
  .ai-text h2 { font-size: 28px; }
  .cta-section h2 { font-size: 28px; }
}
</style>
