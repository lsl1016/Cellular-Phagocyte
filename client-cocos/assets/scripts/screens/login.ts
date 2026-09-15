// 登录屏幕：单主卡片，保留游戏品牌感但避免双栏过度设计。

import { Node, Sprite } from 'cc';
import type { Screen, ScreenCtx } from '../app/context';
import { logger } from '../core/logger';
import { storage, StorageKeys } from '../core/storage';
import { ApiError } from '../net/http';
import {
  button,
  centerIn,
  divider,
  fullBackground,
  label,
  panel,
  pill,
  setLabel,
  uiNode,
} from '../ui/builder';
import { screenLayer } from '../ui/layout';
import { circleTexture, ringTexture } from '../ui/texgen';
import { theme, withAlpha } from '../ui/theme';

export class LoginScreen implements Screen {
  private node: Node | null = null;

  async mount(ctx: ScreenCtx): Promise<void> {
    this.node = screenLayer(ctx.root, 'login-screen');
    fullBackground(this.node);

    let busy = false;
    const btn = button('游客登录', () => {
      if (busy) return;
      void this.login(ctx, btn, () => (busy = true), () => (busy = false));
    }, { width: 300, height: 60 });

    const logo = this.logo();
    const c = panel([
      logo,
      label('吞噬细胞', { width: 360, fontSize: theme.heroSize }),
      label('CELLULAR PHAGOCYTE', {
        width: 360,
        fontSize: theme.smallSize + 1,
        color: theme.primaryBright,
      }),
      label('移动 · 吞噬 · 分裂 · 生存到最后', {
        width: 360,
        fontSize: theme.smallSize + 2,
        color: theme.muted,
      }),
      divider(330),
      btn,
      pill('无需注册 · 进度自动保存', {
        width: 230,
        height: 30,
        color: withAlpha(theme.secondary, 190),
        textColor: theme.muted,
      }),
    ], 460, 10, 28, { height: 500, color: theme.cardStrong });
    centerIn(this.node, c);
  }

  private logo(): Node {
    const root = uiNode('login-logo', 124, 124);
    const ring = root.addComponent(Sprite);
    ring.spriteFrame = ringTexture();
    ring.sizeMode = Sprite.SizeMode.CUSTOM;
    ring.color = withAlpha(theme.primaryBright, 170);

    const cell = uiNode('login-cell', 92, 92);
    const cellSp = cell.addComponent(Sprite);
    cellSp.spriteFrame = circleTexture();
    cellSp.sizeMode = Sprite.SizeMode.CUSTOM;
    cellSp.color = theme.primary;
    root.addChild(cell);

    const nucleus = uiNode('login-nucleus', 28, 28);
    const nucleusSp = nucleus.addComponent(Sprite);
    nucleusSp.spriteFrame = circleTexture();
    nucleusSp.sizeMode = Sprite.SizeMode.CUSTOM;
    nucleusSp.color = theme.accent;
    nucleus.setPosition(14, 14, 0);
    root.addChild(nucleus);
    return root;
  }

  private async login(
    ctx: ScreenCtx,
    btn: Node,
    lock: () => void,
    unlock: () => void,
  ): Promise<void> {
    lock();
    setLabel(btn.children[0], '正在进入...');
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
