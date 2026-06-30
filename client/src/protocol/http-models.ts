// HTTP 接口数据模型，与服务端响应对齐。统一响应封装：{ code, message, data }。

export interface ApiEnvelope<T> {
  code: number;
  message: string;
  data: T;
}

export interface LoginUser {
  userId: string;
  nickname: string;
  avatar: string;
  userType: string;
  level: number;
  coin: number;
  exp: number;
}

export interface LoginData {
  accessToken: string;
  user: LoginUser;
}

export interface MeData {
  userId: string;
  nickname: string;
  avatar: string;
  userType: string;
  status: string;
  level: number;
  exp: number;
  nextLevelExp: number;
  coin: number;
}

export interface AssetData {
  userId: string;
  coin: number;
  exp: number;
  level: number;
  nextLevelExp: number;
}

export interface MatchStartData {
  matchId: string;
  status: string;
  estimatedWaitSeconds: number;
  serverTime: number;
}

export interface MatchStatusData {
  matchId: string;
  status: string; // MATCHING | MATCHED | ...
  stage?: string;
  waitSeconds?: number;
  estimatedWaitSeconds?: number;
  // status === MATCHED 时返回：
  roomId?: string;
  serverId?: string;
  wsUrl?: string;
  enterToken?: string;
  expireAt?: number;
}

export interface MatchConfigData {
  mode: string;
  maxPlayers: number;
  minPlayers: number;
  estimatedWaitSeconds: number;
  maxWaitSeconds: number;
}

export interface RecordEntry {
  roomId: string;
  mode: string;
  modeName: string;
  rank: number;
  totalPlayers: number;
  finalScore: number;
  maxMass?: number;
  eatPlayerCount: number;
  eatFoodCount: number;
  aliveSeconds?: number;
  alive?: boolean;
  coinReward: number;
  expReward: number;
  status?: string;
  settlementStatus?: string;
  startTime?: number;
  endTime: number;
  durationSeconds?: number;
}

export interface RecordListData {
  page: number;
  pageSize: number;
  total: number;
  list: RecordEntry[];
}

export interface RecordSummary {
  totalGames: number;
  firstPlaceCount: number;
  top3Count: number;
  top10Count: number;
  bestRank: number;
  bestScore: number;
  maxMass: number;
  maxEatPlayerCount: number;
  totalEatPlayerCount: number;
  totalEatFoodCount: number;
  totalCoinReward: number;
  totalExpReward: number;
}

export interface RankItem {
  rank: number;
  userId: string;
  nickname: string;
  avatar: string;
  score: number;
  self: boolean;
}

export interface RankSelf {
  rank: number | null;
  score: number;
  onRank: boolean;
}

export interface RankListData {
  rankType: string;
  periodKey: string;
  page: number;
  pageSize: number;
  list: RankItem[];
  selfRank: RankSelf;
  refreshText: string;
}

export interface RankConfigItem {
  rankType: string;
  name: string;
  enabled: boolean;
}

export interface SettlementData {
  roomId: string;
  userId: string;
  rank: number;
  totalPlayers: number;
  finalScore: number;
  maxMass: number;
  eatPlayerCount: number;
  eatFoodCount: number;
  aliveSeconds: number;
  coinReward: number;
  expReward: number;
  isBestScore: boolean;
  status: string;
}
