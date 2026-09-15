// 大厅屏幕：沿用 H5 的清晰结构，单主卡片 + 轻量资产胶囊 + 主操作。

import { Node } from 'cc';
import type { Screen, ScreenCtx } from '../app/context';
import {
  button,
  centerIn,
  divider,
  fullBackground,
  label,
  panel,
  pill,
  row,
  setLabel,
} from '../ui/builder';
import { screenLayer } from '../ui/layout';
import { theme, withAlpha } from '../ui/theme';

export class LobbyScreen implements Screen {
  private node: Node | null = null;

  async mount(ctx: ScreenCtx): Promise<void> {
    this.node = screenLayer(ctx.root, 'lobby-screen');
    fullBackground(this.node);

    const user = ctx.session.user;
    const nameEl = label(user?.nickname ?? '玩家', {
      width: 460,
      fontSize: theme.titleSize,
      align: 'left',
    });
    const levelEl = pill(`Lv.${user?.level ?? 1}`, {
      width: 100,
      color: withAlpha(theme.primary, 55),
      textColor: theme.primaryBright,
    });
    const coinEl = pill(`金币 ${user?.coin ?? 0}`, {
      width: 130,
      color: withAlpha(theme.warning, 38),
      textColor: theme.warning,
    });
    const expEl = pill(`经验 ${user?.exp ?? 0}`, {
      width: 130,
      color: withAlpha(theme.accent, 32),
      textColor: theme.accent,
    });

    const c = panel([
      nameEl,
      label('欢迎回来 · 经典竞技大厅', {
        width: 460,
        fontSize: theme.smallSize + 2,
        color: theme.muted,
        align: 'left',
      }),
      row([levelEl, coinEl, expEl], 10),
      divider(460),
      row([
        pill('CLASSIC', {
          width: 98,
          color: withAlpha(theme.primary, 45),
          textColor: theme.primaryBright,
        }),
        label('经典吞噬战', {
          width: 340,
          fontSize: theme.bigSize,
          align: 'left',
        }),
      ], 14),
      label('控制细胞移动和成长，分裂追击，避开比你更大的对手。', {
        width: 460,
        fontSize: theme.smallSize + 2,
        color: theme.muted,
        align: 'left',
      }),
      button('开始匹配', () => ctx.go('match'), { width: 360, height: 62 }),
      row([
        button('战绩', () => ctx.go('records'), {
          variant: 'secondary',
          width: 170,
          height: 50,
        }),
        button('排行榜', () => ctx.go('rank'), {
          variant: 'secondary',
          width: 170,
          height: 50,
        }),
      ], 16),
      label('Space 分裂 · W 吐球 · 也支持触屏操作', {
        width: 460,
        fontSize: theme.smallSize,
        color: theme.subtle,
      }),
    ], 560, 12, 30, { height: 520, color: theme.cardStrong });
    centerIn(this.node, c);

    try {
      const me = await ctx.api.getMe();
      if (!this.node?.isValid) return;
      if (ctx.session.user) {
        ctx.session.user.level = me.level;
        ctx.session.user.coin = me.coin;
        ctx.session.user.exp = me.exp;
      }
      setLabel(nameEl, me.nickname);
      setLabel(levelEl.children[0], `Lv.${me.level}`);
      setLabel(coinEl.children[0], `金币 ${me.coin}`);
      setLabel(expEl.children[0], `经验 ${me.exp}`);
    } catch {
      /* 刷新失败不阻塞大厅展示。 */
    }
  }

  unmount(): void {
    this.node?.destroy();
    this.node = null;
  }
}
