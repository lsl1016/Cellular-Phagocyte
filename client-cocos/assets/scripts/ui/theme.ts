// 统一视觉主题：深海细胞 + 轻科幻 HUD。

import { Color } from 'cc';

function hex(value: string): Color {
  return new Color().fromHEX(value);
}

export function withAlpha(color: Color, alpha: number): Color {
  return new Color(color.r, color.g, color.b, alpha);
}

export const theme = {
  // 背景
  bg: hex('#07111f'),
  bgTop: hex('#0b1c38'),
  bgBottom: hex('#050a14'),
  bgGlow: hex('#172c55'),

  // Surface
  card: new Color(17, 31, 54, 246),
  cardStrong: new Color(21, 40, 70, 252),
  cardSoft: new Color(18, 34, 59, 226),
  cardBorder: hex('#2c466a'),
  secondary: hex('#1a3152'),
  shadow: new Color(0, 0, 0, 76),
  overlay: new Color(3, 8, 17, 214),

  // Text
  text: hex('#f5f9ff'),
  muted: hex('#8ea6c5'),
  subtle: hex('#607a9d'),
  primaryText: hex('#ffffff'),

  // Brand / semantic
  primary: hex('#5f7cff'),
  primaryBright: hex('#72d7ff'),
  accent: hex('#66e6ca'),
  success: hex('#61dfa0'),
  warning: hex('#ffc86b'),
  danger: hex('#ff6d7f'),
  self: hex('#ffffff'),

  // Typography
  fontSize: 22,
  titleSize: 38,
  heroSize: 46,
  bigSize: 30,
  smallSize: 16,
  microSize: 13,
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
export function parseFoodColor(hexValue: string | undefined): Color {
  if (!hexValue) return hex('#77ffdd');
  try {
    let v = hexValue.replace('#', '').trim();
    if (v.length === 3) v = v.split('').map((ch) => ch + ch).join('');
    if (v.length !== 6 || /[^0-9a-fA-F]/.test(v)) return hex('#77ffdd');
    return new Color().fromHEX('#' + v);
  } catch {
    return hex('#77ffdd');
  }
}
