// 结算屏幕：突出名次和奖励，减少多层卡片堆叠。

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
} from '../ui/builder';
import { screenLayer } from '../ui/layout';
import { theme, withAlpha } from '../ui/theme';

export class SettlementScreen implements Screen {
  private node: Node | null = null;

  async mount(ctx: ScreenCtx): Promise<void> {
    this.node = screenLayer(ctx.root, 'settlement-screen');
    fullBackground(this.node);
    const s = ctx.session.settlement;

    if (!s) {
      const empty = panel([
        label('对局结算', { fontSize: theme.titleSize }),
        label('暂无结算数据', { color: theme.muted }),
        button('返回大厅', () => ctx.go('lobby'), { variant: 'secondary' }),
      ], 440, 16, 28, { height: 280, color: theme.cardStrong });
      centerIn(this.node, empty);
      return;
    }

    const rankTitle = s.rank === 1 ? '冠军' : `第 ${s.rank} 名`;
    const rankColor = s.rank === 1 ? theme.warning : theme.primaryBright;

    const stats1 = row([
      this.metric('最终得分', `${s.finalScore}`, theme.primaryBright),
      this.metric('最大质量', `${s.maxMass}`, theme.accent),
      this.metric('存活时间', `${s.aliveSeconds}s`, theme.warning),
    ], 18);
    const stats2 = row([
      this.metric('吞噬玩家', `${s.eatPlayerCount}`, theme.danger),
      this.metric('吃掉食物', `${s.eatFoodCount}`, theme.accent),
    ], 28);

    const actions = row([
      button('返回大厅', () => ctx.go('lobby'), {
        variant: 'secondary',
        width: 170,
        height: 52,
      }),
      button('再来一局', () => ctx.go('match'), {
        width: 210,
        height: 52,
      }),
    ], 16);

    const c = panel([
      pill(`${rankTitle} / ${s.totalPlayers}`, {
        width: 170,
        height: 30,
        color: withAlpha(rankColor, 36),
        textColor: rankColor,
      }),
      label(rankTitle, { width: 500, fontSize: theme.heroSize, color: rankColor }),
      label('本局表现', { width: 500, fontSize: theme.smallSize + 2, color: theme.muted }),
      stats1,
      stats2,
      divider(480),
      label('本局奖励', {
        width: 480,
        fontSize: theme.smallSize + 1,
        color: theme.muted,
        align: 'left',
      }),
      row([
        pill(`+${s.coinReward} 金币`, {
          width: 160,
          color: withAlpha(theme.warning, 34),
          textColor: theme.warning,
        }),
        pill(`+${s.expReward} 经验`, {
          width: 160,
          color: withAlpha(theme.accent, 28),
          textColor: theme.accent,
        }),
      ], 16),
      actions,
    ], 580, 10, 28, { height: 540, color: theme.cardStrong });
    centerIn(this.node, c);
  }

  private metric(title: string, value: string, color: any): Node {
    return panel([
      label(title, {
        width: 120,
        fontSize: theme.smallSize,
        color: theme.muted,
      }),
      label(value, {
        width: 120,
        fontSize: theme.bigSize - 4,
        color,
      }),
    ], 140, 0, 8, {
      height: 70,
      color: withAlpha(theme.cardSoft, 150),
      borderColor: withAlpha(theme.cardBorder, 90),
      shadow: false,
    });
  }

  unmount(): void {
    this.node?.destroy();
    this.node = null;
  }
}
