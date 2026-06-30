// 匹配场景：发起匹配并轮询状态，成功后进入对战。

import { config } from '../app/config.js';
import type { Scene, SceneCtx } from '../app/context.js';
import { logger } from '../common/logger.js';
import { ApiError } from '../network/http.js';
import { button, clear, h } from '../ui/dom.js';

export class MatchScene implements Scene {
  private matchId: string | null = null;
  private polling = false;
  private timer: ReturnType<typeof setTimeout> | null = null;
  private elapsed = 0;
  private tipEl!: HTMLElement;

  async mount(ctx: SceneCtx): Promise<void> {
    clear(ctx.uiRoot);
    this.tipEl = h('div', { className: 'subtitle', text: '正在匹配...' });
    const card = h('div', { className: 'card' }, [
      h('div', { className: 'title', text: '匹配中' }),
      h('div', { className: 'spinner' }),
      this.tipEl,
      button('取消匹配', () => void this.cancel(ctx), 'btn secondary'),
    ]);
    ctx.uiRoot.append(h('div', { className: 'panel centered' }, [card]));

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

  private poll(ctx: SceneCtx): void {
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
        this.tipEl.textContent = `正在匹配... ${this.elapsed}s`;
        this.poll(ctx);
      } catch (e) {
        this.polling = false;
        const msg = e instanceof ApiError ? e.message : '匹配查询失败';
        ctx.toast(msg);
        ctx.go('lobby');
      }
    }, 1000);
  }

  private async cancel(ctx: SceneCtx): Promise<void> {
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

  unmount(): void {
    this.polling = false;
    if (this.timer) clearTimeout(this.timer);
  }
}
