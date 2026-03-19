<template>
  <div class="calc-page">
    <section class="calc-hero">
      <div class="container">
        <router-link to="/tools" class="back-link">Back to Tools</router-link>
        <h1>Philippine Income Tax Calculator 2026</h1>
        <p>Compute your monthly or annual withholding tax based on the BIR graduated tax table (TRAIN Law).</p>
      </div>
    </section>

    <section class="calc-section">
      <div class="container">
        <div class="calc-layout">
          <div class="calc-form">
            <div class="form-group">
              <label>Monthly Basic Salary (PHP)</label>
              <input v-model.number="monthlySalary" type="number" placeholder="e.g. 25000" min="0" />
            </div>
            <div class="form-group">
              <label>SSS Contribution (auto)</label>
              <input :value="sssEmployee.toFixed(2)" disabled />
            </div>
            <div class="form-group">
              <label>PhilHealth Contribution (auto)</label>
              <input :value="philhealthEmployee.toFixed(2)" disabled />
            </div>
            <div class="form-group">
              <label>Pag-IBIG Contribution (auto)</label>
              <input :value="pagibigEmployee.toFixed(2)" disabled />
            </div>
            <div class="form-group">
              <label>Non-Taxable Allowances (PHP)</label>
              <input v-model.number="allowances" type="number" placeholder="0" min="0" />
            </div>
          </div>

          <div class="calc-result">
            <h3>Tax Computation Summary</h3>
            <div class="result-rows">
              <div class="result-row">
                <span>Monthly gross salary</span>
                <span>{{ formatCurrency(monthlySalary) }}</span>
              </div>
              <div class="result-row">
                <span>SSS (employee share)</span>
                <span class="deduction">- {{ formatCurrency(sssEmployee) }}</span>
              </div>
              <div class="result-row">
                <span>PhilHealth (employee share)</span>
                <span class="deduction">- {{ formatCurrency(philhealthEmployee) }}</span>
              </div>
              <div class="result-row">
                <span>Pag-IBIG (employee share)</span>
                <span class="deduction">- {{ formatCurrency(pagibigEmployee) }}</span>
              </div>
              <div class="result-row divider">
                <span>Monthly taxable income</span>
                <span>{{ formatCurrency(monthlyTaxable) }}</span>
              </div>
              <div class="result-row highlight">
                <span>Monthly withholding tax</span>
                <span>{{ formatCurrency(monthlyTax) }}</span>
              </div>
              <div class="result-row highlight">
                <span>Monthly net pay (take-home)</span>
                <span class="net-pay">{{ formatCurrency(monthlyNetPay) }}</span>
              </div>
              <div class="result-row divider">
                <span>Annual taxable income</span>
                <span>{{ formatCurrency(annualTaxable) }}</span>
              </div>
              <div class="result-row">
                <span>Annual income tax</span>
                <span>{{ formatCurrency(annualTax) }}</span>
              </div>
              <div class="result-row">
                <span>Effective tax rate</span>
                <span>{{ effectiveRate }}%</span>
              </div>
            </div>
            <div class="result-bracket">
              Tax bracket: {{ taxBracketLabel }}
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- Tax Table Reference -->
    <section class="tax-table-section">
      <div class="container">
        <h2>2026 BIR Annual Tax Table (TRAIN Law)</h2>
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Annual Taxable Income</th>
                <th>Tax Rate</th>
              </tr>
            </thead>
            <tbody>
              <tr><td>P0 - P250,000</td><td>0% (exempt)</td></tr>
              <tr><td>P250,001 - P400,000</td><td>15% of excess over P250,000</td></tr>
              <tr><td>P400,001 - P800,000</td><td>P22,500 + 20% of excess over P400,000</td></tr>
              <tr><td>P800,001 - P2,000,000</td><td>P102,500 + 25% of excess over P800,000</td></tr>
              <tr><td>P2,000,001 - P8,000,000</td><td>P402,500 + 30% of excess over P2,000,000</td></tr>
              <tr><td>Over P8,000,000</td><td>P2,202,500 + 35% of excess over P8,000,000</td></tr>
            </tbody>
          </table>
        </div>
      </div>
    </section>

    <section class="calc-cta">
      <div class="container">
        <div class="cta-card">
          <h2>Automate tax calculations for all employees</h2>
          <p>HalaOS computes withholding tax, SSS, PhilHealth, Pag-IBIG, and generates BIR forms automatically during payroll.</p>
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
  title: 'Philippine Income Tax Calculator 2026 - HalaOS',
  meta: [
    { name: 'description', content: 'Free Philippine income tax calculator for 2026. Compute monthly withholding tax based on the BIR TRAIN Law graduated tax table. Includes SSS, PhilHealth, Pag-IBIG auto-deductions.' },
    { property: 'og:title', content: 'Philippine Income Tax Calculator 2026 - HalaOS' },
    { property: 'og:url', content: 'https://halaos.com/tools/tax-calculator' },
    { name: 'keywords', content: 'income tax calculator philippines 2026, BIR tax calculator, withholding tax philippines, TRAIN law tax table, Philippine tax computation' },
  ],
})

const monthlySalary = ref(25000)
const allowances = ref(0)

// SSS computation (simplified 2026 rates: 14% total, 4.5% employee)
const sssEmployee = computed(() => {
  const salary = monthlySalary.value || 0
  if (salary <= 0) return 0
  const msc = Math.min(Math.max(Math.round(salary / 500) * 500, 4000), 30000)
  return msc * 0.045
})

// PhilHealth (5% total, 2.5% employee, floor 10000, ceiling 100000)
const philhealthEmployee = computed(() => {
  const salary = monthlySalary.value || 0
  if (salary <= 0) return 0
  const base = Math.min(Math.max(salary, 10000), 100000)
  return base * 0.025
})

// Pag-IBIG (2% employee, max MSC 10000)
const pagibigEmployee = computed(() => {
  const salary = monthlySalary.value || 0
  if (salary <= 0) return 0
  if (salary <= 1500) return salary * 0.01
  return Math.min(salary, 10000) * 0.02
})

const monthlyTaxable = computed(() => {
  const salary = monthlySalary.value || 0
  return Math.max(0, salary - sssEmployee.value - philhealthEmployee.value - pagibigEmployee.value - (allowances.value || 0))
})

const annualTaxable = computed(() => monthlyTaxable.value * 12)

// TRAIN Law annual tax brackets
function computeAnnualTax(annual: number): number {
  if (annual <= 250000) return 0
  if (annual <= 400000) return (annual - 250000) * 0.15
  if (annual <= 800000) return 22500 + (annual - 400000) * 0.20
  if (annual <= 2000000) return 102500 + (annual - 800000) * 0.25
  if (annual <= 8000000) return 402500 + (annual - 2000000) * 0.30
  return 2202500 + (annual - 8000000) * 0.35
}

const annualTax = computed(() => computeAnnualTax(annualTaxable.value))
const monthlyTax = computed(() => annualTax.value / 12)

const monthlyNetPay = computed(() => {
  const salary = monthlySalary.value || 0
  return salary - sssEmployee.value - philhealthEmployee.value - pagibigEmployee.value - monthlyTax.value
})

const effectiveRate = computed(() => {
  if (annualTaxable.value <= 0) return '0.00'
  return ((annualTax.value / annualTaxable.value) * 100).toFixed(2)
})

const taxBracketLabel = computed(() => {
  const annual = annualTaxable.value
  if (annual <= 250000) return 'Exempt (0%)'
  if (annual <= 400000) return '15%'
  if (annual <= 800000) return '20%'
  if (annual <= 2000000) return '25%'
  if (annual <= 8000000) return '30%'
  return '35%'
})

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
.calc-hero h1 {
  font-size: 36px;
  font-weight: 800;
  color: #0f172a;
  margin: 0 0 12px;
}
.calc-hero p {
  font-size: 17px;
  color: #475569;
  max-width: 560px;
  margin: 0 auto;
}
.calc-section { padding: 40px 0 60px; }
.calc-layout {
  display: grid;
  grid-template-columns: 1fr 1.2fr;
  gap: 32px;
  align-items: start;
}
.calc-form {
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  padding: 28px;
}
.form-group {
  margin-bottom: 16px;
}
.form-group label {
  display: block;
  font-size: 13px;
  font-weight: 600;
  color: #334155;
  margin-bottom: 6px;
}
.form-group input {
  width: 100%;
  padding: 10px 14px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  font-size: 15px;
  color: #0f172a;
  background: #fff;
  box-sizing: border-box;
}
.form-group input:disabled {
  background: #f8fafc;
  color: #64748b;
}
.form-group input:focus {
  outline: none;
  border-color: #4f46e5;
  box-shadow: 0 0 0 3px rgba(79,70,229,0.1);
}
.calc-result {
  background: #fff;
  border: 2px solid #4f46e5;
  border-radius: 12px;
  padding: 28px;
}
.calc-result h3 {
  font-size: 18px;
  font-weight: 700;
  color: #0f172a;
  margin: 0 0 20px;
}
.result-rows { display: flex; flex-direction: column; gap: 10px; }
.result-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 14px;
  color: #475569;
}
.result-row.divider {
  padding-top: 12px;
  margin-top: 4px;
  border-top: 1px solid #f1f5f9;
  font-weight: 600;
  color: #334155;
}
.result-row.highlight {
  font-weight: 600;
  color: #0f172a;
  font-size: 15px;
}
.deduction { color: #ef4444; }
.net-pay { color: #16a34a; font-size: 18px; }
.result-bracket {
  margin-top: 16px;
  padding: 10px 14px;
  background: #eef2ff;
  border-radius: 8px;
  font-size: 13px;
  color: #4f46e5;
  font-weight: 500;
  text-align: center;
}
.tax-table-section {
  padding: 60px 0;
  background: #f8fafc;
}
.tax-table-section h2 {
  font-size: 24px;
  font-weight: 700;
  color: #0f172a;
  margin: 0 0 24px;
  text-align: center;
}
.table-wrap { overflow-x: auto; }
.table-wrap table {
  width: 100%;
  border-collapse: collapse;
  background: #fff;
  border-radius: 8px;
  overflow: hidden;
}
.table-wrap th, .table-wrap td {
  padding: 12px 16px;
  border-bottom: 1px solid #f1f5f9;
  font-size: 14px;
  text-align: left;
}
.table-wrap th {
  background: #f8fafc;
  font-weight: 600;
  color: #0f172a;
}
.calc-cta { padding: 60px 0 80px; }
.cta-card {
  background: linear-gradient(135deg, #4f46e5, #7c3aed);
  border-radius: 16px;
  padding: 40px;
  text-align: center;
  color: #fff;
}
.cta-card h2 { font-size: 24px; font-weight: 800; margin: 0 0 10px; }
.cta-card p { font-size: 15px; opacity: 0.9; margin-bottom: 20px; }
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
  .calc-hero h1 { font-size: 28px; }
  .calc-layout { grid-template-columns: 1fr; }
}
</style>
