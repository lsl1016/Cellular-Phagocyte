// GameState：以服务端快照为准的对局状态。仅做存储与查询，不做权威判定。

import type {
  RankEntry,
  RoomSnapshotData,
  RankUpdateData,
  SelfRank,
  SnapshotEjected,
  SnapshotFood,
  SnapshotPlayer,
} from '../protocol/messages.js';

export class GameState {
  selfUserId = '';
  roomId = '';

  players = new Map<string, SnapshotPlayer>();
  foods = new Map<string, SnapshotFood>();
  ejected = new Map<string, SnapshotEjected>();

  rankTopN: RankEntry[] = [];
  selfRank: SelfRank | null = null;

  tickSeq = 0;
  lastServerTime = 0;
  /** 对局结束的服务端时间戳（毫秒）；0 表示未开始计时。 */
  battleEndAt = 0;

  reset(selfUserId: string, roomId: string): void {
    this.selfUserId = selfUserId;
    this.roomId = roomId;
    this.players.clear();
    this.foods.clear();
    this.ejected.clear();
    this.rankTopN = [];
    this.selfRank = null;
    this.tickSeq = 0;
    this.lastServerTime = 0;
    this.battleEndAt = 0;
  }

  /** 设置对局计时（GAME_START 时调用）。 */
  setBattle(serverStart: number, durationSeconds: number): void {
    this.battleEndAt = serverStart + durationSeconds * 1000;
  }

  applySnapshot(d: RoomSnapshotData): void {
    this.tickSeq = d.tickSeq;
    this.lastServerTime = d.serverTime;

    const seenPlayers = new Set<string>();
    for (const p of d.players) {
      this.players.set(p.userId, p);
      seenPlayers.add(p.userId);
    }
    for (const id of this.players.keys()) {
      if (!seenPlayers.has(id)) this.players.delete(id);
    }

    const seenFoods = new Set<string>();
    for (const f of d.foods) {
      this.foods.set(f.foodId, f);
      seenFoods.add(f.foodId);
    }
    for (const id of this.foods.keys()) {
      if (!seenFoods.has(id)) this.foods.delete(id);
    }

    const seenEjected = new Set<string>();
    for (const e of d.ejected ?? []) {
      this.ejected.set(e.ejectId, e);
      seenEjected.add(e.ejectId);
    }
    for (const id of this.ejected.keys()) {
      if (!seenEjected.has(id)) this.ejected.delete(id);
    }
  }

  applyRank(d: RankUpdateData): void {
    this.rankTopN = d.rankTopN;
    this.selfRank = d.selfRank ?? null;
  }

  self(): SnapshotPlayer | undefined {
    return this.players.get(this.selfUserId);
  }

  /** 剩余秒数（向上取整，下限 0）。 */
  remainingSeconds(now: number): number {
    if (this.battleEndAt === 0) return 0;
    return Math.max(0, Math.ceil((this.battleEndAt - now) / 1000));
  }
}
