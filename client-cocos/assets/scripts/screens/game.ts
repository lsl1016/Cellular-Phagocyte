// 对战屏幕：世界渲染 + 轻量 HUD + 圆形技能按钮 + 状态遮罩。

import { Color, Layers, Node, Sprite, UITransform, Widget } from 'cc';
import type { Screen, ScreenCtx } from '../app/context';
import { BattleManager } from '../battle/battle-manager';
import {
  circleButton,
  clearChildren,
  column,
  label,
  panel,
  pill,
  row,
  setLabel,
  uiNode,
} from '../ui/builder';
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

    this.node = uiNode('game-screen');
    ctx.root.addChild(this.node);

    // ---- 世界根节点（相机作用对象） ----
    const worldRoot = new Node('WorldRoot');
    worldRoot.layer = Layers.Enum.UI_2D;
    this.node.addChild(worldRoot);

    // ---- 左上：质量 / 得分 ----
    this.massEl = label('质量 0', {
      width: 112,
      fontSize: theme.smallSize + 4,
      color: theme.text,
      align: 'left',
    });
    this.scoreEl = label('得分 0', {
      width: 112,
      fontSize: theme.smallSize + 4,
      color: theme.accent,
      align: 'left',
    });
    const statHud = panel([
      pill(user.nickname, {
        width: 210,
        height: 28,
        fontSize: theme.smallSize,
        color: withAlpha(theme.primary, 55),
        textColor: theme.primaryBright,
      }),
      row([this.massEl, this.scoreEl], 8),
    ], 246, 5, 14, {
      height: 86,
      color: withAlpha(theme.card, 230),
      shadow: false,
    });
    this.anchor(statHud, this.node, 'left-top', 22, 22);

    // ---- 顶部中央：倒计时 ----
    this.timerEl = label('--', {
      width: 120,
      fontSize: theme.bigSize,
      color: theme.text,
    });
    const timerHud = panel([this.timerEl], 154, 0, 10, {
      height: 54,
      color: withAlpha(theme.cardStrong, 232),
      shadow: false,
      borderColor: withAlpha(theme.primaryBright, 105),
    });
    this.anchor(timerHud, this.node, 'center-top', 0, 22);

    // ---- 右上：排行榜 ----
    this.rankEl = uiNode('rank-list', 226, 216);
    const rankPanel = panel([
      row([
        label('局内排行', { width: 130, fontSize: theme.smallSize + 5, align: 'left' }),
        pill('TOP', {
          width: 56,
          height: 26,
          fontSize: theme.microSize,
          color: withAlpha(theme.primary, 48),
          textColor: theme.primaryBright,
        }),
      ], 18),
      this.rankEl,
    ], 270, 8, 16, {
      height: 290,
      color: withAlpha(theme.card, 222),
      shadow: false,
    });
    this.anchor(rankPanel, this.node, 'right-top', 22, 22);

    // ---- 右下：技能区 ----
    const splitBtn = circleButton('分裂', () => this.battle?.requestSplit(), {
      size: 88,
      color: theme.primary,
      ringColor: withAlpha(theme.primaryBright, 180),
    });
    const splitGroup = column([
      splitBtn,
      pill('SPACE', {
        width: 72,
        height: 24,
        fontSize: theme.microSize,
        color: withAlpha(theme.secondary, 215),
        textColor: theme.muted,
      }),
    ], 3);
    this.anchor(splitGroup, this.node, 'right-bottom', 138, 24);

    const ejectBtn = circleButton('吐球', () => this.battle?.requestEject(), {
      size: 78,
      color: theme.accent,
      ringColor: withAlpha(theme.accent, 135),
      fontSize: theme.smallSize + 4,
    });
    const ejectGroup = column([
      ejectBtn,
      pill('W', {
        width: 54,
        height: 24,
        fontSize: theme.microSize,
        color: withAlpha(theme.secondary, 215),
        textColor: theme.muted,
      }),
    ], 3);
    this.anchor(ejectGroup, this.node, 'right-bottom', 42, 28);

    const hint = pill('鼠标 / 手指控制移动方向', {
      width: 210,
      height: 30,
      fontSize: theme.microSize + 1,
      color: withAlpha(theme.card, 178),
      textColor: theme.muted,
    });
    this.anchor(hint, this.node, 'left-bottom', 22, 24);

    // ---- 状态遮罩（入房 / 准备 / 倒计时 / 重连 / 结算） ----
    this.statusText = label('正在进入房间...', { width: 380, fontSize: theme.fontSize });
    this.spinner = this.createStatusSpinner();
    const statusCard = panel([
      pill('SYSTEM', {
        width: 92,
        height: 28,
        fontSize: theme.microSize,
        color: withAlpha(theme.primary, 48),
        textColor: theme.primaryBright,
      }),
      this.spinner,
      this.statusText,
      label('连接恢复期间不会丢失已经确认的对局状态', {
        width: 380,
        fontSize: theme.smallSize,
        color: theme.muted,
      }),
    ], 470, 12, 28, { height: 310, color: theme.cardStrong });

    this.statusOverlay = this.createOverlay();
    statusCard.setPosition(0, 0, 0);
    this.statusOverlay.addChild(statusCard);
    this.node.addChild(this.statusOverlay);

    // ---- 对战编排 ----
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
        if (this.statusText?.isValid) setLabel(this.statusText, msg || '对局结束，正在生成结算...');
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
    const overlay = uiNode('status-overlay', 2, 2);
    const sp = overlay.addComponent(Sprite);
    sp.spriteFrame = solidTexture();
    sp.sizeMode = Sprite.SizeMode.CUSTOM;
    sp.color = theme.overlay;
    const wg = overlay.addComponent(Widget);
    wg.isAlignTop = wg.isAlignBottom = wg.isAlignLeft = wg.isAlignRight = true;
    wg.top = wg.bottom = wg.left = wg.right = 0;
    return overlay;
  }

  private createStatusSpinner(): Node {
    const n = uiNode('status-spinner', 72, 72);
    const sp = n.addComponent(Sprite);
    sp.spriteFrame = ringTexture();
    sp.sizeMode = Sprite.SizeMode.CUSTOM;
    sp.color = theme.primaryBright;
    return n;
  }

  /** 把节点锚定到屏幕某个角落（基于 Widget）。 */
  private anchor(n: Node, parent: Node, pos: string, offset: number, offsetY: number): void {
    const wg = n.addComponent(Widget);
    const t = n.getComponent(UITransform)!;
    if (pos === 'left-top') {
      wg.isAlignLeft = wg.isAlignTop = true;
      wg.left = offset + t.width / 2;
      wg.top = offsetY + t.height / 2;
    } else if (pos === 'center-top') {
      wg.isAlignHorizontalCenter = wg.isAlignTop = true;
      wg.horizontalCenter = 0;
      wg.top = offsetY + t.height / 2;
    } else if (pos === 'right-top') {
      wg.isAlignRight = wg.isAlignTop = true;
      wg.right = offset + t.width / 2;
      wg.top = offsetY + t.height / 2;
    } else if (pos === 'right-bottom') {
      wg.isAlignRight = wg.isAlignBottom = true;
      wg.right = offset + t.width / 2;
      wg.bottom = offsetY + t.height / 2;
    } else if (pos === 'left-bottom') {
      wg.isAlignLeft = wg.isAlignBottom = true;
      wg.left = offset + t.width / 2;
      wg.bottom = offsetY + t.height / 2;
    }
    wg.alignMode = Widget.AlignMode.ONCE;
    parent.addChild(n);
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
    setLabel(this.timerEl, `${b.state.remainingSeconds(Date.now())}s`);

    clearChildren(this.rankEl);
    let y = this.rankEl.getComponent(UITransform)!.height / 2 - 16;
    const rows = b.state.rankTopN.slice(0, 6);
    for (const r of rows) {
      const isMe = r.userId === b.state.selfUserId;
      const rankText = r.rank <= 3 ? `TOP ${r.rank}` : `#${r.rank}`;
      const item = row([
        label(rankText, {
          width: 56,
          fontSize: theme.smallSize,
          color: r.rank <= 3 ? theme.warning : theme.muted,
          align: 'left',
        }),
        label(r.nickname, {
          width: 98,
          fontSize: theme.smallSize + 1,
          color: isMe ? theme.primaryBright : theme.text,
          align: 'left',
        }),
        label(`${r.score}`, {
          width: 58,
          fontSize: theme.smallSize + 1,
          color: isMe ? theme.accent : theme.muted,
          align: 'right',
        }),
      ], 4);
      item.setPosition(0, y, 0);
      this.rankEl.addChild(item);
      y -= 32;
    }

    if (b.state.selfRank && !rows.some((r) => r.userId === b.state.selfUserId)) {
      const me = label(`我的排名  #${b.state.selfRank.rank}`, {
        width: 212,
        fontSize: theme.smallSize,
        color: theme.primaryBright,
        align: 'left',
      });
      me.setPosition(0, -this.rankEl.getComponent(UITransform)!.height / 2 + 16, 0);
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
