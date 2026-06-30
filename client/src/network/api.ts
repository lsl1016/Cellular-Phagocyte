// ApiService：把后端 HTTP 接口封装成类型化方法。

import { HttpClient } from './http.js';
import type {
  AssetData,
  LoginData,
  MatchConfigData,
  MatchStartData,
  MatchStatusData,
  MeData,
  SettlementData,
} from '../protocol/http-models.js';

export class ApiService {
  readonly http = new HttpClient();

  setToken(token: string | null): void {
    this.http.setToken(token);
  }

  guestLogin(deviceId: string, clientVersion = '1.0.0'): Promise<LoginData> {
    return this.http.post<LoginData>('/api/auth/guest-login', { deviceId, clientVersion });
  }

  getMe(): Promise<MeData> {
    return this.http.get<MeData>('/api/users/me');
  }

  getAssets(): Promise<AssetData> {
    return this.http.get<AssetData>('/api/assets/me');
  }

  matchStart(mode = 'classic'): Promise<MatchStartData> {
    return this.http.post<MatchStartData>('/api/match/start', { mode, clientVersion: '1.0.0' });
  }

  matchCancel(matchId: string): Promise<{ status: string }> {
    return this.http.post<{ status: string }>('/api/match/cancel', { matchId });
  }

  matchStatus(matchId: string): Promise<MatchStatusData> {
    return this.http.get<MatchStatusData>(`/api/match/status?matchId=${encodeURIComponent(matchId)}`);
  }

  matchConfig(mode = 'classic'): Promise<MatchConfigData> {
    return this.http.get<MatchConfigData>(`/api/match/config?mode=${encodeURIComponent(mode)}`);
  }

  settlementLatest(): Promise<SettlementData> {
    return this.http.get<SettlementData>('/api/settlements/latest');
  }

  settlementByRoom(roomId: string): Promise<SettlementData> {
    return this.http.get<SettlementData>(`/api/settlements/${encodeURIComponent(roomId)}/me`);
  }
}
