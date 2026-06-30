// 本地存储封装。浏览器用 localStorage；Node 等无 localStorage 环境用内存兜底。

interface KV {
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

const kv: KV = typeof localStorage !== 'undefined' ? localStorage : memoryKV;

const PREFIX = 'cp.';

export const storage = {
  get(key: string): string | null {
    return kv.getItem(PREFIX + key);
  },
  set(key: string, value: string): void {
    kv.setItem(PREFIX + key, value);
  },
  remove(key: string): void {
    kv.removeItem(PREFIX + key);
  },
};

// 常用键
export const StorageKeys = {
  AccessToken: 'accessToken',
  UserId: 'userId',
  Nickname: 'nickname',
  DeviceId: 'deviceId',
} as const;
