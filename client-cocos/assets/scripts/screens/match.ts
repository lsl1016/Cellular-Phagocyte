// 匹配屏幕：发起匹配并轮询状态，成功后进入对战。

import { Node } from 'cc';
import { config } from '../core/config';
import type { Screen, ScreenCtx } from '../app/context';
import { logger } from '../core/logger';
import { ApiError } from '../net/http';
import { button, card, centerIn, fullBackground, label, setLabel, uiNode } from '../ui/builder';
import { theme } from '../ui/theme';

export class MatchScreen implements Screen {
  private node: Node | null = null;
  private matchId: string | null = null;
  private polling = false;
  private timer: ReturnType<typeof setTimeout> | null = null;
  private elapsed = 0;
  private tipEl: Node | null = null;
  private spinner: Node | null = null;

  async mount(ctx: ScreenCtx): Promise<void> {
    this.node = uiNode('match-screen');
    ctx.root.addChild(this.node);
    fullBackground(this.node);

    this.tipEl = label('正在匹配...', { color: theme.muted });
    this.spinner = label('◌', { fontSize: 48, color: theme.primary });

    const c = card([
      label('匹配中', { fontSize: theme.titleSize }),
      this.spinner,
      this.tipEl,
      button('取消匹配', () => void this.cancel(ctx), { secondary: true }),
    ]);
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
            // 使用客户端配置推导的 wsUrl，避免服务端返回的 host 与实际不一致
            wsUrl: config.wsUrl,
          };
          logger.info('match_success', { roomId: st.roomId });
          ctx.go('game');
          return;
        }
        this.elapsed += 1;
        if (this.tipEl?.isValid) setLabel(this.tipEl, `正在匹配... ${this.elapsed}s`);
        this.poll(ctx);
      } catch (e) {
        this.polling = false;
        const msg = e instanceof ApiError ? e.message : '匹配查询失败';
        ctx.toast(msg);
        ctx.go('lobby');
      }
    }, 1000);
  }

  private async cancel(ctx: ScreenCtx): Promise<void> {
    this.polling = false;
    if (this.matchId) {
      try {
        await ctx.api.matchCancel(this.matchId);
      } catch {
        /* 取消失败忽略 */
      }
    }
    ctx.go('lobby');
  }

  update(dt: number): void {
    if (this.spinner?.isValid) {
      const r = this.spinner.eulerAngles;
      this.spinner.setRotationFromEuler(0, 0, r.z - dt * 240);
    }
  }

  unmount(): void {
    this.polling = false;
    if (this.timer) clearTimeout(this.timer);
    this.node?.destroy();
    this.node = null;
  }
}
