// 排行榜屏幕：日榜 / 周榜 / 最高分，统一新版 Surface 与列表层级。

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
    this.node = uiNode('rank-screen');
    ctx.root.addChild(this.node);
    fullBackground(this.node);

    this.tabsEl = uiNode('rank-tabs', 420, 46);
    this.listEl = uiNode('rank-list', 620, 330);
    this.selfEl = label('', { width: 620, color: theme.muted });

    const header = row([
      label('排行榜', { width: 460, fontSize: theme.titleSize, align: 'left' }),
      pill('TOP PLAYERS', {
        width: 130,
        color: withAlpha(theme.primary, 45),
        textColor: theme.primaryBright,
      }),
    ], 18);

    const c = panel([
      header,
      this.tabsEl,
      this.listEl,
      this.selfEl,
      button('返回大厅', () => ctx.go('lobby'), {
        variant: 'ghost',
        width: 180,
        height: 50,
      }),
    ], 720, 10, 28, { height: 570, color: theme.cardStrong });
    centerIn(this.node, c);

    this.renderTabs(ctx);
    await this.load(ctx);
  }

  private renderTabs(ctx: ScreenCtx): void {
    if (!this.tabsEl?.isValid) return;
    clearChildren(this.tabsEl);
    const widths = [124, 124, 140];
    const gap = 10;
    const total = widths.reduce((a, b) => a + b, 0) + gap * 2;
    let x = -total / 2;
    for (let i = 0; i < TYPES.length; i++) {
      const t = TYPES[i];
      const w = widths[i];
      const b = button(t.name, () => {
        if (this.current !== t.key) {
          this.current = t.key;
          this.renderTabs(ctx);
          void this.load(ctx);
        }
      }, {
        variant: this.current === t.key ? 'primary' : 'secondary',
        width: w,
        height: 42,
        fontSize: theme.smallSize + 2,
      });
      b.setPosition(x + w / 2, 0, 0);
      this.tabsEl.addChild(b);
      x += w + gap;
    }
  }

  private async load(ctx: ScreenCtx): Promise<void> {
    if (!this.listEl?.isValid || !this.selfEl?.isValid) return;
    clearChildren(this.listEl);
    const loading = label('正在加载榜单...', { width: 620, color: theme.muted });
    this.listEl.addChild(loading);
    setLabel(this.selfEl, '');
    try {
      const data = await ctx.api.ranks(this.current, 1, 10);
      if (!this.listEl?.isValid) return;
      clearChildren(this.listEl);
      if (data.list.length === 0) {
        this.listEl.addChild(label('榜单暂无数据', { width: 620, color: theme.muted }));
      } else {
        const height = this.listEl.getComponent(UITransform)!.height;
        let y = height / 2 - 18;
        for (const it of data.list.slice(0, 10)) {
          const rowNode = this.rankRow(it.rank, it.nickname || it.userId, it.score, it.self);
          rowNode.setPosition(0, y, 0);
          this.listEl.addChild(rowNode);
          y -= 32;
        }
      }
      if (data.selfRank.onRank && data.selfRank.rank !== null) {
        setLabel(this.selfEl, `我的排名  #${data.selfRank.rank}   ·   ${data.selfRank.score} 分`);
      } else {
        setLabel(this.selfEl, '我的排名：暂未上榜');
      }
    } catch {
      clearChildren(this.listEl!);
      this.listEl!.addChild(label('榜单加载失败，请稍后重试', { width: 620, color: theme.danger }));
    }
  }

  private rankRow(rank: number, nickname: string, score: number, self: boolean): Node {
    const rankColor = rank <= 3 ? theme.warning : theme.muted;
    const bg = self ? withAlpha(theme.primary, 45) : withAlpha(theme.cardSoft, 215);
    return panel([
      row([
        label(rank <= 3 ? `TOP ${rank}` : `#${rank}`, {
          width: 82,
          fontSize: theme.smallSize + 1,
          color: rankColor,
          align: 'left',
        }),
        label(nickname, {
          width: 330,
          fontSize: theme.smallSize + 2,
          color: self ? theme.primaryBright : theme.text,
          align: 'left',
        }),
        label(`${score}`, {
          width: 130,
          fontSize: theme.smallSize + 2,
          color: self ? theme.accent : theme.text,
          align: 'right',
        }),
      ], 8),
    ], 620, 0, 8, { height: 28, color: bg, shadow: false, borderColor: bg });
  }

  unmount(): void {
    this.node?.destroy();
    this.node = null;
  }
}
