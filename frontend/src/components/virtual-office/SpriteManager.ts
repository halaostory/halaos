import { Container, Graphics, Text, TextStyle } from 'pixi.js'

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
}

const STATUS_EMOJI: Record<string, string> = {
  working: '💻',
  overtime: '🔥',
  focused: '🎧',
  in_meeting: '🤝',
  on_break: '☕',
  away: '💤',
  on_leave: '🏥',
  offline: '',
}

export class SpriteManager {
  private sprites = new Map<number, SpriteEntry>()
  private container: Container
  private tileSize: number

  onSeatClick: ((seat: SeatData) => void) | null = null

  constructor(parentContainer: Container, tileSize: number) {
    this.tileSize = tileSize
    this.container = new Container()
    parentContainer.addChild(this.container)
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

  private createSprite(seat: SeatData) {
    const c = new Container()
    c.x = seat.seat_x * this.tileSize
    c.y = seat.seat_y * this.tileSize
    c.eventMode = 'static'
    c.cursor = 'pointer'
    c.on('pointertap', () => this.onSeatClick?.(seat))

    const avatar = new Graphics()
    const color = parseInt(seat.avatar_color.replace('#', ''), 16)
    avatar.circle(this.tileSize / 2, this.tileSize / 2, 12)
    avatar.fill(seat.status === 'on_leave' ? 0xBDBDBD : color)
    if (seat.status === 'away') avatar.alpha = 0.5
    c.addChild(avatar)

    const name = new Text({
      text: seat.name.split(' ')[0],
      style: new TextStyle({ fontSize: 8, fill: 0x333333, align: 'center' }),
    })
    name.x = this.tileSize / 2 - name.width / 2
    name.y = this.tileSize - 2
    c.addChild(name)

    const emoji = seat.custom_emoji ?? STATUS_EMOJI[seat.status] ?? ''
    if (emoji) {
      const bubble = new Text({
        text: emoji,
        style: new TextStyle({ fontSize: 12 }),
      })
      bubble.x = this.tileSize - 8
      bubble.y = -4
      c.addChild(bubble)
    }

    if (seat.custom_status) {
      const statusText = new Text({
        text: seat.custom_status.length > 12 ? seat.custom_status.slice(0, 12) + '…' : seat.custom_status,
        style: new TextStyle({ fontSize: 7, fill: 0x666666 }),
      })
      statusText.x = this.tileSize / 2 - statusText.width / 2
      statusText.y = -10
      c.addChild(statusText)
    }

    if (seat.is_late) {
      const late = new Text({
        text: '⚠️',
        style: new TextStyle({ fontSize: 8 }),
      })
      late.x = -4
      late.y = -4
      c.addChild(late)
    }

    this.container.addChild(c)
    this.sprites.set(seat.employee_id, { container: c, data: seat })
  }

  private updateSprite(entry: SpriteEntry, seat: SeatData) {
    if (JSON.stringify(entry.data) !== JSON.stringify(seat)) {
      entry.container.destroy({ children: true })
      this.sprites.delete(seat.employee_id)
      this.createSprite(seat)
    }
  }

  destroy() {
    this.container.destroy({ children: true })
    this.sprites.clear()
  }
}
