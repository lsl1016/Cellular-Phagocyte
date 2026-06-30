// HttpClient：基于 fetch，自动携带 Bearer 令牌，解包 {code,message,data}。

import { config } from '../app/config.js';
import type { ApiEnvelope } from '../protocol/http-models.js';

export class ApiError extends Error {
  constructor(public code: number, message: string) {
    super(message);
    this.name = 'ApiError';
  }
}

export class HttpClient {
  private token: string | null = null;

  setToken(token: string | null): void {
    this.token = token;
  }

  async get<T>(path: string): Promise<T> {
    return this.request<T>('GET', path);
  }

  async post<T>(path: string, body?: unknown): Promise<T> {
    return this.request<T>('POST', path, body);
  }

  private async request<T>(method: string, path: string, body?: unknown): Promise<T> {
    const headers: Record<string, string> = {};
    if (body !== undefined) headers['Content-Type'] = 'application/json';
    if (this.token) headers['Authorization'] = `Bearer ${this.token}`;

    const resp = await fetch(config.apiBase + path, {
      method,
      headers,
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });

    let env: ApiEnvelope<T>;
    try {
      env = (await resp.json()) as ApiEnvelope<T>;
    } catch {
      throw new ApiError(-1, `响应解析失败 (${resp.status})`);
    }
    if (env.code !== 0) {
      throw new ApiError(env.code, env.message || `请求失败 (code ${env.code})`);
    }
    return env.data;
  }
}
