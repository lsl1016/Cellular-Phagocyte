// BattleManager：对战编排。连接 WebSocket、完成入房与准备、接收快照驱动渲染、
// 采集输入发送 MOVE/SPLIT/EJECT，断线后按退避自动重连，并通过回调把对局生命周期
// 事件通知屏幕层。渲染循环由屏幕组件逐帧调用 update(dt)。

import { Node, view } from 'cc';
import { config } from '../core/config';
import { logger } from '../core/logger';
import { WsClient } from '../net/ws';
import { C2S, S2C } from '../core/protocol/messages';
import type {
  CountdownData,
  EnterRoomResultData,
  GameEndData,
  GameStartData,
  RankUpdateData,
  ReconnectResultData,
  RoomSnapshotData,
  SettlementResultData,
  SkillFailedData,
} from '../core/protocol/messages';
import { GameState } from '../core/state/game-state';
import { WorldCamera, zoomForMass } from './camera';
import { InputController } from './input';
import { EntityManager } from './entity-manager';

export interface BattleSession {
  roomId: string;
  userId: string;
  enterToken: string;
  wsUrl: string;
  nickname: string;
}

export interface BattleCallbacks {
  onStatus?: (text: string) => void;
  onCountdown?: (seconds: number) => void;
  onGameStart?: () => void;
  onGameEnd?: (reason: string, message: string) => void;
  onSettlement?: (data: SettlementResultData) => void;
  onSkillFailed?: (message: string) => void;
  onReconnecting?: (attempt: number) => void;
  onReconnected?: () => void;
  onError?: (message: string) => void;
}

const RECONNECT_BACKOFF_MS = [0, 1000, 2000, 3000, 5000, 5000, 5000, 5000];

export class BattleManager {
  readonly state = new GameState();
  readonly camera = new WorldCamera();
  private ws = new WsClient();
  private entities: EntityManager;
  private input: InputController | null = null;

  private session!: BattleSession;
  private reconnectToken = '';

  private running = false;
  private firstFrame = true;
  private gameStarted = false;
  private gameEnded = false;
  private intentionalClose = false;
  private reconnecting = false;
  private reconnectResolver: ((ok: boolean) => void) | null = null;

  /** 基准缩放：设计宽 1280 下约可见 800 世界单位，与旧 Canvas 版视野一致。 */
  private baseScale = 1.6;

  constructor(worldRoot: Node, private cb: BattleCallbacks) {
    this.entities = new EntityManager(worldRoot);
  }

  async start(session: BattleSession): Promise<void> {
    this.session = session;
    this.state.reset(session.userId, session.roomId);
    this.entities.reset();
    this.firstFrame = true;

    this.cb.onStatus?.('正在进入房间...');
    await this.openConnection('enter');
    this.running = true;
  }

  stop(): void {
    this.running = false;
    this.intentionalClose = true;
    this.input?.stop();
    this.ws.close();
    this.entities.reset();
  }

  /** 供 UI 按钮调用：按当前指针方向分裂。 */
  requestSplit(): void {
    if (this.input) this.ws.send(C2S.SPLIT, { direction: this.input.currentDirection(), clientTime: Date.now() });
  }

  /** 供 UI 按钮调用：按当前指针方向吐球。 */
  requestEject(): void {
    if (this.input) this.ws.send(C2S.EJECT, { direction: this.input.currentDirection(), clientTime: Date.now() });
  }

  /** 每帧驱动：相机跟随 + 实体插值与裁剪。由屏幕组件 update 调用。 */
  update(dt: number): void {
    if (!this.running) return;
    this.updateCamera(dt);
    const vs = view.getVisibleSize();
    this.camera.apply(this.entities.worldRoot);
    this.entities.update(dt, this.camera, vs.width, vs.height);
  }

  private updateCamera(dt: number): void {
    const self = this.state.self();
    const balls = self?.balls;
    if (!balls || balls.length === 0) return;

    // 多分身时取所有球的中心，缩放按总质量
    let cx = 0;
    let cy = 0;
    let totalMass = 0;
    for (const b of balls) {
      cx += b.x;
      cy += b.y;
      totalMass += b.mass;
    }
    cx /= balls.length;
    cy /= balls.length;

    const scale = zoomForMass(
      totalMass,
      config.playerBaseMass,
      this.baseScale,
      this.baseScale * 0.12,
      this.baseScale,
    );
    if (this.firstFrame) {
      this.camera.snap(cx, cy, scale);
      this.firstFrame = false;
    } else {
      this.camera.follow(cx, cy, scale, 0.15, dt);
    }
  }

  // ---- 连接与重连 ----

  private async openConnection(mode: 'enter' | 'reconnect'): Promise<void> {
    this.ws = new WsClient();
    this.registerHandlers();
    this.ws.onClose = () => this.handleClose();
    await this.ws.connect(this.session.wsUrl);

    if (mode === 'enter') {
      this.ws.send(C2S.ENTER_ROOM, {
        roomId: this.session.roomId,
        userId: this.session.userId,
        enterToken: this.session.enterToken,
      });
    } else {
      this.ws.send(C2S.RECONNECT, {
        roomId: this.session.roomId,
        userId: this.session.userId,
        reconnectToken: this.reconnectToken,
      });
    }
  }

  private handleClose(): void {
    if (this.intentionalClose || this.gameEnded || this.reconnecting) return;
    if (!this.reconnectToken) {
      this.cb.onError?.('连接已断开');
      return;
    }
    void this.attemptReconnect();
  }

  private async attemptReconnect(): Promise<void> {
    this.reconnecting = true;
    this.input?.stop();
    for (let i = 0; i < RECONNECT_BACKOFF_MS.length; i++) {
      if (this.intentionalClose) return;
      await sleep(RECONNECT_BACKOFF_MS[i]);
      if (this.intentionalClose) return;
      this.cb.onReconnecting?.(i + 1);
      try {
        await this.openConnection('reconnect');
        const ok = await this.waitReconnectResult(3000);
        if (ok) {
          this.reconnecting = false;
          if (this.gameStarted && !this.gameEnded) this.startInput();
          this.cb.onReconnected?.();
          return;
        }
      } catch {
        /* 连接失败，进入下一次退避 */
      }
      this.ws.close();
    }
    this.reconnecting = false;
    this.cb.onError?.('重连失败，已退出对局');
  }

  private waitReconnectResult(timeoutMs: number): Promise<boolean> {
    return new Promise((resolve) => {
      const timer = setTimeout(() => {
        this.reconnectResolver = null;
        resolve(false);
      }, timeoutMs);
      this.reconnectResolver = (ok) => {
        clearTimeout(timer);
        this.reconnectResolver = null;
        resolve(ok);
      };
    });
  }

  private registerHandlers(): void {
    this.ws.on(S2C.ENTER_ROOM_RESULT, (data) => {
      const d = data as EnterRoomResultData;
      if (!d.success) {
        this.cb.onError?.(d.message || '入房失败');
        return;
      }
      if (d.reconnectToken) this.reconnectToken = d.reconnectToken;
      logger.info('ws_auth_success', { roomId: this.session.roomId });
      this.cb.onStatus?.('已入房，等待开始...');
      this.ws.send(C2S.READY, { roomId: this.session.roomId, userId: this.session.userId });
    });

    this.ws.on(S2C.RECONNECT_RESULT, (data) => {
      const d = data as ReconnectResultData;
      this.reconnectResolver?.(d.success);
      if (!d.success) this.cb.onError?.(d.message || '重连失败');
    });

    this.ws.on(S2C.ROOM_RECOVER_SNAPSHOT, (data) => {
      this.state.applySnapshot(data as RoomSnapshotData);
      this.firstFrame = true;
      this.entities.sync(this.state);
    });

    this.ws.on(S2C.START_COUNTDOWN, (data) => {
      this.cb.onCountdown?.((data as CountdownData).countdownSeconds);
    });

    this.ws.on(S2C.GAME_START, (data) => {
      const d = data as GameStartData;
      this.state.setBattle(d.serverTime, d.battleDurationSeconds);
      this.gameStarted = true;
      this.startInput();
      this.cb.onGameStart?.();
    });

    this.ws.on(S2C.ROOM_SNAPSHOT, (data) => {
      this.state.applySnapshot(data as RoomSnapshotData);
      this.entities.sync(this.state);
    });

    this.ws.on(S2C.RANK_UPDATE, (data) => {
      this.state.applyRank(data as RankUpdateData);
    });

    this.ws.on(S2C.SKILL_FAILED, (data) => {
      this.cb.onSkillFailed?.((data as SkillFailedData).message);
    });

    this.ws.on(S2C.GAME_END, (data) => {
      const d = data as GameEndData;
      this.gameEnded = true;
      this.input?.stop();
      this.cb.onGameEnd?.(d.reason, d.message);
    });

    this.ws.on(S2C.SETTLEMENT_RESULT, (data) => {
      this.cb.onSettlement?.(data as SettlementResultData);
    });
  }

  private startInput(): void {
    if (this.input) return;
    this.input = new InputController({
      move: (dir) => this.ws.send(C2S.MOVE, { direction: dir, clientTime: Date.now() }),
      split: (dir) => this.ws.send(C2S.SPLIT, { direction: dir, clientTime: Date.now() }),
      eject: (dir) => this.ws.send(C2S.EJECT, { direction: dir, clientTime: Date.now() }),
    });
    this.input.start();
  }
}

function sleep(ms: number): Promise<void> {
  return new Promise((r) => setTimeout(r, ms));
}
