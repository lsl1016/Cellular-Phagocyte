// 简单节点池：获取/回收节点，避免高频创建销毁。

import { Node } from 'cc';

export class NodePool {
  private free: Node[] = [];

  constructor(private factory: () => Node) {}

  get(): Node {
    const n = this.free.pop();
    if (n && n.isValid) {
      n.active = true;
      return n;
    }
    return this.factory();
  }

  put(n: Node): void {
    if (!n.isValid) return;
    n.active = false;
    n.removeAllChildren();
    this.free.push(n);
  }

  clear(): void {
    for (const n of this.free) {
      if (n.isValid) n.destroy();
    }
    this.free = [];
  }
}
