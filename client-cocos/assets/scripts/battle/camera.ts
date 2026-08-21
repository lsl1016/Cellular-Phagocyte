// 世界相机：跟随自己并按质量缩放视野（越大看得越远）。
// 实现方式：控制 WorldRoot 节点的 position/scale，UI 相机保持不动，
// 与旧 Canvas 版的 worldToScreen 数学完全一致。

import { Node } from 'cc';
import { config } from '../core/config';
import { frameRateAdjusted } from '../core/math';

export interface WorldRect {
  x0: number;
  y0: number;
  x1: number;
  y1: number;
}

export class WorldCamera {
  x = 0; // 世界坐标，画面中心（y 向下，与服务端一致）
  y = 0;
  scale = 1;

  /** 平滑跟随目标。lerp 为 60fps 基准系数，按 dt 换算。 */
  follow(targetX: number, targetY: number, targetScale: number, lerp: number, dt: number): void {
    const f = frameRateAdjusted(lerp, dt);
    this.x += (targetX - this.x) * f;
    this.y += (targetY - this.y) * f;
    this.scale += (targetScale - this.scale) * f;
  }

  snap(targetX: number, targetY: number, targetScale: number): void {
    this.x = targetX;
    this.y = targetY;
    this.scale = targetScale;
  }

  /** 应用到 WorldRoot 节点：世界点 (wx,wy) -> 节点 (wx, H-wy)，画布中心对准相机中心。 */
  apply(worldRoot: Node): void {
    worldRoot.setScale(this.scale, this.scale, 1);
    worldRoot.setPosition(
      -this.x * this.scale,
      -(config.worldHeight - this.y) * this.scale,
      0,
    );
  }

  /** 当前可见世界矩形（含 margin，用于裁剪）。 */
  visibleRect(visW: number, visH: number, margin = 80): WorldRect {
    const halfW = (visW / 2) / this.scale + margin;
    const halfH = (visH / 2) / this.scale + margin;
    return {
      x0: this.x - halfW,
      y0: this.y - halfH,
      x1: this.x + halfW,
      y1: this.y + halfH,
    };
  }
}
