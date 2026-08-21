"use strict";
// 纯函数集合：无引擎依赖，可在 Node 测试中直接复用。
Object.defineProperty(exports, "__esModule", { value: true });
exports.zoomForMass = zoomForMass;
exports.frameRateAdjusted = frameRateAdjusted;
exports.directionFromScreenCenter = directionFromScreenCenter;
/** 根据质量计算相机缩放比例。质量越大 scale 越小，视野越大。 */
function zoomForMass(mass, baseMass, baseScale, minScale = 0.25, maxScale = 2.5) {
    const m = Math.max(mass, baseMass);
    const scale = baseScale * Math.sqrt(baseMass / m);
    return Math.min(maxScale, Math.max(minScale, scale));
}
/** 把"每帧 @60fps 的 lerp 系数"换算为任意帧率下的等效系数。 */
function frameRateAdjusted(base, dt) {
    return 1 - Math.pow(1 - base, dt * 60);
}
/**
 * 屏幕中心指向指针的方向（弧度，服务端约定 Y 向下）。
 * Cocos UI 坐标 Y 向上，故对 dy 取负翻转。
 * 返回值语义：0=右，+PI/2=屏幕下方（服务端世界的 +Y），-PI/2=屏幕上方。
 */
function directionFromScreenCenter(visW, visH, pointerX, pointerY) {
    return Math.atan2(-(pointerY - visH / 2), pointerX - visW / 2);
}
