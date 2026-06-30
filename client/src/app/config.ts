// 客户端运行时配置。同一份代码可在浏览器与 Node（联调测试）下运行：
// - 浏览器：支持 ?api=http://host:port 查询参数覆盖；默认 localhost:8080
// - Node：通过 globalThis.CP_API_BASE / CP_WS_URL 覆盖

export interface ClientConfig {
  apiBase: string;
  wsUrl: string;
  /** MOVE 输入发送间隔（毫秒） */
  inputSendIntervalMs: number;
  /** 快照插值延迟（毫秒），渲染滞后以平滑表现 */
  interpolationDelayMs: number;
  /** 世界尺寸（与服务端默认地图对齐，仅用于绘制网格与边界） */
  worldWidth: number;
  worldHeight: number;
  /** 玩家初始质量（与服务端一致，用于摄像机缩放基准） */
  playerBaseMass: number;
}

function resolveApiBase(): string {
  const g = globalThis as Record<string, unknown>;
  if (typeof g.CP_API_BASE === 'string') return g.CP_API_BASE;
  if (typeof location !== 'undefined') {
    const q = new URLSearchParams(location.search).get('api');
    if (q) return q;
    // 默认假设服务端在 8080；与静态页可不同源（服务端已开 CORS）
    return `${location.protocol}//${location.hostname}:8080`;
  }
  return 'http://localhost:8080';
}

function resolveWsUrl(apiBase: string): string {
  const g = globalThis as Record<string, unknown>;
  if (typeof g.CP_WS_URL === 'string') return g.CP_WS_URL;
  return apiBase.replace(/^http/, 'ws') + '/ws';
}

const apiBase = resolveApiBase();

export const config: ClientConfig = {
  apiBase,
  wsUrl: resolveWsUrl(apiBase),
  inputSendIntervalMs: 80,
  interpolationDelayMs: 120,
  worldWidth: 4000,
  worldHeight: 4000,
  playerBaseMass: 20,
};
