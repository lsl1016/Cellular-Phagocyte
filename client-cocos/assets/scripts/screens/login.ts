// 登录屏幕：品牌 Hero + 游客登录卡片。

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
  row,
  setLabel,
  uiNode,
} from '../ui/builder';
import { circleTexture, ringTexture } from '../ui/texgen';
import { theme, withAlpha } from '../ui/theme';

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
    }, { width: 292, height: 66 });

    const loginCard = panel([
      label('进入竞技场', { width: 300, fontSize: theme.bigSize, align: 'left' }),
      label('创建临时身份，马上加入实时多人对战', {
        width: 300,
        fontSize: theme.smallSize + 2,
        color: theme.muted,
        align: 'left',
      }),
      divider(300),
      btn,
      pill('无需注册 · 自动保存进度', {
        width: 250,
        color: withAlpha(theme.secondary, 215),
        textColor: theme.muted,
      }),
      label('登录即表示进入经典模式大厅', {
        width: 300,
        fontSize: theme.microSize + 1,
        color: theme.subtle,
      }),
    ], 380, 14, 30, { height: 360, color: theme.cardStrong });

    const shell = row([this.brandHero(), loginCard], 42);
    centerIn(this.node, shell, 2);
  }

  private brandHero(): Node {
    const root = uiNode('brand-hero', 320, 380);

    const ring = uiNode('brand-ring', 156, 156);
    const ringSp = ring.addComponent(Sprite);
    ringSp.spriteFrame = ringTexture();
    ringSp.sizeMode = Sprite.SizeMode.CUSTOM;
    ringSp.color = withAlpha(theme.primaryBright, 175);
    ring.setPosition(0, 86, 0);
    root.addChild(ring);

    const cell = uiNode('brand-cell', 120, 120);
    const cellSp = cell.addComponent(Sprite);
    cellSp.spriteFrame = circleTexture();
    cellSp.sizeMode = Sprite.SizeMode.CUSTOM;
    cellSp.color = theme.primary;
    cell.setPosition(0, 86, 0);
    root.addChild(cell);

    const nucleus = uiNode('brand-nucleus', 40, 40);
    const nucleusSp = nucleus.addComponent(Sprite);
    nucleusSp.spriteFrame = circleTexture();
    nucleusSp.sizeMode = Sprite.SizeMode.CUSTOM;
    nucleusSp.color = theme.accent;
    nucleus.setPosition(18, 101, 0);
    root.addChild(nucleus);

    const title = label('吞噬细胞', { width: 320, fontSize: theme.heroSize });
    title.setPosition(0, -28, 0);
    root.addChild(title);

    const subtitle = label('CELLULAR PHAGOCYTE', {
      width: 320,
      fontSize: theme.smallSize + 1,
      color: theme.primaryBright,
    });
    subtitle.setPosition(0, -76, 0);
    root.addChild(subtitle);

    const desc = label('移动 · 吞噬 · 分裂 · 成为最后的巨型细胞', {
      width: 320,
      fontSize: theme.smallSize + 1,
      color: theme.muted,
    });
    desc.setPosition(0, -122, 0);
    root.addChild(desc);

    const mode = pill('实时多人 · CLASSIC', {
      width: 190,
      color: withAlpha(theme.primary, 42),
      textColor: theme.primaryBright,
    });
    mode.setPosition(0, -170, 0);
    root.addChild(mode);
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
