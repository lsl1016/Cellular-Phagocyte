// 排行榜屏幕：可切换日榜/周榜/最高分榜，展示 Top N 与自身名次。

import { Node } from 'cc';
import type { Screen, ScreenCtx } from '../app/context';
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

const TYPES: { key: string; name: string }[] = [
  { key: 'daily', name: '日榜' },
  { key: 'weekly', name: '周榜' },
  { key: 'best_score', name: '最高分' },
];

export class RankScreen implements Screen {
  private node: Node | null = null;
  private current = 'daily';
  private tabsEl: Node | null = null;
  private listEl: Node | null = null;
  private selfEl: Node | null = null;

  async mount(ctx: ScreenCtx): Promise<void> {
    this.node = uiNode('rank-screen');
    ctx.root.addChild(this.node);
    fullBackground(this.node);

    this.tabsEl = row([]);
    this.listEl = column([], 6);
    this.selfEl = label('', { color: theme.muted });

    const c = card([
      label('排行榜', { fontSize: theme.titleSize }),
      this.tabsEl,
      this.listEl,
      this.selfEl,
      button('返回大厅', () => ctx.go('lobby'), { secondary: true }),
    ], 520);
    centerIn(this.node, c);

    this.renderTabs(ctx);
    await this.load(ctx);
  }

  private renderTabs(ctx: ScreenCtx): void {
    if (!this.tabsEl?.isValid) return;
    clearChildren(this.tabsEl);
    let x = -180;
    for (const t of TYPES) {
      const b = button(t.name, () => {
        if (this.current !== t.key) {
          this.current = t.key;
          this.renderTabs(ctx);
          void this.load(ctx);
        }
      }, {
        secondary: this.current !== t.key,
        width: 110,
        height: 48,
        fontSize: theme.smallSize + 4,
      });
      b.setPosition(x + 55, 0, 0);
      this.tabsEl.addChild(b);
      x += 120;
    }
  }

  private async load(ctx: ScreenCtx): Promise<void> {
    if (!this.listEl?.isValid || !this.selfEl?.isValid) return;
    clearChildren(this.listEl);
    this.listEl.addChild(label('加载中...', { color: theme.muted }));
    setLabel(this.selfEl, '');
    try {
      const data = await ctx.api.ranks(this.current, 1, 20);
      if (!this.listEl?.isValid) return;
      clearChildren(this.listEl);
      if (data.list.length === 0) {
        this.listEl.addChild(label('榜单暂无数据', { color: theme.muted }));
      } else {
        for (const it of data.list) {
          this.listEl.addChild(
            label(
              `#${it.rank}  ${it.nickname || it.userId}  ·  ${it.score}${it.self ? '（我）' : ''}`,
              {
                width: 464,
                fontSize: theme.smallSize + 2,
                align: 'left',
                color: it.self ? theme.primaryText : theme.text,
              },
            ),
          );
        }
      }
      if (data.selfRank.onRank && data.selfRank.rank !== null) {
        setLabel(this.selfEl, `我的排名：第 ${data.selfRank.rank} 名 · ${data.selfRank.score} 分`);
      } else {
        setLabel(this.selfEl, '我：未上榜');
      }
    } catch {
      clearChildren(this.listEl!);
      this.listEl!.addChild(label('榜单加载失败', { color: theme.muted }));
    }
  }

  unmount(): void {
    this.node?.destroy();
    this.node = null;
  }
}
