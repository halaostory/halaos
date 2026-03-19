<template>
  <div class="legal-page">
    <div class="container">
      <h1>Terms of Service</h1>
      <p class="updated">Last updated: March 1, 2026</p>

      <section>
        <h2>1. Acceptance of Terms</h2>
        <p>By accessing or using HalaOS ("Service"), you agree to be bound by these Terms of Service ("Terms"). If you do not agree, do not use the Service. These Terms apply to all users, including company administrators and employees.</p>
      </section>

      <section>
        <h2>2. Description of Service</h2>
        <p>HalaOS is a cloud-based Human Resources, Payroll, and Tax Compliance platform designed for businesses in Southeast Asia. The Service includes employee management, payroll processing, tax form generation, attendance tracking, and related features.</p>
      </section>

      <section>
        <h2>3. Account Registration</h2>
        <ul>
          <li>You must provide accurate and complete registration information</li>
          <li>You are responsible for maintaining the security of your account credentials</li>
          <li>You must notify us immediately of any unauthorized access</li>
          <li>One company account per legal entity; multiple user accounts per company are permitted</li>
          <li>You must be at least 18 years old to create an account</li>
        </ul>
      </section>

      <section>
        <h2>4. Free and Paid Plans</h2>
        <h3>4.1 Free Plan</h3>
        <p>The Free plan includes core HR, payroll, and tax calculation features at no cost, with no employee limits. Data retention on the Free plan is limited to 1 year of history.</p>
        <h3>4.2 Paid Plans</h3>
        <p>Pro and Enterprise plans offer additional features including auto e-filing, integrations, extended data retention, and priority support. Pricing is per active employee per month. Billing is monthly or annual (with 20% discount for annual).</p>
        <h3>4.3 Changes to Plans</h3>
        <p>We reserve the right to modify plan features and pricing with 30 days' notice. Existing subscribers will be grandfathered at their current rate for 12 months after any price increase.</p>
      </section>

      <section>
        <h2>5. Your Responsibilities</h2>
        <ul>
          <li>Ensure data entered (employee information, salaries, tax IDs) is accurate and lawful</li>
          <li>Comply with all applicable labor laws and tax regulations in your jurisdiction</li>
          <li>Obtain necessary consents from employees before entering their personal data</li>
          <li>Do not use the Service for any illegal purpose</li>
          <li>Do not attempt to gain unauthorized access to our systems or other users' data</li>
        </ul>
      </section>

      <section>
        <h2>6. Data Ownership</h2>
        <p>You retain full ownership of all data you input into HalaOS. We do not claim any intellectual property rights over your content. You grant us a limited license to process your data solely for the purpose of providing the Service.</p>
      </section>

      <section>
        <h2>7. Data Export</h2>
        <p>You may export your data at any time in standard formats (CSV, Excel, PDF). Upon account termination, you will have 30 days to export your data before it is permanently deleted.</p>
      </section>

      <section>
        <h2>8. Disclaimer of Warranties</h2>
        <p>HalaOS provides tax calculation and form generation tools as aids, not as substitutes for professional tax advice. While we strive for accuracy based on the latest tax tables and regulations:</p>
        <ul>
          <li>We do not guarantee the accuracy of tax computations</li>
          <li>We are not responsible for penalties arising from incorrect filings</li>
          <li>We recommend consulting a certified accountant or tax professional for final verification</li>
          <li>The Service is provided "as is" without warranties of any kind</li>
        </ul>
      </section>

      <section>
        <h2>9. Limitation of Liability</h2>
        <p>To the maximum extent permitted by law, HalaOS shall not be liable for any indirect, incidental, special, consequential, or punitive damages, including but not limited to loss of data, revenue, or business opportunities. Our total liability shall not exceed the amount paid by you in the 12 months preceding the claim.</p>
      </section>

      <section>
        <h2>10. Service Availability</h2>
        <p>We target 99.9% uptime but do not guarantee uninterrupted service. Scheduled maintenance will be announced at least 24 hours in advance. We are not liable for service interruptions due to factors beyond our reasonable control.</p>
      </section>

      <section>
        <h2>11. Termination</h2>
        <ul>
          <li>You may terminate your account at any time through account settings</li>
          <li>We may suspend or terminate accounts that violate these Terms</li>
          <li>Upon termination, you will have 30 days to export your data</li>
          <li>Provisions that by their nature should survive termination will survive</li>
        </ul>
      </section>

      <section>
        <h2>12. Governing Law</h2>
        <p>These Terms are governed by the laws of the Republic of the Philippines. Any disputes shall be resolved in the courts of Makati City, Metro Manila.</p>
      </section>

      <section>
        <h2>13. Changes to Terms</h2>
        <p>We may update these Terms from time to time. Material changes will be communicated via email or in-app notification at least 30 days before they take effect. Continued use after changes constitutes acceptance.</p>
      </section>

      <section>
        <h2>14. Contact</h2>
        <p>For questions about these Terms:</p>
        <ul>
          <li>Email: <a href="mailto:legal@halaos.com">legal@halaos.com</a></li>
          <li>Address: Makati City, Metro Manila, Philippines</li>
        </ul>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useHead } from '@unhead/vue'

useHead({
  title: 'Terms of Service - HalaOS',
  meta: [
    { name: 'description', content: 'HalaOS Terms of Service. Read our terms and conditions for using the platform.' },
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
