<template>
  <div class="minimap-wrap">
    <div class="minimap-title">Mini Map</div>
    <canvas ref="miniCanvas" width="160" height="110" />
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

const STATUS_COLORS: Record<string, string> = {
  working: '#4CAF50',
  overtime: '#FF9800',
  focused: '#2196F3',
  in_meeting: '#9C27B0',
  on_break: '#FF9800',
  away: '#9E9E9E',
  on_leave: '#E53935',
  offline: '#BDBDBD',
}

function draw() {
  const canvas = miniCanvas.value
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  if (!ctx) return

  const cw = 160
  const ch = 110
  const scaleX = cw / (props.templateWidth * props.tileSize)
  const scaleY = ch / (props.templateHeight * props.tileSize)
  const scale = Math.min(scaleX, scaleY)

  ctx.clearRect(0, 0, cw, ch)

  // Floor
  ctx.fillStyle = '#F5F0E8'
  ctx.beginPath()
  ctx.roundRect(0, 0, props.templateWidth * props.tileSize * scale, props.templateHeight * props.tileSize * scale, 4)
  ctx.fill()

  for (const seat of props.seats) {
    if (seat.status === 'offline') continue
    const x = seat.seat_x * props.tileSize * scale
    const y = seat.seat_y * props.tileSize * scale
    ctx.fillStyle = STATUS_COLORS[seat.status] ?? seat.avatar_color
    ctx.beginPath()
    ctx.arc(x + 4, y + 4, 4, 0, Math.PI * 2)
    ctx.fill()
  }
}

onMounted(draw)
watch(() => props.seats, draw, { deep: true })
</script>

<style scoped>
.minimap-wrap {
  background: #fff;
  border: 1px solid #f0f0f0;
  border-radius: 10px;
  padding: 8px;
  box-shadow: 0 1px 3px rgba(0,0,0,0.04);
}
.minimap-title {
  font-size: 11px;
  color: #999;
  font-weight: 600;
  margin-bottom: 4px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}
canvas {
  display: block;
  width: 100%;
  border-radius: 6px;
}
</style>
