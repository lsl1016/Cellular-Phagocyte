// HttpClient：基于 XMLHttpRequest（H5/原生通用），自动携带 Bearer 令牌，
// 解包 {code,message,data}。

import { config } from '../core/config';
import type { ApiEnvelope } from '../core/protocol/http-models';

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

  private request<T>(method: string, path: string, body?: unknown): Promise<T> {
    return new Promise<T>((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      xhr.open(method, config.apiBase + path, true);
      if (body !== undefined) xhr.setRequestHeader('Content-Type', 'application/json');
      if (this.token) xhr.setRequestHeader('Authorization', `Bearer ${this.token}`);

      xhr.onreadystatechange = () => {
        if (xhr.readyState !== 4) return;
        let env: ApiEnvelope<T>;
        try {
          env = JSON.parse(xhr.responseText) as ApiEnvelope<T>;
        } catch {
          reject(new ApiError(-1, `响应解析失败 (${xhr.status})`));
          return;
        }
        if (env.code !== 0) {
          reject(new ApiError(env.code, env.message || `请求失败 (code ${env.code})`));
          return;
        }
        resolve(env.data);
      };
      xhr.onerror = () => reject(new ApiError(-1, '网络请求失败'));
      xhr.send(body !== undefined ? JSON.stringify(body) : undefined);
    });
  }
}
