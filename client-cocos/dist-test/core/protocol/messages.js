"use strict";
// WebSocket 协议定义，与服务端 internal/protocol 对齐。
// 信封：{ type, seq, serverTime, traceId, data }
Object.defineProperty(exports, "__esModule", { value: true });
exports.S2C = exports.C2S = void 0;
// 客户端 -> 服务端
exports.C2S = {
    ENTER_ROOM: 'ENTER_ROOM',
    RECONNECT: 'RECONNECT',
    READY: 'READY',
    MOVE: 'MOVE',
    SPLIT: 'SPLIT',
    EJECT: 'EJECT',
    PING: 'PING',
};
// 服务端 -> 客户端
exports.S2C = {
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
};
