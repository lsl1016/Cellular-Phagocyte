// 匹配屏幕：模式信息、动态匹配环、等待时长和取消操作。

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
    this.node = uiNode('match-screen');
    ctx.root.addChild(this.node);
    fullBackground(this.node);

    this.tipEl = label('正在寻找合适的对手...', {
      width: 340,
      fontSize: theme.smallSize + 2,
      color: theme.muted,
    });
    this.timeEl = label('00:00', { width: 180, fontSize: theme.bigSize, color: theme.text });
    this.spinner = this.createSpinner();

    const info = row([
      pill('CLASSIC', {
        width: 100,
        color: withAlpha(theme.primary, 55),
        textColor: theme.primaryBright,
      }),
      pill('自动匹配', {
        width: 110,
        color: withAlpha(theme.accent, 32),
        textColor: theme.accent,
      }),
    ], 10);

    const c = panel([
      info,
      label('匹配中', { width: 360, fontSize: theme.titleSize }),
      this.spinner,
      this.timeEl,
      this.tipEl,
      label('保持页面开启，匹配成功后会自动进入房间', {
        width: 360,
        fontSize: theme.smallSize,
        color: theme.subtle,
      }),
      button('取消匹配', () => void this.cancel(ctx), {
        variant: 'ghost',
        width: 220,
        height: 54,
      }),
    ], 500, 10, 30, { height: 500, color: theme.cardStrong });
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
    const root = uiNode('match-spinner', 106, 106);
    const ring = root.addComponent(Sprite);
    ring.spriteFrame = ringTexture();
    ring.sizeMode = Sprite.SizeMode.CUSTOM;
    ring.color = theme.primaryBright;

    const cell = uiNode('match-cell', 62, 62);
    const cellSp = cell.addComponent(Sprite);
    cellSp.spriteFrame = circleTexture();
    cellSp.sizeMode = Sprite.SizeMode.CUSTOM;
    cellSp.color = withAlpha(theme.primary, 225);
    root.addChild(cell);

    const nucleus = uiNode('match-nucleus', 18, 18);
    const nucleusSp = nucleus.addComponent(Sprite);
    nucleusSp.spriteFrame = circleTexture();
    nucleusSp.sizeMode = Sprite.SizeMode.CUSTOM;
    nucleusSp.color = theme.accent;
    nucleus.setPosition(10, 10, 0);
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
            // 使用客户端配置推导的 wsUrl，避免服务端返回的 host 与实际不一致。
            wsUrl: config.wsUrl,
          };
          logger.info('match_success', { roomId: st.roomId });
          ctx.go('game');
          return;
        }
        this.elapsed += 1;
        this.updateElapsed();
        if (this.tipEl?.isValid) {
          const text = this.elapsed >= 10 ? '仍在搜索，请稍候...' : '正在寻找合适的对手...';
          setLabel(this.tipEl, text);
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
        /* 取消失败忽略 */
      }
    }
    ctx.go('lobby');
  }

  update(dt: number): void {
    if (this.spinner?.isValid) {
      const r = this.spinner.eulerAngles;
      this.spinner.setRotationFromEuler(0, 0, r.z - dt * 150);
      const nucleus = this.spinner.getChildByName('match-nucleus');
      if (nucleus?.isValid) {
        const nr = nucleus.eulerAngles;
        nucleus.setRotationFromEuler(0, 0, nr.z + dt * 220);
      }
    }
  }

  unmount(): void {
    this.polling = false;
    if (this.timer) clearTimeout(this.timer);
    this.node?.destroy();
    this.node = null;
  }
}
