<template>
  <div class="calc-page">
    <section class="calc-hero">
      <div class="container">
        <router-link to="/tools" class="back-link">Back to Tools</router-link>
        <h1>SSS Contribution Calculator 2026</h1>
        <p>Calculate SSS employee and employer shares based on the 2026 contribution table. Includes Mandatory Provident Fund (MPF).</p>
      </div>
    </section>

    <section class="calc-section">
      <div class="container">
        <div class="calc-layout">
          <div class="calc-form">
            <div class="form-group">
              <label>Monthly Salary (PHP)</label>
              <input v-model.number="salary" type="number" placeholder="e.g. 25000" min="0" />
            </div>
            <p class="form-note">Enter the total monthly compensation including basic pay and allowances. The system will find the correct Monthly Salary Credit (MSC) bracket.</p>
          </div>

          <div class="calc-result">
            <h3>SSS Contribution Breakdown</h3>
            <div class="result-rows">
              <div class="result-row">
                <span>Monthly salary</span>
                <span>{{ formatCurrency(salary || 0) }}</span>
              </div>
              <div class="result-row divider">
                <span>Monthly Salary Credit (MSC)</span>
                <span>{{ formatCurrency(msc) }}</span>
              </div>
              <div class="result-row sub-header">
                <span>Regular SSS (14% of MSC up to P20,000)</span>
                <span></span>
              </div>
              <div class="result-row">
                <span>&nbsp;&nbsp;Employee share (4.5%)</span>
                <span>{{ formatCurrency(regularEE) }}</span>
              </div>
              <div class="result-row">
                <span>&nbsp;&nbsp;Employer share (9.5%)</span>
                <span>{{ formatCurrency(regularER) }}</span>
              </div>
              <div v-if="msc > 20000" class="result-row sub-header">
                <span>MPF (on MSC above P20,000)</span>
                <span></span>
              </div>
              <div v-if="msc > 20000" class="result-row">
                <span>&nbsp;&nbsp;Employee MPF share</span>
                <span>{{ formatCurrency(mpfEE) }}</span>
              </div>
              <div v-if="msc > 20000" class="result-row">
                <span>&nbsp;&nbsp;Employer MPF share</span>
                <span>{{ formatCurrency(mpfER) }}</span>
              </div>
              <div class="result-row divider highlight">
                <span>Total Employee Deduction</span>
                <span>{{ formatCurrency(totalEE) }}</span>
              </div>
              <div class="result-row highlight">
                <span>Total Employer Share</span>
                <span>{{ formatCurrency(totalER) }}</span>
              </div>
              <div class="result-row highlight total">
                <span>Total Monthly Contribution</span>
                <span class="total-amount">{{ formatCurrency(totalContribution) }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- SSS Table -->
    <section class="ref-section">
      <div class="container">
        <h2>2026 SSS Contribution Table (Key Brackets)</h2>
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Salary Range</th>
                <th>MSC</th>
                <th>EE (4.5%)</th>
                <th>ER (9.5%)</th>
                <th>Total</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in tableRows" :key="row.msc">
                <td>{{ row.range }}</td>
                <td>{{ formatCurrency(row.msc) }}</td>
                <td>{{ formatCurrency(row.msc * 0.045) }}</td>
                <td>{{ formatCurrency(row.msc * 0.095) }}</td>
                <td>{{ formatCurrency(row.msc * 0.14) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </section>

    <section class="calc-cta">
      <div class="container">
        <div class="cta-card">
          <h2>Auto-compute SSS for all employees during payroll</h2>
          <p>HalaOS automatically deducts SSS, PhilHealth, Pag-IBIG, and withholding tax. Free for unlimited employees.</p>
          <router-link to="/register" class="btn-primary">Get Started Free</router-link>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useHead } from '@unhead/vue'

useHead({
  title: 'SSS Contribution Calculator 2026 - HalaOS',
  meta: [
    { name: 'description', content: 'Free SSS contribution calculator for 2026. Compute employee and employer shares based on the latest SSS contribution table. Includes MPF (Mandatory Provident Fund).' },
    { property: 'og:title', content: 'SSS Contribution Calculator 2026 - HalaOS' },
    { property: 'og:url', content: 'https://halaos.com/tools/sss-calculator' },
    { name: 'keywords', content: 'SSS contribution table 2026, SSS calculator, SSS employee share, SSS employer share, MPF calculator, Philippine SSS' },
  ],
})

const salary = ref(25000)

const msc = computed(() => {
  const s = salary.value || 0
  if (s <= 0) return 0
  const rounded = Math.round(s / 500) * 500
  return Math.min(Math.max(rounded, 4000), 30000)
})

// Regular SSS: applies on MSC up to 20000
const regularBase = computed(() => Math.min(msc.value, 20000))
const regularEE = computed(() => regularBase.value * 0.045)
const regularER = computed(() => regularBase.value * 0.095)

// MPF: applies on MSC above 20000 (split equally)
const mpfBase = computed(() => Math.max(0, msc.value - 20000))
const mpfEE = computed(() => mpfBase.value * 0.05) // 5% each
const mpfER = computed(() => mpfBase.value * 0.05)

const totalEE = computed(() => regularEE.value + mpfEE.value)
const totalER = computed(() => regularER.value + mpfER.value)
const totalContribution = computed(() => totalEE.value + totalER.value)

const tableRows = [
  { range: 'P4,000 and below', msc: 4000 },
  { range: 'P4,001 - P4,749', msc: 4500 },
  { range: 'P4,750 - P5,249', msc: 5000 },
  { range: 'P7,250 - P7,749', msc: 7500 },
  { range: 'P9,750 - P10,249', msc: 10000 },
  { range: 'P14,750 - P15,249', msc: 15000 },
  { range: 'P19,750 - P20,249', msc: 20000 },
  { range: 'P24,750 - P25,249', msc: 25000 },
  { range: 'P29,750+', msc: 30000 },
]

function formatCurrency(val: number): string {
  return 'P' + val.toLocaleString('en-PH', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}
</script>

<style scoped>
.container { max-width: 1000px; margin: 0 auto; padding: 0 24px; }
.calc-hero {
  padding: 80px 0 32px;
  text-align: center;
  background: linear-gradient(180deg, #f8fafc 0%, #fff 100%);
}
.back-link {
  display: inline-block;
  color: #4f46e5;
  font-size: 14px;
  font-weight: 500;
  text-decoration: none;
  margin-bottom: 16px;
}
.back-link:hover { text-decoration: underline; }
.calc-hero h1 { font-size: 36px; font-weight: 800; color: #0f172a; margin: 0 0 12px; }
.calc-hero p { font-size: 17px; color: #475569; max-width: 580px; margin: 0 auto; }
.calc-section { padding: 40px 0 60px; }
.calc-layout { display: grid; grid-template-columns: 1fr 1.3fr; gap: 32px; align-items: start; }
.calc-form { background: #fff; border: 1px solid #e2e8f0; border-radius: 12px; padding: 28px; }
.form-group { margin-bottom: 16px; }
.form-group label { display: block; font-size: 13px; font-weight: 600; color: #334155; margin-bottom: 6px; }
.form-group input {
  width: 100%; padding: 10px 14px; border: 1px solid #e2e8f0; border-radius: 8px;
  font-size: 15px; color: #0f172a; box-sizing: border-box;
}
.form-group input:focus { outline: none; border-color: #4f46e5; box-shadow: 0 0 0 3px rgba(79,70,229,0.1); }
.form-note { font-size: 13px; color: #94a3b8; line-height: 1.5; margin: 0; }
.calc-result { background: #fff; border: 2px solid #4f46e5; border-radius: 12px; padding: 28px; }
.calc-result h3 { font-size: 18px; font-weight: 700; color: #0f172a; margin: 0 0 20px; }
.result-rows { display: flex; flex-direction: column; gap: 8px; }
.result-row { display: flex; justify-content: space-between; font-size: 14px; color: #475569; }
.result-row.divider { padding-top: 12px; margin-top: 4px; border-top: 1px solid #f1f5f9; font-weight: 600; color: #334155; }
.result-row.highlight { font-weight: 600; color: #0f172a; }
.result-row.total { font-size: 16px; padding-top: 8px; }
.result-row.sub-header { font-size: 12px; color: #94a3b8; text-transform: uppercase; letter-spacing: 0.03em; margin-top: 8px; }
.total-amount { color: #4f46e5; }
.ref-section { padding: 60px 0; background: #f8fafc; }
.ref-section h2 { font-size: 24px; font-weight: 700; color: #0f172a; margin: 0 0 24px; text-align: center; }
.table-wrap { overflow-x: auto; }
.table-wrap table { width: 100%; border-collapse: collapse; background: #fff; border-radius: 8px; overflow: hidden; }
.table-wrap th, .table-wrap td { padding: 12px 16px; border-bottom: 1px solid #f1f5f9; font-size: 14px; text-align: left; }
.table-wrap th { background: #f8fafc; font-weight: 600; color: #0f172a; }
.calc-cta { padding: 60px 0 80px; }
.cta-card { background: linear-gradient(135deg, #4f46e5, #7c3aed); border-radius: 16px; padding: 40px; text-align: center; color: #fff; }
.cta-card h2 { font-size: 24px; font-weight: 800; margin: 0 0 10px; }
.cta-card p { font-size: 15px; opacity: 0.9; margin-bottom: 20px; }
.btn-primary { display: inline-block; background: #fff; color: #4f46e5; font-weight: 600; font-size: 15px; padding: 12px 28px; border-radius: 8px; text-decoration: none; }
.btn-primary:hover { background: #f1f5f9; }
@media (max-width: 768px) {
  .calc-hero h1 { font-size: 28px; }
  .calc-layout { grid-template-columns: 1fr; }
}
</style>
