// WsClient：对战实时通信。负责连接、发送输入、按类型分发服务端消息、心跳。
// 通过全局 WebSocket 构造（H5 浏览器原生；原生端为 Cocos 引擎实现），
// 保持引擎无关以便测试复用。

import { logger } from '../core/logger';
import { C2S, type Envelope } from '../core/protocol/messages';

interface WSLike {
  send(data: string): void;
  close(): void;
  readyState: number;
  onopen: (() => void) | null;
  onerror: (() => void) | null;
  onclose: ((ev: { code?: number; reason?: string }) => void) | null;
  onmessage: ((ev: { data?: unknown }) => void) | null;
}

type WSCtor = new (url: string) => WSLike;

function getWSCtor(): WSCtor {
  const ctor = (globalThis as Record<string, unknown>).WebSocket as WSCtor | undefined;
  if (!ctor) throw new Error('当前环境不支持 WebSocket');
  return ctor;
}

type MsgHandler = (data: unknown, env: Envelope) => void;

export class WsClient {
  private ws: WSLike | null = null;
  private seq = 0;
  private handlers = new Map<string, Set<MsgHandler>>();
  private pingTimer: ReturnType<typeof setInterval> | null = null;

  onOpen: (() => void) | null = null;
  onClose: ((code: number, reason: string) => void) | null = null;
  onError: (() => void) | null = null;

  /** 建立连接，open 时 resolve。 */
  connect(url: string): Promise<void> {
    return new Promise((resolve, reject) => {
      let settled = false;
      const ws = new (getWSCtor())(url);
      this.ws = ws;

      ws.onopen = () => {
        settled = true;
        this.startHeartbeat();
        logger.info('ws_connected', { url });
        this.onOpen?.();
        resolve();
      };
      ws.onerror = () => {
        this.onError?.();
        if (!settled) {
          settled = true;
          reject(new Error('WebSocket 连接失败'));
        }
      };
      ws.onclose = (ev) => {
        this.stopHeartbeat();
        logger.info('ws_disconnected', { code: ev.code });
        this.onClose?.(ev.code ?? 0, ev.reason ?? '');
      };
      ws.onmessage = (ev) => this.dispatch(ev.data);
    });
  }

  /** 注册某类型消息的处理器，返回取消函数。 */
  on(type: string, handler: MsgHandler): () => void {
    let set = this.handlers.get(type);
    if (!set) {
      set = new Set<MsgHandler>();
      this.handlers.set(type, set);
    }
    set.add(handler);
    return () => set!.delete(handler);
  }

  /** 发送一条消息，返回所用 seq。 */
  send(type: string, data?: unknown, seq?: number): number {
    const useSeq = seq ?? ++this.seq;
    const env: Envelope = { type, seq: useSeq, data };
    this.ws?.send(JSON.stringify(env));
    return useSeq;
  }

  close(): void {
    this.stopHeartbeat();
    this.handlers.clear();
    this.ws?.close();
    this.ws = null;
  }

  get connected(): boolean {
    return this.ws !== null && this.ws.readyState === 1; // OPEN
  }

  private dispatch(raw: unknown): void {
    if (typeof raw !== 'string') return;
    let env: Envelope;
    try {
      env = JSON.parse(raw) as Envelope;
    } catch {
      logger.warn('snapshot_parse_failed', { raw });
      return;
    }
    const set = this.handlers.get(env.type);
    if (!set) return;
    for (const h of set) h(env.data, env);
  }

  private startHeartbeat(): void {
    this.stopHeartbeat();
    this.pingTimer = setInterval(() => {
      if (this.connected) this.send(C2S.PING, { clientTime: Date.now() });
    }, 5000);
  }

  private stopHeartbeat(): void {
    if (this.pingTimer !== null) {
      clearInterval(this.pingTimer);
      this.pingTimer = null;
    }
  }
}
