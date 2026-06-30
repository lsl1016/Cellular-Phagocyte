// 战绩场景：分页展示个人历史对局，并显示统计概览。

import type { Scene, SceneCtx } from '../app/context.js';
import type { RecordEntry } from '../protocol/http-models.js';
import { button, clear, h } from '../ui/dom.js';

const PAGE_SIZE = 8;

export class RecordsScene implements Scene {
  private page = 1;

  async mount(ctx: SceneCtx): Promise<void> {
    clear(ctx.uiRoot);

    const summaryEl = h('div', { className: 'muted', text: '加载统计中...' });
    const listEl = h('div', { className: 'rec-list' });
    const pager = h('div', { className: 'row' });

    const card = h('div', { className: 'card', style: 'min-width:420px;max-width:520px' }, [
      h('div', { className: 'title', text: '战绩' }),
      summaryEl,
      listEl,
      pager,
      button('返回大厅', () => ctx.go('lobby'), 'btn secondary'),
    ]);
    ctx.uiRoot.append(h('div', { className: 'panel centered' }, [card]));

    try {
      const sm = await ctx.api.recordSummary();
      summaryEl.textContent = `共 ${sm.totalGames} 局 · 冠军 ${sm.firstPlaceCount} · 前三 ${sm.top3Count} · 最高分 ${sm.bestScore}`;
    } catch {
      summaryEl.textContent = '统计加载失败';
    }

    await this.loadPage(ctx, listEl, pager);
  }

  private async loadPage(ctx: SceneCtx, listEl: HTMLElement, pager: HTMLElement): Promise<void> {
    clear(listEl);
    listEl.append(h('div', { className: 'muted', text: '加载中...' }));
    try {
      const data = await ctx.api.records(this.page, PAGE_SIZE);
      clear(listEl);
      if (data.list.length === 0) {
        listEl.append(h('div', { className: 'muted', text: '暂无战绩，先去玩一局吧' }));
      } else {
        for (const r of data.list) listEl.append(this.row(r));
      }
      this.renderPager(ctx, listEl, pager, data.total);
    } catch {
      clear(listEl);
      listEl.append(h('div', { className: 'muted', text: '战绩加载失败' }));
    }
  }

  private row(r: RecordEntry): HTMLElement {
    const reward = r.status === 'SUCCESS' || r.settlementStatus === 'SUCCESS'
      ? `+${r.coinReward}金 +${r.expReward}经`
      : '结算中';
    const time = new Date(r.endTime).toLocaleString();
    return h('div', { className: 'rec-row' }, [
      h('span', { className: 'rec-rank', text: `第${r.rank}/${r.totalPlayers}` }),
      h('span', { text: `${r.modeName} · ${r.finalScore}分` }),
      h('span', { className: 'muted', text: reward }),
      h('span', { className: 'muted', text: time }),
    ]);
  }

  private renderPager(ctx: SceneCtx, listEl: HTMLElement, pager: HTMLElement, total: number): void {
    clear(pager);
    const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
    const prev = button('上一页', () => {
      if (this.page > 1) {
        this.page--;
        void this.loadPage(ctx, listEl, pager);
      }
    }, 'btn secondary');
    const next = button('下一页', () => {
      if (this.page < totalPages) {
        this.page++;
        void this.loadPage(ctx, listEl, pager);
      }
    }, 'btn secondary');
    prev.disabled = this.page <= 1;
    next.disabled = this.page >= totalPages;
    pager.append(prev, h('span', { className: 'muted', text: `${this.page}/${totalPages}` }), next);
  }

  unmount(): void {}
}
