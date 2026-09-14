import type { RoomSnapshotData } from '../protocol/messages';

export interface MotionState {
  x: number;
  y: number;
  radius: number;
  mass: number;
}

interface MotionFrame {
  serverTime: number;
  receivedAt: number;
  balls: Map<string, MotionState>;
  ejected: Map<string, MotionState>;
}

export interface SnapshotMotionSample {
  targetServerTime: number;
  alpha: number;
  ball(ballId: string): MotionState | undefined;
  ejected(ejectId: string): MotionState | undefined;
}

class MotionSampleView implements SnapshotMotionSample {
  constructor(
    readonly targetServerTime: number,
    readonly alpha: number,
    private readonly a: MotionFrame,
    private readonly b: MotionFrame,
  ) {}

  ball(ballId: string): MotionState | undefined {
    return interpolateMotion(this.a.balls.get(ballId), this.b.balls.get(ballId), this.alpha);
  }

  ejected(ejectId: string): MotionState | undefined {
    return interpolateMotion(this.a.ejected.get(ejectId), this.b.ejected.get(ejectId), this.alpha);
  }
}

/**
 * 保存最近若干服务端快照，并在渲染时间轴上取相邻两帧做线性插值。
 *
 * 这里不做外推：如果网络短暂断帧，最多停在最新权威位置，避免客户端凭空预测
 * 远端玩家。自己的玩家不应该使用这个延迟时间轴，交由 BattleManager/EntityManager
 * 继续追最新权威状态，后续可独立增加 client prediction/reconciliation。
 */
export class SnapshotBuffer {
  private readonly frames: MotionFrame[] = [];

  constructor(
    readonly interpolationDelayMs = 150,
    readonly maxFrames = 8,
  ) {}

  reset(): void {
    this.frames.length = 0;
  }

  get size(): number {
    return this.frames.length;
  }

  push(snapshot: RoomSnapshotData, receivedAt = Date.now()): boolean {
    const last = this.frames[this.frames.length - 1];
    if (last && snapshot.serverTime <= last.serverTime) {
      return false; // 重复/乱序快照不能让渲染时间轴倒退。
    }

    const balls = new Map<string, MotionState>();
    for (const player of snapshot.players) {
      for (const ball of player.balls) {
        balls.set(ball.ballId, {
          x: ball.x,
          y: ball.y,
          radius: ball.radius,
          mass: ball.mass,
        });
      }
    }

    const ejected = new Map<string, MotionState>();
    for (const item of snapshot.ejected ?? []) {
      ejected.set(item.ejectId, {
        x: item.x,
        y: item.y,
        radius: item.radius,
        mass: item.mass,
      });
    }

    this.frames.push({
      serverTime: snapshot.serverTime,
      receivedAt,
      balls,
      ejected,
    });
    while (this.frames.length > Math.max(2, this.maxFrames)) {
      this.frames.shift();
    }
    return true;
  }

  sample(now = Date.now()): SnapshotMotionSample | null {
    if (this.frames.length === 0) return null;

    const latest = this.frames[this.frames.length - 1];
    const elapsedSinceLatest = Math.max(0, now - latest.receivedAt);
    const targetServerTime = latest.serverTime + elapsedSinceLatest - this.interpolationDelayMs;

    if (this.frames.length === 1 || targetServerTime <= this.frames[0].serverTime) {
      const first = this.frames[0];
      return new MotionSampleView(targetServerTime, 0, first, first);
    }

    // 渲染时间已经越过旧帧后，可以释放不再需要的历史数据。
    while (this.frames.length > 2 && this.frames[1].serverTime <= targetServerTime) {
      this.frames.shift();
    }

    const currentLatest = this.frames[this.frames.length - 1];
    if (targetServerTime >= currentLatest.serverTime) {
      return new MotionSampleView(targetServerTime, 0, currentLatest, currentLatest);
    }

    for (let i = 0; i < this.frames.length - 1; i++) {
      const a = this.frames[i];
      const b = this.frames[i + 1];
      if (targetServerTime < a.serverTime || targetServerTime > b.serverTime) continue;
      const span = Math.max(1, b.serverTime - a.serverTime);
      const alpha = clamp01((targetServerTime - a.serverTime) / span);
      return new MotionSampleView(targetServerTime, alpha, a, b);
    }

    return new MotionSampleView(targetServerTime, 0, currentLatest, currentLatest);
  }
}

function interpolateMotion(a: MotionState | undefined, b: MotionState | undefined, alpha: number): MotionState | undefined {
  // 以较早帧的实体集合为准：新实体直到其服务端帧时间到达才出现；
  // 消失实体则保持到下一帧边界。EntityManager 的集合 diff 仍负责最终回收节点。
  if (!a) return undefined;
  if (!b || alpha <= 0) return a;
  if (alpha >= 1) return b;
  return {
    x: lerp(a.x, b.x, alpha),
    y: lerp(a.y, b.y, alpha),
    radius: lerp(a.radius, b.radius, alpha),
    mass: lerp(a.mass, b.mass, alpha),
  };
}

function lerp(a: number, b: number, t: number): number {
  return a + (b - a) * t;
}

function clamp01(v: number): number {
  return Math.max(0, Math.min(1, v));
}
