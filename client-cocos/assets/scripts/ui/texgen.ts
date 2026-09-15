// 程序化生成纹理：圆 / 圆环 / 圆角矩形 / 纯色块 / 纵向渐变。
// 全部使用 Texture2D 直接上传 RGBA 像素，兼容 Web 预览与 JSB / Simulator。

import { Color, SpriteFrame, Texture2D } from 'cc';

function makeSpriteFrame(
  w: number,
  h: number,
  rgba: (x: number, y: number) => [number, number, number, number],
): SpriteFrame {
  const data = new Uint8Array(w * h * 4);
  for (let y = 0; y < h; y++) {
    for (let x = 0; x < w; x++) {
      const i = (y * w + x) * 4;
      const [r, g, b, a] = rgba(x, y);
      data[i] = r;
      data[i + 1] = g;
      data[i + 2] = b;
      data[i + 3] = a;
    }
  }

  // 不经 ImageAsset.reset({_data: Uint8Array})：该路径依赖平台图像实现，
  // Web 预览可用，但 JSB / Cocos Simulator 上可能无法创建原生 ImageAsset，
  // 从而导致启动阶段 UI 纹理全部不可见。
  // Texture2D.reset + uploadData 是 Cocos 3.8 官方提供的原始像素上传路径。
  const tex = new Texture2D();
  tex.reset({
    width: w,
    height: h,
    format: Texture2D.PixelFormat.RGBA8888,
  });
  tex.uploadData(data);

  const sf = new SpriteFrame();
  sf.texture = tex;
  // 内存纹理不能参与动态合图；共享纹理 + 同材质仍可由渲染器自动合批。
  sf.packable = false;
  return sf;
}

let circleCache: SpriteFrame | null = null;

/** 白色圆（64x64，边缘抗锯齿），通过 Sprite color 染色。 */
export function circleTexture(): SpriteFrame {
  if (circleCache) return circleCache;
  const size = 64;
  const c = size / 2;
  const r = c - 1.5;
  circleCache = makeSpriteFrame(size, size, (x, y) => {
    const d = Math.hypot(x - c + 0.5, y - c + 0.5);
    const a = Math.max(0, Math.min(1, r - d + 0.5));
    return [255, 255, 255, Math.round(a * 255)];
  });
  return circleCache;
}

let ringCache: SpriteFrame | null = null;

/** 白色圆环，用于匹配动画与技能按钮装饰。 */
export function ringTexture(): SpriteFrame {
  if (ringCache) return ringCache;
  const size = 64;
  const c = size / 2;
  const outer = c - 1.5;
  const inner = outer - 6;
  ringCache = makeSpriteFrame(size, size, (x, y) => {
    const d = Math.hypot(x - c + 0.5, y - c + 0.5);
    const outerA = Math.max(0, Math.min(1, outer - d + 0.5));
    const innerA = Math.max(0, Math.min(1, d - inner + 0.5));
    const a = Math.min(outerA, innerA);
    return [255, 255, 255, Math.round(a * 255)];
  });
  return ringCache;
}

let roundRectCache: SpriteFrame | null = null;

/** 白色圆角矩形（64x64，r=14），用于按钮/卡片背景，通过 color 染色。 */
export function roundRectTexture(): SpriteFrame {
  if (roundRectCache) return roundRectCache;
  const size = 64;
  const r = 14;
  roundRectCache = makeSpriteFrame(size, size, (x, y) => {
    const px = Math.min(Math.max(x, r), size - r);
    const py = Math.min(Math.max(y, r), size - r);
    const d = Math.hypot(x - px, y - py);
    const a = Math.max(0, Math.min(1, r - d + 0.5));
    return [255, 255, 255, Math.round(a * 255)];
  });
  return roundRectCache;
}

let solidCache: SpriteFrame | null = null;

/** 2x2 纯白块，用于全屏背景等可拉伸区域。 */
export function solidTexture(): SpriteFrame {
  if (solidCache) return solidCache;
  solidCache = makeSpriteFrame(2, 2, () => [255, 255, 255, 255]);
  return solidCache;
}

const gradientCache = new Map<string, SpriteFrame>();

/** 纵向渐变纹理；top / bottom 可带 alpha。 */
export function verticalGradientTexture(top: Color, bottom: Color): SpriteFrame {
  const key = `${top.r},${top.g},${top.b},${top.a}-${bottom.r},${bottom.g},${bottom.b},${bottom.a}`;
  const cached = gradientCache.get(key);
  if (cached) return cached;

  const h = 128;
  const sf = makeSpriteFrame(4, h, (_x, y) => {
    const t = y / (h - 1);
    const mix = (a: number, b: number) => Math.round(a + (b - a) * t);
    // 纹理坐标从下往上写，因此 y=0 使用 bottom。
    return [
      mix(bottom.r, top.r),
      mix(bottom.g, top.g),
      mix(bottom.b, top.b),
      mix(bottom.a, top.a),
    ];
  });
  gradientCache.set(key, sf);
  return sf;
}
