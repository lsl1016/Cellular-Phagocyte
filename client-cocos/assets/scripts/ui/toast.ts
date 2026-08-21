// 全局 Toast 提示：挂载在 Canvas 顶层，顶部居中显示。

import { Label, Node, Sprite, UIOpacity, view } from 'cc';
import { label, uiNode } from './builder';
import { roundRectTexture } from './texgen';
import { theme } from './theme';

let canvasNode: Node | null = null;
let toastNode: Node | null = null;
let toastLabel: Node | null = null;
let hideTimer: ReturnType<typeof setTimeout> | null = null;

export function initToast(canvas: Node): void {
  canvasNode = canvas;
}

export function toast(message: string, ms = 2200): void {
  if (!canvasNode) return;
  if (!toastNode || !toastNode.isValid) {
    toastNode = uiNode('toast', 420, 56);
    const sp = toastNode.addComponent(Sprite);
    sp.spriteFrame = roundRectTexture();
    sp.sizeMode = Sprite.SizeMode.CUSTOM;
    sp.color = theme.cardBorder;
    toastNode.addComponent(UIOpacity);
    toastLabel = label(message, { width: 396, fontSize: theme.smallSize + 4 });
    toastNode.addChild(toastLabel);
    canvasNode.addChild(toastNode);
  }
  const vs = view.getVisibleSize();
  toastNode.setPosition(0, vs.height / 2 - 70, 0);
  toastNode.getComponent(UIOpacity)!.opacity = 255;
  toastNode.active = true;
  const lb = toastLabel!.getComponent(Label);
  if (lb) lb.string = message;

  if (hideTimer) clearTimeout(hideTimer);
  hideTimer = setTimeout(() => {
    if (toastNode && toastNode.isValid) toastNode.active = false;
  }, ms);
}
