// 结算屏幕：展示本局结果与奖励，提供再来一局/返回大厅。

import { Node } from 'cc';
import type { Screen, ScreenCtx } from '../app/context';
import { button, card, centerIn, fullBackground, label, row, uiNode } from '../ui/builder';
import { theme } from '../ui/theme';

export class SettlementScreen implements Screen {
  private node: Node | null = null;

  async mount(ctx: ScreenCtx): Promise<void> {
    this.node = uiNode('settlement-screen');
    ctx.root.addChild(this.node);
    fullBackground(this.node);
    const s = ctx.session.settlement;

    const children: Node[] = [];
    if (s) {
      const kv = (k: string, v: string) =>
        row([
          label(k, { width: 140, color: theme.muted, align: 'left' }),
          label(v, { width: 240, align: 'left' }),
        ], 8);
      children.push(
        label(`名次: 第 ${s.rank} / ${s.totalPlayers} 名`),
        label(`最终得分: ${s.finalScore}`),
        label(`最大质量: ${s.maxMass}`),
        label(`吞噬玩家: ${s.eatPlayerCount} · 吃掉食物: ${s.eatFoodCount}`),
        label(`存活时间: ${s.aliveSeconds}s`),
        label(`奖励: +${s.coinReward}金币 +${s.expReward}经验`),
      );
    } else {
      children.push(label('暂无结算数据', { color: theme.muted }));
    }
    children.push(
      row([
        button('返回大厅', () => ctx.go('lobby'), { secondary: true, width: 160 }),
        button('再来一局', () => ctx.go('match'), { width: 160 }),
      ]),
    );

    const c = card([label('结算', { fontSize: theme.titleSize }), ...children], 440, 14);
    centerIn(this.node, c);
  }

  unmount(): void {
    this.node?.destroy();
    this.node = null;
  }
}
