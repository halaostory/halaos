<template>
  <div class="calc-page">
    <section class="calc-hero">
      <div class="container">
        <router-link to="/tools" class="back-link">Back to Tools</router-link>
        <h1>13th Month Pay Calculator (Philippines)</h1>
        <p>Compute 13th month pay for full-year and pro-rated employees. Based on PD 851 and DOLE guidelines.</p>
      </div>
    </section>

    <section class="calc-section">
      <div class="container">
        <div class="calc-layout">
          <div class="calc-form">
            <div class="form-group">
              <label>Monthly Basic Salary (PHP)</label>
              <input v-model.number="basicSalary" type="number" placeholder="e.g. 25000" min="0" />
            </div>
            <div class="form-group">
              <label>Months Worked This Year</label>
              <select v-model.number="monthsWorked">
                <option v-for="m in 12" :key="m" :value="m">{{ m }} month{{ m > 1 ? 's' : '' }}</option>
              </select>
            </div>
            <div class="form-group">
              <label>Had salary change during the year?</label>
              <div class="toggle-row">
                <button :class="{ active: hasSalaryChange }" @click="hasSalaryChange = true">Yes</button>
                <button :class="{ active: !hasSalaryChange }" @click="hasSalaryChange = false">No</button>
              </div>
            </div>
            <template v-if="hasSalaryChange">
              <div class="form-group">
                <label>Previous Monthly Salary (PHP)</label>
                <input v-model.number="prevSalary" type="number" placeholder="e.g. 20000" min="0" />
              </div>
              <div class="form-group">
                <label>Months at previous salary</label>
                <select v-model.number="prevMonths">
                  <option v-for="m in maxPrevMonths" :key="m" :value="m">{{ m }} month{{ m > 1 ? 's' : '' }}</option>
                </select>
              </div>
            </template>
            <p class="form-note">13th month pay = Total Basic Salary Earned / 12. Only basic pay is included — overtime, holiday pay, allowances, and commissions are excluded.</p>
          </div>

          <div class="calc-result">
            <h3>13th Month Pay Computation</h3>
            <div class="result-rows">
              <template v-if="hasSalaryChange">
                <div class="result-row">
                  <span>Previous salary x {{ prevMonths }} months</span>
                  <span>{{ formatCurrency((prevSalary || 0) * prevMonths) }}</span>
                </div>
                <div class="result-row">
                  <span>Current salary x {{ currentMonths }} months</span>
                  <span>{{ formatCurrency((basicSalary || 0) * currentMonths) }}</span>
                </div>
              </template>
              <template v-else>
                <div class="result-row">
                  <span>Basic salary x {{ monthsWorked }} months</span>
                  <span>{{ formatCurrency((basicSalary || 0) * monthsWorked) }}</span>
                </div>
              </template>
              <div class="result-row divider">
                <span>Total basic salary earned</span>
                <span>{{ formatCurrency(totalEarned) }}</span>
              </div>
              <div class="result-row">
                <span>Divided by 12</span>
                <span>/ 12</span>
              </div>
              <div class="result-row highlight total">
                <span>13th Month Pay</span>
                <span class="total-amount">{{ formatCurrency(thirteenthMonthPay) }}</span>
              </div>
              <div v-if="monthsWorked < 12" class="result-row note">
                <span>Pro-rated ({{ monthsWorked }}/12 months)</span>
                <span></span>
              </div>
            </div>

            <div class="tax-note">
              <strong>Tax treatment:</strong> 13th month pay up to P90,000 is tax-exempt (combined with other benefits). Amount exceeding P90,000 is taxable.
            </div>

            <div class="deadline-note">
              <strong>Deadline:</strong> Must be paid on or before December 24.
            </div>
          </div>
        </div>
      </div>
    </section>

    <section class="calc-cta">
      <div class="container">
        <div class="cta-card">
          <h2>Auto-compute 13th month pay for all employees</h2>
          <p>HalaOS computes 13th month pay automatically — handles pro-rated amounts, salary changes, mid-year hires, and generates payslips.</p>
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
  title: '13th Month Pay Calculator Philippines - HalaOS',
  meta: [
    { name: 'description', content: 'Free 13th month pay calculator for Philippine employers. Compute pro-rated amounts for mid-year hires, handle salary changes, and ensure compliance with PD 851.' },
    { property: 'og:title', content: '13th Month Pay Calculator Philippines - HalaOS' },
    { property: 'og:url', content: 'https://halaos.com/tools/13th-month-calculator' },
    { name: 'keywords', content: '13th month pay calculator, 13th month pay computation philippines, pro rated 13th month pay, PD 851, how to compute 13th month pay' },
  ],
})

const basicSalary = ref(25000)
const monthsWorked = ref(12)
const hasSalaryChange = ref(false)
const prevSalary = ref(20000)
const prevMonths = ref(6)

const maxPrevMonths = computed(() => Math.max(1, monthsWorked.value - 1))
const currentMonths = computed(() => monthsWorked.value - prevMonths.value)

const totalEarned = computed(() => {
  if (hasSalaryChange.value) {
    return ((prevSalary.value || 0) * prevMonths.value) + ((basicSalary.value || 0) * currentMonths.value)
  }
  return (basicSalary.value || 0) * monthsWorked.value
})

const thirteenthMonthPay = computed(() => totalEarned.value / 12)

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
  display: inline-block; color: #4f46e5; font-size: 14px; font-weight: 500;
  text-decoration: none; margin-bottom: 16px;
}
.back-link:hover { text-decoration: underline; }
.calc-hero h1 { font-size: 36px; font-weight: 800; color: #0f172a; margin: 0 0 12px; }
.calc-hero p { font-size: 17px; color: #475569; max-width: 580px; margin: 0 auto; }
.calc-section { padding: 40px 0 60px; }
.calc-layout { display: grid; grid-template-columns: 1fr 1.2fr; gap: 32px; align-items: start; }
.calc-form { background: #fff; border: 1px solid #e2e8f0; border-radius: 12px; padding: 28px; }
.form-group { margin-bottom: 16px; }
.form-group label { display: block; font-size: 13px; font-weight: 600; color: #334155; margin-bottom: 6px; }
.form-group input, .form-group select {
  width: 100%; padding: 10px 14px; border: 1px solid #e2e8f0; border-radius: 8px;
  font-size: 15px; color: #0f172a; box-sizing: border-box; background: #fff;
}
.form-group input:focus, .form-group select:focus {
  outline: none; border-color: #4f46e5; box-shadow: 0 0 0 3px rgba(79,70,229,0.1);
}
.toggle-row { display: flex; gap: 8px; }
.toggle-row button {
  flex: 1; padding: 8px; border: 1px solid #e2e8f0; border-radius: 8px;
  background: #fff; font-size: 14px; color: #475569; cursor: pointer; transition: all 0.2s;
}
.toggle-row button.active { background: #4f46e5; color: #fff; border-color: #4f46e5; }
.form-note { font-size: 13px; color: #94a3b8; line-height: 1.5; margin: 8px 0 0; }
.calc-result { background: #fff; border: 2px solid #4f46e5; border-radius: 12px; padding: 28px; }
.calc-result h3 { font-size: 18px; font-weight: 700; color: #0f172a; margin: 0 0 20px; }
.result-rows { display: flex; flex-direction: column; gap: 10px; }
.result-row { display: flex; justify-content: space-between; font-size: 14px; color: #475569; }
.result-row.divider { padding-top: 12px; margin-top: 4px; border-top: 1px solid #f1f5f9; font-weight: 600; color: #334155; }
.result-row.highlight { font-weight: 600; color: #0f172a; }
.result-row.total { font-size: 16px; padding-top: 8px; }
.result-row.note { font-size: 12px; color: #94a3b8; }
.total-amount { color: #4f46e5; font-size: 20px; }
.tax-note, .deadline-note {
  margin-top: 16px; padding: 10px 14px; background: #fef3c7; border-radius: 8px;
  font-size: 13px; color: #92400e; line-height: 1.5;
}
.deadline-note { background: #dbeafe; color: #1e40af; margin-top: 8px; }
.calc-cta { padding: 60px 0 80px; }
.cta-card {
  background: linear-gradient(135deg, #4f46e5, #7c3aed); border-radius: 16px;
  padding: 40px; text-align: center; color: #fff;
}
.cta-card h2 { font-size: 24px; font-weight: 800; margin: 0 0 10px; }
.cta-card p { font-size: 15px; opacity: 0.9; margin-bottom: 20px; }
.btn-primary {
  display: inline-block; background: #fff; color: #4f46e5; font-weight: 600;
  font-size: 15px; padding: 12px 28px; border-radius: 8px; text-decoration: none;
}
.btn-primary:hover { background: #f1f5f9; }
@media (max-width: 768px) {
  .calc-hero h1 { font-size: 28px; }
  .calc-layout { grid-template-columns: 1fr; }
}
</style>
