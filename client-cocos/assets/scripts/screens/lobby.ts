// 大厅屏幕：展示用户资料，提供开始游戏入口。

import { Node } from 'cc';
import type { Screen, ScreenCtx } from '../app/context';
import { button, card, centerIn, fullBackground, label, row, setLabel, uiNode } from '../ui/builder';
import { theme } from '../ui/theme';

export class LobbyScreen implements Screen {
  private node: Node | null = null;

  async mount(ctx: ScreenCtx): Promise<void> {
    this.node = uiNode('lobby-screen');
    ctx.root.addChild(this.node);
    fullBackground(this.node);

    const user = ctx.session.user;
    const nameEl = label(user?.nickname ?? '玩家', { fontSize: theme.bigSize });
    const levelEl = label(`Lv.${user?.level ?? 1}`, { width: 110 });
    const coinEl = label(`金币 ${user?.coin ?? 0}`, { width: 130 });
    const expEl = label(`经验 ${user?.exp ?? 0}`, { width: 130 });

    const c = card([
      label('大厅', { fontSize: theme.titleSize }),
      nameEl,
      row([levelEl, coinEl, expEl], 8),
      button('开始游戏', () => ctx.go('match')),
      row([
        button('战绩', () => ctx.go('records'), { secondary: true, width: 160 }),
        button('排行榜', () => ctx.go('rank'), { secondary: true, width: 160 }),
      ]),
    ]);
    centerIn(this.node, c);

    // 刷新最新资产（结算后金币/经验会变化）
    try {
      const me = await ctx.api.getMe();
      if (!this.node?.isValid) return;
      if (ctx.session.user) {
        ctx.session.user.level = me.level;
        ctx.session.user.coin = me.coin;
        ctx.session.user.exp = me.exp;
      }
      setLabel(levelEl, `Lv.${me.level}`);
      setLabel(coinEl, `金币 ${me.coin}`);
      setLabel(expEl, `经验 ${me.exp}`);
      setLabel(nameEl, me.nickname);
    } catch {
      /* 刷新失败不阻塞大厅展示 */
    }
  }

  unmount(): void {
    this.node?.destroy();
    this.node = null;
  }
}
