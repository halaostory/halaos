<template>
  <div class="about-page">
    <!-- Hero -->
    <section class="about-hero">
      <div class="container">
        <h1>Building the future of HR in Asia</h1>
        <p>
          HalaOS was born from a simple belief: every company in Southeast Asia deserves
          world-class HR technology that understands local regulations and culture.
        </p>
      </div>
    </section>

    <!-- Mission -->
    <section class="mission">
      <div class="container">
        <div class="mission-grid">
          <div class="mission-card">
            <h3>Our Mission</h3>
            <p>
              To empower businesses across Asia with an intelligent, unified platform
              that makes HR management, payroll processing, and tax compliance effortless —
              so companies can focus on what matters most: their people.
            </p>
          </div>
          <div class="mission-card">
            <h3>Our Vision</h3>
            <p>
              To become the most trusted HR technology platform in Southeast Asia,
              serving millions of employees across multiple countries with
              AI-powered tools that anticipate needs and automate the mundane.
            </p>
          </div>
        </div>
      </div>
    </section>

    <!-- Values -->
    <section class="values">
      <div class="container">
        <h2 class="section-title">What drives us</h2>
        <div class="values-grid">
          <div v-for="v in values" :key="v.title" class="value-card">
            <span class="value-icon">{{ v.icon }}</span>
            <h3>{{ v.title }}</h3>
            <p>{{ v.desc }}</p>
          </div>
        </div>
      </div>
    </section>

    <!-- Stats -->
    <section class="stats">
      <div class="container">
        <div class="stats-grid">
          <div v-for="s in stats" :key="s.label" class="stat-item">
            <span class="stat-num">{{ s.num }}</span>
            <span class="stat-label">{{ s.label }}</span>
          </div>
        </div>
      </div>
    </section>

    <!-- Story -->
    <section class="story">
      <div class="container">
        <h2 class="section-title">Our Story</h2>
        <div class="story-content">
          <p>
            Founded in 2025, HalaOS started as a solution to a real problem:
            managing HR across multiple Southeast Asian countries with different
            labor laws, tax regulations, and compliance requirements.
          </p>
          <p>
            Our founders experienced firsthand the pain of juggling spreadsheets,
            multiple government portals, and disconnected HR tools. They set out
            to build a single platform that could handle it all — from hiring
            an employee in Manila to processing payroll in Colombo.
          </p>
          <p>
            Today, HalaOS serves companies across the Philippines, Sri Lanka,
            Singapore, and Indonesia, with more countries on the way. Our
            AI-powered platform processes thousands of payroll runs monthly
            and has automated millions of tax calculations.
          </p>
          <p>
            We're backed by a passionate team of engineers, HR experts, and
            compliance specialists who are committed to making HR technology
            accessible to businesses of all sizes.
          </p>
        </div>
      </div>
    </section>

    <!-- Team -->
    <section class="team" id="careers">
      <div class="container">
        <h2 class="section-title">Join our team</h2>
        <p class="section-sub">
          We're always looking for talented people who share our passion for
          building great products and solving real problems.
        </p>
        <div class="careers-card">
          <h3>Open Positions</h3>
          <p>
            We're a remote-first company with team members across Southeast Asia.
            Check back soon for open positions, or send us your resume at
            <a href="mailto:careers@halaos.com">careers@halaos.com</a>.
          </p>
          <router-link to="/contact" class="btn-outline">Get in Touch</router-link>
        </div>
      </div>
    </section>

    <!-- CTA -->
    <section class="about-cta">
      <div class="container">
        <h2>Ready to transform your HR?</h2>
        <p>Join hundreds of companies already using HalaOS.</p>
        <router-link to="/register" class="btn-primary btn-lg">Get Started Free</router-link>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
const values = [
  { icon: '🌏', title: 'Local First', desc: 'Built for Asia, by Asia. We understand local regulations, culture, and business practices.' },
  { icon: '🧠', title: 'AI-Native', desc: 'AI is not an afterthought — it\'s woven into every feature to make HR smarter.' },
  { icon: '🔒', title: 'Trust & Security', desc: 'Enterprise-grade security with complete data privacy and compliance.' },
  { icon: '🤝', title: 'Customer Obsessed', desc: 'We succeed when our customers succeed. Every feature is built with users in mind.' },
  { icon: '⚡', title: 'Ship Fast', desc: 'We iterate quickly, listen to feedback, and continuously improve the platform.' },
  { icon: '🌱', title: 'Free for All', desc: 'HalaOS is completely free — because every business deserves great HR tools regardless of size.' },
]

const stats = [
  { num: '500+', label: 'Companies' },
  { num: '50,000+', label: 'Employees Managed' },
  { num: '5', label: 'Countries' },
  { num: '99.9%', label: 'Uptime' },
]
</script>

<style scoped>
.container {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 24px;
}

.about-hero {
  padding: 100px 0 60px;
  text-align: center;
  background: linear-gradient(180deg, #f8fafc 0%, #fff 100%);
}
.about-hero h1 {
  font-size: 48px;
  font-weight: 800;
  color: #0f172a;
  margin: 0 0 16px;
  letter-spacing: -1px;
}
.about-hero p {
  font-size: 18px;
  color: #475569;
  max-width: 640px;
  margin: 0 auto;
  line-height: 1.7;
}

.mission {
  padding: 80px 0;
}
.mission-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 32px;
}
.mission-card {
  background: #f8fafc;
  border-radius: 16px;
  padding: 36px;
}
.mission-card h3 {
  font-size: 22px;
  font-weight: 700;
  color: #0f172a;
  margin: 0 0 12px;
}
.mission-card p {
  color: #475569;
  line-height: 1.7;
  margin: 0;
}

.values {
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
.values-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 24px;
  margin-top: 40px;
}
.value-card {
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  padding: 28px;
}
.value-icon {
  font-size: 32px;
  display: block;
  margin-bottom: 12px;
}
.value-card h3 {
  font-size: 18px;
  font-weight: 700;
  color: #0f172a;
  margin: 0 0 8px;
}
.value-card p {
  font-size: 14px;
  color: #64748b;
  line-height: 1.6;
  margin: 0;
}

.stats {
  padding: 64px 0;
}
.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 32px;
  text-align: center;
}
.stat-num {
  display: block;
  font-size: 40px;
  font-weight: 800;
  color: #4f46e5;
}
.stat-label {
  color: #64748b;
  font-size: 14px;
  font-weight: 500;
}

.story {
  padding: 80px 0;
  background: #f8fafc;
}
.story-content {
  max-width: 720px;
  margin: 24px auto 0;
}
.story-content p {
  color: #475569;
  line-height: 1.8;
  font-size: 16px;
  margin-bottom: 20px;
}

.team {
  padding: 80px 0;
}
.careers-card {
  max-width: 600px;
  margin: 0 auto;
  background: #f8fafc;
  border-radius: 16px;
  padding: 36px;
  text-align: center;
}
.careers-card h3 {
  font-size: 20px;
  font-weight: 700;
  color: #0f172a;
  margin: 0 0 12px;
}
.careers-card p {
  color: #475569;
  line-height: 1.7;
  margin-bottom: 20px;
}
.careers-card a[href^="mailto"] {
  color: #4f46e5;
  font-weight: 600;
  text-decoration: none;
}
.btn-outline {
  display: inline-block;
  border: 1.5px solid #e2e8f0;
  color: #334155;
  font-weight: 600;
  font-size: 15px;
  padding: 10px 24px;
  border-radius: 8px;
  text-decoration: none;
  transition: all 0.2s;
}
.btn-outline:hover { border-color: #4f46e5; color: #4f46e5; }

.about-cta {
  padding: 80px 0;
  text-align: center;
  background: linear-gradient(135deg, #4f46e5, #7c3aed);
  color: #fff;
}
.about-cta h2 {
  font-size: 36px;
  font-weight: 800;
  margin: 0 0 12px;
}
.about-cta p {
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
  .about-hero h1 { font-size: 32px; }
  .mission-grid { grid-template-columns: 1fr; }
  .values-grid { grid-template-columns: 1fr; }
  .stats-grid { grid-template-columns: 1fr 1fr; }
  .section-title { font-size: 28px; }
  .about-cta h2 { font-size: 28px; }
}
</style>
