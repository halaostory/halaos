<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { NButton, NCard, NTag, NEmpty, NModal, NForm, NFormItem, NInput, NSelect, NSwitch, NSpace, NSpin, NPopconfirm, useMessage } from 'naive-ui'
import { benefitAPI } from '../../api/client'

// Perks (Warp: partner perks) — offers the company curates; employees
// claim one to reveal its code and link.
interface Perk {
  id: number; title: string; partner: string; category: string; description: string; offer: string
  expires_on: string | null; active: boolean; has_code: boolean; has_link: boolean
  code?: string; url?: string; claims?: number
}
const props = defineProps<{ admin: boolean }>()
const { t } = useI18n()
const message = useMessage()
const perks = ref<Perk[]>([])
const loading = ref(true)
const filter = ref('all')
const revealed = ref<Record<number, { code: string; url: string }>>({})
const categories = ['health', 'wellness', 'food', 'learning', 'travel', 'finance', 'software', 'other']
const catOptions = computed(() => categories.map(c => ({ label: t('perks.cat_' + c), value: c })))
const shown = computed(() => perks.value.filter(p => filter.value === 'all' || p.category === filter.value))
const usedCats = computed(() => categories.filter(c => perks.value.some(p => p.category === c)))

const blank = () => ({ title: '', partner: '', category: 'other', description: '', offer: '', code: '', url: '', expires_on: '', active: true })
const form = ref(blank())
const editing = ref<number | null>(null)
const showForm = ref(false)
const saving = ref(false)

function errMsg(e: unknown) { return (e as { data?: { error?: { message?: string } } }).data?.error?.message || t('common.failed') }

async function load() {
  loading.value = true
  try { perks.value = ((await benefitAPI.listPerks()) as { data: Perk[] }).data || [] } catch { perks.value = [] } finally { loading.value = false }
}
async function claim(p: Perk) {
  try {
    const d = ((await benefitAPI.claimPerk(p.id)) as { data: { code: string; url: string } }).data
    revealed.value = { ...revealed.value, [p.id]: d }
    if (d.url && !d.code) window.open(d.url, '_blank', 'noopener')
  } catch (e) { message.error(errMsg(e)) }
}
async function copy(code: string) {
  try { await navigator.clipboard.writeText(code); message.success(t('perks.copied')) } catch { /* clipboard blocked */ }
}
function openNew() { editing.value = null; form.value = blank(); showForm.value = true }
function openEdit(p: Perk) {
  editing.value = p.id
  form.value = { title: p.title, partner: p.partner, category: p.category, description: p.description, offer: p.offer,
    code: p.code || '', url: p.url || '', expires_on: p.expires_on || '', active: p.active }
  showForm.value = true
}
async function save() {
  saving.value = true
  try {
    if (editing.value) await benefitAPI.updatePerk(editing.value, { ...form.value })
    else await benefitAPI.createPerk({ ...form.value })
    showForm.value = false
    await load()
  } catch (e) { message.error(errMsg(e)) } finally { saving.value = false }
}
async function remove(p: Perk) {
  try { await benefitAPI.deletePerk(p.id); await load() } catch (e) { message.error(errMsg(e)) }
}
onMounted(load)
</script>

<template>
  <div>
    <div class="bar">
      <NSpace>
        <NTag :checkable="true" :checked="filter === 'all'" @update:checked="filter = 'all'">{{ t('perks.all') }}</NTag>
        <NTag v-for="c in usedCats" :key="c" :checkable="true" :checked="filter === c" @update:checked="filter = c">{{ t('perks.cat_' + c) }}</NTag>
      </NSpace>
      <NButton v-if="props.admin" type="primary" @click="openNew">{{ t('perks.add') }}</NButton>
    </div>
    <NSpin :show="loading">
      <NEmpty v-if="!loading && !shown.length" :description="props.admin ? t('perks.emptyAdmin') : t('perks.empty')" style="margin: 32px 0" />
      <div class="grid">
        <NCard v-for="p in shown" :key="p.id" size="small" :class="{ inactive: !p.active }">
          <div class="head">
            <div>
              <div class="partner">{{ p.partner || t('perks.cat_' + p.category) }}</div>
              <div class="title">{{ p.title }}</div>
            </div>
            <NTag size="small" round :bordered="false">{{ t('perks.cat_' + p.category) }}</NTag>
          </div>
          <div v-if="p.offer" class="offer">{{ p.offer }}</div>
          <p v-if="p.description" class="desc">{{ p.description }}</p>
          <div v-if="p.expires_on" class="muted">{{ t('perks.until', { d: p.expires_on }) }}</div>
          <div class="foot">
            <template v-if="revealed[p.id]">
              <NButton v-if="revealed[p.id].code" size="small" dashed @click="copy(revealed[p.id].code)">{{ revealed[p.id].code }} · {{ t('perks.copy') }}</NButton>
              <NButton v-if="revealed[p.id].url" size="small" tag="a" :href="revealed[p.id].url" target="_blank" rel="noopener">{{ t('perks.open') }}</NButton>
            </template>
            <NButton v-else-if="p.active && (p.has_code || p.has_link)" size="small" type="primary" secondary @click="claim(p)">{{ t('perks.claim') }}</NButton>
            <template v-if="props.admin">
              <span class="muted">{{ t('perks.claims', { n: p.claims || 0 }) }}<template v-if="!p.active"> · {{ t('perks.hidden') }}</template></span>
              <NButton size="tiny" quaternary @click="openEdit(p)">{{ t('common.edit') }}</NButton>
              <NPopconfirm @positive-click="remove(p)"><template #trigger><NButton size="tiny" quaternary type="error">{{ t('common.delete') }}</NButton></template>{{ t('perks.confirmDelete') }}</NPopconfirm>
            </template>
          </div>
        </NCard>
      </div>
    </NSpin>

    <NModal v-model:show="showForm" preset="card" :title="editing ? t('perks.edit') : t('perks.add')" style="max-width: 560px; width: 95vw">
      <NForm label-placement="top">
        <NFormItem :label="t('perks.fTitle')"><NInput v-model:value="form.title" :placeholder="t('perks.phTitle')" /></NFormItem>
        <NSpace :wrap="false" style="width: 100%" item-style="flex: 1">
          <NFormItem :label="t('perks.fPartner')"><NInput v-model:value="form.partner" /></NFormItem>
          <NFormItem :label="t('perks.fCategory')"><NSelect v-model:value="form.category" :options="catOptions" /></NFormItem>
        </NSpace>
        <NFormItem :label="t('perks.fOffer')"><NInput v-model:value="form.offer" :placeholder="t('perks.phOffer')" /></NFormItem>
        <NFormItem :label="t('perks.fDescription')"><NInput v-model:value="form.description" type="textarea" :rows="3" /></NFormItem>
        <NSpace :wrap="false" style="width: 100%" item-style="flex: 1">
          <NFormItem :label="t('perks.fCode')"><NInput v-model:value="form.code" /></NFormItem>
          <NFormItem :label="t('perks.fExpires')"><NInput v-model:value="form.expires_on" placeholder="YYYY-MM-DD" /></NFormItem>
        </NSpace>
        <NFormItem :label="t('perks.fUrl')"><NInput v-model:value="form.url" placeholder="https://" /></NFormItem>
        <NFormItem :label="t('perks.fActive')"><NSwitch v-model:value="form.active" /></NFormItem>
        <NButton type="primary" :loading="saving" @click="save">{{ t('common.save') }}</NButton>
      </NForm>
    </NModal>
  </div>
</template>

<style scoped>
.bar { display: flex; justify-content: space-between; align-items: center; gap: 12px; margin-bottom: 16px; flex-wrap: wrap; }
.grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 12px; }
.head { display: flex; justify-content: space-between; gap: 8px; align-items: flex-start; }
.partner { font-size: 12px; opacity: .6; text-transform: uppercase; letter-spacing: .04em; }
.title { font-weight: 600; font-size: 15px; margin-top: 2px; }
.offer { font-size: 18px; font-weight: 700; margin: 10px 0 4px; color: var(--n-color-target, #18a058); }
.desc { margin: 6px 0; font-size: 13px; opacity: .8; }
.muted { font-size: 12px; opacity: .6; }
.foot { display: flex; gap: 6px; align-items: center; flex-wrap: wrap; margin-top: 12px; }
.inactive { opacity: .55; }
</style>
