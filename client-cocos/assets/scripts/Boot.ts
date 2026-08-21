// 启动组件：挂在 Main 场景 Canvas 上。构建 ScreenRoot、注册屏幕、进入登录。

import { _decorator, Component, sys, Widget } from 'cc';
import { registerKV } from './core/storage';
import { ApiService } from './net/api';
import { SceneManager } from './app/scene-manager';
import { GameScreen } from './screens/game';
import { LobbyScreen } from './screens/lobby';
import { LoginScreen } from './screens/login';
import { MatchScreen } from './screens/match';
import { RankScreen } from './screens/rank';
import { RecordsScreen } from './screens/records';
import { SettlementScreen } from './screens/settlement';
import { uiNode } from './ui/builder';
import { initToast, toast } from './ui/toast';

const { ccclass } = _decorator;

@ccclass('Boot')
export class Boot extends Component {
  private manager: SceneManager | null = null;

  start(): void {
    // 跨端本地存储：H5 有 localStorage；原生端注入 sys.localStorage
    const g = globalThis as Record<string, unknown>;
    if (!g.localStorage) {
      registerKV({
        getItem: (k) => sys.localStorage.getItem(k),
        setItem: (k, v) => sys.localStorage.setItem(k, v),
        removeItem: (k) => sys.localStorage.removeItem(k),
      });
    }

    // 全屏 ScreenRoot
    const screenRoot = uiNode('ScreenRoot');
    const wg = screenRoot.addComponent(Widget);
    wg.isAlignTop = wg.isAlignBottom = wg.isAlignLeft = wg.isAlignRight = true;
    wg.top = wg.bottom = wg.left = wg.right = 0;
    this.node.addChild(screenRoot);

    initToast(this.node);

    const manager = new SceneManager({
      api: new ApiService(),
      root: screenRoot,
      session: { token: null, user: null, match: null, settlement: null },
      toast,
    });
    manager.register('login', () => new LoginScreen());
    manager.register('lobby', () => new LobbyScreen());
    manager.register('match', () => new MatchScreen());
    manager.register('game', () => new GameScreen());
    manager.register('settlement', () => new SettlementScreen());
    manager.register('records', () => new RecordsScreen());
    manager.register('rank', () => new RankScreen());
    this.manager = manager;
    manager.go('login');
  }

  update(dt: number): void {
    this.manager?.update(dt);
  }
}
