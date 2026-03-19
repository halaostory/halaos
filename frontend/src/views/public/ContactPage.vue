<template>
  <div class="contact-page">
    <!-- Hero -->
    <section class="contact-hero">
      <div class="container">
        <h1>Get in touch</h1>
        <p>Have a question? We'd love to hear from you. Send us a message and we'll respond as soon as possible.</p>
      </div>
    </section>

    <!-- Content -->
    <section class="contact-content">
      <div class="container">
        <div class="contact-grid">
          <!-- Form -->
          <div class="contact-form-wrap">
            <form v-if="!submitted" @submit.prevent="handleSubmit" class="contact-form">
              <div class="form-row">
                <div class="form-group">
                  <label>First Name</label>
                  <input v-model="form.firstName" type="text" required placeholder="John" />
                </div>
                <div class="form-group">
                  <label>Last Name</label>
                  <input v-model="form.lastName" type="text" required placeholder="Doe" />
                </div>
              </div>
              <div class="form-group">
                <label>Email</label>
                <input v-model="form.email" type="email" required placeholder="john@company.com" />
              </div>
              <div class="form-group">
                <label>Company</label>
                <input v-model="form.company" type="text" placeholder="Your company name" />
              </div>
              <div class="form-group">
                <label>Subject</label>
                <select v-model="form.subject">
                  <option value="general">General Inquiry</option>
                  <option value="sales">Sales / Pricing</option>
                  <option value="support">Technical Support</option>
                  <option value="partnership">Partnership</option>
                  <option value="other">Other</option>
                </select>
              </div>
              <div class="form-group">
                <label>Message</label>
                <textarea v-model="form.message" required rows="5" placeholder="Tell us how we can help..."></textarea>
              </div>
              <button type="submit" class="btn-primary" :disabled="sending">
                {{ sending ? 'Sending...' : 'Send Message' }}
              </button>
            </form>

            <div v-else class="success-msg">
              <div class="success-icon">✓</div>
              <h3>Message sent!</h3>
              <p>Thank you for reaching out. We'll get back to you within 24 hours.</p>
              <button class="btn-outline" @click="resetForm">Send Another Message</button>
            </div>
          </div>

          <!-- Info -->
          <div class="contact-info">
            <div class="info-card">
              <h3>Contact Information</h3>
              <div class="info-items">
                <div class="info-item">
                  <span class="info-icon">📧</span>
                  <div>
                    <strong>Email</strong>
                    <p><a href="mailto:hello@halaos.com">hello@halaos.com</a></p>
                  </div>
                </div>
                <div class="info-item">
                  <span class="info-icon">💬</span>
                  <div>
                    <strong>Sales</strong>
                    <p><a href="mailto:sales@halaos.com">sales@halaos.com</a></p>
                  </div>
                </div>
                <div class="info-item">
                  <span class="info-icon">🛠️</span>
                  <div>
                    <strong>Support</strong>
                    <p><a href="mailto:support@halaos.com">support@halaos.com</a></p>
                  </div>
                </div>
              </div>
            </div>

            <div class="info-card">
              <h3>Office Hours</h3>
              <p class="office-hours">Monday - Friday: 9:00 AM - 6:00 PM (GMT+8)</p>
              <p class="office-note">We typically respond within 24 hours during business days.</p>
            </div>

            <div class="info-card">
              <h3>Offices</h3>
              <div class="offices">
                <div class="office">
                  <strong>Philippines (HQ)</strong>
                  <p>Makati City, Metro Manila</p>
                </div>
                <div class="office">
                  <strong>Singapore</strong>
                  <p>One Raffles Place</p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'

const sending = ref(false)
const submitted = ref(false)

const form = reactive({
  firstName: '',
  lastName: '',
  email: '',
  company: '',
  subject: 'general',
  message: '',
})

async function handleSubmit() {
  sending.value = true
  try {
    const res = await fetch('/api/v1/contact', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        first_name: form.firstName,
        last_name: form.lastName,
        email: form.email,
        company: form.company,
        subject: form.subject,
        message: form.message,
      }),
    })
    if (!res.ok) throw new Error('Failed to send')
    submitted.value = true
  } catch {
    alert('Failed to send message. Please try again or email us directly.')
  } finally {
    sending.value = false
  }
}

function resetForm() {
  form.firstName = ''
  form.lastName = ''
  form.email = ''
  form.company = ''
  form.subject = 'general'
  form.message = ''
  submitted.value = false
}
</script>

<style scoped>
.container {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 24px;
}

.contact-hero {
  padding: 100px 0 60px;
  text-align: center;
  background: linear-gradient(180deg, #f8fafc 0%, #fff 100%);
}
.contact-hero h1 {
  font-size: 48px;
  font-weight: 800;
  color: #0f172a;
  margin: 0 0 16px;
}
.contact-hero p {
  font-size: 18px;
  color: #475569;
  max-width: 560px;
  margin: 0 auto;
}

.contact-content {
  padding: 0 0 80px;
}
.contact-grid {
  display: grid;
  grid-template-columns: 1.2fr 0.8fr;
  gap: 48px;
  align-items: start;
}

.contact-form {
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 16px;
  padding: 36px;
}
.form-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}
.form-group {
  margin-bottom: 20px;
}
.form-group label {
  display: block;
  font-size: 14px;
  font-weight: 600;
  color: #334155;
  margin-bottom: 6px;
}
.form-group input,
.form-group select,
.form-group textarea {
  width: 100%;
  padding: 10px 14px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  font-size: 14px;
  color: #0f172a;
  background: #fff;
  transition: border-color 0.2s;
  font-family: inherit;
  box-sizing: border-box;
}
.form-group input:focus,
.form-group select:focus,
.form-group textarea:focus {
  outline: none;
  border-color: #4f46e5;
  box-shadow: 0 0 0 3px rgba(79, 70, 229, 0.1);
}
.form-group textarea {
  resize: vertical;
}

.btn-primary {
  display: inline-block;
  background: #4f46e5;
  color: #fff;
  font-weight: 600;
  font-size: 15px;
  padding: 12px 28px;
  border-radius: 8px;
  border: none;
  cursor: pointer;
  transition: background 0.2s;
  text-decoration: none;
}
.btn-primary:hover { background: #4338ca; }
.btn-primary:disabled { opacity: 0.6; cursor: not-allowed; }

.btn-outline {
  display: inline-block;
  border: 1.5px solid #e2e8f0;
  background: none;
  color: #334155;
  font-weight: 600;
  font-size: 15px;
  padding: 10px 24px;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
}
.btn-outline:hover { border-color: #4f46e5; color: #4f46e5; }

.success-msg {
  text-align: center;
  padding: 48px 36px;
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 16px;
}
.success-icon {
  width: 56px;
  height: 56px;
  background: #dcfce7;
  color: #16a34a;
  font-size: 28px;
  font-weight: 700;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 0 auto 16px;
}
.success-msg h3 {
  font-size: 22px;
  font-weight: 700;
  color: #0f172a;
  margin: 0 0 8px;
}
.success-msg p {
  color: #475569;
  margin-bottom: 24px;
}

.info-card {
  background: #f8fafc;
  border-radius: 12px;
  padding: 24px;
  margin-bottom: 20px;
}
.info-card h3 {
  font-size: 16px;
  font-weight: 700;
  color: #0f172a;
  margin: 0 0 16px;
}
.info-items {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.info-item {
  display: flex;
  gap: 12px;
  align-items: flex-start;
}
.info-icon { font-size: 20px; }
.info-item strong {
  display: block;
  font-size: 13px;
  color: #64748b;
  font-weight: 500;
}
.info-item p {
  margin: 2px 0 0;
  font-size: 14px;
}
.info-item a {
  color: #4f46e5;
  text-decoration: none;
  font-weight: 500;
}
.office-hours {
  font-size: 14px;
  color: #334155;
  font-weight: 500;
  margin: 0 0 8px;
}
.office-note {
  font-size: 13px;
  color: #94a3b8;
  margin: 0;
}
.offices {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.office strong {
  font-size: 14px;
  color: #334155;
}
.office p {
  font-size: 13px;
  color: #64748b;
  margin: 2px 0 0;
}

@media (max-width: 768px) {
  .contact-hero h1 { font-size: 32px; }
  .contact-grid { grid-template-columns: 1fr; }
  .form-row { grid-template-columns: 1fr; }
}
</style>
