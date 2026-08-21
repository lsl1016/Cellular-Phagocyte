// 本地存储封装：引擎无关。H5 直接用 localStorage；
// 原生端由 Boot 注入 sys.localStorage；测试/兜底用内存实现。

export interface KV {
  getItem(k: string): string | null;
  setItem(k: string, v: string): void;
  removeItem(k: string): void;
}

const memory = new Map<string, string>();
const memoryKV: KV = {
  getItem: (k) => (memory.has(k) ? memory.get(k)! : null),
  setItem: (k, v) => void memory.set(k, v),
  removeItem: (k) => void memory.delete(k),
};

let adapter: KV | null = null;

/** 引擎侧注入跨端 KV（如 Cocos sys.localStorage）。 */
export function registerKV(kv: KV): void {
  adapter = kv;
}

function pick(): KV {
  if (adapter) return adapter;
  const g = globalThis as Record<string, unknown>;
  const ls = g.localStorage as KV | undefined;
  if (ls && typeof ls.getItem === 'function') return ls;
  return memoryKV;
}

const PREFIX = 'cp.';

export const storage = {
  get(key: string): string | null {
    try {
      return pick().getItem(PREFIX + key);
    } catch {
      return null;
    }
  },
  set(key: string, value: string): void {
    try {
      pick().setItem(PREFIX + key, value);
    } catch {
      /* 存储不可用时忽略 */
    }
  },
  remove(key: string): void {
    try {
      pick().removeItem(PREFIX + key);
    } catch {
      /* 忽略 */
    }
  },
};

// 常用键
export const StorageKeys = {
  AccessToken: 'accessToken',
  UserId: 'userId',
  Nickname: 'nickname',
  DeviceId: 'deviceId',
} as const;
