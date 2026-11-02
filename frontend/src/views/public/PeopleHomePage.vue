<script setup lang="ts">
import { computed } from 'vue'
import { edition } from '../../edition'

const live = computed(() => edition.config.countries.filter((c) => c.status === 'live'))
const soon = computed(() => edition.config.countries.filter((c) => c.status !== 'live'))

const pillars = [
  {
    title: 'Hire in one step',
    body: 'Create the employee, salary, leave balances, onboarding checklist, offer letter and app accounts with a single form. New hires finish their own details and sign documents from one link.',
  },
  {
    title: 'Payroll that files itself',
    body: 'Run payroll with SSS, PhilHealth, Pag-IBIG and BIR withholding built in. HalaPeople prepares every statutory return, checks it, and submits it once you approve.',
  },
  {
    title: 'Government portals, handled',
    body: 'Our filing agent logs in to eFPS, My.SSS, EPRS and eSRS for you. It hands you only what a person must do — a CAPTCHA, an OTP, a final yes — and keeps the receipt.',
  },
  {
    title: 'Offboard without loose ends',
    body: 'One action schedules clearance, locks devices, revokes app access on the last day and computes final pay under the Labor Code, ready to release through payroll.',
  },
  {
    title: 'An inbox, not a to-do list',
    body: 'Everything waiting on HR — approvals, filings, OTPs, signatures, device returns — in one place, with an AI brief of what to do first.',
  },
  {
    title: 'AI that asks before it acts',
    body: 'Ask the assistant to hire, offboard or prepare a filing. Every change it proposes waits for your confirmation.',
  },
]

const included = [
  'Employee records & 201 file', 'One-step hire & terminate', 'Self-onboarding portal', 'Offer letters & e-signature',
  'Payroll & payslips', '13th month & final pay', 'Government filing agent', 'Filing calendar autopilot',
  'Leave & overtime', 'Benefits & loans', 'Expense reimbursement', 'Devices & app provisioning',
  'HR inbox with AI brief', 'Accounting sync',
]
</script>

<template>
  <div class="people-home">
    <section class="hero">
      <div class="container">
        <div class="badge">Built for Southeast Asia · Live in the Philippines</div>
        <h1>Payroll, compliance and HR<br /><span class="accent">on autopilot.</span></h1>
        <p class="sub">
          {{ edition.config.product_name }} runs the busywork of employing people — payroll, government filings,
          onboarding and offboarding — so a small team can run HR in minutes a week.
        </p>
        <div class="actions">
          <router-link to="/register" class="btn-primary">Start free</router-link>
          <router-link to="/contact" class="btn-outline">Book a demo</router-link>
        </div>
      </div>
    </section>

    <section class="section">
      <div class="container grid">
        <div v-for="p in pillars" :key="p.title" class="card">
          <h3>{{ p.title }}</h3>
          <p>{{ p.body }}</p>
        </div>
      </div>
    </section>

    <section class="section alt">
      <div class="container">
        <h2>Everything a growing team needs to employ people</h2>
        <ul class="included">
          <li v-for="i in included" :key="i">✓ {{ i }}</li>
        </ul>
      </div>
    </section>

    <section class="section">
      <div class="container">
        <h2>Where we operate</h2>
        <div class="countries">
          <div v-for="c in live" :key="c.code" class="country live"><b>{{ c.name }}</b><span>Live</span></div>
          <div v-for="c in soon" :key="c.code" class="country"><b>{{ c.name }}</b><span>Coming soon</span></div>
        </div>
        <p class="note">
          We only call a country live when payroll, statutory contributions and government filings are fully supported there.
        </p>
      </div>
    </section>

    <section class="section cta">
      <div class="container">
        <h2>Spend your time on people, not paperwork.</h2>
        <router-link to="/register" class="btn-primary">Get started</router-link>
      </div>
    </section>
  </div>
</template>

<style scoped>
.people-home { color: #0f172a; }
.container { max-width: 1120px; margin: 0 auto; padding: 0 20px; }
.hero { padding: 96px 0 72px; background: linear-gradient(180deg, #f0f7ff 0%, #fff 100%); text-align: center; }
.badge { display: inline-block; padding: 6px 14px; border-radius: 999px; background: #e0edff; color: #1d4ed8; font-size: 13px; font-weight: 600; margin-bottom: 20px; }
h1 { font-size: clamp(34px, 5vw, 56px); line-height: 1.1; margin: 0 0 18px; }
.accent { color: #2563eb; }
.sub { font-size: 18px; color: #475569; max-width: 720px; margin: 0 auto 28px; line-height: 1.6; }
.actions { display: flex; gap: 12px; justify-content: center; flex-wrap: wrap; }
.btn-primary, .btn-outline { padding: 12px 22px; border-radius: 10px; font-weight: 600; text-decoration: none; }
.btn-primary { background: #2563eb; color: #fff; }
.btn-outline { border: 1px solid #cbd5e1; color: #0f172a; background: #fff; }
.section { padding: 64px 0; }
.section.alt { background: #f8fafc; }
.section h2 { font-size: 28px; margin: 0 0 24px; text-align: center; }
.grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; }
.card { border: 1px solid #e2e8f0; border-radius: 14px; padding: 24px; background: #fff; }
.card h3 { margin: 0 0 8px; font-size: 18px; }
.card p { margin: 0; color: #475569; line-height: 1.6; }
.included { list-style: none; padding: 0; margin: 0 auto; max-width: 860px; display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap: 10px 24px; }
.included li { color: #334155; }
.countries { display: flex; gap: 12px; justify-content: center; flex-wrap: wrap; }
.country { border: 1px solid #e2e8f0; border-radius: 12px; padding: 12px 18px; display: flex; flex-direction: column; align-items: center; min-width: 130px; color: #64748b; }
.country.live { border-color: #22c55e; color: #15803d; background: #f0fdf4; }
.country b { color: #0f172a; }
.note { text-align: center; color: #64748b; font-size: 14px; margin-top: 16px; }
.cta { text-align: center; background: #0f172a; color: #fff; }
.cta h2 { color: #fff; }
</style>
