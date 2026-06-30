// 摄像机：跟随自己并按质量缩放视野（越大看得越远）。

/**
 * 纯函数：根据质量计算缩放比例（每世界单位对应的像素数）。
 * 质量越大，scale 越小，视野越大。在 [minScale, maxScale] 内夹紧。
 */
export function zoomForMass(
  mass: number,
  baseMass: number,
  baseScale: number,
  minScale = 0.25,
  maxScale = 2.5,
): number {
  const m = Math.max(mass, baseMass);
  const scale = baseScale * Math.sqrt(baseMass / m);
  return Math.min(maxScale, Math.max(minScale, scale));
}

export class Camera {
  x = 0; // 世界坐标，画面中心
  y = 0;
  scale = 1;

  /** 平滑跟随目标世界坐标与目标缩放。 */
  follow(targetX: number, targetY: number, targetScale: number, lerp: number): void {
    this.x += (targetX - this.x) * lerp;
    this.y += (targetY - this.y) * lerp;
    this.scale += (targetScale - this.scale) * lerp;
  }

  snap(targetX: number, targetY: number, targetScale: number): void {
    this.x = targetX;
    this.y = targetY;
    this.scale = targetScale;
  }

  /** 世界坐标 -> 屏幕坐标。 */
  worldToScreen(wx: number, wy: number, canvasW: number, canvasH: number): [number, number] {
    return [
      (wx - this.x) * this.scale + canvasW / 2,
      (wy - this.y) * this.scale + canvasH / 2,
    ];
  }
}
