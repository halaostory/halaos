<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { useI18n } from "vue-i18n";
import { NavBar, Cell, CellGroup, Tag, Empty, Button, Popup, Field, Picker, Switch, showToast, PullRefresh } from "vant";
import { format } from "date-fns";
import { expenseAPI } from "../api/client";

interface Claim { id: number; claim_number: string; description: string; amount: string; currency: string; expense_date: string; status: string; category_name: string; rejection_reason: string | null }
interface Category { id: number; name: string; max_amount?: string; requires_receipt?: boolean }

const { t } = useI18n();
const claims = ref<Claim[]>([]);
const categories = ref<Category[]>([]);
const loading = ref(false);
const showNew = ref(false);
const showPicker = ref(false);
const saving = ref(false);
const form = ref({ category_id: 0, description: "", amount: "", expense_date: format(new Date(), "yyyy-MM-dd"), notes: "", submit: true });

const categoryName = computed(() => categories.value.find((c) => c.id === form.value.category_id)?.name || "");
const columns = computed(() => categories.value.map((c) => ({ text: c.name, value: c.id })));
const tone: Record<string, "primary" | "success" | "warning" | "danger" | "default"> = {
  draft: "default", submitted: "warning", approved: "primary", paid: "success", rejected: "danger",
};

async function load() {
  loading.value = true;
  try {
    const [c, cat] = await Promise.all([expenseAPI.my(), expenseAPI.categories()]);
    claims.value = ((c as { data?: Claim[] }).data ?? []) as Claim[];
    categories.value = ((cat as { data?: Category[] }).data ?? []) as Category[];
  } finally {
    loading.value = false;
  }
}

function openNew() {
  form.value = { category_id: categories.value[0]?.id ?? 0, description: "", amount: "", expense_date: format(new Date(), "yyyy-MM-dd"), notes: "", submit: true };
  showNew.value = true;
}

async function save() {
  if (!form.value.category_id || !form.value.description || !Number(form.value.amount)) {
    showToast(t("expenses.fill"));
    return;
  }
  saving.value = true;
  try {
    await expenseAPI.create({ ...form.value, amount: Number(form.value.amount), notes: form.value.notes || null });
    showNew.value = false;
    showToast({ message: form.value.submit ? t("expenses.submitted") : t("expenses.saved"), type: "success" });
    await load();
  } catch (e) {
    showToast({ message: (e as { data?: { error?: { message?: string } } }).data?.error?.message || t("expenses.failed"), type: "fail" });
  } finally {
    saving.value = false;
  }
}

async function submitDraft(c: Claim) {
  await expenseAPI.submit(c.id);
  showToast({ message: t("expenses.submitted"), type: "success" });
  await load();
}

const money = (c: Claim) => Number(c.amount).toLocaleString(undefined, { style: "currency", currency: c.currency || "PHP" });
onMounted(load);
</script>

<template>
  <div class="expenses">
    <NavBar :title="t('tab.expenses')" />
    <PullRefresh :model-value="loading" @refresh="load">
      <Empty v-if="!loading && !claims.length" :description="t('expenses.empty')" />
      <CellGroup v-else inset>
        <Cell v-for="c in claims" :key="c.id" :title="c.description" :label="`${c.category_name} · ${format(new Date(c.expense_date), 'MMM d')}${c.rejection_reason ? ' · ' + c.rejection_reason : ''}`">
          <template #value>
            <div class="amount">{{ money(c) }}</div>
            <Tag :type="tone[c.status] || 'default'" plain>{{ t('expenses.status_' + c.status) }}</Tag>
            <Button v-if="c.status === 'draft'" size="mini" type="primary" plain class="send" @click.stop="submitDraft(c)">{{ t('expenses.send') }}</Button>
          </template>
        </Cell>
      </CellGroup>
    </PullRefresh>

    <div class="new-btn">
      <Button round block type="primary" icon="plus" :disabled="!categories.length" @click="openNew">{{ t('expenses.new') }}</Button>
    </div>

    <Popup v-model:show="showNew" position="bottom" round closeable :style="{ maxHeight: '85%' }">
      <div class="sheet">
        <h3>{{ t('expenses.new') }}</h3>
        <CellGroup inset>
          <Field :model-value="categoryName" is-link readonly :label="t('expenses.category')" @click="showPicker = true" />
          <Field v-model="form.description" :label="t('expenses.what')" :placeholder="t('expenses.whatPh')" />
          <Field v-model="form.amount" type="number" :label="t('expenses.amount')" placeholder="0.00" />
          <Field v-model="form.expense_date" type="date" :label="t('expenses.date')" />
          <Field v-model="form.notes" :label="t('expenses.notes')" type="textarea" rows="2" autosize />
          <Cell :title="t('expenses.submitNow')"><template #right-icon><Switch v-model="form.submit" size="20" /></template></Cell>
        </CellGroup>
        <div class="sheet-actions">
          <Button round block type="primary" :loading="saving" @click="save">{{ form.submit ? t('expenses.submit') : t('expenses.saveDraft') }}</Button>
        </div>
      </div>
    </Popup>
    <Popup v-model:show="showPicker" position="bottom" round>
      <Picker :columns="columns" @confirm="({ selectedValues }: { selectedValues: number[] }) => { form.category_id = selectedValues[0]; showPicker = false }" @cancel="showPicker = false" />
    </Popup>
  </div>
</template>

<style scoped>
.expenses { padding-bottom: 80px; }
.amount { font-weight: 600; color: var(--van-text-color); }
.send { margin-left: 6px; }
.new-btn { position: fixed; left: 16px; right: 92px; bottom: calc(66px + env(safe-area-inset-bottom)); }
.sheet { padding: 16px 0 24px; }
.sheet h3 { margin: 0 16px 12px; font-size: 17px; }
.sheet-actions { padding: 16px; }
</style>
