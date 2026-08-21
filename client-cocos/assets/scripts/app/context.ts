// 屏幕上下文与屏幕接口：各屏幕通过 ScreenCtx 访问 API、根节点、导航与共享会话。

import type { Node } from 'cc';
import type { ApiService } from '../net/api';
import type { LoginUser, SettlementData } from '../core/protocol/http-models';

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

export interface ScreenCtx {
  api: ApiService;
  root: Node;
  session: Session;
  go(screen: ScreenName): void;
  toast(message: string): void;
}

export type ScreenName =
  | 'login'
  | 'lobby'
  | 'match'
  | 'game'
  | 'settlement'
  | 'records'
  | 'rank';

export interface Screen {
  mount(ctx: ScreenCtx): void | Promise<void>;
  unmount(): void;
  update?(dt: number): void;
}
