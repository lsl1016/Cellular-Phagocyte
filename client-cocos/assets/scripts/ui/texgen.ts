// 程序化生成纹理：白色圆 / 圆角矩形 / 纯色块。
// 所有实体共享同一张圆纹理，由 Sprite tint 上色，可享受自动合批。

import { ImageAsset, SpriteFrame, Texture2D } from 'cc';

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
  const img = new ImageAsset();
  img.reset({
    _data: data,
    width: w,
    height: h,
    format: Texture2D.PixelFormat.RGBA8888,
    _compressed: false,
  });
  const tex = new Texture2D();
  tex.image = img;
  const sf = new SpriteFrame();
  sf.texture = tex;
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
