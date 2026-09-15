// 大厅屏幕：玩家概览 + 主玩法入口 + 快捷功能区。

import { Node } from 'cc';
import type { Screen, ScreenCtx } from '../app/context';
import {
  actionTile,
  button,
  centerIn,
  column,
  fullBackground,
  label,
  panel,
  pill,
  row,
  setLabel,
  setStatValue,
  statCard,
  uiNode,
} from '../ui/builder';
import { theme, withAlpha } from '../ui/theme';

export class LobbyScreen implements Screen {
  private node: Node | null = null;

  async mount(ctx: ScreenCtx): Promise<void> {
    this.node = uiNode('lobby-screen');
    ctx.root.addChild(this.node);
    fullBackground(this.node);

    const user = ctx.session.user;
    const nameEl = label(user?.nickname ?? '玩家', {
      width: 390,
      fontSize: theme.bigSize + 2,
      align: 'left',
    });
    const welcome = label('欢迎回来 · 经典竞技大厅', {
      width: 390,
      fontSize: theme.smallSize + 1,
      color: theme.muted,
      align: 'left',
    });
    const identity = column([nameEl, welcome], 0);

    const levelCard = statCard('等级', `Lv.${user?.level ?? 1}`, {
      width: 120,
      height: 78,
      accent: theme.primaryBright,
    });
    const coinCard = statCard('金币', `${user?.coin ?? 0}`, {
      width: 140,
      height: 78,
      accent: theme.warning,
      valueColor: theme.warning,
    });
    const expCard = statCard('经验', `${user?.exp ?? 0}`, {
      width: 140,
      height: 78,
      accent: theme.accent,
    });

    const header = row([identity, row([levelCard, coinCard, expCard], 10)], 18);

    const playCard = panel([
      row([
        pill('CLASSIC', {
          width: 100,
          color: withAlpha(theme.primary, 55),
          textColor: theme.primaryBright,
        }),
        pill('实时多人', {
          width: 110,
          color: withAlpha(theme.accent, 36),
          textColor: theme.accent,
        }),
      ], 10),
      label('经典吞噬战', { width: 440, fontSize: theme.titleSize, align: 'left' }),
      label('控制细胞移动和成长，分裂追击，躲避更大的对手。', {
        width: 440,
        fontSize: theme.smallSize + 2,
        color: theme.muted,
        align: 'left',
      }),
      button('开始匹配', () => ctx.go('match'), { width: 300, height: 64 }),
    ], 520, 10, 30, { height: 250, color: theme.cardStrong });

    const quick = column([
      actionTile('战绩', '回顾最近对局与奖励', () => ctx.go('records'), 240),
      actionTile('排行榜', '查看日榜 / 周榜 / 最高分', () => ctx.go('rank'), 240),
    ], 16);

    const shell = column([
      header,
      row([playCard, quick], 18),
      label('提示：对局中可使用 Space 分裂、W 吐球，也支持触屏按钮操作。', {
        width: 780,
        fontSize: theme.smallSize,
        color: theme.subtle,
      }),
    ], 22);
    centerIn(this.node, shell, 0);

    // 刷新最新资产（结算后金币/经验会变化）。
    try {
      const me = await ctx.api.getMe();
      if (!this.node?.isValid) return;
      if (ctx.session.user) {
        ctx.session.user.level = me.level;
        ctx.session.user.coin = me.coin;
        ctx.session.user.exp = me.exp;
      }
      setLabel(nameEl, me.nickname);
      setStatValue(levelCard, `Lv.${me.level}`);
      setStatValue(coinCard, `${me.coin}`);
      setStatValue(expCard, `${me.exp}`);
    } catch {
      /* 刷新失败不阻塞大厅展示 */
    }
  }

  unmount(): void {
    this.node?.destroy();
    this.node = null;
  }
}
