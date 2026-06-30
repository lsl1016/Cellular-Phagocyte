// 场景上下文与场景接口。各场景通过 SceneCtx 访问 API、画布、导航与共享会话。

import type { ApiService } from '../network/api.js';
import type { LoginUser, SettlementData } from '../protocol/http-models.js';

export interface BattleEntry {
  roomId: string;
  enterToken: string;
  wsUrl: string;
}

export interface Session {
  token: string | null;
  user: LoginUser | null;
  match: BattleEntry | null;
  settlement: SettlementData | null;
}

export interface SceneCtx {
  api: ApiService;
  uiRoot: HTMLElement;
  canvas: HTMLCanvasElement;
  session: Session;
  go(scene: SceneName, params?: unknown): void;
  toast(message: string): void;
}

export type SceneName = 'login' | 'lobby' | 'match' | 'game' | 'settlement' | 'records' | 'rank';

export interface Scene {
  mount(ctx: SceneCtx, params?: unknown): void | Promise<void>;
  unmount(): void;
}
