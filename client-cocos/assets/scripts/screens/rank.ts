// 排行榜屏幕：紧凑榜单 + 轻量 tab，保持与 H5 相似的信息密度。

import { Node, UITransform } from 'cc';
import type { Screen, ScreenCtx } from '../app/context';
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
    this.node = screenLayer(ctx.root, 'rank-screen');
    fullBackground(this.node);

    this.tabsEl = uiNode('rank-tabs', 390, 42);
    this.listEl = uiNode('rank-list', 560, 320);
    this.selfEl = label('', { width: 560, color: theme.muted });

    const c = panel([
      label('排行榜', { width: 560, fontSize: theme.titleSize, align: 'left' }),
      label('日榜 / 周榜 / 历史最高分', {
        width: 560,
        fontSize: theme.smallSize + 1,
        color: theme.muted,
        align: 'left',
      }),
      this.tabsEl,
      this.listEl,
      this.selfEl,
      button('返回大厅', () => ctx.go('lobby'), {
        variant: 'secondary',
        width: 170,
        height: 46,
      }),
    ], 630, 10, 28, { height: 550, color: theme.cardStrong });
    centerIn(this.node, c);

    this.renderTabs(ctx);
    await this.load(ctx);
  }

  private renderTabs(ctx: ScreenCtx): void {
    if (!this.tabsEl?.isValid) return;
    clearChildren(this.tabsEl);
    const buttons = TYPES.map((t) => button(t.name, () => {
      if (this.current === t.key) return;
      this.current = t.key;
      this.renderTabs(ctx);
      void this.load(ctx);
    }, {
      variant: this.current === t.key ? 'primary' : 'secondary',
      width: t.key === 'best_score' ? 126 : 110,
      height: 38,
      fontSize: theme.smallSize,
    }));
    this.tabsEl.addChild(row(buttons, 10));
  }

  private async load(ctx: ScreenCtx): Promise<void> {
    if (!this.listEl?.isValid || !this.selfEl?.isValid) return;
    clearChildren(this.listEl);
    this.listEl.addChild(label('正在加载榜单...', { width: 560, color: theme.muted }));
    setLabel(this.selfEl, '');
    try {
      const data = await ctx.api.ranks(this.current, 1, 10);
      if (!this.listEl?.isValid) return;
      clearChildren(this.listEl);
      if (data.list.length === 0) {
        this.listEl.addChild(label('榜单暂无数据', { width: 560, color: theme.muted }));
      } else {
        const height = this.listEl.getComponent(UITransform)!.height;
        let y = height / 2 - 18;
        for (const it of data.list.slice(0, 10)) {
          const line = this.rankRow(it.rank, it.nickname || it.userId, it.score, it.self);
          line.setPosition(0, y, 0);
          this.listEl.addChild(line);
          y -= 31;
        }
      }
      if (data.selfRank.onRank && data.selfRank.rank !== null) {
        setLabel(this.selfEl, `我的排名  #${data.selfRank.rank} · ${data.selfRank.score} 分`);
      } else {
        setLabel(this.selfEl, '我的排名：暂未上榜');
      }
    } catch {
      clearChildren(this.listEl!);
      this.listEl!.addChild(label('榜单加载失败，请稍后重试', { width: 560, color: theme.danger }));
    }
  }

  private rankRow(rank: number, nickname: string, score: number, self: boolean): Node {
    const rankColor = rank <= 3 ? theme.warning : theme.muted;
    const bg = self ? withAlpha(theme.primary, 38) : withAlpha(theme.cardSoft, 100);
    return panel([
      row([
        label(rank <= 3 ? `TOP ${rank}` : `#${rank}`, {
          width: 76,
          fontSize: theme.smallSize,
          color: rankColor,
          align: 'left',
        }),
        label(nickname, {
          width: 320,
          fontSize: theme.smallSize + 1,
          color: self ? theme.primaryBright : theme.text,
          align: 'left',
        }),
        label(`${score}`, {
          width: 110,
          fontSize: theme.smallSize + 1,
          color: self ? theme.accent : theme.text,
          align: 'right',
        }),
      ], 8),
    ], 560, 0, 8, {
      height: 27,
      color: bg,
      borderColor: bg,
      shadow: false,
    });
  }

  unmount(): void {
    this.node?.destroy();
    this.node = null;
  }
}
