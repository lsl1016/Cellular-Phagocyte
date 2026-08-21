"use strict";
// ApiService：把后端 HTTP 接口封装成类型化方法。
Object.defineProperty(exports, "__esModule", { value: true });
exports.ApiService = void 0;
const http_1 = require("./http");
class ApiService {
    http = new http_1.HttpClient();
    setToken(token) {
        this.http.setToken(token);
    }
    guestLogin(deviceId, clientVersion = '1.0.0') {
        return this.http.post('/api/auth/guest-login', { deviceId, clientVersion });
    }
    getMe() {
        return this.http.get('/api/users/me');
    }
    getAssets() {
        return this.http.get('/api/assets/me');
    }
    matchStart(mode = 'classic') {
        return this.http.post('/api/match/start', { mode, clientVersion: '1.0.0' });
    }
    matchCancel(matchId) {
        return this.http.post('/api/match/cancel', { matchId });
    }
    matchStatus(matchId) {
        return this.http.get(`/api/match/status?matchId=${encodeURIComponent(matchId)}`);
    }
    matchConfig(mode = 'classic') {
        return this.http.get(`/api/match/config?mode=${encodeURIComponent(mode)}`);
    }
    settlementLatest() {
        return this.http.get('/api/settlements/latest');
    }
    settlementByRoom(roomId) {
        return this.http.get(`/api/settlements/${encodeURIComponent(roomId)}/me`);
    }
    records(page = 1, pageSize = 10, mode = '') {
        const q = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
        if (mode)
            q.set('mode', mode);
        return this.http.get(`/api/records?${q.toString()}`);
    }
    recordDetail(roomId) {
        return this.http.get(`/api/records/${encodeURIComponent(roomId)}`);
    }
    recordSummary() {
        return this.http.get('/api/records/summary');
    }
    ranks(rankType = 'daily', page = 1, pageSize = 50) {
        const q = new URLSearchParams({ rankType, page: String(page), pageSize: String(pageSize) });
        return this.http.get(`/api/ranks?${q.toString()}`);
    }
    rankMe(rankType = 'daily') {
        return this.http.get(`/api/ranks/me?rankType=${encodeURIComponent(rankType)}`);
    }
    rankConfig() {
        return this.http.get('/api/ranks/config');
    }
}
exports.ApiService = ApiService;
