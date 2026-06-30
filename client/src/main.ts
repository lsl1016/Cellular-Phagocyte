// 客户端入口：装配上下文、注册场景、进入登录。

import { config } from './app/config.js';
import type { Session } from './app/context.js';
import { logger } from './common/logger.js';
import { ApiService } from './network/api.js';
import { GameScene } from './scenes/game.js';
import { LobbyScene } from './scenes/lobby.js';
import { LoginScene } from './scenes/login.js';
import { MatchScene } from './scenes/match.js';
import { RankScene } from './scenes/rank.js';
import { RecordsScene } from './scenes/records.js';
import { SceneManager } from './scenes/scene-manager.js';
import { SettlementScene } from './scenes/settlement.js';
import { toast } from './ui/dom.js';

function main(): void {
  const canvas = document.getElementById('game-canvas') as HTMLCanvasElement | null;
  const uiRoot = document.getElementById('ui-root');
  if (!canvas || !uiRoot) {
    console.error('缺少 #game-canvas 或 #ui-root');
    return;
  }

  const session: Session = { token: null, user: null, match: null, settlement: null };

  const manager = new SceneManager({
    api: new ApiService(),
    uiRoot,
    canvas,
    session,
    toast,
  });

  manager.register('login', () => new LoginScene());
  manager.register('lobby', () => new LobbyScene());
  manager.register('match', () => new MatchScene());
  manager.register('game', () => new GameScene());
  manager.register('settlement', () => new SettlementScene());
  manager.register('records', () => new RecordsScene());
  manager.register('rank', () => new RankScene());

  logger.info('client_start', { apiBase: config.apiBase, wsUrl: config.wsUrl });
  manager.go('login');
}

main();
