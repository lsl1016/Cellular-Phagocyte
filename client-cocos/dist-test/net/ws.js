"use strict";
// WsClient：对战实时通信。负责连接、发送输入、按类型分发服务端消息、心跳。
// 通过全局 WebSocket 构造（H5 浏览器原生；原生端为 Cocos 引擎实现），
// 保持引擎无关以便测试复用。
Object.defineProperty(exports, "__esModule", { value: true });
exports.WsClient = void 0;
const logger_1 = require("../core/logger");
const messages_1 = require("../core/protocol/messages");
function getWSCtor() {
    const ctor = globalThis.WebSocket;
    if (!ctor)
        throw new Error('当前环境不支持 WebSocket');
    return ctor;
}
class WsClient {
    ws = null;
    seq = 0;
    handlers = new Map();
    pingTimer = null;
    onOpen = null;
    onClose = null;
    onError = null;
    /** 建立连接，open 时 resolve。 */
    connect(url) {
        return new Promise((resolve, reject) => {
            let settled = false;
            const ws = new (getWSCtor())(url);
            this.ws = ws;
            ws.onopen = () => {
                settled = true;
                this.startHeartbeat();
                logger_1.logger.info('ws_connected', { url });
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
                logger_1.logger.info('ws_disconnected', { code: ev.code });
                this.onClose?.(ev.code ?? 0, ev.reason ?? '');
            };
            ws.onmessage = (ev) => this.dispatch(ev.data);
        });
    }
    /** 注册某类型消息的处理器，返回取消函数。 */
    on(type, handler) {
        let set = this.handlers.get(type);
        if (!set) {
            set = new Set();
            this.handlers.set(type, set);
        }
        set.add(handler);
        return () => set.delete(handler);
    }
    /** 发送一条消息，返回所用 seq。 */
    send(type, data, seq) {
        const useSeq = seq ?? ++this.seq;
        const env = { type, seq: useSeq, data };
        this.ws?.send(JSON.stringify(env));
        return useSeq;
    }
    close() {
        this.stopHeartbeat();
        this.handlers.clear();
        this.ws?.close();
        this.ws = null;
    }
    get connected() {
        return this.ws !== null && this.ws.readyState === 1; // OPEN
    }
    dispatch(raw) {
        if (typeof raw !== 'string')
            return;
        let env;
        try {
            env = JSON.parse(raw);
        }
        catch {
            logger_1.logger.warn('snapshot_parse_failed', { raw });
            return;
        }
        const set = this.handlers.get(env.type);
        if (!set)
            return;
        for (const h of set)
            h(env.data, env);
    }
    startHeartbeat() {
        this.stopHeartbeat();
        this.pingTimer = setInterval(() => {
            if (this.connected)
                this.send(messages_1.C2S.PING, { clientTime: Date.now() });
        }, 5000);
    }
    stopHeartbeat() {
        if (this.pingTimer !== null) {
            clearInterval(this.pingTimer);
            this.pingTimer = null;
        }
    }
}
exports.WsClient = WsClient;
