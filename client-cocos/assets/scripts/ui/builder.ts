// 程序化 UI 构建辅助：节点/文本/按钮/卡片/布局，全部代码创建，无 Prefab 依赖。

import { Color, Label, Layers, Node, Sprite, UITransform, Widget } from 'cc';
import { roundRectTexture, solidTexture } from './texgen';
import { theme } from './theme';

/** 创建基础 UI 节点（UI_2D 层 + UITransform）。 */
export function uiNode(name: string, w = 0, h = 0): Node {
  const n = new Node(name);
  n.layer = Layers.Enum.UI_2D;
  const t = n.addComponent(UITransform);
  if (w > 0 && h > 0) t.setContentSize(w, h);
  return n;
}

export interface LabelOpts {
  width?: number;
  fontSize?: number;
  color?: Color;
  align?: 'left' | 'center';
}

export function label(text: string, opts: LabelOpts = {}): Node {
  const fontSize = opts.fontSize ?? theme.fontSize;
  const width = opts.width ?? 360;
  const n = uiNode('label', width, Math.ceil(fontSize * 1.6));
  const lb = n.addComponent(Label);
  lb.string = text;
  lb.fontSize = fontSize;
  lb.lineHeight = Math.ceil(fontSize * 1.4);
  lb.color = opts.color ?? theme.text;
  lb.overflow = Label.Overflow.SHRINK;
  lb.horizontalAlign =
    opts.align === 'left' ? Label.HorizontalAlign.LEFT : Label.HorizontalAlign.CENTER;
  lb.verticalAlign = Label.VerticalAlign.CENTER;
  return n;
}

export function setLabel(n: Node, text: string): void {
  const lb = n.getComponent(Label);
  if (lb) lb.string = text;
}

export interface ButtonOpts {
  width?: number;
  height?: number;
  fontSize?: number;
  secondary?: boolean;
  disabled?: boolean;
}

export function button(
  text: string,
  onClick: () => void,
  opts: ButtonOpts = {},
): Node {
  const w = opts.width ?? 220;
  const h = opts.height ?? 64;
  const n = uiNode('button', w, h);
  const sp = n.addComponent(Sprite);
  sp.spriteFrame = roundRectTexture();
  sp.sizeMode = Sprite.SizeMode.CUSTOM;
  sp.color = opts.disabled ? theme.cardBorder : opts.secondary ? theme.secondary : theme.primary;

  const txt = label(text, {
    width: w - 16,
    fontSize: opts.fontSize ?? theme.fontSize,
    color: opts.secondary && !opts.disabled ? theme.text : theme.primaryText,
  });
  n.addChild(txt);

  if (!opts.disabled) {
    n.on(Node.EventType.TOUCH_END, (ev) => {
      ev.propagationStopped = true;
      onClick();
    });
  }
  return n;
}

/** 水平排布子节点（居中）。 */
export function row(children: Node[], gap = 16): Node {
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

/** 卡片容器：圆角背景 + 内边距 + 垂直布局。 */
export function card(children: Node[], width = 440, gap = 18, padding = 28): Node {
  const inner = column(children, gap);
  const innerT = inner.getComponent(UITransform)!;
  innerT.width = Math.min(innerT.width, width - padding * 2);
  const h = innerT.height + padding * 2;
  const n = uiNode('card', width, h);
  const sp = n.addComponent(Sprite);
  sp.spriteFrame = roundRectTexture();
  sp.sizeMode = Sprite.SizeMode.CUSTOM;
  sp.color = theme.card;
  n.addChild(inner);
  return n;
}

/** 全屏纯色背景（自动拉伸）。 */
export function fullBackground(parent: Node, color = theme.bg): Node {
  const n = uiNode('bg', 2, 2);
  const sp = n.addComponent(Sprite);
  sp.spriteFrame = solidTexture();
  sp.sizeMode = Sprite.SizeMode.CUSTOM;
  sp.color = color;
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
  return n;
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
