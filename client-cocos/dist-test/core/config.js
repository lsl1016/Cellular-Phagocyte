"use strict";
// 客户端运行时配置。
// - H5：支持 ?api=http://host:port 查询参数覆盖；默认 localhost:8080
// - 其它环境：通过 globalThis.CP_API_BASE / CP_WS_URL 覆盖
Object.defineProperty(exports, "__esModule", { value: true });
exports.config = void 0;
function resolveApiBase() {
    const g = globalThis;
    if (typeof g.CP_API_BASE === 'string')
        return g.CP_API_BASE;
    if (typeof location !== 'undefined') {
        const q = new URLSearchParams(location.search).get('api');
        if (q)
            return q;
        return `${location.protocol}//${location.hostname}:8080`;
    }
    return 'http://localhost:8080';
}
function resolveWsUrl(apiBase) {
    const g = globalThis;
    if (typeof g.CP_WS_URL === 'string')
        return g.CP_WS_URL;
    return apiBase.replace(/^http/, 'ws') + '/ws';
}
const apiBase = resolveApiBase();
exports.config = {
    apiBase,
    wsUrl: resolveWsUrl(apiBase),
    inputSendIntervalMs: 80,
    worldWidth: 4000,
    worldHeight: 4000,
    playerBaseMass: 20,
};
