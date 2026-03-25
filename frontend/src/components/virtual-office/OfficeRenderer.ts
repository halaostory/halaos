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

const ZONE_COLORS: Record<string, number> = {
  desk_area: 0xF5F5DC,
  meeting_room: 0xE8F5E9,
  cafe: 0xFFF3E0,
  lounge: 0xE3F2FD,
  phone_booth: 0xFCE4EC,
}

const FURNITURE_COLORS: Record<string, number> = {
  desk: 0x8D6E63,
  plant: 0x4CAF50,
  whiteboard: 0xEEEEEE,
  coffee_machine: 0x795548,
  computer: 0x37474F,
}

export class OfficeRenderer {
  private container: Container
  private tileSize: number = 32
  private template: OfficeTemplate | null = null

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
    this.drawGrid()
  }

  getTemplate(): OfficeTemplate | null {
    return this.template
  }

  private drawFloor() {
    if (!this.template) return
    const floor = new Graphics()
    floor.rect(0, 0, this.template.width * this.tileSize, this.template.height * this.tileSize)
    floor.fill(0xFAFAFA)
    floor.stroke({ color: 0xE0E0E0, width: 1 })
    this.container.addChild(floor)
  }

  private drawZones() {
    if (!this.template) return
    for (const zone of this.template.zones) {
      const g = new Graphics()
      const color = ZONE_COLORS[zone.type] ?? 0xF5F5F5
      g.rect(
        zone.x * this.tileSize,
        zone.y * this.tileSize,
        zone.w * this.tileSize,
        zone.h * this.tileSize,
      )
      g.fill({ color, alpha: 0.5 })
      g.stroke({ color: 0xBDBDBD, width: 1 })
      this.container.addChild(g)

      const label = new Text({
        text: zone.label,
        style: new TextStyle({ fontSize: 10, fill: 0x757575 }),
      })
      label.x = zone.x * this.tileSize + 4
      label.y = zone.y * this.tileSize + 2
      this.container.addChild(label)

      const seats = zone.seats ?? zone.meeting_seats ?? []
      for (const s of seats) {
        const marker = new Graphics()
        marker.circle(s.x * this.tileSize + this.tileSize / 2, s.y * this.tileSize + this.tileSize / 2, 6)
        marker.fill({ color: 0xE0E0E0, alpha: 0.5 })
        this.container.addChild(marker)
      }
    }
  }

  private drawFurniture() {
    if (!this.template) return
    for (const f of this.template.furniture) {
      const g = new Graphics()
      const color = FURNITURE_COLORS[f.type] ?? 0x9E9E9E
      g.rect(
        f.x * this.tileSize + 4,
        f.y * this.tileSize + 4,
        this.tileSize - 8,
        this.tileSize - 8,
      )
      g.fill(color)
      this.container.addChild(g)
    }
  }

  private drawGrid() {
    if (!this.template) return
    const grid = new Graphics()
    for (let x = 0; x <= this.template.width; x++) {
      grid.moveTo(x * this.tileSize, 0)
      grid.lineTo(x * this.tileSize, this.template.height * this.tileSize)
    }
    for (let y = 0; y <= this.template.height; y++) {
      grid.moveTo(0, y * this.tileSize)
      grid.lineTo(this.template.width * this.tileSize, y * this.tileSize)
    }
    grid.stroke({ color: 0xF0F0F0, width: 0.5 })
    this.container.addChild(grid)
  }

  destroy() {
    this.container.destroy({ children: true })
  }
}
