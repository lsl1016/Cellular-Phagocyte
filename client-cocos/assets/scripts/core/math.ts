// 纯函数集合：无引擎依赖，可在 Node 测试中直接复用。

/** 根据质量计算相机缩放比例。质量越大 scale 越小，视野越大。 */
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

/** 把"每帧 @60fps 的 lerp 系数"换算为任意帧率下的等效系数。 */
export function frameRateAdjusted(base: number, dt: number): number {
  return 1 - Math.pow(1 - base, dt * 60);
}

/**
 * 屏幕中心指向指针的方向（弧度，服务端约定 Y 向下）。
 * Cocos UI 坐标 Y 向上，故对 dy 取负翻转。
 * 返回值语义：0=右，+PI/2=屏幕下方（服务端世界的 +Y），-PI/2=屏幕上方。
 */
export function directionFromScreenCenter(
  visW: number,
  visH: number,
  pointerX: number,
  pointerY: number,
): number {
  return Math.atan2(-(pointerY - visH / 2), pointerX - visW / 2);
}

/**
 * 相机位置 clamp 到地图边界：靠近边缘时镜头不越出地图。
 * 若当前缩放下可见范围大于整张地图，则居中显示。
 */
export function clampCameraToMap(
  cx: number,
  cy: number,
  scale: number,
  visW: number,
  visH: number,
  worldW: number,
  worldH: number,
): [number, number] {
  const halfW = visW / 2 / scale;
  const halfH = visH / 2 / scale;
  const x =
    halfW * 2 >= worldW
      ? worldW / 2
      : Math.min(Math.max(cx, halfW), worldW - halfW);
  const y =
    halfH * 2 >= worldH
      ? worldH / 2
      : Math.min(Math.max(cy, halfH), worldH - halfH);
  return [x, y];
}
