// 输入：把指针位置换算成移动方向，并按节流频率发送 MOVE；
// 同时支持分裂(Space)与吐球(W)，方向取当前指针方向。
// 摄像机始终把自己居中，故方向 = 屏幕中心指向指针。

import { config } from '../app/config.js';

/** 纯函数：屏幕中心指向指针的方向（弧度）。 */
export function directionFromScreenCenter(
  canvasW: number,
  canvasH: number,
  pointerX: number,
  pointerY: number,
): number {
  return Math.atan2(pointerY - canvasH / 2, pointerX - canvasW / 2);
}

export interface InputCallbacks {
  move: (direction: number) => void;
  split: (direction: number) => void;
  eject: (direction: number) => void;
}

export class InputController {
  private canvas: HTMLCanvasElement;
  private pointerX = 0;
  private pointerY = 0;
  private active = false;
  private timer: ReturnType<typeof setInterval> | null = null;
  private cb: InputCallbacks;

  private onMove = (e: PointerEvent) => {
    const rect = this.canvas.getBoundingClientRect();
    this.pointerX = e.clientX - rect.left;
    this.pointerY = e.clientY - rect.top;
    this.active = true;
  };

  private onKey = (e: KeyboardEvent) => {
    if (e.code === 'Space') {
      e.preventDefault();
      this.cb.split(this.currentDirection());
    } else if (e.code === 'KeyW') {
      this.cb.eject(this.currentDirection());
    }
  };

  constructor(canvas: HTMLCanvasElement, cb: InputCallbacks) {
    this.canvas = canvas;
    this.cb = cb;
    this.pointerX = canvas.width / 2 + 1;
    this.pointerY = canvas.height / 2;
  }

  /** 当前指针对应的移动方向（供分裂/吐球按钮复用）。 */
  currentDirection(): number {
    return directionFromScreenCenter(this.canvas.width, this.canvas.height, this.pointerX, this.pointerY);
  }

  start(): void {
    this.canvas.addEventListener('pointermove', this.onMove);
    this.canvas.addEventListener('pointerdown', this.onMove);
    window.addEventListener('keydown', this.onKey);
    this.timer = setInterval(() => {
      if (!this.active) return;
      this.cb.move(this.currentDirection());
    }, config.inputSendIntervalMs);
  }

  stop(): void {
    this.canvas.removeEventListener('pointermove', this.onMove);
    this.canvas.removeEventListener('pointerdown', this.onMove);
    window.removeEventListener('keydown', this.onKey);
    if (this.timer !== null) {
      clearInterval(this.timer);
      this.timer = null;
    }
  }
}
