// 结算场景：展示本局结果与奖励，提供再来一局/返回大厅。

import type { Scene, SceneCtx } from '../app/context.js';
import { button, clear, h } from '../ui/dom.js';

export class SettlementScene implements Scene {
  async mount(ctx: SceneCtx): Promise<void> {
    clear(ctx.uiRoot);
    const s = ctx.session.settlement;

    const rows: HTMLElement[] = [];
    const kv = (k: string, v: string) =>
      h('div', { className: 'kv' }, [h('span', { className: 'k', text: k }), h('span', { text: v })]);

    if (s) {
      rows.push(
        kv('名次', `第 ${s.rank} / ${s.totalPlayers} 名`),
        kv('最终得分', String(s.finalScore)),
        kv('最大质量', String(s.maxMass)),
        kv('吞噬玩家', String(s.eatPlayerCount)),
        kv('吃掉食物', String(s.eatFoodCount)),
        kv('存活时间', `${s.aliveSeconds}s`),
        kv('金币奖励', `+${s.coinReward}`),
        kv('经验奖励', `+${s.expReward}`),
      );
    } else {
      rows.push(h('div', { className: 'subtitle', text: '暂无结算数据' }));
    }

    const card = h('div', { className: 'card' }, [
      h('div', { className: 'title', text: '结算' }),
      ...rows,
      h('div', { className: 'row' }, [
        button('返回大厅', () => ctx.go('lobby'), 'btn secondary'),
        button('再来一局', () => ctx.go('match')),
      ]),
    ]);
    ctx.uiRoot.append(h('div', { className: 'panel centered' }, [card]));
  }

  unmount(): void {}
}
