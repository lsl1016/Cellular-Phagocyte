// Canvas 渲染器：绘制地图网格、食物、玩家球体、昵称与自身高亮。
// 对球体位置做轻量平滑（向快照目标插值），缓解 10Hz 快照的跳变。

import { config } from '../app/config.js';
import type { GameState } from '../state/game-state.js';
import { Camera } from './camera.js';

interface DisplayBall {
  x: number;
  y: number;
  radius: number;
  mass: number;
  color: string;
  nickname: string;
  isSelf: boolean;
}

export class Renderer {
  private ctx: CanvasRenderingContext2D;
  private display = new Map<string, DisplayBall>();

  constructor(private canvas: HTMLCanvasElement) {
    const ctx = canvas.getContext('2d');
    if (!ctx) throw new Error('无法获取 2D 画布上下文');
    this.ctx = ctx;
  }

  resize(): void {
    const dpr = window.devicePixelRatio || 1;
    this.canvas.width = Math.floor(window.innerWidth * dpr);
    this.canvas.height = Math.floor(window.innerHeight * dpr);
    this.canvas.style.width = window.innerWidth + 'px';
    this.canvas.style.height = window.innerHeight + 'px';
  }

  render(state: GameState, camera: Camera): void {
    const ctx = this.ctx;
    const w = this.canvas.width;
    const h = this.canvas.height;

    ctx.fillStyle = '#0b1020';
    ctx.fillRect(0, 0, w, h);

    this.drawGrid(camera, w, h);
    this.drawBorder(camera, w, h);
    this.drawFoods(state, camera, w, h);
    this.drawEjected(state, camera, w, h);
    this.syncDisplay(state);
    this.drawBalls(camera, w, h);
  }

  private drawEjected(state: GameState, camera: Camera, w: number, h: number): void {
    const ctx = this.ctx;
    for (const e of state.ejected.values()) {
      const [sx, sy] = camera.worldToScreen(e.x, e.y, w, h);
      if (sx < -10 || sy < -10 || sx > w + 10 || sy > h + 10) continue;
      const r = Math.max(3, e.radius * camera.scale);
      ctx.fillStyle = colorFor(e.ownerId);
      ctx.beginPath();
      ctx.arc(sx, sy, r, 0, Math.PI * 2);
      ctx.fill();
      ctx.strokeStyle = 'rgba(255,255,255,0.5)';
      ctx.lineWidth = 1;
      ctx.stroke();
    }
  }

  private drawGrid(camera: Camera, w: number, h: number): void {
    const ctx = this.ctx;
    const step = 200;
    ctx.strokeStyle = 'rgba(255,255,255,0.05)';
    ctx.lineWidth = 1;
    const startX = Math.floor((camera.x - w / 2 / camera.scale) / step) * step;
    const endX = camera.x + w / 2 / camera.scale;
    for (let gx = startX; gx <= endX; gx += step) {
      const [sx] = camera.worldToScreen(gx, 0, w, h);
      ctx.beginPath();
      ctx.moveTo(sx, 0);
      ctx.lineTo(sx, h);
      ctx.stroke();
    }
    const startY = Math.floor((camera.y - h / 2 / camera.scale) / step) * step;
    const endY = camera.y + h / 2 / camera.scale;
    for (let gy = startY; gy <= endY; gy += step) {
      const [, sy] = camera.worldToScreen(0, gy, w, h);
      ctx.beginPath();
      ctx.moveTo(0, sy);
      ctx.lineTo(w, sy);
      ctx.stroke();
    }
  }

  private drawBorder(camera: Camera, w: number, h: number): void {
    const ctx = this.ctx;
    const [x0, y0] = camera.worldToScreen(0, 0, w, h);
    const [x1, y1] = camera.worldToScreen(config.worldWidth, config.worldHeight, w, h);
    ctx.strokeStyle = 'rgba(106,209,255,0.4)';
    ctx.lineWidth = 2;
    ctx.strokeRect(x0, y0, x1 - x0, y1 - y0);
  }

  private drawFoods(state: GameState, camera: Camera, w: number, h: number): void {
    const ctx = this.ctx;
    for (const f of state.foods.values()) {
      const [sx, sy] = camera.worldToScreen(f.x, f.y, w, h);
      if (sx < -10 || sy < -10 || sx > w + 10 || sy > h + 10) continue;
      const r = Math.max(2, 5 * camera.scale);
      ctx.fillStyle = f.color || '#7fd';
      ctx.beginPath();
      ctx.arc(sx, sy, r, 0, Math.PI * 2);
      ctx.fill();
    }
  }

  /** 将快照玩家同步进 display，并对位置插值；移除已消失的球。 */
  private syncDisplay(state: GameState): void {
    const seen = new Set<string>();
    for (const p of state.players.values()) {
      const isSelf = p.userId === state.selfUserId;
      for (const b of p.balls) {
        seen.add(b.ballId);
        const color = colorFor(p.userId);
        const cur = this.display.get(b.ballId);
        if (!cur) {
          this.display.set(b.ballId, {
            x: b.x, y: b.y, radius: b.radius, mass: b.mass, color, nickname: p.nickname, isSelf,
          });
        } else {
          cur.x += (b.x - cur.x) * 0.3;
          cur.y += (b.y - cur.y) * 0.3;
          cur.radius += (b.radius - cur.radius) * 0.3;
          cur.mass = b.mass;
          cur.isSelf = isSelf;
          cur.nickname = p.nickname;
        }
      }
    }
    for (const id of this.display.keys()) {
      if (!seen.has(id)) this.display.delete(id);
    }
  }

  private drawBalls(camera: Camera, w: number, h: number): void {
    const ctx = this.ctx;
    const balls = [...this.display.values()].sort((a, b) => a.mass - b.mass);
    for (const b of balls) {
      const [sx, sy] = camera.worldToScreen(b.x, b.y, w, h);
      const r = Math.max(4, b.radius * camera.scale);
      if (sx < -r || sy < -r || sx > w + r || sy > h + r) continue;

      ctx.fillStyle = b.color;
      ctx.beginPath();
      ctx.arc(sx, sy, r, 0, Math.PI * 2);
      ctx.fill();

      if (b.isSelf) {
        ctx.strokeStyle = '#ffffff';
        ctx.lineWidth = 3;
        ctx.stroke();
      }

      if (r > 14) {
        ctx.fillStyle = 'rgba(0,0,0,0.85)';
        ctx.font = `${Math.max(11, Math.min(r * 0.5, 22))}px sans-serif`;
        ctx.textAlign = 'center';
        ctx.textBaseline = 'middle';
        ctx.fillText(b.nickname, sx, sy - r * 0.15);
        ctx.fillText(String(Math.round(b.mass)), sx, sy + r * 0.4);
      }
    }
  }
}

function colorFor(userId: string): string {
  let hsh = 0;
  for (let i = 0; i < userId.length; i++) {
    hsh = (hsh * 31 + userId.charCodeAt(i)) % 360;
  }
  return `hsl(${hsh}, 65%, 55%)`;
}
