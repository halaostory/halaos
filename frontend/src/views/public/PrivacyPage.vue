<template>
  <div class="legal-page">
    <div class="container">
      <h1>Privacy Policy</h1>
      <p class="updated">Last updated: March 1, 2026</p>

      <section>
        <h2>1. Introduction</h2>
        <p>HalaOS ("we", "us", "our") is committed to protecting your privacy. This Privacy Policy explains how we collect, use, disclose, and safeguard your information when you use our platform at halaos.com and related services.</p>
      </section>

      <section>
        <h2>2. Information We Collect</h2>
        <h3>2.1 Account Information</h3>
        <p>When you register, we collect your name, email address, company name, and country of operation.</p>
        <h3>2.2 Employee Data</h3>
        <p>As an HR platform, you may input employee information including names, contact details, government IDs (TIN, SSS, PhilHealth, Pag-IBIG, CPF numbers), salary information, and employment records. This data is stored securely and processed solely for providing HR and payroll services.</p>
        <h3>2.3 Usage Data</h3>
        <p>We automatically collect information about how you interact with our platform, including pages visited, features used, and browser/device information.</p>
        <h3>2.4 Cookies</h3>
        <p>We use essential cookies for authentication and session management. We use Cloudflare analytics for aggregate traffic analysis. No third-party advertising cookies are used.</p>
      </section>

      <section>
        <h2>3. How We Use Your Information</h2>
        <ul>
          <li>Provide, maintain, and improve our HR, payroll, and tax compliance services</li>
          <li>Process payroll calculations and generate tax forms</li>
          <li>Send transactional emails (verification, payslips, compliance reminders)</li>
          <li>Provide customer support</li>
          <li>Detect and prevent fraud or security issues</li>
          <li>Comply with legal obligations</li>
        </ul>
      </section>

      <section>
        <h2>4. Data Storage & Security</h2>
        <p>Your data is stored on secure cloud infrastructure with the following protections:</p>
        <ul>
          <li><strong>Encryption at rest</strong>: AES-256 encryption for all stored data</li>
          <li><strong>Encryption in transit</strong>: TLS 1.2/1.3 for all data transmission</li>
          <li><strong>Access control</strong>: Role-based access control (RBAC) with audit logging</li>
          <li><strong>Backups</strong>: Daily automated backups with 30-day retention</li>
          <li><strong>Infrastructure</strong>: Hosted on AWS with SOC 2 compliant data centers</li>
        </ul>
      </section>

      <section>
        <h2>5. Data Sharing</h2>
        <p>We do not sell your data. We may share information only in these circumstances:</p>
        <ul>
          <li><strong>Service providers</strong>: Email delivery (Resend), cloud hosting (AWS), CDN (Cloudflare) — bound by data processing agreements</li>
          <li><strong>Legal requirements</strong>: When required by law, court order, or government regulation</li>
          <li><strong>Integration partners</strong>: Only when you explicitly connect third-party services (e.g., accounting integration)</li>
        </ul>
      </section>

      <section>
        <h2>6. Data Retention</h2>
        <p>We retain your data for as long as your account is active. Free tier accounts retain data for 1 year of history. Upon account deletion, all data is permanently removed within 30 days, except where retention is required by law (e.g., tax records may be retained for the statutory period).</p>
      </section>

      <section>
        <h2>7. Your Rights</h2>
        <p>You have the right to:</p>
        <ul>
          <li><strong>Access</strong>: Request a copy of your data at any time</li>
          <li><strong>Correction</strong>: Update or correct inaccurate information</li>
          <li><strong>Deletion</strong>: Request deletion of your account and associated data</li>
          <li><strong>Export</strong>: Export your data in standard formats (CSV, Excel, PDF)</li>
          <li><strong>Restrict processing</strong>: Request limitation of data processing</li>
        </ul>
        <p>To exercise these rights, contact us at <a href="mailto:privacy@halaos.com">privacy@halaos.com</a>.</p>
      </section>

      <section>
        <h2>8. Children's Privacy</h2>
        <p>HalaOS is not intended for individuals under 18. We do not knowingly collect personal information from children.</p>
      </section>

      <section>
        <h2>9. International Data Transfers</h2>
        <p>Your data may be processed in countries where our servers are located (Singapore, United States). We ensure appropriate safeguards are in place for cross-border data transfers.</p>
      </section>

      <section>
        <h2>10. Changes to This Policy</h2>
        <p>We may update this Privacy Policy from time to time. We will notify you of any material changes via email or in-app notification. Continued use of the platform after changes constitutes acceptance.</p>
      </section>

      <section>
        <h2>11. Contact Us</h2>
        <p>For privacy-related inquiries:</p>
        <ul>
          <li>Email: <a href="mailto:privacy@halaos.com">privacy@halaos.com</a></li>
          <li>Address: Makati City, Metro Manila, Philippines</li>
        </ul>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useHead } from '@unhead/vue'

useHead({
  title: 'Privacy Policy - HalaOS',
  meta: [
    { name: 'description', content: 'HalaOS Privacy Policy. Learn how we collect, use, and protect your data.' },
  ],
})
</script>

<style scoped>
.legal-page {
  padding: 100px 0 80px;
}
.container {
  max-width: 780px;
  margin: 0 auto;
  padding: 0 24px;
}
h1 {
  font-size: 40px;
  font-weight: 800;
  color: #0f172a;
  margin: 0 0 8px;
  letter-spacing: -0.5px;
}
.updated {
  font-size: 14px;
  color: #94a3b8;
  margin-bottom: 40px;
}
section {
  margin-bottom: 32px;
}
h2 {
  font-size: 22px;
  font-weight: 700;
  color: #0f172a;
  margin: 0 0 12px;
}
h3 {
  font-size: 17px;
  font-weight: 600;
  color: #1e293b;
  margin: 16px 0 8px;
}
p {
  font-size: 15px;
  color: #475569;
  line-height: 1.7;
  margin: 0 0 12px;
}
ul {
  padding-left: 20px;
  margin: 0 0 12px;
}
li {
  font-size: 15px;
  color: #475569;
  line-height: 1.7;
  margin-bottom: 6px;
}
a {
  color: #4f46e5;
  text-decoration: none;
}
a:hover {
  text-decoration: underline;
}

@media (max-width: 768px) {
  h1 { font-size: 28px; }
}
</style>
