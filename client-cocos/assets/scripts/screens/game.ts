// 对战屏幕：世界渲染 + HUD + 技能按钮 + 状态遮罩，驱动 BattleManager。

import { Color, Layers, Node, Sprite, UITransform, Widget, view } from 'cc';
import type { Screen, ScreenCtx } from '../app/context';
import { BattleManager } from '../battle/battle-manager';
import { button, card, centerIn, clearChildren, label, setLabel, uiNode } from '../ui/builder';
import { roundRectTexture } from '../ui/texgen';
import { theme } from '../ui/theme';

export class GameScreen implements Screen {
  private node: Node | null = null;
  private battle: BattleManager | null = null;
  private hudTimer: ReturnType<typeof setInterval> | null = null;
  private massEl: Node | null = null;
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

    // ---- HUD（锚定屏幕角落） ----
    const vs = view.getVisibleSize();
    this.massEl = label('质量 0 · 得分 0', { width: 300, align: 'left', color: theme.text });
    this.anchor(this.massEl, this.node, 'left-top', 24, 40);

    this.timerEl = label('--', { width: 160, color: theme.text });
    this.anchor(this.timerEl, this.node, 'center-top', 0, 40);

    this.rankEl = uiNode('rank', 260, 300);
    this.anchor(this.rankEl, this.node, 'right-top', 24, 100);

    // ---- 技能按钮（分裂/吐球） ----
    const splitBtn = button('分裂', () => this.battle?.requestSplit(), { width: 130, height: 64 });
    this.anchor(splitBtn, this.node, 'right-bottom', 170, 60);
    const ejectBtn = button('吐球', () => this.battle?.requestEject(), { width: 130, height: 64 });
    this.anchor(ejectBtn, this.node, 'right-bottom', 30, 60);

    const hint = label('鼠标/手指控制方向 · Space 分裂 · W 吐球', { color: theme.muted, fontSize: theme.smallSize + 2 });
    this.anchor(hint, this.node, 'center-bottom', 0, 28);

    // ---- 状态遮罩（入房/准备/倒计时/重连/结算） ----
    this.statusText = label('正在进入房间...', { width: 420 });
    this.spinner = label('◌', { fontSize: 44, color: theme.primary });
    const c = card([this.spinner, this.statusText], 480);
    this.statusOverlay = uiNode('status-overlay', vs.width, vs.height);
    const bg = this.statusOverlay.addComponent(Sprite);
    bg.spriteFrame = roundRectTexture();
    bg.sizeMode = Sprite.SizeMode.CUSTOM;
    bg.color = new Color(11, 16, 32, 180);
    const wg = this.statusOverlay.addComponent(Widget);
    wg.isAlignTop = wg.isAlignBottom = wg.isAlignLeft = wg.isAlignRight = true;
    wg.top = wg.bottom = wg.left = wg.right = 0;
    this.statusOverlay.addChild(c);
    this.node.addChild(this.statusOverlay);

    // ---- 对战编排 ----
    this.battle = new BattleManager(worldRoot, {
      onStatus: (t) => {
        if (this.statusText?.isValid) setLabel(this.statusText, t);
      },
      onCountdown: (s) => {
        if (this.statusText?.isValid) setLabel(this.statusText, `即将开始 ${s}...`);
      },
      onGameStart: () => {
        if (this.statusOverlay?.isValid) this.statusOverlay.active = false;
        this.startHud(ctx);
      },
      onSkillFailed: (msg) => ctx.toast(msg),
      onReconnecting: (n) => {
        if (this.statusText?.isValid) setLabel(this.statusText, `连接断开，正在重连... (第 ${n} 次)`);
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

  /** 把节点锚定到屏幕某个角落（基于 Widget）。 */
  private anchor(n: Node, parent: Node, pos: string, offset: number, offsetY: number): void {
    const wg = n.addComponent(Widget);
    if (pos === 'left-top') {
      wg.isAlignLeft = wg.isAlignTop = true;
      wg.left = offset + n.getComponent(UITransform)!.width / 2;
      wg.top = offsetY + n.getComponent(UITransform)!.height / 2;
    } else if (pos === 'center-top') {
      wg.isAlignHorizontalCenter = wg.isAlignTop = true;
      wg.isAlignVerticalCenter = false;
      wg.horizontalCenter = 0;
      wg.top = offsetY + n.getComponent(UITransform)!.height / 2;
    } else if (pos === 'right-top') {
      wg.isAlignRight = wg.isAlignTop = true;
      wg.right = offset + n.getComponent(UITransform)!.width / 2;
      wg.top = offsetY + n.getComponent(UITransform)!.height / 2;
    } else if (pos === 'right-bottom') {
      wg.isAlignRight = wg.isAlignBottom = true;
      wg.right = offset + n.getComponent(UITransform)!.width / 2;
      wg.bottom = offsetY + n.getComponent(UITransform)!.height / 2;
    } else if (pos === 'center-bottom') {
      wg.isAlignHorizontalCenter = wg.isAlignBottom = true;
      wg.horizontalCenter = 0;
      wg.bottom = offsetY + n.getComponent(UITransform)!.height / 2;
    }
    wg.alignMode = Widget.AlignMode.ONCE;
    parent.addChild(n);
  }

  private startHud(ctx: ScreenCtx): void {
    if (this.hudTimer) return;
    this.hudTimer = setInterval(() => this.updateHud(), 500);
  }

  private updateHud(): void {
    const b = this.battle;
    if (!b || !this.massEl?.isValid || !this.timerEl?.isValid || !this.rankEl?.isValid) return;
    const self = b.state.self();
    const mass = Math.round(self?.balls?.[0]?.mass ?? 0);
    const score = self?.score ?? 0;
    setLabel(this.massEl, `质量 ${mass} · 得分 ${score}`);
    setLabel(this.timerEl, `${b.state.remainingSeconds(Date.now())}s`);

    clearChildren(this.rankEl);
    this.rankEl.addChild(label('局内排行', { fontSize: theme.smallSize + 6 }));
    for (const r of b.state.rankTopN) {
      const isMe = r.userId === b.state.selfUserId;
      this.rankEl.addChild(
        label(`${r.rank}. ${r.nickname} (${r.score})`, {
          width: 260,
          fontSize: theme.smallSize + 2,
          align: 'left',
          color: isMe ? theme.primaryText : theme.text,
        }),
      );
    }
    if (b.state.selfRank && !b.state.rankTopN.some((r) => r.userId === b.state.selfUserId)) {
      this.rankEl.addChild(
        label(`我: 第 ${b.state.selfRank.rank} 名`, { width: 260, fontSize: theme.smallSize + 2, align: 'left' }),
      );
    }
    // 垂直排布
    let y = 0;
    for (const k of this.rankEl.children) {
      const h = k.getComponent(UITransform)!.height;
      k.setPosition(0, y - h / 2, 0);
      y -= h + 4;
    }
  }

  update(dt: number): void {
    this.battle?.update(dt);
    if (this.spinner?.isValid) {
      const r = this.spinner.eulerAngles;
      this.spinner.setRotationFromEuler(0, 0, r.z - dt * 240);
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
