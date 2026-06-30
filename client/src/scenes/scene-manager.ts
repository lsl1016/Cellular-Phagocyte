// 场景管理器：注册场景、按名切换，负责 mount/unmount。

import type { Scene, SceneCtx, SceneName } from '../app/context.js';
import { logger } from '../common/logger.js';

type SceneFactory = () => Scene;

export class SceneManager {
  private factories = new Map<SceneName, SceneFactory>();
  private current: Scene | null = null;
  private ctx: SceneCtx;

  constructor(base: Omit<SceneCtx, 'go'>) {
    this.ctx = { ...base, go: (name, params) => this.go(name, params) };
  }

  register(name: SceneName, factory: SceneFactory): void {
    this.factories.set(name, factory);
  }

  go(name: SceneName, params?: unknown): void {
    const factory = this.factories.get(name);
    if (!factory) {
      logger.error('scene_not_found', { name });
      return;
    }
    this.current?.unmount();
    const scene = factory();
    this.current = scene;
    logger.info('scene_enter', { name });
    void scene.mount(this.ctx, params);
  }
}
