"use strict";
// HttpClient：自动携带 Bearer 令牌，解包 {code,message,data}。
// 传输层双通道：优先 fetch（浏览器 / Node / Cocos 原生），
// 无 fetch 环境回退 XMLHttpRequest（Cocos 原生保底）。
Object.defineProperty(exports, "__esModule", { value: true });
exports.HttpClient = exports.ApiError = void 0;
const config_1 = require("../core/config");
class ApiError extends Error {
    code;
    constructor(code, message) {
        super(message);
        this.code = code;
        this.name = 'ApiError';
    }
}
exports.ApiError = ApiError;
class HttpClient {
    token = null;
    setToken(token) {
        this.token = token;
    }
    async get(path) {
        return this.request('GET', path);
    }
    async post(path, body) {
        return this.request('POST', path, body);
    }
    async request(method, path, body) {
        if (typeof fetch !== 'undefined') {
            return this.fetchRequest(method, path, body);
        }
        return this.xhrRequest(method, path, body);
    }
    async fetchRequest(method, path, body) {
        const headers = {};
        if (body !== undefined)
            headers['Content-Type'] = 'application/json';
        if (this.token)
            headers['Authorization'] = `Bearer ${this.token}`;
        let resp;
        try {
            resp = await fetch(config_1.config.apiBase + path, {
                method,
                headers,
                body: body !== undefined ? JSON.stringify(body) : undefined,
            });
        }
        catch {
            throw new ApiError(-1, '网络请求失败');
        }
        let env;
        try {
            env = (await resp.json());
        }
        catch {
            throw new ApiError(-1, `响应解析失败 (${resp.status})`);
        }
        if (env.code !== 0) {
            throw new ApiError(env.code, env.message || `请求失败 (code ${env.code})`);
        }
        return env.data;
    }
    xhrRequest(method, path, body) {
        return new Promise((resolve, reject) => {
            const xhr = new XMLHttpRequest();
            xhr.open(method, config_1.config.apiBase + path, true);
            if (body !== undefined)
                xhr.setRequestHeader('Content-Type', 'application/json');
            if (this.token)
                xhr.setRequestHeader('Authorization', `Bearer ${this.token}`);
            xhr.onreadystatechange = () => {
                if (xhr.readyState !== 4)
                    return;
                let env;
                try {
                    env = JSON.parse(xhr.responseText);
                }
                catch {
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
exports.HttpClient = HttpClient;
