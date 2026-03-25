<template>
  <div ref="canvasContainer" class="office-canvas" />
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch } from 'vue'
import { Application } from 'pixi.js'
import { OfficeRenderer, type OfficeTemplate } from './OfficeRenderer'
import { SpriteManager, type SeatData } from './SpriteManager'

const props = defineProps<{
  template: OfficeTemplate | null
  seats: SeatData[]
}>()

const emit = defineEmits<{
  (e: 'select', seat: SeatData): void
}>()

const canvasContainer = ref<HTMLElement>()
let app: Application | null = null
let renderer: OfficeRenderer | null = null
let spriteManager: SpriteManager | null = null

onMounted(async () => {
  if (!canvasContainer.value) return

  app = new Application()
  await app.init({
    background: '#FAFAFA',
    resizeTo: canvasContainer.value,
    antialias: true,
  })
  canvasContainer.value.appendChild(app.canvas as HTMLCanvasElement)

  renderer = new OfficeRenderer(app)
  spriteManager = new SpriteManager(app.stage, 32)
  spriteManager.onSeatClick = (seat) => emit('select', seat)

  if (props.template) {
    renderer.loadTemplate(props.template)
  }
  if (props.seats.length) {
    spriteManager.update(props.seats)
  }
})

watch(() => props.template, (tmpl) => {
  if (tmpl && renderer) renderer.loadTemplate(tmpl)
})

watch(() => props.seats, (seats) => {
  if (spriteManager) spriteManager.update(seats)
}, { deep: true })

onBeforeUnmount(() => {
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
</style>
