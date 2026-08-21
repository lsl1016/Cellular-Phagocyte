// 场景管理器：注册屏幕、按名切换 mount/unmount，并逐帧驱动当前屏幕。
// 业务不直接使用 director.loadScene —— 全部屏幕在同一 Main 场景内以面板方式切换，
// 与旧 Canvas 客户端的 SceneManager 行为一致。

import type { Screen, ScreenCtx, ScreenName } from './context';
import { logger } from '../core/logger';

type ScreenFactory = () => Screen;

export class SceneManager {
  private factories = new Map<ScreenName, ScreenFactory>();
  private current: Screen | null = null;
  private ctx: ScreenCtx;

  constructor(base: Omit<ScreenCtx, 'go'>) {
    this.ctx = { ...base, go: (name) => this.go(name) };
  }

  register(name: ScreenName, factory: ScreenFactory): void {
    this.factories.set(name, factory);
  }

  go(name: ScreenName): void {
    const factory = this.factories.get(name);
    if (!factory) {
      logger.error('screen_not_found', { name });
      return;
    }
    this.current?.unmount();
    const screen = factory();
    this.current = screen;
    logger.info('screen_enter', { name });
    void screen.mount(this.ctx);
  }

  update(dt: number): void {
    this.current?.update?.(dt);
  }
}
