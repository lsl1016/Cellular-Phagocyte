// 轻量客户端日志，区分级别并带事件名，便于排查。

type Level = 'debug' | 'info' | 'warn' | 'error';

const order: Record<Level, number> = { debug: 0, info: 1, warn: 2, error: 3 };
let threshold: Level = 'info';

export function setLogLevel(level: Level): void {
  threshold = level;
}

function log(level: Level, event: string, data?: unknown): void {
  if (order[level] < order[threshold]) return;
  const ts = new Date().toISOString();
  const prefix = `[${ts}] [${level.toUpperCase()}] ${event}`;
  if (data !== undefined) {
    console[level === 'debug' ? 'log' : level](prefix, data);
  } else {
    console[level === 'debug' ? 'log' : level](prefix);
  }
}

export const logger = {
  debug: (event: string, data?: unknown) => log('debug', event, data),
  info: (event: string, data?: unknown) => log('info', event, data),
  warn: (event: string, data?: unknown) => log('warn', event, data),
  error: (event: string, data?: unknown) => log('error', event, data),
};
