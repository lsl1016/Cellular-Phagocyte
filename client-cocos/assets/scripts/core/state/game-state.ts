// GameState：以服务端快照为准的对局状态。仅做存储与查询，不做权威判定。

import {
  isAOIDeltaData,
  type AOIDeltaData,
  type RankEntry,
  type RoomSnapshotData,
  type RankUpdateData,
  type SelfRank,
  type SnapshotEjected,
  type SnapshotEvent,
  type SnapshotFood,
  type SnapshotPayload,
  type SnapshotPlayer,
} from '../protocol/messages';

export class GameState {
  selfUserId = '';
  roomId = '';

  players = new Map<string, SnapshotPlayer>();
  foods = new Map<string, SnapshotFood>();
  ejected = new Map<string, SnapshotEjected>();

  rankTopN: RankEntry[] = [];
  selfRank: SelfRank | null = null;

  snapshotSeq = 0;
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
    this.snapshotSeq = 0;
    this.tickSeq = 0;
    this.lastServerTime = 0;
    this.battleEndAt = 0;
  }

  /** 设置对局计时（GAME_START 时调用）。 */
  setBattle(serverStart: number, durationSeconds: number): void {
    this.battleEndAt = serverStart + durationSeconds * 1000;
  }

  /**
   * FULL 直接替换当前可见状态；AOI_DELTA 只有 baseSeq 连续时才应用。
   * 返回 false 表示客户端必须请求 FULL_SYNC，调用方不能继续把该帧送入渲染时间轴。
   */
  applySnapshot(d: SnapshotPayload): boolean {
    if (isAOIDeltaData(d)) return this.applyDelta(d);

    this.snapshotSeq = d.snapshotSeq;
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
    return true;
  }

  private applyDelta(d: AOIDeltaData): boolean {
    if (d.baseSeq !== this.snapshotSeq) return false;

    for (const p of d.entered.players ?? []) this.players.set(p.userId, p);
    for (const p of d.updated.players ?? []) this.players.set(p.userId, p);
    for (const f of d.entered.foods ?? []) this.foods.set(f.foodId, f);
    for (const f of d.updated.foods ?? []) this.foods.set(f.foodId, f);
    for (const e of d.entered.ejected ?? []) this.ejected.set(e.ejectId, e);
    for (const e of d.updated.ejected ?? []) this.ejected.set(e.ejectId, e);

    for (const id of d.left.playerIds ?? []) this.players.delete(id);
    for (const id of d.deleted.playerIds ?? []) this.players.delete(id);
    for (const id of d.left.foodIds ?? []) this.foods.delete(id);
    for (const id of d.deleted.foodIds ?? []) this.foods.delete(id);
    for (const id of d.left.ejectedIds ?? []) this.ejected.delete(id);
    for (const id of d.deleted.ejectedIds ?? []) this.ejected.delete(id);

    this.snapshotSeq = d.snapshotSeq;
    this.tickSeq = d.tickSeq;
    this.lastServerTime = d.serverTime;
    return true;
  }

  /** 把应用完 DELTA 后的 Map 状态物化成一帧完整状态，供插值/渲染层继续复用原接口。 */
  materializeSnapshot(events: SnapshotEvent[] = []): RoomSnapshotData {
    return {
      roomId: this.roomId,
      snapshotType: 'AOI_MATERIALIZED',
      snapshotSeq: this.snapshotSeq,
      tickSeq: this.tickSeq,
      serverTime: this.lastServerTime,
      players: [...this.players.values()],
      foods: [...this.foods.values()],
      ejected: [...this.ejected.values()],
      events,
    };
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
