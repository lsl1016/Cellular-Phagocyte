// 对战场景：承载画布与 HUD，由 BattleManager 驱动。

import type { Scene, SceneCtx } from '../app/context.js';
import { BattleManager } from '../battle/battle-manager.js';
import type { BattleSession } from '../battle/battle-manager.js';
import { button, clear, h } from '../ui/dom.js';

export class GameScene implements Scene {
  private battle: BattleManager | null = null;
  private hudTimer: ReturnType<typeof setInterval> | null = null;

  private statusOverlay!: HTMLElement;
  private statusText!: HTMLElement;
  private massEl!: HTMLElement;
  private timerEl!: HTMLElement;
  private rankEl!: HTMLElement;

  async mount(ctx: SceneCtx): Promise<void> {
    clear(ctx.uiRoot);

    const match = ctx.session.match;
    const user = ctx.session.user;
    if (!match || !user) {
      ctx.toast('对局信息缺失');
      ctx.go('lobby');
      return;
    }

    // HUD（不拦截指针，使画布可接收输入）
    this.massEl = h('div', {}, ['质量 0 · 得分 0']);
    this.timerEl = h('div', { className: 'timer', text: '--' });
    this.rankEl = h('div', { className: 'rank' });
    const hud = h('div', { className: 'hud' }, [
      h('div', { className: 'top-left' }, [this.massEl]),
      this.timerEl,
      this.rankEl,
    ]);

    // 技能按钮（分裂/吐球）
    const skills = h('div', { className: 'skills' }, [
      button('分裂', () => this.battle?.requestSplit(), 'skill split'),
      button('吐球', () => this.battle?.requestEject(), 'skill eject'),
    ]);
    const hint = h('div', { className: 'hint', text: '鼠标控制方向 · Space 分裂 · W 吐球' });

    // 状态遮罩（入房/准备/倒计时/重连/结算）
    this.statusText = h('div', { className: 'subtitle', text: '正在进入房间...' });
    this.statusOverlay = h('div', { className: 'panel centered' }, [
      h('div', { className: 'card' }, [h('div', { className: 'spinner' }), this.statusText]),
    ]);

    ctx.uiRoot.append(hud, skills, hint, this.statusOverlay);

    const session: BattleSession = {
      roomId: match.roomId,
      userId: user.userId,
      enterToken: match.enterToken,
      wsUrl: match.wsUrl,
      nickname: user.nickname,
    };

    this.battle = new BattleManager(ctx.canvas, {
      onStatus: (t) => (this.statusText.textContent = t),
      onCountdown: (s) => (this.statusText.textContent = `即将开始 ${s}...`),
      onGameStart: () => {
        this.statusOverlay.classList.add('hidden');
        this.startHud();
      },
      onSkillFailed: (msg) => ctx.toast(msg),
      onReconnecting: (n) => {
        this.statusText.textContent = `连接断开，正在重连... (第 ${n} 次)`;
        this.statusOverlay.classList.remove('hidden');
      },
      onReconnected: () => this.statusOverlay.classList.add('hidden'),
      onGameEnd: (_reason, msg) => {
        this.statusText.textContent = msg || '对局结束，正在结算...';
        this.statusOverlay.classList.remove('hidden');
      },
      onSettlement: (d) => {
        ctx.session.settlement = d;
        ctx.go('settlement');
      },
      onError: (msg) => {
        ctx.toast(msg);
        ctx.go('lobby');
      },
    });

    try {
      await this.battle.start(session);
    } catch {
      ctx.toast('连接服务器失败');
      ctx.go('lobby');
    }
  }

  private startHud(): void {
    if (this.hudTimer) return;
    this.hudTimer = setInterval(() => this.updateHud(), 200);
  }

  private updateHud(): void {
    const b = this.battle;
    if (!b) return;
    const self = b.state.self();
    const mass = Math.round(self?.balls?.[0]?.mass ?? 0);
    const score = self?.score ?? 0;
    this.massEl.textContent = `质量 ${mass} · 得分 ${score}`;
    this.timerEl.textContent = `${b.state.remainingSeconds(Date.now())}s`;

    clear(this.rankEl);
    this.rankEl.append(h('h4', { text: '局内排行' }));
    for (const r of b.state.rankTopN) {
      const isMe = r.userId === b.state.selfUserId;
      this.rankEl.append(
        h('div', { className: isMe ? 'me' : '' }, [`${r.rank}. ${r.nickname} (${r.score})`]),
      );
    }
    if (b.state.selfRank && !b.state.rankTopN.some((r) => r.userId === b.state.selfUserId)) {
      this.rankEl.append(h('div', { className: 'me' }, [`我: 第 ${b.state.selfRank.rank} 名`]));
    }
  }

  unmount(): void {
    if (this.hudTimer) clearInterval(this.hudTimer);
    this.hudTimer = null;
    this.battle?.stop();
    this.battle = null;
  }
}
