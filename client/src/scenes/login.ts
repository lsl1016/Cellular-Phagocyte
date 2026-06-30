// 登录场景：游客登录。

import type { Scene, SceneCtx } from '../app/context.js';
import { logger } from '../common/logger.js';
import { storage, StorageKeys } from '../common/storage.js';
import { ApiError } from '../network/http.js';
import { button, clear, h } from '../ui/dom.js';

export class LoginScene implements Scene {
  async mount(ctx: SceneCtx): Promise<void> {
    clear(ctx.uiRoot);

    const btn = button('游客登录', () => void this.login(ctx, btn));
    const card = h('div', { className: 'card' }, [
      h('div', { className: 'title', text: '吞噬细胞' }),
      h('div', { className: 'subtitle', text: '2D 实时多人对战 · MVP' }),
      btn,
      h('div', { className: 'muted', text: '点击进入，无需注册' }),
    ]);
    ctx.uiRoot.append(h('div', { className: 'panel centered' }, [card]));
  }

  private async login(ctx: SceneCtx, btn: HTMLButtonElement): Promise<void> {
    btn.disabled = true;
    btn.textContent = '登录中...';
    try {
      let deviceId = storage.get(StorageKeys.DeviceId);
      if (!deviceId) {
        deviceId = 'web-' + Math.random().toString(36).slice(2, 10);
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
      btn.disabled = false;
      btn.textContent = '游客登录';
    }
  }

  unmount(): void {}
}
