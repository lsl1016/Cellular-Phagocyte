// 排行榜场景：可切换日榜/周榜/最高分榜，展示 Top N 与自身名次。

import type { Scene, SceneCtx } from '../app/context.js';
import { button, clear, h } from '../ui/dom.js';

const TYPES: { key: string; name: string }[] = [
  { key: 'daily', name: '日榜' },
  { key: 'weekly', name: '周榜' },
  { key: 'best_score', name: '最高分' },
];

export class RankScene implements Scene {
  private current = 'daily';
  private tabsEl!: HTMLElement;
  private listEl!: HTMLElement;
  private selfEl!: HTMLElement;

  async mount(ctx: SceneCtx): Promise<void> {
    clear(ctx.uiRoot);
    this.tabsEl = h('div', { className: 'row' });
    this.listEl = h('div', { className: 'rec-list' });
    this.selfEl = h('div', { className: 'muted', text: '' });

    const card = h('div', { className: 'card', style: 'min-width:420px;max-width:520px' }, [
      h('div', { className: 'title', text: '排行榜' }),
      this.tabsEl,
      this.listEl,
      this.selfEl,
      button('返回大厅', () => ctx.go('lobby'), 'btn secondary'),
    ]);
    ctx.uiRoot.append(h('div', { className: 'panel centered' }, [card]));

    this.renderTabs(ctx);
    await this.load(ctx);
  }

  private renderTabs(ctx: SceneCtx): void {
    clear(this.tabsEl);
    for (const t of TYPES) {
      const b = button(t.name, () => {
        if (this.current !== t.key) {
          this.current = t.key;
          this.renderTabs(ctx);
          void this.load(ctx);
        }
      }, this.current === t.key ? 'btn' : 'btn secondary');
      this.tabsEl.append(b);
    }
  }

  private async load(ctx: SceneCtx): Promise<void> {
    clear(this.listEl);
    this.listEl.append(h('div', { className: 'muted', text: '加载中...' }));
    this.selfEl.textContent = '';
    try {
      const data = await ctx.api.ranks(this.current, 1, 20);
      clear(this.listEl);
      if (data.list.length === 0) {
        this.listEl.append(h('div', { className: 'muted', text: '榜单暂无数据' }));
      } else {
        for (const it of data.list) {
          this.listEl.append(
            h('div', { className: it.self ? 'rec-row me' : 'rec-row' }, [
              h('span', { className: 'rec-rank', text: `#${it.rank}` }),
              h('span', { text: it.nickname || it.userId }),
              h('span', { className: 'muted', text: String(it.score) }),
            ]),
          );
        }
      }
      if (data.selfRank.onRank && data.selfRank.rank !== null) {
        this.selfEl.textContent = `我的排名：第 ${data.selfRank.rank} 名 · ${data.selfRank.score} 分`;
      } else {
        this.selfEl.textContent = '我：未上榜';
      }
    } catch {
      clear(this.listEl);
      this.listEl.append(h('div', { className: 'muted', text: '榜单加载失败' }));
    }
  }

  unmount(): void {}
}
