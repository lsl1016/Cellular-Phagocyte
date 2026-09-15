// 程序化 UI 构建辅助：统一屏幕、卡片、按钮、统计块与 HUD 控件。

import { Color, Label, Layers, Node, Sprite, UITransform, Widget, view } from 'cc';
import {
  circleTexture,
  ringTexture,
  roundRectTexture,
  solidTexture,
  verticalGradientTexture,
} from './texgen';
import { theme, withAlpha } from './theme';

/** 创建基础 UI 节点（UI_2D 层 + UITransform）。 */
export function uiNode(name: string, w = 0, h = 0): Node {
  const n = new Node(name);
  n.layer = Layers.Enum.UI_2D;
  const t = n.addComponent(UITransform);
  if (w > 0 || h > 0) t.setContentSize(Math.max(0, w), Math.max(0, h));
  return n;
}

export interface LabelOpts {
  width?: number;
  height?: number;
  fontSize?: number;
  color?: Color;
  align?: 'left' | 'center' | 'right';
}

export function label(text: string, opts: LabelOpts = {}): Node {
  const fontSize = opts.fontSize ?? theme.fontSize;
  const width = opts.width ?? 360;
  const height = opts.height ?? Math.ceil(fontSize * 1.6);
  const n = uiNode('label', width, height);
  const lb = n.addComponent(Label);
  lb.string = text;
  lb.fontSize = fontSize;
  lb.lineHeight = Math.ceil(fontSize * 1.35);
  lb.color = opts.color ?? theme.text;
  lb.overflow = Label.Overflow.SHRINK;
  if (opts.align === 'left') lb.horizontalAlign = Label.HorizontalAlign.LEFT;
  else if (opts.align === 'right') lb.horizontalAlign = Label.HorizontalAlign.RIGHT;
  else lb.horizontalAlign = Label.HorizontalAlign.CENTER;
  lb.verticalAlign = Label.VerticalAlign.CENTER;
  return n;
}

export function setLabel(n: Node, text: string): void {
  const lb = n.getComponent(Label);
  if (lb) lb.string = text;
}

function addSpriteLayer(
  parent: Node,
  name: string,
  width: number,
  height: number,
  color: Color,
  inset = 0,
): Node {
  const n = uiNode(name, Math.max(1, width - inset * 2), Math.max(1, height - inset * 2));
  const sp = n.addComponent(Sprite);
  sp.spriteFrame = roundRectTexture();
  sp.sizeMode = Sprite.SizeMode.CUSTOM;
  sp.color = color;
  parent.addChild(n);
  return n;
}

function bindPress(n: Node, onClick: () => void, disabled = false): void {
  if (disabled) return;
  const restore = () => n.setScale(1, 1, 1);
  n.on(Node.EventType.TOUCH_START, (ev) => {
    ev.propagationStopped = true;
    n.setScale(0.97, 0.97, 1);
  });
  n.on(Node.EventType.TOUCH_CANCEL, (ev) => {
    ev.propagationStopped = true;
    restore();
  });
  n.on(Node.EventType.TOUCH_END, (ev) => {
    ev.propagationStopped = true;
    restore();
    onClick();
  });
}

export type ButtonVariant = 'primary' | 'secondary' | 'danger' | 'ghost';

export interface ButtonOpts {
  width?: number;
  height?: number;
  fontSize?: number;
  secondary?: boolean;
  disabled?: boolean;
  variant?: ButtonVariant;
}

export function button(
  text: string,
  onClick: () => void,
  opts: ButtonOpts = {},
): Node {
  const w = opts.width ?? 220;
  const h = opts.height ?? 62;
  const variant: ButtonVariant = opts.variant ?? (opts.secondary ? 'secondary' : 'primary');
  const n = uiNode('button', w, h);
  const sp = n.addComponent(Sprite);
  sp.spriteFrame = roundRectTexture();
  sp.sizeMode = Sprite.SizeMode.CUSTOM;

  let bg = theme.primary;
  let fg = theme.primaryText;
  if (variant === 'secondary') bg = theme.secondary;
  else if (variant === 'danger') bg = theme.danger;
  else if (variant === 'ghost') {
    bg = withAlpha(theme.cardBorder, 140);
    fg = theme.text;
  }
  if (opts.disabled) {
    bg = withAlpha(theme.cardBorder, 150);
    fg = theme.subtle;
  }
  sp.color = bg;

  // 保持 label 为第一个 child，兼容现有 setLabel(btn.children[0], ...) 调用。
  const txt = label(text, {
    width: w - 20,
    height: h - 8,
    fontSize: opts.fontSize ?? theme.fontSize,
    color: fg,
  });
  n.addChild(txt);
  bindPress(n, onClick, opts.disabled);
  return n;
}

export interface CircleButtonOpts {
  size?: number;
  color?: Color;
  ringColor?: Color;
  fontSize?: number;
  disabled?: boolean;
}

/** 圆形技能按钮。 */
export function circleButton(
  text: string,
  onClick: () => void,
  opts: CircleButtonOpts = {},
): Node {
  const size = opts.size ?? 88;
  const n = uiNode('circle-button', size, size);
  const sp = n.addComponent(Sprite);
  sp.spriteFrame = circleTexture();
  sp.sizeMode = Sprite.SizeMode.CUSTOM;
  sp.color = opts.disabled ? withAlpha(theme.cardBorder, 160) : (opts.color ?? theme.primary);

  const ring = uiNode('circle-ring', size + 10, size + 10);
  const ringSp = ring.addComponent(Sprite);
  ringSp.spriteFrame = ringTexture();
  ringSp.sizeMode = Sprite.SizeMode.CUSTOM;
  ringSp.color = opts.ringColor ?? withAlpha(theme.primaryBright, 165);
  n.addChild(ring);

  n.addChild(label(text, {
    width: size - 18,
    height: size - 18,
    fontSize: opts.fontSize ?? theme.smallSize + 5,
    color: theme.primaryText,
  }));
  bindPress(n, onClick, opts.disabled);
  return n;
}

/** 水平排布子节点（居中）。 */
export function row(children: Node[], gap = 16): Node {
  if (children.length === 0) return uiNode('row', 0, 0);
  const widths = children.map((c) => c.getComponent(UITransform)!.width);
  const totalW = widths.reduce((a, b) => a + b, 0) + gap * (children.length - 1);
  const maxH = Math.max(...children.map((c) => c.getComponent(UITransform)!.height));
  const n = uiNode('row', totalW, maxH);
  let x = -totalW / 2;
  for (let i = 0; i < children.length; i++) {
    const c = children[i];
    c.setPosition(x + widths[i] / 2, 0, 0);
    n.addChild(c);
    x += widths[i] + gap;
  }
  return n;
}

/** 垂直排布子节点（居中）。 */
export function column(children: Node[], gap = 18): Node {
  if (children.length === 0) return uiNode('column', 0, 0);
  const heights = children.map((c) => c.getComponent(UITransform)!.height);
  const totalH = heights.reduce((a, b) => a + b, 0) + gap * (children.length - 1);
  const maxW = Math.max(...children.map((c) => c.getComponent(UITransform)!.width));
  const n = uiNode('column', maxW, totalH);
  let y = totalH / 2;
  for (let i = 0; i < children.length; i++) {
    const c = children[i];
    c.setPosition(0, y - heights[i] / 2, 0);
    n.addChild(c);
    y -= heights[i] + gap;
  }
  return n;
}

export interface PanelOpts {
  height?: number;
  color?: Color;
  borderColor?: Color;
  shadow?: boolean;
  contentOffsetY?: number;
}

/** 带阴影和边框的 Surface 容器。 */
export function panel(
  children: Node[],
  width = 440,
  gap = 18,
  padding = 28,
  opts: PanelOpts = {},
): Node {
  const inner = column(children, gap);
  inner.name = 'panel-content';
  const innerT = inner.getComponent(UITransform)!;
  innerT.width = Math.min(Math.max(innerT.width, 1), Math.max(1, width - padding * 2));
  const h = opts.height ?? Math.max(48, innerT.height + padding * 2);
  const n = uiNode('panel', width, h);

  if (opts.shadow !== false) {
    const shadow = addSpriteLayer(n, 'panel-shadow', width, h, theme.shadow);
    shadow.setPosition(0, -7, 0);
  }
  addSpriteLayer(n, 'panel-border', width, h, opts.borderColor ?? theme.cardBorder);
  addSpriteLayer(n, 'panel-surface', width, h, opts.color ?? theme.card, 2);

  inner.setPosition(0, opts.contentOffsetY ?? 0, 0);
  n.addChild(inner);
  return n;
}

/** 兼容旧 API 的卡片。 */
export function card(children: Node[], width = 440, gap = 18, padding = 28): Node {
  return panel(children, width, gap, padding);
}

export interface PillOpts {
  width?: number;
  height?: number;
  fontSize?: number;
  color?: Color;
  textColor?: Color;
}

/** 轻量状态/资产胶囊。 */
export function pill(text: string, opts: PillOpts = {}): Node {
  const fontSize = opts.fontSize ?? theme.smallSize + 1;
  const w = opts.width ?? Math.max(76, text.length * (fontSize * 0.82) + 30);
  const h = opts.height ?? 34;
  const n = uiNode('pill', w, h);
  const sp = n.addComponent(Sprite);
  sp.spriteFrame = roundRectTexture();
  sp.sizeMode = Sprite.SizeMode.CUSTOM;
  sp.color = opts.color ?? withAlpha(theme.secondary, 230);
  n.addChild(label(text, {
    width: w - 16,
    height: h - 4,
    fontSize,
    color: opts.textColor ?? theme.text,
  }));
  return n;
}

export interface StatCardOpts {
  width?: number;
  height?: number;
  accent?: Color;
  valueColor?: Color;
}

/** 两行统计卡片，值节点命名为 stat-value，便于异步刷新。 */
export function statCard(
  title: string,
  value: string,
  opts: StatCardOpts = {},
): Node {
  const titleEl = label(title, {
    width: (opts.width ?? 150) - 28,
    fontSize: theme.smallSize,
    color: theme.muted,
    align: 'left',
  });
  titleEl.name = 'stat-title';
  const valueEl = label(value, {
    width: (opts.width ?? 150) - 28,
    fontSize: theme.bigSize - 4,
    color: opts.valueColor ?? theme.text,
    align: 'left',
  });
  valueEl.name = 'stat-value';
  const n = panel(
    [titleEl, valueEl],
    opts.width ?? 150,
    2,
    14,
    { height: opts.height ?? 86, color: theme.cardSoft, shadow: false },
  );
  const accent = uiNode('stat-accent', 4, (opts.height ?? 86) - 24);
  const sp = accent.addComponent(Sprite);
  sp.spriteFrame = solidTexture();
  sp.sizeMode = Sprite.SizeMode.CUSTOM;
  sp.color = opts.accent ?? theme.primaryBright;
  accent.setPosition(-(opts.width ?? 150) / 2 + 10, 0, 0);
  n.addChild(accent);
  return n;
}

export function setStatValue(n: Node, value: string): void {
  const content = n.getChildByName('panel-content');
  const valueEl = content?.getChildByName('stat-value');
  if (valueEl) setLabel(valueEl, value);
}

/** 可点击的功能入口卡片。 */
export function actionTile(
  title: string,
  subtitle: string,
  onClick: () => void,
  width = 220,
): Node {
  const titleEl = label(title, {
    width: width - 50,
    fontSize: theme.fontSize,
    align: 'left',
  });
  const subEl = label(subtitle, {
    width: width - 50,
    fontSize: theme.smallSize,
    color: theme.muted,
    align: 'left',
  });
  const n = panel([titleEl, subEl], width, 0, 18, {
    height: 92,
    color: theme.cardSoft,
    shadow: false,
  });
  const arrow = label('>', { width: 24, fontSize: theme.bigSize, color: theme.primaryBright });
  arrow.setPosition(width / 2 - 28, 0, 0);
  n.addChild(arrow);
  bindPress(n, onClick);
  return n;
}

export function divider(width: number, color = withAlpha(theme.cardBorder, 180)): Node {
  const n = uiNode('divider', width, 1);
  const sp = n.addComponent(Sprite);
  sp.spriteFrame = solidTexture();
  sp.sizeMode = Sprite.SizeMode.CUSTOM;
  sp.color = color;
  return n;
}

export function progressBar(value: number, width = 260, height = 10): Node {
  const root = uiNode('progress', width, height);
  const bg = root.addComponent(Sprite);
  bg.spriteFrame = roundRectTexture();
  bg.sizeMode = Sprite.SizeMode.CUSTOM;
  bg.color = theme.secondary;

  const clamped = Math.max(0, Math.min(1, value));
  const fillWidth = Math.max(height, width * clamped);
  const fill = uiNode('progress-fill', fillWidth, height);
  const fillSp = fill.addComponent(Sprite);
  fillSp.spriteFrame = roundRectTexture();
  fillSp.sizeMode = Sprite.SizeMode.CUSTOM;
  fillSp.color = theme.accent;
  fill.setPosition(-width / 2 + fillWidth / 2, 0, 0);
  root.addChild(fill);
  return root;
}

/** 全屏渐变背景 + 低透明细胞光斑。 */
export function fullBackground(parent: Node, color?: Color): Node {
  const n = uiNode('bg', 2, 2);
  const sp = n.addComponent(Sprite);
  sp.spriteFrame = color ? solidTexture() : verticalGradientTexture(theme.bgTop, theme.bgBottom);
  sp.sizeMode = Sprite.SizeMode.CUSTOM;
  sp.color = color ?? new Color(255, 255, 255, 255);
  const wg = n.addComponent(Widget);
  wg.isAlignTop = true;
  wg.isAlignBottom = true;
  wg.isAlignLeft = true;
  wg.isAlignRight = true;
  wg.top = 0;
  wg.bottom = 0;
  wg.left = 0;
  wg.right = 0;
  parent.addChild(n);
  n.setSiblingIndex(0);

  if (!color) {
    const vs = view.getVisibleSize();
    addAmbientOrb(parent, 360, -vs.width * 0.43, vs.height * 0.37, withAlpha(theme.primary, 28));
    addAmbientOrb(parent, 300, vs.width * 0.44, -vs.height * 0.39, withAlpha(theme.accent, 20));
    addAmbientOrb(parent, 120, vs.width * 0.26, vs.height * 0.34, withAlpha(theme.primaryBright, 18));
  }
  return n;
}

function addAmbientOrb(parent: Node, size: number, x: number, y: number, color: Color): void {
  const orb = uiNode('ambient-orb', size, size);
  const sp = orb.addComponent(Sprite);
  sp.spriteFrame = circleTexture();
  sp.sizeMode = Sprite.SizeMode.CUSTOM;
  sp.color = color;
  orb.setPosition(x, y, 0);
  parent.addChild(orb);
}

/** 居中放置节点（画布中心）。 */
export function centerIn(parent: Node, child: Node, y = 0): void {
  child.setPosition(0, y, 0);
  parent.addChild(child);
}

/** 清空容器。 */
export function clearChildren(n: Node): void {
  for (let i = n.children.length - 1; i >= 0; i--) {
    n.children[i].destroy();
  }
}
