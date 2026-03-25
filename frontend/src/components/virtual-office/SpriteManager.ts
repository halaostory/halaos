import { Container, Graphics, Text, TextStyle, Ticker } from 'pixi.js'

export interface SeatData {
  seat_id: number
  employee_id: number
  name: string
  position: string
  department: string
  floor: number
  zone: string
  seat_x: number
  seat_y: number
  avatar_type: string
  avatar_color: string
  status: string
  is_late: boolean
  custom_status: string | null
  custom_emoji: string | null
  clock_in_at: string | null
  leave_type: string | null
  meeting_room_zone: string | null
}

interface SpriteEntry {
  container: Container
  data: SeatData
  ring: Graphics
  targetRingColor: number
  currentRingAlpha: number
  entranceProgress: number
}

const STATUS_RING: Record<string, number> = {
  working: 0x4CAF50,
  overtime: 0xFF9800,
  focused: 0x2196F3,
  in_meeting: 0x9C27B0,
  on_break: 0xFF9800,
  away: 0x9E9E9E,
  on_leave: 0xE53935,
  offline: 0xBDBDBD,
}

const STATUS_EMOJI: Record<string, string> = {
  working: '',
  overtime: '\uD83D\uDD25',
  focused: '\uD83C\uDFA7',
  in_meeting: '\uD83E\uDD1D',
  on_break: '\u2615',
  away: '\uD83D\uDCA4',
  on_leave: '\uD83C\uDFD6\uFE0F',
  offline: '',
}

const FONT = '-apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif'

export class SpriteManager {
  private sprites = new Map<number, SpriteEntry>()
  private container: Container
  private tileSize: number
  private filterIds: Set<number> | null = null
  private ticker: Ticker
  private focusedTime = 0

  onSeatClick: ((seat: SeatData) => void) | null = null
  onSeatHover: ((seat: SeatData, globalX: number, globalY: number) => void) | null = null
  onSeatLeave: (() => void) | null = null

  constructor(parentContainer: Container, tileSize: number) {
    this.tileSize = tileSize
    this.container = new Container()
    parentContainer.addChild(this.container)
    this.ticker = new Ticker()
    this.ticker.add(this.animate, this)
    this.ticker.start()
  }

  /** Set filter: pass matching employee IDs to highlight, null to clear filter */
  setFilter(matchIds: number[] | null) {
    this.filterIds = matchIds ? new Set(matchIds) : null
    for (const [id, entry] of this.sprites) {
      if (!this.filterIds) {
        entry.container.alpha = 1
      } else {
        entry.container.alpha = this.filterIds.has(id) ? 1 : 0.15
      }
    }
  }

  update(seats: SeatData[]) {
    const currentIds = new Set(seats.map(s => s.employee_id))

    for (const [id, entry] of this.sprites) {
      if (!currentIds.has(id)) {
        entry.container.destroy({ children: true })
        this.sprites.delete(id)
      }
    }

    for (const seat of seats) {
      if (seat.status === 'offline' && !seat.custom_status) {
        const existing = this.sprites.get(seat.employee_id)
        if (existing) {
          existing.container.destroy({ children: true })
          this.sprites.delete(seat.employee_id)
        }
        continue
      }

      const existing = this.sprites.get(seat.employee_id)
      if (existing) {
        this.updateSprite(existing, seat)
      } else {
        this.createSprite(seat)
      }
    }
  }

  /** Get the screen bounds for a seat sprite (for tooltip positioning) */
  getSeatBounds(employeeId: number): { x: number; y: number; width: number; height: number } | null {
    const entry = this.sprites.get(employeeId)
    if (!entry) return null
    const bounds = entry.container.getBounds()
    return { x: bounds.x, y: bounds.y, width: bounds.width, height: bounds.height }
  }

  private createSprite(seat: SeatData) {
    const c = new Container()
    c.x = seat.seat_x * this.tileSize
    c.y = seat.seat_y * this.tileSize
    c.eventMode = 'static'
    c.cursor = 'pointer'
    c.on('pointertap', () => this.onSeatClick?.(seat))
    c.on('pointerover', (e) => {
      this.onSeatHover?.(seat, e.globalX, e.globalY)
    })
    c.on('pointerout', () => {
      this.onSeatLeave?.()
    })

    const cx = this.tileSize / 2
    const cy = this.tileSize / 2
    const radius = 13

    // Shadow
    const shadow = new Graphics()
    shadow.circle(cx + 1, cy + 1, radius + 2)
    shadow.fill({ color: 0x000000, alpha: 0.12 })
    c.addChild(shadow)

    // Status ring
    const ringColor = STATUS_RING[seat.status] ?? 0xBDBDBD
    const ring = new Graphics()
    ring.circle(cx, cy, radius + 2)
    ring.fill(ringColor)
    c.addChild(ring)

    // Avatar circle
    const color = parseInt(seat.avatar_color.replace('#', ''), 16)
    const avatar = new Graphics()
    avatar.circle(cx, cy, radius)
    avatar.fill(seat.status === 'on_leave' ? 0xCFD8DC : color)
    if (seat.status === 'away') avatar.alpha = 0.6
    c.addChild(avatar)

    // Initials
    const names = seat.name.split(' ')
    const initials = names.length >= 2
      ? (names[0][0] + names[names.length - 1][0]).toUpperCase()
      : names[0].substring(0, 2).toUpperCase()
    const initialText = new Text({
      text: initials,
      style: new TextStyle({
        fontSize: 11,
        fontFamily: FONT,
        fill: 0xFFFFFF,
        fontWeight: '700',
        align: 'center',
      }),
    })
    initialText.x = cx - initialText.width / 2
    initialText.y = cy - initialText.height / 2
    c.addChild(initialText)

    // Name label with background pill
    const firstName = seat.name.split(' ')[0]
    const nameText = new Text({
      text: firstName,
      style: new TextStyle({
        fontSize: 9,
        fontFamily: FONT,
        fill: 0x444444,
        fontWeight: '500',
        align: 'center',
      }),
    })
    const pillPad = 3
    const pillW = nameText.width + pillPad * 2
    const pillH = nameText.height + 2
    const pillX = cx - pillW / 2
    const pillY = this.tileSize + 2

    const pill = new Graphics()
    pill.roundRect(pillX, pillY, pillW, pillH, 4)
    pill.fill({ color: 0xFFFFFF, alpha: 0.9 })
    pill.stroke({ color: 0xE0E0E0, width: 0.5 })
    c.addChild(pill)

    nameText.x = pillX + pillPad
    nameText.y = pillY + 1
    c.addChild(nameText)

    // Status emoji badge
    const emoji = seat.custom_emoji ?? STATUS_EMOJI[seat.status] ?? ''
    if (emoji) {
      const badgeBg = new Graphics()
      badgeBg.circle(this.tileSize - 2, 2, 8)
      badgeBg.fill({ color: 0xFFFFFF, alpha: 0.95 })
      c.addChild(badgeBg)

      const badge = new Text({
        text: emoji,
        style: new TextStyle({ fontSize: 10 }),
      })
      badge.x = this.tileSize - 2 - badge.width / 2
      badge.y = 2 - badge.height / 2
      c.addChild(badge)
    }

    // Custom status bubble
    if (seat.custom_status) {
      const statusStr = seat.custom_status.length > 14
        ? seat.custom_status.slice(0, 14) + '\u2026'
        : seat.custom_status
      const statusText = new Text({
        text: statusStr,
        style: new TextStyle({
          fontSize: 8,
          fontFamily: FONT,
          fill: 0x555555,
          fontWeight: '400',
        }),
      })
      const bubblePad = 4
      const bubbleW = statusText.width + bubblePad * 2
      const bubbleH = statusText.height + 3
      const bubbleX = cx - bubbleW / 2
      const bubbleY = -bubbleH - 4

      const bubble = new Graphics()
      bubble.roundRect(bubbleX, bubbleY, bubbleW, bubbleH, 6)
      bubble.fill({ color: 0xFFFFFF, alpha: 0.95 })
      bubble.stroke({ color: 0xE0E0E0, width: 0.5 })
      c.addChild(bubble)

      statusText.x = bubbleX + bubblePad
      statusText.y = bubbleY + 1
      c.addChild(statusText)
    }

    // Late indicator
    if (seat.is_late) {
      const lateBg = new Graphics()
      lateBg.circle(2, 2, 7)
      lateBg.fill(0xFFF3E0)
      lateBg.stroke({ color: 0xFF9800, width: 1 })
      c.addChild(lateBg)
      const late = new Text({
        text: '!',
        style: new TextStyle({ fontSize: 9, fill: 0xFF9800, fontWeight: '700', fontFamily: FONT }),
      })
      late.x = 2 - late.width / 2
      late.y = 2 - late.height / 2
      c.addChild(late)
    }

    // Entrance animation: start small and transparent
    c.alpha = 0
    c.scale.set(0.5)

    this.container.addChild(c)

    // Apply filter dimming if active
    if (this.filterIds && !this.filterIds.has(seat.employee_id)) {
      c.alpha = 0.15
    }

    this.sprites.set(seat.employee_id, {
      container: c,
      data: seat,
      ring,
      targetRingColor: ringColor,
      currentRingAlpha: 1,
      entranceProgress: 0,
    })
  }

  private updateSprite(entry: SpriteEntry, seat: SeatData) {
    if (JSON.stringify(entry.data) !== JSON.stringify(seat)) {
      entry.container.destroy({ children: true })
      this.sprites.delete(seat.employee_id)
      this.createSprite(seat)
    }
  }

  private animate = () => {
    this.focusedTime += 0.02

    for (const [, entry] of this.sprites) {
      // Entrance animation (200ms = ~12 frames at 60fps)
      if (entry.entranceProgress < 1) {
        entry.entranceProgress = Math.min(1, entry.entranceProgress + 0.08)
        const t = easeOutCubic(entry.entranceProgress)

        // Only animate alpha if not filtered out
        const isFiltered = this.filterIds && !this.filterIds.has(entry.data.employee_id)
        if (!isFiltered) {
          entry.container.alpha = t
        }
        entry.container.scale.set(0.5 + 0.5 * t)
      }

      // Focused pulse: oscillate ring alpha between 0.4 and 1.0
      if (entry.data.status === 'focused') {
        const pulse = 0.7 + 0.3 * Math.sin(this.focusedTime * 3)
        entry.ring.alpha = pulse
      }
    }
  }

  destroy() {
    this.ticker.stop()
    this.ticker.destroy()
    this.container.destroy({ children: true })
    this.sprites.clear()
  }
}

function easeOutCubic(t: number): number {
  return 1 - Math.pow(1 - t, 3)
}
