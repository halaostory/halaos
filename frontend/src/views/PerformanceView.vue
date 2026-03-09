<script setup lang="ts">
import { ref, h, computed, onMounted } from "vue";
import { useI18n } from "vue-i18n";
import {
  NDataTable,
  NButton,
  NSpace,
  NTabs,
  NTabPane,
  NTag,
  NModal,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NSelect,
  NDatePicker,
  NRate,
  NCard,
  NDescriptions,
  NDescriptionsItem,
  useMessage,
  useDialog,
  type DataTableColumns,
} from "naive-ui";
import { performanceAPI } from "../api/client";
import { useAuthStore } from "../stores/auth";

const { t } = useI18n();
const message = useMessage();
const dialog = useDialog();
const auth = useAuthStore();

const activeTab = ref("cycles");
const loading = ref(false);

// --- Cycles ---
const cycles = ref<Record<string, unknown>[]>([]);
const showCycleModal = ref(false);
const cycleForm = ref({
  name: "",
  cycle_type: "annual",
  period_start: null as number | null,
  period_end: null as number | null,
  review_deadline: null as number | null,
});

const cycleTypeOptions = computed(() => [
  { label: t("performance.annual"), value: "annual" },
  { label: t("performance.quarterly"), value: "quarterly" },
  { label: t("performance.probation"), value: "probation" },
  { label: t("performance.project"), value: "project" },
]);

const statusColorMap: Record<string, "default" | "info" | "success" | "warning"> = {
  draft: "default",
  active: "info",
  closed: "success",
  pending: "default",
  self_review: "info",
  manager_review: "warning",
  completed: "success",
};

const cycleColumns: DataTableColumns = [
  { title: t("performance.cycleName"), key: "name" },
  {
    title: t("performance.cycleType"),
    key: "cycle_type",
    width: 100,
    render(row) {
      const key = row.cycle_type as string;
      const map: Record<string, string> = {
        annual: t("performance.annual"),
        quarterly: t("performance.quarterly"),
        probation: t("performance.probation"),
        project: t("performance.project"),
      };
      return map[key] || key;
    },
  },
  {
    title: t("performance.periodStart"),
    key: "period_start",
    width: 120,
    render(row) {
      const d = row.period_start as string;
      return d ? d.substring(0, 10) : "-";
    },
  },
  {
    title: t("performance.periodEnd"),
    key: "period_end",
    width: 120,
    render(row) {
      const d = row.period_end as string;
      return d ? d.substring(0, 10) : "-";
    },
  },
  {
    title: t("common.status"),
    key: "status",
    width: 100,
    render(row) {
      const s = row.status as string;
      return h(NTag, { type: statusColorMap[s] || "default", size: "small" }, () => s);
    },
  },
  {
    title: t("common.actions"),
    key: "actions",
    width: 220,
    render(row) {
      const btns = [
        h(
          NButton,
          { size: "small", onClick: () => viewCycleReviews(row) },
          () => t("performance.reviews"),
        ),
      ];
      if (row.status === "draft" && auth.isAdmin) {
        btns.push(
          h(
            NButton,
            { size: "small", type: "primary", onClick: () => handleInitiate(row) },
            () => t("performance.initiate"),
          ),
        );
      }
      return h(NSpace, { size: "small" }, () => btns);
    },
  },
];

async function loadCycles() {
  loading.value = true;
  try {
    const res = (await performanceAPI.listCycles()) as { data?: Record<string, unknown>[] };
    cycles.value = (res.data || (Array.isArray(res) ? res : [])) as Record<string, unknown>[];
  } catch {
    cycles.value = [];
  } finally {
    loading.value = false;
  }
}

function formatDate(ts: number | null): string {
  if (!ts) return "";
  const d = new Date(ts);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

async function handleCreateCycle() {
  if (!cycleForm.value.name || !cycleForm.value.period_start || !cycleForm.value.period_end) {
    message.warning(t("common.fillAllFields"));
    return;
  }
  try {
    await performanceAPI.createCycle({
      name: cycleForm.value.name,
      cycle_type: cycleForm.value.cycle_type,
      period_start: formatDate(cycleForm.value.period_start),
      period_end: formatDate(cycleForm.value.period_end),
      review_deadline: cycleForm.value.review_deadline
        ? formatDate(cycleForm.value.review_deadline)
        : undefined,
    });
    showCycleModal.value = false;
    cycleForm.value = {
      name: "",
      cycle_type: "annual",
      period_start: null,
      period_end: null,
      review_deadline: null,
    };
    message.success(t("performance.cycleCreated"));
    await loadCycles();
  } catch {
    message.error(t("common.failed"));
  }
}

function handleInitiate(row: Record<string, unknown>) {
  dialog.warning({
    title: t("performance.initiate"),
    content: t("performance.initiateConfirm"),
    positiveText: t("common.confirm"),
    negativeText: t("common.cancel"),
    onPositiveClick: async () => {
      try {
        await performanceAPI.initiateCycle(row.id as number);
        message.success(t("performance.initiated"));
        await loadCycles();
      } catch {
        message.error(t("common.failed"));
      }
    },
  });
}

// --- Cycle Reviews ---
const selectedCycle = ref<Record<string, unknown> | null>(null);
const cycleReviews = ref<Record<string, unknown>[]>([]);
const cycleStats = ref<Record<string, unknown>[]>([]);
const showCycleReviews = ref(false);

const reviewColumns: DataTableColumns = [
  {
    title: t("performance.employee"),
    key: "employee_name",
    render(row) {
      return `${row.first_name} ${row.last_name}`;
    },
  },
  { title: t("employee.employeeNo"), key: "employee_no", width: 100 },
  {
    title: t("performance.reviewer"),
    key: "reviewer_name",
    width: 150,
    render(row) {
      if (row.reviewer_first_name) {
        return `${row.reviewer_first_name} ${row.reviewer_last_name}`;
      }
      return "-";
    },
  },
  {
    title: t("common.status"),
    key: "status",
    width: 130,
    render(row) {
      const s = row.status as string;
      const labelMap: Record<string, string> = {
        pending: t("performance.pending"),
        self_review: t("performance.selfReviewStatus"),
        manager_review: t("performance.managerReviewStatus"),
        completed: t("performance.completed"),
      };
      return h(
        NTag,
        { type: statusColorMap[s] || "default", size: "small" },
        () => labelMap[s] || s,
      );
    },
  },
  {
    title: t("performance.selfRating"),
    key: "self_rating",
    width: 100,
    render(row) {
      return row.self_rating != null ? `${row.self_rating}/5` : "-";
    },
  },
  {
    title: t("performance.finalRating"),
    key: "final_rating",
    width: 100,
    render(row) {
      return row.final_rating != null ? `${row.final_rating}/5` : "-";
    },
  },
  {
    title: t("common.actions"),
    key: "actions",
    width: 100,
    render(row) {
      return h(
        NButton,
        { size: "small", onClick: () => viewReviewDetail(row) },
        () => t("common.view"),
      );
    },
  },
];

async function viewCycleReviews(cycle: Record<string, unknown>) {
  selectedCycle.value = cycle;
  showCycleReviews.value = true;
  try {
    const res = (await performanceAPI.listReviewsByCycle(cycle.id as number)) as {
      data?: { reviews: Record<string, unknown>[]; stats: Record<string, unknown>[] };
    };
    const data = res.data || (res as unknown as { reviews: Record<string, unknown>[]; stats: Record<string, unknown>[] });
    cycleReviews.value = data.reviews || [];
    cycleStats.value = data.stats || [];
  } catch {
    cycleReviews.value = [];
    cycleStats.value = [];
  }
}

// --- Review Detail ---
const showReviewDetail = ref(false);
const reviewDetail = ref<Record<string, unknown> | null>(null);
const reviewGoals = ref<Record<string, unknown>[]>([]);
const showSelfModal = ref(false);
const showManagerModal = ref(false);

const selfForm = ref({ self_rating: 3, self_comments: "" });
const managerForm = ref({
  manager_rating: 3,
  manager_comments: "",
  strengths: "",
  improvements: "",
  final_rating: 3,
  final_comments: "",
});

async function viewReviewDetail(row: Record<string, unknown>) {
  try {
    const res = (await performanceAPI.getReview(row.id as number)) as {
      data?: { review: Record<string, unknown>; goals: Record<string, unknown>[] };
    };
    const data = res.data || (res as unknown as { review: Record<string, unknown>; goals: Record<string, unknown>[] });
    reviewDetail.value = data.review || null;
    reviewGoals.value = data.goals || [];
    showReviewDetail.value = true;
  } catch {
    message.error(t("common.failed"));
  }
}

async function handleSubmitSelf() {
  if (!reviewDetail.value) return;
  try {
    await performanceAPI.submitSelfReview(reviewDetail.value.id as number, {
      self_rating: selfForm.value.self_rating,
      self_comments: selfForm.value.self_comments,
    });
    showSelfModal.value = false;
    message.success(t("performance.selfSubmitted"));
    await viewReviewDetail(reviewDetail.value);
  } catch {
    message.error(t("common.failed"));
  }
}

async function handleSubmitManager() {
  if (!reviewDetail.value) return;
  try {
    await performanceAPI.submitManagerReview(reviewDetail.value.id as number, {
      manager_rating: managerForm.value.manager_rating,
      manager_comments: managerForm.value.manager_comments,
      strengths: managerForm.value.strengths,
      improvements: managerForm.value.improvements,
      final_rating: managerForm.value.final_rating,
      final_comments: managerForm.value.final_comments,
    });
    showManagerModal.value = false;
    message.success(t("performance.managerSubmitted"));
    await viewReviewDetail(reviewDetail.value);
  } catch {
    message.error(t("common.failed"));
  }
}

// --- My Reviews ---
const myReviews = ref<Record<string, unknown>[]>([]);

const myReviewColumns: DataTableColumns = [
  { title: t("performance.cycleName"), key: "cycle_name" },
  { title: t("performance.cycleType"), key: "cycle_type", width: 100 },
  {
    title: t("performance.periodStart"),
    key: "period_start",
    width: 120,
    render(row) {
      const d = row.period_start as string;
      return d ? d.substring(0, 10) : "-";
    },
  },
  {
    title: t("common.status"),
    key: "status",
    width: 130,
    render(row) {
      const s = row.status as string;
      const labelMap: Record<string, string> = {
        pending: t("performance.pending"),
        self_review: t("performance.selfReviewStatus"),
        manager_review: t("performance.managerReviewStatus"),
        completed: t("performance.completed"),
      };
      return h(
        NTag,
        { type: statusColorMap[s] || "default", size: "small" },
        () => labelMap[s] || s,
      );
    },
  },
  {
    title: t("performance.selfRating"),
    key: "self_rating",
    width: 100,
    render(row) {
      return row.self_rating != null ? `${row.self_rating}/5` : "-";
    },
  },
  {
    title: t("performance.finalRating"),
    key: "final_rating",
    width: 100,
    render(row) {
      return row.final_rating != null ? `${row.final_rating}/5` : "-";
    },
  },
  {
    title: t("common.actions"),
    key: "actions",
    width: 100,
    render(row) {
      return h(
        NButton,
        { size: "small", onClick: () => viewReviewDetail(row) },
        () => t("common.view"),
      );
    },
  },
];

async function loadMyReviews() {
  try {
    const res = (await performanceAPI.listMyReviews()) as { data?: Record<string, unknown>[] };
    myReviews.value = (res.data || (Array.isArray(res) ? res : [])) as Record<string, unknown>[];
  } catch {
    myReviews.value = [];
  }
}

// --- Goals ---
const goals = ref<Record<string, unknown>[]>([]);
const showGoalModal = ref(false);
const goalForm = ref({
  employee_id: null as number | null,
  review_cycle_id: null as number | null,
  title: "",
  description: "",
  category: "individual",
  weight: 0,
  target_value: "",
  due_date: null as number | null,
});

const categoryOptions = computed(() => [
  { label: t("performance.individual"), value: "individual" },
  { label: t("performance.team"), value: "team" },
  { label: t("performance.company"), value: "company" },
]);

const goalColumns: DataTableColumns = [
  { title: t("performance.goalTitle"), key: "title" },
  {
    title: t("performance.category"),
    key: "category",
    width: 100,
    render(row) {
      const key = row.category as string;
      const map: Record<string, string> = {
        individual: t("performance.individual"),
        team: t("performance.team"),
        company: t("performance.company"),
      };
      return map[key] || key;
    },
  },
  {
    title: t("performance.weight"),
    key: "weight",
    width: 80,
    render(row) {
      const w = row.weight as string;
      return w ? `${w}%` : "-";
    },
  },
  {
    title: t("performance.targetValue"),
    key: "target_value",
    width: 120,
    render(row) {
      return (row.target_value as string) || "-";
    },
  },
  {
    title: t("common.status"),
    key: "status",
    width: 100,
    render(row) {
      const s = row.status as string;
      return h(NTag, { type: statusColorMap[s] || "default", size: "small" }, () => s);
    },
  },
  {
    title: t("performance.selfRating"),
    key: "self_rating",
    width: 90,
    render(row) {
      return row.self_rating != null ? `${row.self_rating}/5` : "-";
    },
  },
  {
    title: t("performance.managerRating"),
    key: "manager_rating",
    width: 90,
    render(row) {
      return row.manager_rating != null ? `${row.manager_rating}/5` : "-";
    },
  },
];

async function loadGoals() {
  try {
    const res = (await performanceAPI.listGoals()) as { data?: Record<string, unknown>[] };
    goals.value = (res.data || (Array.isArray(res) ? res : [])) as Record<string, unknown>[];
  } catch {
    goals.value = [];
  }
}

async function handleCreateGoal() {
  if (!goalForm.value.title || !goalForm.value.employee_id) {
    message.warning(t("common.fillAllFields"));
    return;
  }
  try {
    await performanceAPI.createGoal({
      employee_id: goalForm.value.employee_id,
      review_cycle_id: goalForm.value.review_cycle_id || undefined,
      title: goalForm.value.title,
      description: goalForm.value.description || undefined,
      category: goalForm.value.category,
      weight: goalForm.value.weight,
      target_value: goalForm.value.target_value || undefined,
      due_date: goalForm.value.due_date ? formatDate(goalForm.value.due_date) : undefined,
    });
    showGoalModal.value = false;
    goalForm.value = {
      employee_id: null,
      review_cycle_id: null,
      title: "",
      description: "",
      category: "individual",
      weight: 0,
      target_value: "",
      due_date: null,
    };
    message.success(t("performance.goalCreated"));
    await loadGoals();
  } catch {
    message.error(t("common.failed"));
  }
}

function ratingLabel(r: number): string {
  const map: Record<number, string> = {
    5: t("performance.outstanding"),
    4: t("performance.exceeds"),
    3: t("performance.meets"),
    2: t("performance.below"),
    1: t("performance.unsatisfactory"),
  };
  return map[r] || String(r);
}

function onTabChange(tab: string | number) {
  activeTab.value = String(tab);
  if (tab === "cycles") loadCycles();
  else if (tab === "my-reviews") loadMyReviews();
  else if (tab === "goals") loadGoals();
}

onMounted(loadCycles);
</script>

<template>
  <div>
    <NSpace justify="space-between" style="margin-bottom: 16px">
      <h2>{{ t("performance.title") }}</h2>
    </NSpace>

    <NTabs type="line" :value="activeTab" @update:value="onTabChange">
      <!-- Review Cycles Tab -->
      <NTabPane name="cycles" :tab="t('performance.cycles')">
        <NSpace style="margin-bottom: 12px">
          <NButton v-if="auth.isAdmin" type="primary" @click="showCycleModal = true">
            {{ t("performance.createCycle") }}
          </NButton>
        </NSpace>
        <NDataTable :columns="cycleColumns" :data="cycles" :loading="loading" size="small" />
      </NTabPane>

      <!-- My Reviews Tab -->
      <NTabPane name="my-reviews" :tab="t('performance.myReviews')">
        <NDataTable :columns="myReviewColumns" :data="myReviews" size="small" />
      </NTabPane>

      <!-- Goals Tab -->
      <NTabPane name="goals" :tab="t('performance.goals')">
        <NSpace style="margin-bottom: 12px">
          <NButton v-if="auth.isAdmin || auth.isManager" type="primary" @click="showGoalModal = true">
            {{ t("performance.createGoal") }}
          </NButton>
        </NSpace>
        <NDataTable :columns="goalColumns" :data="goals" size="small" />
      </NTabPane>
    </NTabs>

    <!-- Create Cycle Modal -->
    <NModal v-model:show="showCycleModal" preset="card" :title="t('performance.createCycle')" style="max-width: 500px; width: 95vw">
      <NForm label-placement="left" label-width="120">
        <NFormItem :label="t('performance.cycleName')" required>
          <NInput v-model:value="cycleForm.name" />
        </NFormItem>
        <NFormItem :label="t('performance.cycleType')">
          <NSelect v-model:value="cycleForm.cycle_type" :options="cycleTypeOptions" />
        </NFormItem>
        <NFormItem :label="t('performance.periodStart')" required>
          <NDatePicker v-model:value="cycleForm.period_start" type="date" style="width: 100%" />
        </NFormItem>
        <NFormItem :label="t('performance.periodEnd')" required>
          <NDatePicker v-model:value="cycleForm.period_end" type="date" style="width: 100%" />
        </NFormItem>
        <NFormItem :label="t('performance.reviewDeadline')">
          <NDatePicker v-model:value="cycleForm.review_deadline" type="date" style="width: 100%" />
        </NFormItem>
        <NFormItem>
          <NButton type="primary" @click="handleCreateCycle">{{ t("common.create") }}</NButton>
        </NFormItem>
      </NForm>
    </NModal>

    <!-- Create Goal Modal -->
    <NModal v-model:show="showGoalModal" preset="card" :title="t('performance.createGoal')" style="max-width: 500px; width: 95vw">
      <NForm label-placement="left" label-width="120">
        <NFormItem :label="t('performance.employee')" required>
          <NInputNumber v-model:value="goalForm.employee_id" :min="1" style="width: 100%" :placeholder="t('common.placeholder.employeeId')" />
        </NFormItem>
        <NFormItem :label="t('performance.goalTitle')" required>
          <NInput v-model:value="goalForm.title" />
        </NFormItem>
        <NFormItem :label="t('performance.goalDescription')">
          <NInput v-model:value="goalForm.description" type="textarea" />
        </NFormItem>
        <NFormItem :label="t('performance.category')">
          <NSelect v-model:value="goalForm.category" :options="categoryOptions" />
        </NFormItem>
        <NFormItem :label="t('performance.weight')">
          <NInputNumber v-model:value="goalForm.weight" :min="0" :max="100" style="width: 100%" />
        </NFormItem>
        <NFormItem :label="t('performance.targetValue')">
          <NInput v-model:value="goalForm.target_value" />
        </NFormItem>
        <NFormItem :label="t('performance.dueDate')">
          <NDatePicker v-model:value="goalForm.due_date" type="date" style="width: 100%" />
        </NFormItem>
        <NFormItem>
          <NButton type="primary" @click="handleCreateGoal">{{ t("common.create") }}</NButton>
        </NFormItem>
      </NForm>
    </NModal>

    <!-- Cycle Reviews Modal -->
    <NModal v-model:show="showCycleReviews" preset="card" :title="selectedCycle?.name as string || t('performance.reviews')" style="max-width: 900px; width: 95vw">
      <NSpace v-if="cycleStats.length" style="margin-bottom: 12px" :size="8">
        <NTag v-for="stat in cycleStats" :key="(stat.rating_label as string)" :type="stat.rating_label === 'Pending' ? 'default' : 'info'" size="small">
          {{ stat.rating_label }}: {{ stat.count }}
        </NTag>
      </NSpace>
      <NDataTable :columns="reviewColumns" :data="cycleReviews" size="small" />
    </NModal>

    <!-- Review Detail Modal -->
    <NModal v-model:show="showReviewDetail" preset="card" :title="t('performance.reviews')" style="max-width: 700px; width: 95vw">
      <template v-if="reviewDetail">
        <NDescriptions bordered :column="2" size="small" style="margin-bottom: 16px">
          <NDescriptionsItem :label="t('common.status')">
            <NTag :type="statusColorMap[(reviewDetail.status as string)] || 'default'" size="small">
              {{ reviewDetail.status }}
            </NTag>
          </NDescriptionsItem>
          <NDescriptionsItem v-if="reviewDetail.self_rating != null" :label="t('performance.selfRating')">
            {{ reviewDetail.self_rating }}/5 — {{ ratingLabel(reviewDetail.self_rating as number) }}
          </NDescriptionsItem>
          <NDescriptionsItem v-if="reviewDetail.self_comments" :label="t('performance.selfReview')" :span="2">
            {{ reviewDetail.self_comments }}
          </NDescriptionsItem>
          <NDescriptionsItem v-if="reviewDetail.manager_rating != null" :label="t('performance.managerRating')">
            {{ reviewDetail.manager_rating }}/5
          </NDescriptionsItem>
          <NDescriptionsItem v-if="reviewDetail.final_rating != null" :label="t('performance.finalRating')">
            {{ reviewDetail.final_rating }}/5 — {{ ratingLabel(reviewDetail.final_rating as number) }}
          </NDescriptionsItem>
          <NDescriptionsItem v-if="reviewDetail.strengths" :label="t('performance.strengths')" :span="2">
            {{ reviewDetail.strengths }}
          </NDescriptionsItem>
          <NDescriptionsItem v-if="reviewDetail.improvements" :label="t('performance.improvements')" :span="2">
            {{ reviewDetail.improvements }}
          </NDescriptionsItem>
        </NDescriptions>

        <!-- Goals for this review -->
        <NCard v-if="reviewGoals.length" :title="t('performance.goals')" size="small" style="margin-bottom: 16px">
          <NDataTable :columns="goalColumns" :data="reviewGoals" size="small" :pagination="false" />
        </NCard>

        <!-- Action buttons -->
        <NSpace>
          <NButton
            v-if="reviewDetail.status === 'pending' || reviewDetail.status === 'self_review'"
            type="primary"
            @click="showSelfModal = true"
          >
            {{ t("performance.submitSelf") }}
          </NButton>
          <NButton
            v-if="(reviewDetail.status === 'manager_review') && (auth.isAdmin || auth.isManager)"
            type="primary"
            @click="showManagerModal = true"
          >
            {{ t("performance.submitManager") }}
          </NButton>
        </NSpace>
      </template>
    </NModal>

    <!-- Self Review Modal -->
    <NModal v-model:show="showSelfModal" preset="card" :title="t('performance.selfReview')" style="max-width: 500px; width: 95vw">
      <NForm label-placement="left" label-width="100">
        <NFormItem :label="t('performance.selfRating')">
          <NRate v-model:value="selfForm.self_rating" :count="5" />
          <span style="margin-left: 8px">{{ ratingLabel(selfForm.self_rating) }}</span>
        </NFormItem>
        <NFormItem :label="t('performance.comments')">
          <NInput v-model:value="selfForm.self_comments" type="textarea" :rows="4" />
        </NFormItem>
        <NFormItem>
          <NButton type="primary" @click="handleSubmitSelf">{{ t("common.submit") }}</NButton>
        </NFormItem>
      </NForm>
    </NModal>

    <!-- Manager Review Modal -->
    <NModal v-model:show="showManagerModal" preset="card" :title="t('performance.managerReview')" style="max-width: 600px; width: 95vw">
      <NForm label-placement="left" label-width="130">
        <NFormItem :label="t('performance.managerRating')">
          <NRate v-model:value="managerForm.manager_rating" :count="5" />
          <span style="margin-left: 8px">{{ ratingLabel(managerForm.manager_rating) }}</span>
        </NFormItem>
        <NFormItem :label="t('performance.comments')">
          <NInput v-model:value="managerForm.manager_comments" type="textarea" :rows="3" />
        </NFormItem>
        <NFormItem :label="t('performance.strengths')">
          <NInput v-model:value="managerForm.strengths" type="textarea" :rows="2" />
        </NFormItem>
        <NFormItem :label="t('performance.improvements')">
          <NInput v-model:value="managerForm.improvements" type="textarea" :rows="2" />
        </NFormItem>
        <NFormItem :label="t('performance.finalRating')">
          <NRate v-model:value="managerForm.final_rating" :count="5" />
          <span style="margin-left: 8px">{{ ratingLabel(managerForm.final_rating) }}</span>
        </NFormItem>
        <NFormItem :label="t('performance.comments')">
          <NInput v-model:value="managerForm.final_comments" type="textarea" :rows="2" :placeholder="t('performance.finalComments')" />
        </NFormItem>
        <NFormItem>
          <NButton type="primary" @click="handleSubmitManager">{{ t("common.submit") }}</NButton>
        </NFormItem>
      </NForm>
    </NModal>
  </div>
</template>
