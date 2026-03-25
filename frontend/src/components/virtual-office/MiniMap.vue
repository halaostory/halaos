<template>
  <div class="minimap" style="width: 150px; height: 100px; border: 1px solid #eee; border-radius: 4px; overflow: hidden; background: #fafafa">
    <canvas ref="miniCanvas" width="150" height="100" />
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import type { SeatData } from './SpriteManager'

const props = defineProps<{
  templateWidth: number
  templateHeight: number
  tileSize: number
  seats: SeatData[]
}>()

const miniCanvas = ref<HTMLCanvasElement>()

function draw() {
  const canvas = miniCanvas.value
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  if (!ctx) return

  const scaleX = 150 / (props.templateWidth * props.tileSize)
  const scaleY = 100 / (props.templateHeight * props.tileSize)
  const scale = Math.min(scaleX, scaleY)

  ctx.clearRect(0, 0, 150, 100)
  ctx.fillStyle = '#fafafa'
  ctx.fillRect(0, 0, 150, 100)

  for (const seat of props.seats) {
    if (seat.status === 'offline') continue
    const x = seat.seat_x * props.tileSize * scale
    const y = seat.seat_y * props.tileSize * scale
    ctx.fillStyle = seat.status === 'on_leave' ? '#d03050' : seat.avatar_color
    ctx.beginPath()
    ctx.arc(x + 3, y + 3, 3, 0, Math.PI * 2)
    ctx.fill()
  }
}

onMounted(draw)
watch(() => props.seats, draw, { deep: true })
</script>
