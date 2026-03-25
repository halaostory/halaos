import { Application, Container, Graphics, Text, TextStyle } from 'pixi.js'

export interface TemplateZone {
  id: string
  type: string
  label: string
  x: number; y: number; w: number; h: number
  capacity?: number
  seats?: { x: number; y: number }[]
  meeting_seats?: { x: number; y: number }[]
}

export interface OfficeTemplate {
  id: string
  name: string
  width: number
  height: number
  tileSize: number
  zones: TemplateZone[]
  furniture: { type: string; x: number; y: number }[]
}

const ZONE_STYLES: Record<string, { bg: number; border: number; labelColor: number; icon: string }> = {
  desk_area:    { bg: 0xF8F4E8, border: 0xD4C89A, labelColor: 0x8B7D5B, icon: '🖥️' },
  meeting_room: { bg: 0xE8F5E9, border: 0xA5D6A7, labelColor: 0x4E8C51, icon: '🤝' },
  cafe:         { bg: 0xFFF8E1, border: 0xFFD54F, labelColor: 0x9E8430, icon: '☕' },
  lounge:       { bg: 0xE3F2FD, border: 0x90CAF9, labelColor: 0x5083AB, icon: '🛋️' },
  phone_booth:  { bg: 0xFCE4EC, border: 0xF48FB1, labelColor: 0xAD4E6E, icon: '📞' },
}

export class OfficeRenderer {
  private container: Container
  private tileSize: number = 32
  private template: OfficeTemplate | null = null
  private adminMode: boolean = false
  onEmptySeatClick: ((position: { floor: number; zone: string; seat_x: number; seat_y: number }) => void) | null = null

  constructor(app: Application) {
    this.container = new Container()
    app.stage.addChild(this.container)
  }

  loadTemplate(template: OfficeTemplate) {
    this.template = template
    this.tileSize = template.tileSize
    this.container.removeChildren()
    this.drawFloor()
    this.drawZones()
    this.drawFurniture()
  }

  getTemplate(): OfficeTemplate | null {
    return this.template
  }

  private drawFloor() {
    if (!this.template) return
    const w = this.template.width * this.tileSize
    const h = this.template.height * this.tileSize

    // Floor with subtle gradient feel
    const floor = new Graphics()
    floor.roundRect(0, 0, w, h, 12)
    floor.fill(0xF5F0E8)
    this.container.addChild(floor)

    // Subtle tile pattern
    const tiles = new Graphics()
    for (let x = 0; x < this.template.width; x++) {
      for (let y = 0; y < this.template.height; y++) {
        if ((x + y) % 2 === 0) {
          tiles.rect(x * this.tileSize, y * this.tileSize, this.tileSize, this.tileSize)
          tiles.fill({ color: 0xEDE8DC, alpha: 0.4 })
        }
      }
    }
    this.container.addChild(tiles)
  }

  private drawZones() {
    if (!this.template) return
    for (const zone of this.template.zones) {
      const style = ZONE_STYLES[zone.type] ?? { bg: 0xF5F5F5, border: 0xBDBDBD, labelColor: 0x757575, icon: '' }
      const zx = zone.x * this.tileSize
      const zy = zone.y * this.tileSize
      const zw = zone.w * this.tileSize
      const zh = zone.h * this.tileSize

      // Zone shadow
      const shadow = new Graphics()
      shadow.roundRect(zx + 2, zy + 2, zw, zh, 8)
      shadow.fill({ color: 0x000000, alpha: 0.06 })
      this.container.addChild(shadow)

      // Zone background
      const g = new Graphics()
      g.roundRect(zx, zy, zw, zh, 8)
      g.fill({ color: style.bg, alpha: 0.85 })
      g.stroke({ color: style.border, width: 1.5 })
      this.container.addChild(g)

      // Zone label with icon
      const labelText = `${style.icon} ${zone.label}`
      const label = new Text({
        text: labelText,
        style: new TextStyle({
          fontSize: 11,
          fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif',
          fill: style.labelColor,
          fontWeight: '600',
        }),
      })
      label.x = zx + 8
      label.y = zy + 5
      this.container.addChild(label)

      // Desk seat indicators (subtle dots)
      const seats = zone.seats ?? zone.meeting_seats ?? []
      for (const s of seats) {
        const marker = new Graphics()
        const cx = s.x * this.tileSize + this.tileSize / 2
        const cy = s.y * this.tileSize + this.tileSize / 2
        const radius = this.adminMode ? 6 : 4
        marker.circle(cx, cy, radius)
        marker.fill({ color: style.border, alpha: this.adminMode ? 0.5 : 0.3 })
        if (this.adminMode) {
          marker.eventMode = 'static'
          marker.cursor = 'pointer'
          const zoneId = zone.id
          const seatX = s.x
          const seatY = s.y
          marker.on('pointertap', () => {
            this.onEmptySeatClick?.({ floor: 1, zone: zoneId, seat_x: seatX, seat_y: seatY })
          })
        }
        this.container.addChild(marker)
      }
    }
  }

  private drawFurniture() {
    if (!this.template) return
    for (const f of this.template.furniture) {
      const fx = f.x * this.tileSize
      const fy = f.y * this.tileSize

      if (f.type === 'plant') {
        // Draw a nicer plant
        const plant = new Graphics()
        // Pot
        plant.roundRect(fx + 8, fy + 18, 16, 10, 3)
        plant.fill(0xA1887F)
        // Leaves
        plant.circle(fx + 16, fy + 14, 8)
        plant.fill(0x66BB6A)
        plant.circle(fx + 12, fy + 10, 5)
        plant.fill(0x81C784)
        plant.circle(fx + 20, fy + 11, 5)
        plant.fill(0x4CAF50)
        this.container.addChild(plant)
      } else if (f.type === 'whiteboard') {
        const wb = new Graphics()
        wb.roundRect(fx + 2, fy + 2, this.tileSize - 4, this.tileSize - 4, 3)
        wb.fill(0xFFFFFF)
        wb.stroke({ color: 0xBDBDBD, width: 1.5 })
        // Small lines on whiteboard
        wb.moveTo(fx + 6, fy + 10)
        wb.lineTo(fx + 22, fy + 10)
        wb.stroke({ color: 0xE0E0E0, width: 1 })
        wb.moveTo(fx + 6, fy + 16)
        wb.lineTo(fx + 18, fy + 16)
        wb.stroke({ color: 0xE0E0E0, width: 1 })
        this.container.addChild(wb)
      } else if (f.type === 'coffee_machine') {
        const cm = new Graphics()
        cm.roundRect(fx + 6, fy + 4, 20, 24, 4)
        cm.fill(0x5D4037)
        cm.roundRect(fx + 9, fy + 8, 14, 10, 2)
        cm.fill(0x3E2723)
        // Steam
        const steam = new Text({
          text: '~',
          style: new TextStyle({ fontSize: 10, fill: 0xBDBDBD }),
        })
        steam.x = fx + 14
        steam.y = fy - 4
        this.container.addChild(cm)
        this.container.addChild(steam)
      } else if (f.type === 'desk') {
        // Draw desk as a rounded rectangle
        const desk = new Graphics()
        desk.roundRect(fx + 2, fy + 2, this.tileSize - 4, this.tileSize - 4, 4)
        desk.fill(0xBCAAA4)
        desk.stroke({ color: 0xA1887F, width: 1 })
        this.container.addChild(desk)
      }
    }
  }

  setAdminMode(enabled: boolean) {
    this.adminMode = enabled
    if (this.template) this.loadTemplate(this.template)
  }

  destroy() {
    this.container.destroy({ children: true })
  }
}
