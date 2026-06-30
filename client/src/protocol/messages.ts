// WebSocket 协议定义，与服务端 internal/protocol 对齐。
// 信封：{ type, seq, serverTime, traceId, data }

// 客户端 -> 服务端
export const C2S = {
  ENTER_ROOM: 'ENTER_ROOM',
  RECONNECT: 'RECONNECT',
  READY: 'READY',
  MOVE: 'MOVE',
  SPLIT: 'SPLIT',
  EJECT: 'EJECT',
  PING: 'PING',
} as const;

// 服务端 -> 客户端
export const S2C = {
  ENTER_ROOM_RESULT: 'ENTER_ROOM_RESULT',
  RECONNECT_RESULT: 'RECONNECT_RESULT',
  ROOM_RECOVER_SNAPSHOT: 'ROOM_RECOVER_SNAPSHOT',
  PLAYER_READY: 'PLAYER_READY',
  START_COUNTDOWN: 'START_COUNTDOWN',
  GAME_START: 'GAME_START',
  ROOM_SNAPSHOT: 'ROOM_SNAPSHOT',
  RANK_UPDATE: 'RANK_UPDATE',
  GAME_END: 'GAME_END',
  SETTLEMENT_RESULT: 'SETTLEMENT_RESULT',
  SKILL_FAILED: 'SKILL_FAILED',
  PONG: 'PONG',
  ERROR: 'ERROR',
} as const;

export interface Envelope<T = unknown> {
  type: string;
  seq?: number;
  serverTime?: number;
  traceId?: string;
  data?: T;
}

export interface EnterRoomData {
  roomId: string;
  userId: string;
  enterToken: string;
}

export interface EnterRoomResultData {
  success: boolean;
  roomId?: string;
  status?: string;
  serverTime?: number;
  reconnectToken?: string;
  errorCode?: number;
  message?: string;
}

export interface ReconnectData {
  roomId: string;
  userId: string;
  reconnectToken: string;
}

export interface ReconnectResultData {
  success: boolean;
  roomId?: string;
  status?: string;
  reason?: string;
  message?: string;
}

export interface SkillFailedData {
  skillType: string;
  reason: string;
  message: string;
}

export interface ReadyData {
  roomId: string;
  userId: string;
  clientLoadCostMs?: number;
}

export interface InputData {
  direction: number;
  clientTime?: number;
}

export interface Ball {
  ballId: string;
  x: number;
  y: number;
  radius: number;
  mass: number;
}

export interface SnapshotPlayer {
  userId: string;
  nickname: string;
  status: string;
  score: number;
  mass: number;
  balls: Ball[];
}

export interface SnapshotFood {
  foodId: string;
  x: number;
  y: number;
  mass: number;
  color: string;
}

export interface SnapshotEjected {
  ejectId: string;
  ownerId: string;
  x: number;
  y: number;
  radius: number;
  mass: number;
}

export interface SnapshotEvent {
  type: string;
  data?: unknown;
}

export interface RoomSnapshotData {
  roomId: string;
  snapshotType: string;
  tickSeq: number;
  serverTime: number;
  players: SnapshotPlayer[];
  foods: SnapshotFood[];
  ejected: SnapshotEjected[];
  events: SnapshotEvent[];
}

export interface RankEntry {
  rank: number;
  userId: string;
  nickname: string;
  score: number;
}

export interface SelfRank {
  rank: number;
  score: number;
}

export interface RankUpdateData {
  roomId: string;
  rankTopN: RankEntry[];
  selfRank?: SelfRank;
}

export interface CountdownData {
  roomId: string;
  countdownSeconds: number;
  serverStartTime: number;
}

export interface GameStartData {
  roomId: string;
  serverTime: number;
  battleDurationSeconds: number;
}

export interface GameEndData {
  roomId: string;
  reason: string;
  message: string;
}

export interface SettlementResultData {
  roomId: string;
  userId: string;
  rank: number;
  totalPlayers: number;
  finalScore: number;
  maxMass: number;
  eatPlayerCount: number;
  eatFoodCount: number;
  aliveSeconds: number;
  coinReward: number;
  expReward: number;
  isBestScore: boolean;
  status: string;
}
