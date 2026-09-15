// 启动组件：挂在 Main 场景 Canvas 上。构建 ScreenRoot、注册屏幕、进入登录。

import { _decorator, Component, sys, UITransform, view } from 'cc';
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
    // 跨端本地存储：H5 有 localStorage；原生端注入 sys.localStorage。
    const g = globalThis as Record<string, unknown>;
    if (!g.localStorage) {
      registerKV({
        getItem: (k) => sys.localStorage.getItem(k),
        setItem: (k, v) => sys.localStorage.setItem(k, v),
        removeItem: (k) => sys.localStorage.removeItem(k),
      });
    }

    // ScreenRoot 必须从创建时就拥有真实尺寸；否则子 Screen 的 Widget/HUD 会以 0x0 父节点计算。
    const visible = view.getVisibleSize();
    const canvasT = this.node.getComponent(UITransform);
    const screenRoot = uiNode(
      'ScreenRoot',
      canvasT && canvasT.width > 0 ? canvasT.width : visible.width,
      canvasT && canvasT.height > 0 ? canvasT.height : visible.height,
    );
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
