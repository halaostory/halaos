<script setup lang="ts">
import { ref, computed, onMounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { NCard, NInput, NCheckbox, NButton, NSpace, NAlert, NResult, NFormItem } from 'naive-ui'

const props = defineProps<{
  doc: { title: string; body: string; company_name: string; signer_name: string; status: string; signed_at?: string }
  submit: (payload: Record<string, unknown>) => Promise<void>
}>()

const { t } = useI18n()
// Same light formatting as the signed PDF: "# " / "## " lines are headings.
const blocks = computed(() => props.doc.body.split(/\r?\n/).map((line) => {
  const m = line.match(/^#{1,2}\s+(.*)$/)
  return m ? { heading: true, text: m[1] } : { heading: false, text: line }
}))
const name = ref('')
const consent = ref(false)
const error = ref('')
const busy = ref(false)
const declining = ref(false)
const declineReason = ref('')
const canvas = ref<HTMLCanvasElement | null>(null)
const drawn = ref(false)
let drawing = false

function pos(e: PointerEvent) {
  const r = canvas.value!.getBoundingClientRect()
  return [e.clientX - r.left, e.clientY - r.top]
}
function start(e: PointerEvent) {
  drawing = true
  const ctx = canvas.value!.getContext('2d')!
  const [x, y] = pos(e)
  ctx.beginPath()
  ctx.moveTo(x, y)
}
function move(e: PointerEvent) {
  if (!drawing) return
  const ctx = canvas.value!.getContext('2d')!
  const [x, y] = pos(e)
  ctx.lineTo(x, y)
  ctx.stroke()
  drawn.value = true
}
function stop() { drawing = false }
function clear() {
  const c = canvas.value!
  c.getContext('2d')!.clearRect(0, 0, c.width, c.height)
  drawn.value = false
}
onMounted(async () => {
  await nextTick()
  const ctx = canvas.value?.getContext('2d')
  if (ctx) { ctx.lineWidth = 2.2; ctx.lineCap = 'round'; ctx.strokeStyle = '#1a1a1a' }
})

async function send(decline: boolean) {
  error.value = ''
  busy.value = true
  try {
    await props.submit(decline
      ? { signed_name: name.value || props.doc.signer_name, decline: true, decline_reason: declineReason.value }
      : { signed_name: name.value, consent: consent.value, signature_image: drawn.value ? canvas.value!.toDataURL('image/png') : '' })
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string; fields?: Record<string, string> } } }
    error.value = Object.values(err.data?.error?.fields || {})[0] || err.data?.error?.message || t('common.saveFailed')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <NCard v-if="doc.status === 'signed'">
    <NResult status="success" :title="t('portal.signedTitle')" :description="t('portal.signedDesc', { date: doc.signed_at ? new Date(doc.signed_at).toLocaleString() : '' })" />
  </NCard>
  <NCard v-else-if="doc.status === 'declined'">
    <NResult status="warning" :title="t('portal.declinedTitle')" />
  </NCard>
  <NCard v-else :title="doc.title">
    <div class="doc-body">
      <template v-for="(b, i) in blocks" :key="i">
        <div v-if="b.heading" class="doc-h">{{ b.text }}</div>
        <div v-else-if="b.text.trim()">{{ b.text }}</div>
        <div v-else class="doc-gap" />
      </template>
    </div>
    <NSpace vertical style="margin-top: 20px" :size="12">
      <NFormItem :label="t('portal.typeName', { name: doc.signer_name })" :show-feedback="false">
        <NInput v-model:value="name" :placeholder="doc.signer_name" style="max-width: 360px; font-family: 'Times New Roman', serif; font-style: italic; font-size: 18px" />
      </NFormItem>
      <div>
        <div style="font-size: 13px; opacity: 0.75; margin-bottom: 4px">{{ t('portal.drawOptional') }}</div>
        <canvas ref="canvas" width="360" height="110" class="sig-pad" @pointerdown="start" @pointermove="move" @pointerup="stop" @pointerleave="stop" />
        <div><NButton text size="small" @click="clear">{{ t('portal.clear') }}</NButton></div>
      </div>
      <NCheckbox v-model:checked="consent">{{ t('portal.consent', { company: doc.company_name }) }}</NCheckbox>
      <NAlert v-if="error" type="error" :show-icon="false">{{ error }}</NAlert>
      <NSpace>
        <NButton type="primary" :loading="busy" :disabled="!name || !consent" @click="send(false)">{{ t('portal.sign') }}</NButton>
        <NButton quaternary @click="declining = !declining">{{ t('portal.decline') }}</NButton>
      </NSpace>
      <NSpace v-if="declining">
        <NInput v-model:value="declineReason" :placeholder="t('portal.declineReason')" style="width: 300px" />
        <NButton type="warning" :loading="busy" @click="send(true)">{{ t('portal.confirmDecline') }}</NButton>
      </NSpace>
    </NSpace>
  </NCard>
</template>

<style scoped>
.doc-body { line-height: 1.6; max-height: 55vh; overflow: auto; padding: 12px; border: 1px solid rgba(128,128,128,.25); border-radius: 6px; }
.doc-h { font-weight: 600; margin-top: 6px; }
.doc-gap { height: 10px; }
.sig-pad { border: 1px dashed rgba(128,128,128,.6); border-radius: 6px; touch-action: none; background: #fff; max-width: 100%; }
</style>
