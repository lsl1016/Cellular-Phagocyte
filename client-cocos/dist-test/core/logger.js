"use strict";
// 轻量客户端日志，区分级别并带事件名，便于排查。
Object.defineProperty(exports, "__esModule", { value: true });
exports.logger = void 0;
exports.setLogLevel = setLogLevel;
const order = { debug: 0, info: 1, warn: 2, error: 3 };
let threshold = 'info';
function setLogLevel(level) {
    threshold = level;
}
function log(level, event, data) {
    if (order[level] < order[threshold])
        return;
    const ts = new Date().toISOString();
    const prefix = `[${ts}] [${level.toUpperCase()}] ${event}`;
    if (data !== undefined) {
        console[level === 'debug' ? 'log' : level](prefix, data);
    }
    else {
        console[level === 'debug' ? 'log' : level](prefix);
    }
}
exports.logger = {
    debug: (event, data) => log('debug', event, data),
    info: (event, data) => log('info', event, data),
    warn: (event, data) => log('warn', event, data),
    error: (event, data) => log('error', event, data),
};
