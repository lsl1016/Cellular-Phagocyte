// 全屏布局辅助：避免 Screen 根节点为 0x0 时 Widget/HUD 全部挤到屏幕中央。

import { Node, UITransform, view } from 'cc';
import { uiNode } from './builder';

export type Pin =
  | 'left-top'
  | 'center-top'
  | 'right-top'
  | 'left-bottom'
  | 'center-bottom'
  | 'right-bottom'
  | 'center';

/**
 * 创建一个真正具有可见区尺寸的 Screen 根节点。
 * 之前 screen 只是 0x0 的 Node；大厅居中元素还能显示，但 HUD Widget 会以 0x0 父节点计算。
 */
export function screenLayer(parent: Node, name: string): Node {
  const visible = view.getVisibleSize();
  const pt = parent.getComponent(UITransform);
  const width = pt && pt.width > 0 ? pt.width : visible.width;
  const height = pt && pt.height > 0 ? pt.height : visible.height;
  const n = uiNode(name, width, height);
  n.setPosition(0, 0, 0);
  parent.addChild(n);
  return n;
}

/** 直接按父节点可见尺寸定位，避免 HUD 依赖 Widget 对齐时序。 */
export function pin(
  n: Node,
  parent: Node,
  pos: Pin,
  offsetX = 16,
  offsetY = 16,
): void {
  const visible = view.getVisibleSize();
  const pt = parent.getComponent(UITransform);
  const pw = pt && pt.width > 0 ? pt.width : visible.width;
  const ph = pt && pt.height > 0 ? pt.height : visible.height;
  const t = n.getComponent(UITransform)!;
  const hw = pw / 2;
  const hh = ph / 2;

  let x = 0;
  let y = 0;
  if (pos.includes('left')) x = -hw + offsetX + t.width / 2;
  else if (pos.includes('right')) x = hw - offsetX - t.width / 2;

  if (pos.includes('top')) y = hh - offsetY - t.height / 2;
  else if (pos.includes('bottom')) y = -hh + offsetY + t.height / 2;

  n.setPosition(x, y, 0);
  parent.addChild(n);
}

/** 创建与 parent 等大的节点，常用于遮罩。 */
export function fullSizeNode(parent: Node, name: string): Node {
  const visible = view.getVisibleSize();
  const pt = parent.getComponent(UITransform);
  const width = pt && pt.width > 0 ? pt.width : visible.width;
  const height = pt && pt.height > 0 ? pt.height : visible.height;
  return uiNode(name, width, height);
}
