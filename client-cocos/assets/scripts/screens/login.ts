// 登录屏幕：游客登录。

import { Node } from 'cc';
import type { Screen, ScreenCtx } from '../app/context';
import { logger } from '../core/logger';
import { storage, StorageKeys } from '../core/storage';
import { ApiError } from '../net/http';
import { button, card, centerIn, fullBackground, label, setLabel, uiNode } from '../ui/builder';
import { theme } from '../ui/theme';

export class LoginScreen implements Screen {
  private node: Node | null = null;

  async mount(ctx: ScreenCtx): Promise<void> {
    this.node = uiNode('login-screen');
    ctx.root.addChild(this.node);
    fullBackground(this.node);

    let busy = false;
    const btn = button('游客登录', () => {
      if (busy) return;
      void this.login(ctx, btn, () => (busy = true), () => (busy = false));
    });
    const c = card([
      label('吞噬细胞', { fontSize: theme.titleSize }),
      label('2D 实时多人对战 · Cocos', { fontSize: theme.smallSize + 4, color: theme.muted }),
      btn,
      label('点击进入，无需注册', { fontSize: theme.smallSize, color: theme.muted }),
    ]);
    centerIn(this.node, c);
  }

  private async login(
    ctx: ScreenCtx,
    btn: Node,
    lock: () => void,
    unlock: () => void,
  ): Promise<void> {
    lock();
    setLabel(btn.children[0], '登录中...');
    try {
      let deviceId = storage.get(StorageKeys.DeviceId);
      if (!deviceId) {
        deviceId = 'cocos-' + Math.random().toString(36).slice(2, 10);
        storage.set(StorageKeys.DeviceId, deviceId);
      }
      const data = await ctx.api.guestLogin(deviceId);
      ctx.api.setToken(data.accessToken);
      ctx.session.token = data.accessToken;
      ctx.session.user = data.user;
      storage.set(StorageKeys.AccessToken, data.accessToken);
      storage.set(StorageKeys.UserId, data.user.userId);
      logger.info('guest_login_success', { userId: data.user.userId });
      ctx.go('lobby');
    } catch (e) {
      const msg = e instanceof ApiError ? e.message : '登录失败，请重试';
      ctx.toast(msg);
      unlock();
      setLabel(btn.children[0], '游客登录');
    }
  }

  unmount(): void {
    this.node?.destroy();
    this.node = null;
  }
}
