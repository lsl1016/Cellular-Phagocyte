"use strict";
// 本地存储封装：引擎无关。H5 直接用 localStorage；
// 原生端由 Boot 注入 sys.localStorage；测试/兜底用内存实现。
Object.defineProperty(exports, "__esModule", { value: true });
exports.StorageKeys = exports.storage = void 0;
exports.registerKV = registerKV;
const memory = new Map();
const memoryKV = {
    getItem: (k) => (memory.has(k) ? memory.get(k) : null),
    setItem: (k, v) => void memory.set(k, v),
    removeItem: (k) => void memory.delete(k),
};
let adapter = null;
/** 引擎侧注入跨端 KV（如 Cocos sys.localStorage）。 */
function registerKV(kv) {
    adapter = kv;
}
function pick() {
    if (adapter)
        return adapter;
    const g = globalThis;
    const ls = g.localStorage;
    if (ls && typeof ls.getItem === 'function')
        return ls;
    return memoryKV;
}
const PREFIX = 'cp.';
exports.storage = {
    get(key) {
        try {
            return pick().getItem(PREFIX + key);
        }
        catch {
            return null;
        }
    },
    set(key, value) {
        try {
            pick().setItem(PREFIX + key, value);
        }
        catch {
            /* 存储不可用时忽略 */
        }
    },
    remove(key) {
        try {
            pick().removeItem(PREFIX + key);
        }
        catch {
            /* 忽略 */
        }
    },
};
// 常用键
exports.StorageKeys = {
    AccessToken: 'accessToken',
    UserId: 'userId',
    Nickname: 'nickname',
    DeviceId: 'deviceId',
};
