// 匹配屏幕：单卡片、明确状态、较少装饰，等待信息优先。

import { Node, Sprite } from 'cc';
import { config } from '../core/config';
import type { Screen, ScreenCtx } from '../app/context';
import { logger } from '../core/logger';
import { ApiError } from '../net/http';
import {
  button,
  centerIn,
  fullBackground,
  label,
  panel,
  pill,
  row,
  setLabel,
  uiNode,
} from '../ui/builder';
import { screenLayer } from '../ui/layout';
import { circleTexture, ringTexture } from '../ui/texgen';
import { theme, withAlpha } from '../ui/theme';

export class MatchScreen implements Screen {
  private node: Node | null = null;
  private matchId: string | null = null;
  private polling = false;
  private timer: ReturnType<typeof setTimeout> | null = null;
  private elapsed = 0;
  private tipEl: Node | null = null;
  private timeEl: Node | null = null;
  private spinner: Node | null = null;

  async mount(ctx: ScreenCtx): Promise<void> {
    this.node = screenLayer(ctx.root, 'match-screen');
    fullBackground(this.node);

    this.spinner = this.createSpinner();
    this.timeEl = label('00:00', { width: 160, fontSize: theme.bigSize + 2 });
    this.tipEl = label('正在寻找合适的对手...', {
      width: 340,
      fontSize: theme.smallSize + 2,
      color: theme.muted,
    });

    const c = panel([
      row([
        pill('CLASSIC', {
          width: 100,
          color: withAlpha(theme.primary, 45),
          textColor: theme.primaryBright,
        }),
        pill('自动匹配', {
          width: 106,
          color: withAlpha(theme.accent, 28),
          textColor: theme.accent,
        }),
      ], 10),
      label('匹配中', { width: 340, fontSize: theme.titleSize }),
      this.spinner,
      this.timeEl,
      this.tipEl,
      label('匹配成功后会自动进入房间', {
        width: 340,
        fontSize: theme.smallSize,
        color: theme.subtle,
      }),
      button('取消匹配', () => void this.cancel(ctx), {
        variant: 'secondary',
        width: 220,
        height: 50,
      }),
    ], 440, 10, 28, { height: 450, color: theme.cardStrong });
    centerIn(this.node, c);

    try {
      const res = await ctx.api.matchStart('classic');
      this.matchId = res.matchId;
      this.polling = true;
      this.poll(ctx);
    } catch (e) {
      const msg = e instanceof ApiError ? e.message : '匹配失败';
      ctx.toast(msg);
      ctx.go('lobby');
    }
  }

  private createSpinner(): Node {
    const root = uiNode('match-spinner', 92, 92);
    const ring = root.addComponent(Sprite);
    ring.spriteFrame = ringTexture();
    ring.sizeMode = Sprite.SizeMode.CUSTOM;
    ring.color = theme.primaryBright;

    const cell = uiNode('match-cell', 54, 54);
    const cellSp = cell.addComponent(Sprite);
    cellSp.spriteFrame = circleTexture();
    cellSp.sizeMode = Sprite.SizeMode.CUSTOM;
    cellSp.color = theme.primary;
    root.addChild(cell);

    const nucleus = uiNode('match-nucleus', 16, 16);
    const nucleusSp = nucleus.addComponent(Sprite);
    nucleusSp.spriteFrame = circleTexture();
    nucleusSp.sizeMode = Sprite.SizeMode.CUSTOM;
    nucleusSp.color = theme.accent;
    nucleus.setPosition(9, 9, 0);
    root.addChild(nucleus);
    return root;
  }

  private poll(ctx: ScreenCtx): void {
    if (!this.polling || !this.matchId) return;
    this.timer = setTimeout(async () => {
      if (!this.polling || !this.matchId) return;
      try {
        const st = await ctx.api.matchStatus(this.matchId);
        if (st.status === 'MATCHED' && st.roomId && st.enterToken) {
          this.polling = false;
          ctx.session.match = {
            roomId: st.roomId,
            enterToken: st.enterToken,
            wsUrl: config.wsUrl,
          };
          logger.info('match_success', { roomId: st.roomId });
          ctx.go('game');
          return;
        }
        this.elapsed += 1;
        this.updateElapsed();
        if (this.tipEl?.isValid) {
          setLabel(this.tipEl, this.elapsed >= 10 ? '仍在搜索，请稍候...' : '正在寻找合适的对手...');
        }
        this.poll(ctx);
      } catch (e) {
        this.polling = false;
        const msg = e instanceof ApiError ? e.message : '匹配查询失败';
        ctx.toast(msg);
        ctx.go('lobby');
      }
    }, 1000);
  }

  private updateElapsed(): void {
    if (!this.timeEl?.isValid) return;
    const minutes = Math.floor(this.elapsed / 60).toString().padStart(2, '0');
    const seconds = (this.elapsed % 60).toString().padStart(2, '0');
    setLabel(this.timeEl, `${minutes}:${seconds}`);
  }

  private async cancel(ctx: ScreenCtx): Promise<void> {
    this.polling = false;
    if (this.matchId) {
      try {
        await ctx.api.matchCancel(this.matchId);
      } catch {
        /* 取消失败忽略。 */
      }
    }
    ctx.go('lobby');
  }

  update(dt: number): void {
    if (!this.spinner?.isValid) return;
    const r = this.spinner.eulerAngles;
    this.spinner.setRotationFromEuler(0, 0, r.z - dt * 150);
  }

  unmount(): void {
    this.polling = false;
    if (this.timer) clearTimeout(this.timer);
    this.node?.destroy();
    this.node = null;
  }
}
