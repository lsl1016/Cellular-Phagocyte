// 战绩屏幕：轻量统计胶囊 + 对局列表，减少大块卡片。

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
  setLabel,
  uiNode,
} from '../ui/builder';
import { screenLayer } from '../ui/layout';
import { theme, withAlpha } from '../ui/theme';

const PAGE_SIZE = 6;

export class RecordsScreen implements Screen {
  private node: Node | null = null;
  private page = 1;
  private listEl: Node | null = null;
  private pagerEl: Node | null = null;

  async mount(ctx: ScreenCtx): Promise<void> {
    this.node = screenLayer(ctx.root, 'records-screen');
    fullBackground(this.node);

    const total = pill('总局数 --', { width: 116 });
    const first = pill('冠军 --', {
      width: 116,
      color: withAlpha(theme.warning, 32),
      textColor: theme.warning,
    });
    const top3 = pill('Top3 --', {
      width: 116,
      color: withAlpha(theme.accent, 26),
      textColor: theme.accent,
    });
    const best = pill('最高分 --', {
      width: 126,
      color: withAlpha(theme.primary, 36),
      textColor: theme.primaryBright,
    });
    const mass = pill('最大质量 --', { width: 136 });

    this.listEl = uiNode('records-list', 650, 270);
    this.pagerEl = uiNode('records-pager', 390, 46);

    const c = panel([
      label('战绩', { width: 650, fontSize: theme.titleSize, align: 'left' }),
      label('最近对局与奖励记录', {
        width: 650,
        fontSize: theme.smallSize + 1,
        color: theme.muted,
        align: 'left',
      }),
      row([total, first, top3, best, mass], 8),
      this.listEl,
      this.pagerEl,
      button('返回大厅', () => ctx.go('lobby'), {
        variant: 'secondary',
        width: 170,
        height: 46,
      }),
    ], 720, 10, 28, { height: 550, color: theme.cardStrong });
    centerIn(this.node, c);

    try {
      const s = await ctx.api.recordSummary();
      if (!this.node?.isValid) return;
      setLabel(total.children[0], `总局数 ${s.totalGames}`);
      setLabel(first.children[0], `冠军 ${s.firstPlaceCount}`);
      setLabel(top3.children[0], `Top3 ${s.top3Count}`);
      setLabel(best.children[0], `最高分 ${s.bestScore}`);
      setLabel(mass.children[0], `最大质量 ${s.maxMass}`);
    } catch {
      /* 统计失败不阻塞列表。 */
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
        let y = height / 2 - 22;
        for (const r of data.list) {
          const line = this.recordRow(r);
          line.setPosition(0, y, 0);
          this.listEl.addChild(line);
          y -= 42;
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
    const reward = rewardOk ? `+${r.coinReward}金 +${r.expReward}经` : '结算中';
    const time = new Date(r.endTime).toLocaleString();
    const rankColor = r.rank === 1 ? theme.warning : r.rank <= 3 ? theme.accent : theme.muted;
    return panel([
      row([
        label(`#${r.rank}/${r.totalPlayers}`, {
          width: 82,
          fontSize: theme.smallSize,
          color: rankColor,
          align: 'left',
        }),
        label(r.modeName, {
          width: 98,
          fontSize: theme.smallSize,
          align: 'left',
        }),
        label(`${r.finalScore} 分`, {
          width: 92,
          fontSize: theme.smallSize,
          color: theme.primaryBright,
          align: 'left',
        }),
        label(reward, {
          width: 128,
          fontSize: theme.smallSize,
          color: rewardOk ? theme.accent : theme.muted,
          align: 'left',
        }),
        label(time, {
          width: 188,
          fontSize: theme.microSize + 1,
          color: theme.subtle,
          align: 'right',
        }),
      ], 6),
    ], 650, 0, 8, {
      height: 36,
      color: withAlpha(theme.cardSoft, 120),
      borderColor: withAlpha(theme.cardBorder, 70),
      shadow: false,
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
      width: 110,
      height: 40,
      fontSize: theme.smallSize,
      disabled: this.page <= 1,
    });
    const current = pill(`${this.page} / ${totalPages}`, {
      width: 86,
      height: 32,
      color: withAlpha(theme.primary, 32),
      textColor: theme.primaryBright,
    });
    const next = button('下一页', () => {
      if (this.page < totalPages) {
        this.page++;
        void this.loadPage(ctx);
      }
    }, {
      variant: 'secondary',
      width: 110,
      height: 40,
      fontSize: theme.smallSize,
      disabled: this.page >= totalPages,
    });
    this.pagerEl.addChild(row([prev, current, next], 12));
  }

  unmount(): void {
    this.node?.destroy();
    this.node = null;
  }
}
