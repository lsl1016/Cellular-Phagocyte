// 大厅场景：展示用户资料，提供开始游戏入口。

import type { Scene, SceneCtx } from '../app/context.js';
import { button, clear, h } from '../ui/dom.js';

export class LobbyScene implements Scene {
  async mount(ctx: SceneCtx): Promise<void> {
    clear(ctx.uiRoot);

    const user = ctx.session.user;
    const nameEl = h('div', { className: 'big', text: user?.nickname ?? '玩家' });
    const levelEl = h('span', { text: `Lv.${user?.level ?? 1}` });
    const coinEl = h('span', { text: `金币 ${user?.coin ?? 0}` });
    const expEl = h('span', { text: `经验 ${user?.exp ?? 0}` });

    const card = h('div', { className: 'card' }, [
      h('div', { className: 'title', text: '大厅' }),
      nameEl,
      h('div', { className: 'row' }, [levelEl, coinEl, expEl]),
      button('开始游戏', () => ctx.go('match')),
      h('div', { className: 'row' }, [
        button('战绩', () => ctx.go('records'), 'btn secondary'),
        button('排行榜', () => ctx.go('rank'), 'btn secondary'),
      ]),
    ]);
    ctx.uiRoot.append(h('div', { className: 'panel centered' }, [card]));

    // 刷新最新资产（结算后金币/经验会变化）
    try {
      const me = await ctx.api.getMe();
      if (ctx.session.user) {
        ctx.session.user.level = me.level;
        ctx.session.user.coin = me.coin;
        ctx.session.user.exp = me.exp;
      }
      levelEl.textContent = `Lv.${me.level}`;
      coinEl.textContent = `金币 ${me.coin}`;
      expEl.textContent = `经验 ${me.exp}`;
      nameEl.textContent = me.nickname;
    } catch {
      /* 刷新失败不阻塞大厅展示 */
    }
  }

  unmount(): void {}
}
