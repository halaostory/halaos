<template>
  <div class="tools-page">
    <section class="tools-hero">
      <div class="container">
        <h1>Free HR & Tax Calculators</h1>
        <p>Compute SSS, PhilHealth, Pag-IBIG contributions, income tax, and 13th month pay instantly. No signup required.</p>
      </div>
    </section>

    <section class="tools-grid-section">
      <div class="container">
        <div class="tools-grid">
          <router-link v-for="tool in tools" :key="tool.to" :to="tool.to" class="tool-card">
            <div class="tool-icon">{{ tool.icon }}</div>
            <h3>{{ tool.title }}</h3>
            <p>{{ tool.desc }}</p>
            <span class="tool-link-text">Open Calculator &rarr;</span>
          </router-link>
        </div>
      </div>
    </section>

    <section class="tools-cta">
      <div class="container">
        <div class="cta-card">
          <h2>Want to automate this for all employees?</h2>
          <p>HalaOS computes SSS, PhilHealth, Pag-IBIG, withholding tax, 13th month pay, and more — automatically during payroll. Free for unlimited employees.</p>
          <router-link to="/register" class="btn-primary">Get Started Free</router-link>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { useHead } from '@unhead/vue'

useHead({
  title: 'Free HR & Tax Calculators - HalaOS',
  meta: [
    { name: 'description', content: 'Free online calculators for Philippine income tax, SSS contributions, PhilHealth, Pag-IBIG, 13th month pay, and more. Instant results, no signup required.' },
    { property: 'og:title', content: 'Free HR & Tax Calculators - HalaOS' },
    { property: 'og:url', content: 'https://halaos.com/tools' },
    { name: 'keywords', content: 'SSS calculator, income tax calculator philippines, 13th month pay calculator, PhilHealth calculator, Pag-IBIG calculator, free payroll calculator' },
  ],
})

const tools = [
  {
    icon: '🧮',
    title: 'PH Income Tax Calculator',
    desc: 'Compute monthly and annual withholding tax based on the 2026 BIR tax table. Includes graduated tax rates.',
    to: '/tools/tax-calculator',
  },
  {
    icon: '🏦',
    title: 'SSS Contribution Calculator',
    desc: 'Calculate SSS employee and employer shares including MPF. Based on the 2026 contribution schedule.',
    to: '/tools/sss-calculator',
  },
  {
    icon: '🎄',
    title: '13th Month Pay Calculator',
    desc: 'Compute 13th month pay for full-year and pro-rated employees. Handles salary changes and mid-year hires.',
    to: '/tools/13th-month-calculator',
  },
]
</script>

<style scoped>
.container {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 24px;
}
.tools-hero {
  padding: 100px 0 40px;
  text-align: center;
  background: linear-gradient(180deg, #f8fafc 0%, #fff 100%);
}
.tools-hero h1 {
  font-size: 48px;
  font-weight: 800;
  color: #0f172a;
  margin: 0 0 12px;
  letter-spacing: -1px;
}
.tools-hero p {
  font-size: 18px;
  color: #475569;
  max-width: 600px;
  margin: 0 auto;
}
.tools-grid-section {
  padding: 48px 0 80px;
}
.tools-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 24px;
}
.tool-card {
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 16px;
  padding: 32px;
  text-decoration: none;
  transition: all 0.2s;
  display: block;
}
.tool-card:hover {
  border-color: #4f46e5;
  box-shadow: 0 8px 24px rgba(79,70,229,0.1);
  transform: translateY(-2px);
}
.tool-icon { font-size: 40px; margin-bottom: 16px; }
.tool-card h3 {
  font-size: 20px;
  font-weight: 700;
  color: #0f172a;
  margin: 0 0 8px;
}
.tool-card p {
  font-size: 14px;
  color: #64748b;
  line-height: 1.6;
  margin: 0 0 16px;
}
.tool-link-text {
  color: #4f46e5;
  font-weight: 600;
  font-size: 14px;
}
.tools-cta { padding: 0 0 80px; }
.cta-card {
  background: linear-gradient(135deg, #4f46e5, #7c3aed);
  border-radius: 16px;
  padding: 48px;
  text-align: center;
  color: #fff;
}
.cta-card h2 {
  font-size: 28px;
  font-weight: 800;
  margin: 0 0 12px;
}
.cta-card p {
  font-size: 16px;
  opacity: 0.9;
  margin-bottom: 24px;
  max-width: 560px;
  margin-left: auto;
  margin-right: auto;
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
}
.btn-primary:hover { background: #f1f5f9; }
@media (max-width: 768px) {
  .tools-hero h1 { font-size: 32px; }
  .tools-grid { grid-template-columns: 1fr; }
}
</style>
