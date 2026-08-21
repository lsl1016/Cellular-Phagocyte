"use strict";
// GameState：以服务端快照为准的对局状态。仅做存储与查询，不做权威判定。
Object.defineProperty(exports, "__esModule", { value: true });
exports.GameState = void 0;
class GameState {
    selfUserId = '';
    roomId = '';
    players = new Map();
    foods = new Map();
    ejected = new Map();
    rankTopN = [];
    selfRank = null;
    tickSeq = 0;
    lastServerTime = 0;
    /** 对局结束的服务端时间戳（毫秒）；0 表示未开始计时。 */
    battleEndAt = 0;
    reset(selfUserId, roomId) {
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
    setBattle(serverStart, durationSeconds) {
        this.battleEndAt = serverStart + durationSeconds * 1000;
    }
    applySnapshot(d) {
        this.tickSeq = d.tickSeq;
        this.lastServerTime = d.serverTime;
        const seenPlayers = new Set();
        for (const p of d.players) {
            this.players.set(p.userId, p);
            seenPlayers.add(p.userId);
        }
        for (const id of this.players.keys()) {
            if (!seenPlayers.has(id))
                this.players.delete(id);
        }
        const seenFoods = new Set();
        for (const f of d.foods) {
            this.foods.set(f.foodId, f);
            seenFoods.add(f.foodId);
        }
        for (const id of this.foods.keys()) {
            if (!seenFoods.has(id))
                this.foods.delete(id);
        }
        const seenEjected = new Set();
        for (const e of d.ejected ?? []) {
            this.ejected.set(e.ejectId, e);
            seenEjected.add(e.ejectId);
        }
        for (const id of this.ejected.keys()) {
            if (!seenEjected.has(id))
                this.ejected.delete(id);
        }
    }
    applyRank(d) {
        this.rankTopN = d.rankTopN;
        this.selfRank = d.selfRank ?? null;
    }
    self() {
        return this.players.get(this.selfUserId);
    }
    /** 剩余秒数（向上取整，下限 0）。 */
    remainingSeconds(now) {
        if (this.battleEndAt === 0)
            return 0;
        return Math.max(0, Math.ceil((this.battleEndAt - now) / 1000));
    }
}
exports.GameState = GameState;
