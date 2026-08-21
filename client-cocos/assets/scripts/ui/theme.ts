// 视觉主题常量（对齐 Canvas 版配色）。

import { Color } from 'cc';

export const theme = {
  bg: new Color().fromHEX('#0b1020'),
  card: new Color().fromHEX('#151c33'),
  cardBorder: new Color().fromHEX('#2a3350'),
  text: new Color().fromHEX('#e8ecf5'),
  muted: new Color().fromHEX('#8b94ad'),
  primary: new Color().fromHEX('#6a73ff'),
  primaryText: new Color().fromHEX('#ffffff'),
  secondary: new Color().fromHEX('#232c48'),
  danger: new Color().fromHEX('#e05b5b'),
  self: new Color().fromHEX('#ffffff'),
  fontSize: 22,
  titleSize: 36,
  bigSize: 30,
  smallSize: 16,
};

/** userId -> HSL 颜色（与 Canvas 版 colorFor 一致）。 */
export function colorForUserId(userId: string): Color {
  let hsh = 0;
  for (let i = 0; i < userId.length; i++) {
    hsh = (hsh * 31 + userId.charCodeAt(i)) % 360;
  }
  return hslToColor(hsh, 0.65, 0.55);
}

export function hslToColor(h: number, s: number, l: number): Color {
  const c = (1 - Math.abs(2 * l - 1)) * s;
  const hp = h / 60;
  const x = c * (1 - Math.abs((hp % 2) - 1));
  let r = 0;
  let g = 0;
  let b = 0;
  if (hp >= 0 && hp < 1) { r = c; g = x; }
  else if (hp < 2) { r = x; g = c; }
  else if (hp < 3) { g = c; b = x; }
  else if (hp < 4) { g = x; b = c; }
  else if (hp < 5) { r = x; b = c; }
  else { r = c; b = x; }
  const m = l - c / 2;
  return new Color(
    Math.round((r + m) * 255),
    Math.round((g + m) * 255),
    Math.round((b + m) * 255),
    255,
  );
}

/** 解析服务端下发的食物颜色（#rgb / #rrggbb），失败返回兜底色。 */
export function parseFoodColor(hex: string | undefined): Color {
  if (!hex) return new Color().fromHEX('#77ffdd');
  try {
    let v = hex.replace('#', '').trim();
    if (v.length === 3) v = v.split('').map((ch) => ch + ch).join('');
    if (v.length !== 6 || /[^0-9a-fA-F]/.test(v)) return new Color().fromHEX('#77ffdd');
    return new Color().fromHEX('#' + v);
  } catch {
    return new Color().fromHEX('#77ffdd');
  }
}
