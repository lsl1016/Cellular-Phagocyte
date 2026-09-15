// 全局 Toast：统一新版卡片风格，顶部居中展示。

import { Label, Node, UIOpacity, view } from 'cc';
import { label, panel } from './builder';
import { theme, withAlpha } from './theme';

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
    toastLabel = label(message, {
      width: 410,
      fontSize: theme.smallSize + 2,
      color: theme.text,
    });
    toastNode = panel([toastLabel], 460, 0, 18, {
      height: 58,
      color: theme.cardStrong,
      borderColor: withAlpha(theme.primaryBright, 120),
    });
    toastNode.name = 'toast';
    toastNode.addComponent(UIOpacity);
    canvasNode.addChild(toastNode);
  }
  const vs = view.getVisibleSize();
  toastNode.setPosition(0, vs.height / 2 - 72, 0);
  toastNode.getComponent(UIOpacity)!.opacity = 255;
  toastNode.active = true;
  const lb = toastLabel!.getComponent(Label);
  if (lb) lb.string = message;

  if (hideTimer) clearTimeout(hideTimer);
  hideTimer = setTimeout(() => {
    if (toastNode && toastNode.isValid) toastNode.active = false;
  }, ms);
}
