<template>
  <div ref="canvasWrap" class="office-canvas" style="position: relative">
    <div ref="canvasContainer" style="width: 100%; height: 100%" />
    <SeatTooltip :seat="hoveredSeat" :position="tooltipPos" />
    <button v-if="zoomed" class="reset-zoom-btn" @click="resetZoom">
      {{ t('virtualOffice.resetZoom') }}
    </button>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Application, Container } from 'pixi.js'
import { OfficeRenderer, type OfficeTemplate } from './OfficeRenderer'
import { SpriteManager, type SeatData } from './SpriteManager'
import SeatTooltip from './SeatTooltip.vue'

const props = withDefaults(defineProps<{
  template: OfficeTemplate | null
  seats: SeatData[]
  isAdmin?: boolean
}>(), { isAdmin: false })

const emit = defineEmits<{
  (e: 'select', seat: SeatData): void
  (e: 'emptySeatClick', position: { floor: number; zone: string; seat_x: number; seat_y: number }): void
}>()

const { t } = useI18n()
const canvasWrap = ref<HTMLElement>()
const canvasContainer = ref<HTMLElement>()
const hoveredSeat = ref<SeatData | null>(null)
const tooltipPos = ref({ x: 0, y: 0 })
const zoomed = ref(false)

let app: Application | null = null
let renderer: OfficeRenderer | null = null
let spriteManager: SpriteManager | null = null
let stage: Container | null = null

// Zoom/pan state
let currentScale = 1
const MIN_SCALE = 0.5
const MAX_SCALE = 4
let isDragging = false
let dragStartX = 0
let dragStartY = 0
let stageStartX = 0
let stageStartY = 0

onMounted(async () => {
  if (!canvasContainer.value) return

  app = new Application()
  await app.init({
    background: '#FAFAFA',
    resizeTo: canvasContainer.value,
    antialias: true,
    resolution: window.devicePixelRatio || 1,
    autoDensity: true,
  })
  canvasContainer.value.appendChild(app.canvas as HTMLCanvasElement)

  stage = app.stage
  renderer = new OfficeRenderer(app)
  renderer.onEmptySeatClick = (pos) => emit('emptySeatClick', pos)
  if (props.isAdmin) renderer.setAdminMode(true)

  spriteManager = new SpriteManager(stage, 32)

  spriteManager.onSeatClick = (seat) => {
    emit('select', seat)
    zoomToSeat(seat)
  }

  spriteManager.onSeatHover = (seat, gx, gy) => {
    hoveredSeat.value = seat
    if (canvasWrap.value) {
      const rect = canvasWrap.value.getBoundingClientRect()
      tooltipPos.value = { x: gx - rect.left, y: gy - rect.top }
    }
  }

  spriteManager.onSeatLeave = () => {
    hoveredSeat.value = null
  }

  if (props.template) renderer.loadTemplate(props.template)
  if (props.seats.length) spriteManager.update(props.seats)

  // Mouse wheel zoom
  const canvas = app.canvas as HTMLCanvasElement
  canvas.addEventListener('wheel', onWheel, { passive: false })

  // Drag-to-pan
  canvas.addEventListener('pointerdown', onDragStart)
  window.addEventListener('pointermove', onDragMove)
  window.addEventListener('pointerup', onDragEnd)
})

watch(() => props.template, (tmpl) => {
  if (tmpl && renderer) renderer.loadTemplate(tmpl)
})

watch(() => props.seats, (seats) => {
  if (spriteManager) spriteManager.update(seats)
}, { deep: true })

watch(() => props.isAdmin, (admin) => {
  renderer?.setAdminMode(admin)
})

function onWheel(e: WheelEvent) {
  e.preventDefault()
  if (!stage) return
  const delta = e.deltaY > 0 ? 0.9 : 1.1
  const newScale = Math.max(MIN_SCALE, Math.min(MAX_SCALE, currentScale * delta))

  const rect = (e.target as HTMLElement).getBoundingClientRect()
  const cx = e.clientX - rect.left
  const cy = e.clientY - rect.top
  const worldX = (cx - stage.x) / currentScale
  const worldY = (cy - stage.y) / currentScale

  currentScale = newScale
  stage.scale.set(currentScale)
  stage.x = cx - worldX * currentScale
  stage.y = cy - worldY * currentScale

  zoomed.value = Math.abs(currentScale - 1) > 0.05 || Math.abs(stage.x) > 5 || Math.abs(stage.y) > 5
}

function onDragStart(e: PointerEvent) {
  if (!stage) return
  isDragging = true
  dragStartX = e.clientX
  dragStartY = e.clientY
  stageStartX = stage.x
  stageStartY = stage.y
}

function onDragMove(e: PointerEvent) {
  if (!isDragging || !stage) return
  const dx = e.clientX - dragStartX
  const dy = e.clientY - dragStartY
  if (Math.abs(dx) > 3 || Math.abs(dy) > 3) {
    stage.x = stageStartX + dx
    stage.y = stageStartY + dy
    zoomed.value = true
  }
}

function onDragEnd() {
  isDragging = false
}

function zoomToSeat(seat: SeatData) {
  if (!stage || !canvasContainer.value) return
  const containerRect = canvasContainer.value.getBoundingClientRect()
  const targetZoom = 2.5
  const seatWorldX = seat.seat_x * 32 + 16
  const seatWorldY = seat.seat_y * 32 + 16

  const targetX = containerRect.width / 2 - seatWorldX * targetZoom
  const targetY = containerRect.height / 2 - seatWorldY * targetZoom

  const startX = stage.x
  const startY = stage.y
  const startScale = currentScale
  const startTime = performance.now()
  const duration = 300

  function animateZoom() {
    if (!stage) return
    const elapsed = performance.now() - startTime
    const progress = Math.min(1, elapsed / duration)
    const ease = easeOutCubic(progress)

    stage.x = startX + (targetX - startX) * ease
    stage.y = startY + (targetY - startY) * ease
    const s = startScale + (targetZoom - startScale) * ease
    stage.scale.set(s)
    currentScale = s

    if (progress < 1) {
      requestAnimationFrame(animateZoom)
    } else {
      zoomed.value = true
    }
  }
  requestAnimationFrame(animateZoom)
}

function resetZoom() {
  if (!stage) return
  const startX = stage.x
  const startY = stage.y
  const startScale = currentScale
  const startTime = performance.now()
  const duration = 300

  function animateReset() {
    if (!stage) return
    const elapsed = performance.now() - startTime
    const progress = Math.min(1, elapsed / duration)
    const ease = easeOutCubic(progress)

    stage.x = startX * (1 - ease)
    stage.y = startY * (1 - ease)
    const s = startScale + (1 - startScale) * ease
    stage.scale.set(s)
    currentScale = s

    if (progress < 1) {
      requestAnimationFrame(animateReset)
    } else {
      zoomed.value = false
    }
  }
  requestAnimationFrame(animateReset)
}

function setFilter(matchIds: number[] | null) {
  spriteManager?.setFilter(matchIds)
}

defineExpose({ setFilter })

function easeOutCubic(t: number): number {
  return 1 - Math.pow(1 - t, 3)
}

onBeforeUnmount(() => {
  if (app) {
    const canvas = app.canvas as HTMLCanvasElement
    canvas.removeEventListener('wheel', onWheel)
    canvas.removeEventListener('pointerdown', onDragStart)
  }
  window.removeEventListener('pointermove', onDragMove)
  window.removeEventListener('pointerup', onDragEnd)
  spriteManager?.destroy()
  renderer?.destroy()
  app?.destroy(true)
})
</script>

<style scoped>
.office-canvas {
  width: 100%;
  height: 100%;
  min-height: 400px;
  border-radius: 8px;
  overflow: hidden;
}
.reset-zoom-btn {
  position: absolute;
  bottom: 12px;
  right: 12px;
  padding: 4px 12px;
  font-size: 12px;
  border-radius: 6px;
  border: 1px solid #ddd;
  background: #fff;
  cursor: pointer;
  box-shadow: 0 1px 4px rgba(0,0,0,0.1);
  z-index: 10;
}
.reset-zoom-btn:hover {
  background: #f5f5f5;
}
</style>
