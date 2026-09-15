// 结算屏幕：突出名次、关键战斗数据与奖励。

import { Node } from 'cc';
import type { Screen, ScreenCtx } from '../app/context';
import {
  button,
  centerIn,
  fullBackground,
  label,
  panel,
  pill,
  row,
  statCard,
  uiNode,
} from '../ui/builder';
import { theme, withAlpha } from '../ui/theme';

export class SettlementScreen implements Screen {
  private node: Node | null = null;

  async mount(ctx: ScreenCtx): Promise<void> {
    this.node = uiNode('settlement-screen');
    ctx.root.addChild(this.node);
    fullBackground(this.node);
    const s = ctx.session.settlement;

    if (!s) {
      const empty = panel([
        label('对局结算', { fontSize: theme.titleSize }),
        label('暂无结算数据', { color: theme.muted }),
        button('返回大厅', () => ctx.go('lobby'), { variant: 'secondary' }),
      ], 460, 18, 30, { height: 300, color: theme.cardStrong });
      centerIn(this.node, empty);
      return;
    }

    const rankTitle = s.rank === 1 ? '冠军' : `第 ${s.rank} 名`;
    const rankColor = s.rank === 1 ? theme.warning : theme.primaryBright;
    const rankBadge = pill(`${rankTitle} / ${s.totalPlayers}`, {
      width: 180,
      height: 34,
      color: withAlpha(rankColor, 42),
      textColor: rankColor,
    });

    const primaryStats = row([
      statCard('最终得分', `${s.finalScore}`, {
        width: 170,
        accent: theme.primaryBright,
        valueColor: theme.primaryBright,
      }),
      statCard('最大质量', `${s.maxMass}`, {
        width: 170,
        accent: theme.accent,
      }),
      statCard('存活时间', `${s.aliveSeconds}s`, {
        width: 170,
        accent: theme.warning,
      }),
    ], 12);

    const battleStats = row([
      statCard('吞噬玩家', `${s.eatPlayerCount}`, {
        width: 170,
        height: 76,
        accent: theme.danger,
      }),
      statCard('吃掉食物', `${s.eatFoodCount}`, {
        width: 170,
        height: 76,
        accent: theme.accent,
      }),
    ], 12);

    const reward = panel([
      label('本局奖励', {
        width: 480,
        fontSize: theme.smallSize + 2,
        color: theme.muted,
        align: 'left',
      }),
      row([
        pill(`+${s.coinReward} 金币`, {
          width: 160,
          color: withAlpha(theme.warning, 42),
          textColor: theme.warning,
        }),
        pill(`+${s.expReward} 经验`, {
          width: 160,
          color: withAlpha(theme.accent, 35),
          textColor: theme.accent,
        }),
      ], 14),
    ], 540, 6, 18, { height: 92, color: theme.cardSoft, shadow: false });

    const actions = row([
      button('返回大厅', () => ctx.go('lobby'), {
        variant: 'secondary',
        width: 180,
        height: 58,
      }),
      button('再来一局', () => ctx.go('match'), { width: 220, height: 58 }),
    ], 16);

    const c = panel([
      rankBadge,
      label(rankTitle, { width: 560, fontSize: theme.heroSize, color: rankColor }),
      label('本局表现', { width: 560, fontSize: theme.smallSize + 2, color: theme.muted }),
      primaryStats,
      battleStats,
      reward,
      actions,
    ], 650, 10, 28, { height: 560, color: theme.cardStrong });
    centerIn(this.node, c);
  }

  unmount(): void {
    this.node?.destroy();
    this.node = null;
  }
}
