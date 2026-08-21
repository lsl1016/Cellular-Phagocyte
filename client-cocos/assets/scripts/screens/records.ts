// 战绩屏幕：分页展示个人历史对局，并显示统计概览。

import { Node } from 'cc';
import type { Screen, ScreenCtx } from '../app/context';
import type { RecordEntry } from '../core/protocol/http-models';
import {
  button,
  card,
  centerIn,
  clearChildren,
  column,
  fullBackground,
  label,
  row,
  setLabel,
  uiNode,
} from '../ui/builder';
import { theme } from '../ui/theme';

const PAGE_SIZE = 8;

export class RecordsScreen implements Screen {
  private node: Node | null = null;
  private page = 1;
  private summaryEl: Node | null = null;
  private listEl: Node | null = null;
  private pagerEl: Node | null = null;

  async mount(ctx: ScreenCtx): Promise<void> {
    this.node = uiNode('records-screen');
    ctx.root.addChild(this.node);
    fullBackground(this.node);

    this.summaryEl = label('加载统计中...', { color: theme.muted });
    this.listEl = column([]);
    this.pagerEl = row([]);

    const c = card([
      label('战绩', { fontSize: theme.titleSize }),
      this.summaryEl,
      this.listEl,
      this.pagerEl,
      button('返回大厅', () => ctx.go('lobby'), { secondary: true }),
    ], 520);
    centerIn(this.node, c);

    try {
      const s = await ctx.api.recordSummary();
      if (!this.summaryEl?.isValid) return;
      setLabel(
        this.summaryEl,
        `总局数 ${s.totalGames} · 冠军 ${s.firstPlaceCount} · Top3 ${s.top3Count} · ` +
        `最高分 ${s.bestScore} · 最大质量 ${s.maxMass}`,
      );
    } catch {
      if (this.summaryEl?.isValid) setLabel(this.summaryEl, '统计加载失败');
    }

    await this.loadPage(ctx);
  }

  private async loadPage(ctx: ScreenCtx): Promise<void> {
    if (!this.listEl?.isValid || !this.pagerEl?.isValid) return;
    clearChildren(this.listEl);
    this.listEl.addChild(label('加载中...', { color: theme.muted }));
    try {
      const data = await ctx.api.records(this.page, PAGE_SIZE);
      if (!this.listEl?.isValid) return;
      clearChildren(this.listEl);
      if (data.list.length === 0) {
        this.listEl.addChild(label('暂无战绩，先去玩一局吧', { color: theme.muted }));
      } else {
        for (const r of data.list) this.listEl.addChild(this.row(r));
      }
      this.renderPager(ctx, data.total);
    } catch {
      clearChildren(this.listEl!);
      this.listEl!.addChild(label('战绩加载失败', { color: theme.muted }));
    }
  }

  private row(r: RecordEntry): Node {
    const reward =
      r.status === 'SUCCESS' || r.settlementStatus === 'SUCCESS'
        ? `+${r.coinReward}金 +${r.expReward}经`
        : '结算中';
    const time = new Date(r.endTime).toLocaleString();
    return label(
      `第${r.rank}/${r.totalPlayers} · ${r.modeName} · ${r.finalScore}分 · ${reward} · ${time}`,
      { width: 464, fontSize: theme.smallSize + 2, align: 'left', color: theme.text },
    );
  }

  private renderPager(ctx: ScreenCtx, total: number): void {
    if (!this.pagerEl?.isValid) return;
    clearChildren(this.pagerEl);
    const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
    this.pagerEl.addChild(
      button('上一页', () => {
        if (this.page > 1) {
          this.page--;
          void this.loadPage(ctx);
        }
      }, { secondary: true, width: 120, height: 48, disabled: this.page <= 1 }),
    );
    this.pagerEl.addChild(label(`${this.page}/${totalPages}`, { width: 80, color: theme.muted }));
    this.pagerEl.addChild(
      button('下一页', () => {
        if (this.page < totalPages) {
          this.page++;
          void this.loadPage(ctx);
        }
      }, { secondary: true, width: 120, height: 48, disabled: this.page >= totalPages }),
    );
    // 手动水平排布（pager 是 row 容器，但子节点动态变化，直接定位）
    const kids = this.pagerEl.children;
    let x = -150;
    for (const k of kids) {
      k.setPosition(x + 60, 0, 0);
      x += 140;
    }
  }

  unmount(): void {
    this.node?.destroy();
    this.node = null;
  }
}
