<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";
import {
  NCard,
  NGrid,
  NGi,
  NDescriptions,
  NDescriptionsItem,
  NTag,
  NSpace,
  NProgress,
  NEmpty,
  NSpin,
  NList,
  NListItem,
  NThing,
  NButton,
} from "naive-ui";
import { selfServiceAPI, leaveAPI, loanAPI } from "../api/client";
import { useCurrency } from "../composables/useCurrency";

const { t } = useI18n();
const router = useRouter();
const { formatCurrency } = useCurrency();

const loading = ref(true);
const employee = ref<Record<string, unknown> | null>(null);
const profile = ref<Record<string, unknown> | null>(null);
const team = ref<Record<string, unknown>[]>([]);
const directReports = ref<Record<string, unknown>[]>([]);
const manager = ref<Record<string, unknown> | null>(null);
const salary = ref<Record<string, unknown> | null>(null);
const latestPayslip = ref<Record<string, unknown> | null>(null);
const onboardingTasks = ref<Record<string, unknown>[]>([]);
const onboardingProgress = ref<Record<string, unknown> | null>(null);
const leaveBalances = ref<Record<string, unknown>[]>([]);
const activeLoans = ref<Record<string, unknown>[]>([]);

function fmtDate(d: unknown): string {
  if (!d) return "-";
  const s = String(d);
  return s.length >= 10 ? s.substring(0, 10) : s;
}

const taskStatusColor: Record<string, "default" | "info" | "success" | "warning"> = {
  pending: "default",
  in_progress: "info",
  completed: "success",
  skipped: "warning",
};

onMounted(async () => {
  try {
    const [infoRes, teamRes, compRes, obRes, balRes, loanRes] = await Promise.allSettled([
      selfServiceAPI.getMyInfo(),
      selfServiceAPI.getMyTeam(),
      selfServiceAPI.getMyCompensation(),
      selfServiceAPI.getMyOnboarding(),
      leaveAPI.getBalances(),
      loanAPI.listMy(),
    ]);

    if (infoRes.status === "fulfilled") {
      const d = (infoRes.value as { data?: Record<string, unknown> }).data ||
        (infoRes.value as Record<string, unknown>);
      employee.value = (d.employee as Record<string, unknown>) || null;
      profile.value = (d.profile as Record<string, unknown>) || null;
    }

    if (teamRes.status === "fulfilled") {
      const d = (teamRes.value as { data?: Record<string, unknown> }).data ||
        (teamRes.value as Record<string, unknown>);
      team.value = (d.team as Record<string, unknown>[]) || [];
      directReports.value = (d.direct_reports as Record<string, unknown>[]) || [];
      manager.value = (d.manager as Record<string, unknown>) || null;
    }

    if (compRes.status === "fulfilled") {
      const d = (compRes.value as { data?: Record<string, unknown> }).data ||
        (compRes.value as Record<string, unknown>);
      salary.value = (d.salary as Record<string, unknown>) || null;
      latestPayslip.value = (d.latest_payslip as Record<string, unknown>) || null;
    }

    if (obRes.status === "fulfilled") {
      const d = (obRes.value as { data?: Record<string, unknown> }).data ||
        (obRes.value as Record<string, unknown>);
      onboardingTasks.value = (d.tasks as Record<string, unknown>[]) || [];
      onboardingProgress.value = (d.progress as Record<string, unknown>) || null;
    }

    if (balRes.status === "fulfilled") {
      const d = (balRes.value as { data?: Record<string, unknown>[] }).data ||
        (Array.isArray(balRes.value) ? balRes.value : []);
      leaveBalances.value = d as Record<string, unknown>[];
    }

    if (loanRes.status === "fulfilled") {
      const d = (loanRes.value as { data?: Record<string, unknown>[] }).data ||
        (Array.isArray(loanRes.value) ? loanRes.value : []);
      activeLoans.value = d as Record<string, unknown>[];
    }
  } finally {
    loading.value = false;
  }
});
</script>

<template>
  <NSpin :show="loading">
    <NSpace vertical :size="16">
      <h2>{{ t("selfService.title") }}</h2>

      <!-- Quick Actions -->
      <NCard :title="t('selfService.quickActions')" size="small" style="margin-bottom: 4px;">
        <NSpace>
          <NButton type="primary" @click="router.push({ name: 'attendance' })">{{ t('selfService.goAttendance') }}</NButton>
          <NButton @click="router.push({ name: 'leaves' })">{{ t('selfService.goLeave') }}</NButton>
          <NButton @click="router.push({ name: 'overtime' })">{{ t('selfService.goOvertime') }}</NButton>
          <NButton @click="router.push({ name: 'expenses' })">{{ t('selfService.goExpenses') }}</NButton>
          <NButton @click="router.push({ name: 'payslips' })">{{ t('selfService.goPayslips') }}</NButton>
          <NButton @click="router.push({ name: 'training' })">{{ t('selfService.goTraining') }}</NButton>
        </NSpace>
      </NCard>

      <NGrid :cols="2" :x-gap="16" :y-gap="16" responsive="screen" :item-responsive="true">
        <!-- My Info -->
        <NGi span="2 m:1">
          <NCard :title="t('selfService.myInfo')" size="small">
            <template v-if="employee">
              <NDescriptions label-placement="left" :column="1" bordered size="small">
                <NDescriptionsItem :label="t('employee.employeeNo')">
                  {{ employee.employee_no }}
                </NDescriptionsItem>
                <NDescriptionsItem :label="t('employee.name')">
                  {{ employee.first_name }} {{ employee.last_name }}
                </NDescriptionsItem>
                <NDescriptionsItem :label="t('employee.department')">
                  {{ employee.department_name || "-" }}
                </NDescriptionsItem>
                <NDescriptionsItem :label="t('employee.position')">
                  {{ employee.position_title || "-" }}
                </NDescriptionsItem>
                <NDescriptionsItem :label="t('employee.hireDate')">
                  {{ fmtDate(employee.hire_date) }}
                </NDescriptionsItem>
                <NDescriptionsItem :label="t('employee.employmentType')">
                  {{ employee.employment_type }}
                </NDescriptionsItem>
                <NDescriptionsItem :label="t('common.status')">
                  <NTag :type="employee.status === 'active' ? 'success' : 'default'" size="small">
                    {{ employee.status }}
                  </NTag>
                </NDescriptionsItem>
                <NDescriptionsItem v-if="employee.manager_first_name" :label="t('selfService.manager')">
                  {{ employee.manager_first_name }} {{ employee.manager_last_name }}
                </NDescriptionsItem>
              </NDescriptions>
            </template>
            <NEmpty v-else :description="t('employee.notFound')" />
          </NCard>
        </NGi>

        <!-- Compensation -->
        <NGi span="2 m:1">
          <NCard :title="t('selfService.myCompensation')" size="small">
            <template v-if="salary">
              <NDescriptions label-placement="left" :column="1" bordered size="small" style="margin-bottom: 12px">
                <NDescriptionsItem :label="t('selfService.currentSalary')">
                  {{ formatCurrency(salary.basic_salary) }}
                </NDescriptionsItem>
                <NDescriptionsItem v-if="salary.structure_name" :label="t('selfService.structure')">
                  {{ salary.structure_name }}
                </NDescriptionsItem>
                <NDescriptionsItem :label="t('selfService.effectiveFrom')">
                  {{ fmtDate(salary.effective_from) }}
                </NDescriptionsItem>
              </NDescriptions>
            </template>
            <p v-else style="color: #999; margin-bottom: 12px">{{ t("selfService.noSalary") }}</p>

            <template v-if="latestPayslip">
              <h4 style="margin-bottom: 8px">{{ t("selfService.latestPayslip") }}</h4>
              <NDescriptions label-placement="left" :column="1" bordered size="small">
                <NDescriptionsItem :label="t('selfService.period')">
                  {{ latestPayslip.cycle_name }} ({{ fmtDate(latestPayslip.period_start) }} ~ {{ fmtDate(latestPayslip.period_end) }})
                </NDescriptionsItem>
                <NDescriptionsItem :label="t('selfService.grossPay')">
                  {{ formatCurrency(latestPayslip.gross_pay) }}
                </NDescriptionsItem>
                <NDescriptionsItem :label="t('selfService.deductions')">
                  {{ formatCurrency(latestPayslip.total_deductions) }}
                </NDescriptionsItem>
                <NDescriptionsItem :label="t('selfService.netPay')">
                  <strong>{{ formatCurrency(latestPayslip.net_pay) }}</strong>
                </NDescriptionsItem>
              </NDescriptions>
            </template>
            <p v-else style="color: #999">{{ t("selfService.noPayslip") }}</p>
          </NCard>
        </NGi>

        <!-- Leave Balances -->
        <NGi span="2 m:1">
          <NCard :title="t('selfService.leaveBalances')" size="small">
            <NList v-if="leaveBalances.length" bordered size="small">
              <NListItem v-for="lb in leaveBalances" :key="String(lb.leave_type_id)">
                <NThing :title="(lb.leave_type_name as string) || (lb.code as string) || '-'">
                  <template #description>
                    <NSpace :size="12">
                      <span>{{ t('leave.earned') }}: {{ lb.earned }}</span>
                      <span>{{ t('leave.used') }}: {{ lb.used }}</span>
                      <span>{{ t('leave.carried') }}: {{ lb.carried }}</span>
                      <NTag type="info" size="small">
                        {{ t('selfService.remaining') }}: {{ Number(lb.earned || 0) + Number(lb.carried || 0) - Number(lb.used || 0) }}
                      </NTag>
                    </NSpace>
                  </template>
                </NThing>
              </NListItem>
            </NList>
            <NEmpty v-else :description="t('selfService.noLeaveBalance')" />
          </NCard>
        </NGi>

        <!-- Active Loans -->
        <NGi span="2 m:1">
          <NCard :title="t('selfService.activeLoans')" size="small">
            <NList v-if="activeLoans.length" bordered size="small">
              <NListItem v-for="loan in activeLoans" :key="(loan.id as number)">
                <NThing :title="(loan.loan_type_name as string) || '-'">
                  <template #description>
                    <NSpace :size="12">
                      <span>{{ t('selfService.principal') }}: {{ formatCurrency(loan.principal_amount) }}</span>
                      <span>{{ t('selfService.remaining') }}: {{ formatCurrency(loan.remaining_balance) }}</span>
                      <span>{{ t('selfService.monthly') }}: {{ formatCurrency(loan.monthly_amortization) }}</span>
                      <NTag :type="loan.status === 'active' ? 'success' : 'info'" size="small">
                        {{ loan.status }}
                      </NTag>
                    </NSpace>
                  </template>
                </NThing>
              </NListItem>
            </NList>
            <NEmpty v-else :description="t('selfService.noLoans')" />
          </NCard>
        </NGi>

        <!-- Team -->
        <NGi span="2 m:1">
          <NCard :title="t('selfService.myTeam')" size="small">
            <!-- Manager -->
            <template v-if="manager">
              <h4 style="margin-bottom: 8px">{{ t("selfService.manager") }}</h4>
              <NDescriptions label-placement="left" :column="1" bordered size="small" style="margin-bottom: 12px">
                <NDescriptionsItem :label="t('employee.name')">
                  {{ manager.first_name }} {{ manager.last_name }}
                </NDescriptionsItem>
                <NDescriptionsItem :label="t('employee.position')">
                  {{ manager.position_title || "-" }}
                </NDescriptionsItem>
              </NDescriptions>
            </template>

            <!-- Teammates -->
            <template v-if="team.length">
              <h4 style="margin-bottom: 8px">{{ t("selfService.teammates") }}</h4>
              <NList bordered size="small" style="margin-bottom: 12px">
                <NListItem v-for="m in team" :key="(m.id as number)">
                  <NThing
                    :title="`${m.first_name} ${m.last_name}`"
                    :description="(m.position_title as string) || (m.department_name as string) || ''"
                  />
                </NListItem>
              </NList>
            </template>

            <!-- Direct Reports -->
            <template v-if="directReports.length">
              <h4 style="margin-bottom: 8px">{{ t("selfService.directReports") }}</h4>
              <NList bordered size="small">
                <NListItem v-for="m in directReports" :key="(m.id as number)">
                  <NThing
                    :title="`${m.first_name} ${m.last_name}`"
                    :description="(m.position_title as string) || (m.department_name as string) || ''"
                  />
                </NListItem>
              </NList>
            </template>

            <NEmpty v-if="!manager && !team.length && !directReports.length" :description="t('selfService.noTeam')" />
          </NCard>
        </NGi>

        <!-- Onboarding Progress -->
        <NGi span="2 m:1">
          <NCard :title="t('selfService.myOnboarding')" size="small">
            <template v-if="onboardingProgress">
              <NProgress
                type="line"
                :percentage="Number(onboardingProgress.total_tasks) > 0
                  ? Math.round((Number(onboardingProgress.completed_tasks) / Number(onboardingProgress.total_tasks)) * 100)
                  : 0"
                :indicator-placement="'inside'"
                style="margin-bottom: 12px"
              />
              <p style="color: #666; margin-bottom: 8px">
                {{ onboardingProgress.completed_tasks }} {{ t("selfService.of") }} {{ onboardingProgress.total_tasks }} {{ t("selfService.tasksDone") }}
              </p>
            </template>
            <NList v-if="onboardingTasks.length" bordered size="small">
              <NListItem v-for="task in onboardingTasks" :key="(task.id as number)">
                <NThing :title="(task.title as string)">
                  <template #description>
                    <NSpace :size="4">
                      <NTag :type="taskStatusColor[(task.status as string)] || 'default'" size="small">
                        {{ task.status }}
                      </NTag>
                      <span v-if="task.due_date">{{ fmtDate(task.due_date) }}</span>
                    </NSpace>
                  </template>
                </NThing>
              </NListItem>
            </NList>
            <NEmpty v-else :description="t('selfService.noOnboarding')" />
          </NCard>
        </NGi>
      </NGrid>
    </NSpace>
  </NSpin>
</template>
