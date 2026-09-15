// 对战屏幕：轻量 HUD，四角贴边，不遮挡核心战场。

import { Layers, Node, Sprite, UITransform } from 'cc';
import type { Screen, ScreenCtx } from '../app/context';
import { BattleManager } from '../battle/battle-manager';
import {
  centerIn,
  circleButton,
  clearChildren,
  label,
  panel,
  pill,
  row,
  setLabel,
  uiNode,
} from '../ui/builder';
import { fullSizeNode, pin, screenLayer } from '../ui/layout';
import { ringTexture, solidTexture } from '../ui/texgen';
import { theme, withAlpha } from '../ui/theme';

export class GameScreen implements Screen {
  private node: Node | null = null;
  private battle: BattleManager | null = null;
  private hudTimer: ReturnType<typeof setInterval> | null = null;
  private massEl: Node | null = null;
  private scoreEl: Node | null = null;
  private timerEl: Node | null = null;
  private rankEl: Node | null = null;
  private statusOverlay: Node | null = null;
  private statusText: Node | null = null;
  private spinner: Node | null = null;

  async mount(ctx: ScreenCtx): Promise<void> {
    const match = ctx.session.match;
    const user = ctx.session.user;
    if (!match || !user) {
      ctx.toast('对局信息缺失');
      ctx.go('lobby');
      return;
    }

    this.node = screenLayer(ctx.root, 'game-screen');

    const worldRoot = new Node('WorldRoot');
    worldRoot.layer = Layers.Enum.UI_2D;
    this.node.addChild(worldRoot);

    // 左上：玩家状态。尺寸与 H5 HUD 接近，避免大卡片遮挡战场。
    this.massEl = label('质量 0', {
      width: 82,
      fontSize: theme.smallSize + 1,
      align: 'left',
    });
    this.scoreEl = label('得分 0', {
      width: 82,
      fontSize: theme.smallSize + 1,
      color: theme.accent,
      align: 'right',
    });
    const statHud = panel([
      label(user.nickname, {
        width: 158,
        fontSize: theme.smallSize + 1,
        color: theme.primaryBright,
        align: 'left',
      }),
      row([this.massEl, this.scoreEl], 4),
    ], 184, 2, 10, {
      height: 66,
      color: withAlpha(theme.card, 185),
      borderColor: withAlpha(theme.cardBorder, 100),
      shadow: false,
    });
    pin(statHud, this.node, 'left-top', 14, 14);

    // 顶部中央：只保留倒计时。
    this.timerEl = label('--', {
      width: 92,
      fontSize: theme.bigSize - 4,
    });
    const timerHud = panel([this.timerEl], 112, 0, 8, {
      height: 44,
      color: withAlpha(theme.card, 190),
      borderColor: withAlpha(theme.primaryBright, 90),
      shadow: false,
    });
    pin(timerHud, this.node, 'center-top', 0, 14);

    // 右上：紧凑排行榜，最多展示 5 人。
    this.rankEl = uiNode('rank-list', 174, 146);
    const rankPanel = panel([
      label('局内排行', {
        width: 174,
        fontSize: theme.smallSize + 2,
        color: theme.warning,
        align: 'left',
      }),
      this.rankEl,
    ], 206, 6, 12, {
      height: 210,
      color: withAlpha(theme.card, 175),
      borderColor: withAlpha(theme.cardBorder, 90),
      shadow: false,
    });
    pin(rankPanel, this.node, 'right-top', 14, 14);

    // 右下：技能键固定贴边，不再堆在屏幕中央。
    const splitBtn = circleButton('分裂', () => this.battle?.requestSplit(), {
      size: 72,
      color: theme.primary,
      ringColor: withAlpha(theme.primaryBright, 170),
      fontSize: theme.smallSize + 2,
    });
    const ejectBtn = circleButton('吐球', () => this.battle?.requestEject(), {
      size: 64,
      color: theme.accent,
      ringColor: withAlpha(theme.accent, 120),
      fontSize: theme.smallSize + 1,
    });
    const skills = row([splitBtn, ejectBtn], 12);
    pin(skills, this.node, 'right-bottom', 18, 18);

    const hint = pill('鼠标 / 手指移动 · Space 分裂 · W 吐球', {
      width: 270,
      height: 28,
      fontSize: theme.microSize + 1,
      color: withAlpha(theme.card, 145),
      textColor: theme.muted,
    });
    pin(hint, this.node, 'left-bottom', 14, 18);

    // 状态遮罩只在入房、倒计时、重连和结算时出现。
    this.statusText = label('正在进入房间...', { width: 320, fontSize: theme.fontSize });
    this.spinner = this.createStatusSpinner();
    const statusCard = panel([
      this.spinner,
      this.statusText,
      label('请保持连接', {
        width: 320,
        fontSize: theme.smallSize,
        color: theme.muted,
      }),
    ], 380, 10, 24, { height: 230, color: theme.cardStrong });

    this.statusOverlay = this.createOverlay();
    centerIn(this.statusOverlay, statusCard);
    this.node.addChild(this.statusOverlay);

    this.battle = new BattleManager(worldRoot, {
      onStatus: (t) => {
        if (this.statusText?.isValid) setLabel(this.statusText, t);
      },
      onCountdown: (s) => {
        if (this.statusText?.isValid) setLabel(this.statusText, `即将开始 · ${s}`);
      },
      onGameStart: () => {
        if (this.statusOverlay?.isValid) this.statusOverlay.active = false;
        this.startHud();
      },
      onSkillFailed: (msg) => ctx.toast(msg),
      onReconnecting: (n) => {
        if (this.statusText?.isValid) setLabel(this.statusText, `连接中断，正在重连（第 ${n} 次）`);
        if (this.statusOverlay?.isValid) this.statusOverlay.active = true;
      },
      onReconnected: () => {
        if (this.statusOverlay?.isValid) this.statusOverlay.active = false;
      },
      onGameEnd: (_reason, msg) => {
        if (this.statusText?.isValid) setLabel(this.statusText, msg || '对局结束，正在结算...');
        if (this.statusOverlay?.isValid) this.statusOverlay.active = true;
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
      await this.battle.start({
        roomId: match.roomId,
        userId: user.userId,
        enterToken: match.enterToken,
        wsUrl: match.wsUrl,
        nickname: user.nickname,
      });
    } catch {
      ctx.toast('连接服务器失败');
      ctx.go('lobby');
    }
  }

  private createOverlay(): Node {
    const overlay = fullSizeNode(this.node!, 'status-overlay');
    const sp = overlay.addComponent(Sprite);
    sp.spriteFrame = solidTexture();
    sp.sizeMode = Sprite.SizeMode.CUSTOM;
    sp.color = theme.overlay;
    return overlay;
  }

  private createStatusSpinner(): Node {
    const n = uiNode('status-spinner', 58, 58);
    const sp = n.addComponent(Sprite);
    sp.spriteFrame = ringTexture();
    sp.sizeMode = Sprite.SizeMode.CUSTOM;
    sp.color = theme.primaryBright;
    return n;
  }

  private startHud(): void {
    if (this.hudTimer) return;
    this.updateHud();
    this.hudTimer = setInterval(() => this.updateHud(), 500);
  }

  private updateHud(): void {
    const b = this.battle;
    if (!b || !this.massEl?.isValid || !this.scoreEl?.isValid || !this.timerEl?.isValid || !this.rankEl?.isValid) {
      return;
    }

    const self = b.state.self();
    const mass = Math.round((self?.balls ?? []).reduce((sum, ball) => sum + (ball.mass ?? 0), 0));
    const score = self?.score ?? 0;
    setLabel(this.massEl, `质量 ${mass}`);
    setLabel(this.scoreEl, `得分 ${score}`);

    const remaining = Math.max(0, b.state.remainingSeconds(Date.now()));
    const minutes = Math.floor(remaining / 60).toString().padStart(2, '0');
    const seconds = (remaining % 60).toString().padStart(2, '0');
    setLabel(this.timerEl, `${minutes}:${seconds}`);

    clearChildren(this.rankEl);
    const t = this.rankEl.getComponent(UITransform)!;
    let y = t.height / 2 - 14;
    const rows = b.state.rankTopN.slice(0, 5);
    for (const r of rows) {
      const isMe = r.userId === b.state.selfUserId;
      const item = row([
        label(r.rank <= 3 ? `${r.rank}` : `#${r.rank}`, {
          width: 30,
          fontSize: theme.smallSize,
          color: r.rank <= 3 ? theme.warning : theme.muted,
          align: 'left',
        }),
        label(r.nickname, {
          width: 92,
          fontSize: theme.smallSize,
          color: isMe ? theme.primaryBright : theme.text,
          align: 'left',
        }),
        label(`${r.score}`, {
          width: 44,
          fontSize: theme.smallSize,
          color: isMe ? theme.accent : theme.muted,
          align: 'right',
        }),
      ], 4);
      item.setPosition(0, y, 0);
      this.rankEl.addChild(item);
      y -= 27;
    }

    if (b.state.selfRank && !rows.some((r) => r.userId === b.state.selfUserId)) {
      const me = label(`我的排名 #${b.state.selfRank.rank}`, {
        width: 170,
        fontSize: theme.smallSize,
        color: theme.primaryBright,
        align: 'left',
      });
      me.setPosition(0, -t.height / 2 + 12, 0);
      this.rankEl.addChild(me);
    }
  }

  update(dt: number): void {
    this.battle?.update(dt);
    if (this.spinner?.isValid) {
      const r = this.spinner.eulerAngles;
      this.spinner.setRotationFromEuler(0, 0, r.z - dt * 160);
    }
  }

  unmount(): void {
    if (this.hudTimer) clearInterval(this.hudTimer);
    this.hudTimer = null;
    this.battle?.stop();
    this.battle = null;
    this.node?.destroy();
    this.node = null;
  }
}
