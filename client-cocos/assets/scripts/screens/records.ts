// 战绩屏幕：统计概览 + 紧凑对局列表 + 分页。

import { Node, UITransform } from 'cc';
import type { Screen, ScreenCtx } from '../app/context';
import type { RecordEntry } from '../core/protocol/http-models';
import {
  button,
  centerIn,
  clearChildren,
  fullBackground,
  label,
  panel,
  pill,
  row,
  setStatValue,
  statCard,
  uiNode,
} from '../ui/builder';
import { theme, withAlpha } from '../ui/theme';

const PAGE_SIZE = 6;

export class RecordsScreen implements Screen {
  private node: Node | null = null;
  private page = 1;
  private listEl: Node | null = null;
  private pagerEl: Node | null = null;

  async mount(ctx: ScreenCtx): Promise<void> {
    this.node = uiNode('records-screen');
    ctx.root.addChild(this.node);
    fullBackground(this.node);

    const totalCard = statCard('总局数', '--', { width: 116, height: 72, accent: theme.primaryBright });
    const firstCard = statCard('冠军', '--', { width: 116, height: 72, accent: theme.warning, valueColor: theme.warning });
    const top3Card = statCard('Top 3', '--', { width: 116, height: 72, accent: theme.accent });
    const scoreCard = statCard('最高分', '--', { width: 116, height: 72, accent: theme.primary });
    const massCard = statCard('最大质量', '--', { width: 116, height: 72, accent: theme.danger });

    this.listEl = uiNode('records-list', 650, 286);
    this.pagerEl = uiNode('records-pager', 400, 48);

    const header = row([
      label('战绩', { width: 500, fontSize: theme.titleSize, align: 'left' }),
      pill('HISTORY', {
        width: 110,
        color: withAlpha(theme.primary, 45),
        textColor: theme.primaryBright,
      }),
    ], 18);

    const c = panel([
      header,
      row([totalCard, firstCard, top3Card, scoreCard, massCard], 8),
      this.listEl,
      this.pagerEl,
      button('返回大厅', () => ctx.go('lobby'), {
        variant: 'ghost',
        width: 180,
        height: 48,
      }),
    ], 750, 8, 28, { height: 590, color: theme.cardStrong });
    centerIn(this.node, c);

    try {
      const s = await ctx.api.recordSummary();
      if (!this.node?.isValid) return;
      setStatValue(totalCard, `${s.totalGames}`);
      setStatValue(firstCard, `${s.firstPlaceCount}`);
      setStatValue(top3Card, `${s.top3Count}`);
      setStatValue(scoreCard, `${s.bestScore}`);
      setStatValue(massCard, `${s.maxMass}`);
    } catch {
      /* 列表仍可独立加载，统计失败不阻塞页面。 */
    }

    await this.loadPage(ctx);
  }

  private async loadPage(ctx: ScreenCtx): Promise<void> {
    if (!this.listEl?.isValid || !this.pagerEl?.isValid) return;
    clearChildren(this.listEl);
    this.listEl.addChild(label('正在加载最近对局...', { width: 650, color: theme.muted }));
    try {
      const data = await ctx.api.records(this.page, PAGE_SIZE);
      if (!this.listEl?.isValid) return;
      clearChildren(this.listEl);
      if (data.list.length === 0) {
        this.listEl.addChild(label('暂无战绩，先去完成一局经典模式吧', {
          width: 650,
          color: theme.muted,
        }));
      } else {
        const height = this.listEl.getComponent(UITransform)!.height;
        let y = height / 2 - 24;
        for (const r of data.list) {
          const line = this.recordRow(r);
          line.setPosition(0, y, 0);
          this.listEl.addChild(line);
          y -= 46;
        }
      }
      this.renderPager(ctx, data.total);
    } catch {
      clearChildren(this.listEl!);
      this.listEl!.addChild(label('战绩加载失败，请稍后重试', { width: 650, color: theme.danger }));
    }
  }

  private recordRow(r: RecordEntry): Node {
    const rewardOk = r.status === 'SUCCESS' || r.settlementStatus === 'SUCCESS';
    const reward = rewardOk ? `+${r.coinReward}金  +${r.expReward}经` : '结算中';
    const time = new Date(r.endTime).toLocaleString();
    const rankColor = r.rank === 1 ? theme.warning : r.rank <= 3 ? theme.accent : theme.muted;
    return panel([
      row([
        label(`#${r.rank}/${r.totalPlayers}`, {
          width: 82,
          fontSize: theme.smallSize + 1,
          color: rankColor,
          align: 'left',
        }),
        label(r.modeName, {
          width: 100,
          fontSize: theme.smallSize + 1,
          align: 'left',
        }),
        label(`${r.finalScore} 分`, {
          width: 92,
          fontSize: theme.smallSize + 1,
          color: theme.primaryBright,
          align: 'left',
        }),
        label(reward, {
          width: 130,
          fontSize: theme.smallSize,
          color: rewardOk ? theme.accent : theme.muted,
          align: 'left',
        }),
        label(time, {
          width: 190,
          fontSize: theme.microSize + 1,
          color: theme.subtle,
          align: 'right',
        }),
      ], 6),
    ], 650, 0, 8, {
      height: 40,
      color: withAlpha(theme.cardSoft, 215),
      shadow: false,
      borderColor: withAlpha(theme.cardBorder, 150),
    });
  }

  private renderPager(ctx: ScreenCtx, total: number): void {
    if (!this.pagerEl?.isValid) return;
    clearChildren(this.pagerEl);
    const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

    const prev = button('上一页', () => {
      if (this.page > 1) {
        this.page--;
        void this.loadPage(ctx);
      }
    }, {
      variant: 'secondary',
      width: 118,
      height: 42,
      fontSize: theme.smallSize + 1,
      disabled: this.page <= 1,
    });
    const current = pill(`${this.page} / ${totalPages}`, {
      width: 92,
      height: 34,
      color: withAlpha(theme.primary, 42),
      textColor: theme.primaryBright,
    });
    const next = button('下一页', () => {
      if (this.page < totalPages) {
        this.page++;
        void this.loadPage(ctx);
      }
    }, {
      variant: 'secondary',
      width: 118,
      height: 42,
      fontSize: theme.smallSize + 1,
      disabled: this.page >= totalPages,
    });

    const line = row([prev, current, next], 14);
    this.pagerEl.addChild(line);
  }

  unmount(): void {
    this.node?.destroy();
    this.node = null;
  }
}
