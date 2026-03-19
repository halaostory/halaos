<template>
  <div class="pricing-page">
    <!-- Hero -->
    <section class="price-hero">
      <div class="container">
        <h1>Free for everyone</h1>
        <p>HalaOS is completely free. All features, unlimited employees, no credit card required.</p>
      </div>
    </section>

    <!-- Free Card -->
    <section class="free-section">
      <div class="container">
        <div class="free-card">
          <div class="free-price">$0</div>
          <h2>Everything included</h2>
          <p class="free-desc">We believe every company deserves world-class HR technology. That's why HalaOS is free — no trials, no tiers, no limits.</p>
          <router-link to="/register" class="btn-primary btn-lg">Get Started Free</router-link>
        </div>
      </div>
    </section>

    <!-- What's Included -->
    <section class="included">
      <div class="container">
        <h2 class="section-title">What's included</h2>
        <div class="included-grid">
          <div v-for="cat in categories" :key="cat.title" class="included-card">
            <span class="included-icon">{{ cat.icon }}</span>
            <h3>{{ cat.title }}</h3>
            <ul>
              <li v-for="item in cat.items" :key="item">{{ item }}</li>
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
        <p>No credit card. No trial period. Just sign up and go.</p>
        <router-link to="/register" class="btn-primary btn-lg">Create Free Account</router-link>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
const categories = [
  {
    icon: '👥',
    title: 'HR Management',
    items: ['Unlimited employees', 'Employee profiles & 201 files', 'Onboarding & offboarding', 'Org chart & directory', 'Self-service portal', 'Disciplinary & grievance tracking'],
  },
  {
    icon: '💰',
    title: 'Payroll & Compensation',
    items: ['Full payroll processing', 'Multi-frequency pay periods', 'Auto tax calculations', 'Overtime & holiday pay', 'Loan management', 'Digital payslips'],
  },
  {
    icon: '📊',
    title: 'Tax & Compliance',
    items: ['BIR form auto-generation', 'EPF/ETF filings (Sri Lanka)', 'Compliance calendar', 'Audit trail', 'Multi-jurisdiction support', 'Tax filing automation'],
  },
  {
    icon: '📅',
    title: 'Time & Attendance',
    items: ['GPS clock in/out', 'Geofence zones', 'Shift scheduling', 'Leave management', 'Overtime tracking', 'DTR reports'],
  },
  {
    icon: '🤖',
    title: 'AI Features',
    items: ['Natural language queries', 'Compliance monitoring', 'Workforce analytics', 'Smart document processing', 'AI-assisted reviews', 'Knowledge base & RAG'],
  },
  {
    icon: '📈',
    title: 'Analytics & More',
    items: ['Workforce dashboards', 'Custom reports', 'Performance management', 'Training & certification', 'Import/export data', 'API access'],
  },
]

const faqs = [
  { q: 'Is HalaOS really free?', a: 'Yes, 100% free. All features are included with no usage limits, no credit card required, and no trial period.' },
  { q: 'How do you make money?', a: 'We are currently focused on building the best HR platform for Southeast Asia. Premium add-ons and enterprise services may be offered in the future, but the core platform will always be free.' },
  { q: 'Are there any employee limits?', a: 'No. You can add unlimited employees to your HalaOS account at no cost.' },
  { q: 'Which countries are supported?', a: 'HalaOS currently supports Philippines, Sri Lanka, Singapore, and Indonesia payroll and tax regulations. More countries are being added regularly.' },
  { q: 'Is my data secure?', a: 'Absolutely. We use AES-256 encryption, TLS 1.3, role-based access control, and maintain complete audit trails. Your data is hosted on secure cloud infrastructure.' },
  { q: 'Can I export my data?', a: 'Yes. You can export all your data at any time in Excel, CSV, or PDF format. Your data belongs to you.' },
]
</script>

<style scoped>
.container {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 24px;
}

.price-hero {
  padding: 100px 0 40px;
  text-align: center;
  background: linear-gradient(180deg, #f8fafc 0%, #fff 100%);
}
.price-hero h1 {
  font-size: 48px;
  font-weight: 800;
  color: #0f172a;
  margin: 0 0 16px;
  letter-spacing: -1px;
}
.price-hero p {
  font-size: 18px;
  color: #475569;
  max-width: 560px;
  margin: 0 auto;
}

.free-section {
  padding: 40px 0 80px;
}
.free-card {
  max-width: 520px;
  margin: 0 auto;
  text-align: center;
  background: #fff;
  border: 2px solid #4f46e5;
  border-radius: 16px;
  padding: 48px 40px;
  box-shadow: 0 4px 24px rgba(79, 70, 229, 0.1);
}
.free-price {
  font-size: 64px;
  font-weight: 800;
  color: #4f46e5;
  line-height: 1;
  margin-bottom: 8px;
}
.free-card h2 {
  font-size: 24px;
  font-weight: 700;
  color: #0f172a;
  margin: 0 0 12px;
}
.free-desc {
  color: #64748b;
  font-size: 15px;
  line-height: 1.7;
  margin-bottom: 28px;
}

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
.btn-lg { padding: 14px 36px; font-size: 16px; }

.included {
  padding: 80px 0;
  background: #f8fafc;
}
.section-title {
  text-align: center;
  font-size: 36px;
  font-weight: 800;
  color: #0f172a;
  margin-bottom: 40px;
}
.included-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 24px;
}
.included-card {
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  padding: 28px;
}
.included-icon {
  font-size: 28px;
  display: block;
  margin-bottom: 12px;
}
.included-card h3 {
  font-size: 18px;
  font-weight: 700;
  color: #0f172a;
  margin: 0 0 12px;
}
.included-card ul {
  list-style: none;
  padding: 0;
  margin: 0;
}
.included-card li {
  padding: 4px 0;
  font-size: 14px;
  color: #475569;
}
.included-card li::before {
  content: '✓';
  color: #4f46e5;
  font-weight: 700;
  margin-right: 8px;
}

.faq {
  padding: 80px 0;
}
.faq-list {
  max-width: 720px;
  margin: 0 auto;
}
.faq-list details {
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  margin-bottom: 12px;
  overflow: hidden;
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

@media (max-width: 768px) {
  .price-hero h1 { font-size: 32px; }
  .included-grid { grid-template-columns: 1fr; }
  .section-title { font-size: 28px; }
  .price-cta h2 { font-size: 28px; }
  .free-price { font-size: 48px; }
}
</style>
