// 实体管理器：快照 diff -> 创建/更新/回收节点。
// - 球体：插值平滑（display lerp 向 snapshot 目标）
// - 食物/吐出物：直接使用快照位置
// - 视野裁剪：可见矩形外的节点 active=false
// - 排序：按质量升序设置渲染顺序（小者先画、大者覆盖）

import { Color, Graphics, Label, Layers, Node, Sprite, UITransform } from 'cc';
import { config } from '../core/config';
import type { GameState } from '../core/state/game-state';
import { circleTexture } from '../ui/texgen';
import { colorForUserId, parseFoodColor, theme } from '../ui/theme';
import { NodePool } from './object-pool';
import { WorldCamera } from './camera';
import { frameRateAdjusted } from '../core/math';

interface BallEntry {
  key: string;
  node: Node;          // 根节点：自身=白色描边圆，他人=身体圆
  body: Node | null;   // 自身时为根节点内的身体圆（覆盖描边）
  nameLabel: Node | null;
  massLabel: Node | null;
  tx: number;
  ty: number;
  tr: number;
  dx: number;
  dy: number;
  dr: number;
  mass: number;
}

interface SimpleEntry {
  key: string;
  node: Node;
  tx: number;
  ty: number;
  radius: number;
}

function newCircleNode(color: Color, diameter: number): Node {
  const n = new Node('circle');
  n.layer = Layers.Enum.UI_2D;
  const t = n.addComponent(UITransform);
  t.setContentSize(diameter, diameter);
  const sp = n.addComponent(Sprite);
  sp.spriteFrame = circleTexture();
  sp.sizeMode = Sprite.SizeMode.CUSTOM;
  sp.color = color.clone();
  return n;
}

function newLabelNode(text: string, fontSize: number, color: Color): Node {
  const n = new Node('lbl');
  n.layer = Layers.Enum.UI_2D;
  n.addComponent(UITransform);
  const lb = n.addComponent(Label);
  lb.string = text;
  lb.fontSize = fontSize;
  lb.lineHeight = Math.ceil(fontSize * 1.2);
  lb.color = color;
  lb.overflow = Label.Overflow.SHRINK;
  lb.horizontalAlign = Label.HorizontalAlign.CENTER;
  lb.verticalAlign = Label.VerticalAlign.CENTER;
  n.getComponent(UITransform)!.setContentSize(400, fontSize * 1.4);
  return n;
}

export class EntityManager {
  readonly worldRoot: Node;
  private foodLayer: Node;
  private ejectedLayer: Node;
  private playerLayer: Node;
  private balls = new Map<string, BallEntry>();
  private foods = new Map<string, SimpleEntry>();
  private ejects = new Map<string, SimpleEntry>();
  private sortedBallKeys: string[] = [];

  private ballPool: NodePool;
  private simplePool: NodePool;

  constructor(worldRoot: Node) {
    this.worldRoot = worldRoot;
    this.foodLayer = this.childLayer('FoodLayer');
    this.ejectedLayer = this.childLayer('EjectedLayer');
    this.playerLayer = this.childLayer('PlayerLayer');
    this.drawGrid();

    this.ballPool = new NodePool(() => newCircleNode(theme.text, 8));
    this.simplePool = new NodePool(() => newCircleNode(theme.text, 6));
  }

  private childLayer(name: string): Node {
    const n = new Node(name);
    n.layer = Layers.Enum.UI_2D;
    this.worldRoot.addChild(n);
    return n;
  }

  /** 网格与边界只画一次（静态）。 */
  private drawGrid(): void {
    const n = new Node('Grid');
    n.layer = Layers.Enum.UI_2D;
    this.worldRoot.addChild(n);
    n.setSiblingIndex(0);
    const g = n.addComponent(Graphics);
    const step = 200;
    g.lineWidth = 1 / 1.6; // 近似 1px（基准缩放下）
    g.strokeColor = new Color(255, 255, 255, 13);
    for (let gx = 0; gx <= config.worldWidth; gx += step) {
      g.moveTo(gx, 0);
      g.lineTo(gx, config.worldHeight);
    }
    for (let gy = 0; gy <= config.worldHeight; gy += step) {
      g.moveTo(0, gy);
      g.lineTo(config.worldWidth, gy);
    }
    g.stroke();

    // 地图边界
    g.lineWidth = 3 / 1.6;
    g.strokeColor = new Color(106, 209, 255, 102);
    g.rect(0, 0, config.worldWidth, config.worldHeight);
    g.stroke();
  }

  /** 快照到达：diff 实体集合 + 按质量重排渲染顺序。 */
  sync(state: GameState): void {
    // ---- 球体 ----
    const seen = new Set<string>();
    const all: { key: string; mass: number }[] = [];
    for (const p of state.players.values()) {
      const color = colorForUserId(p.userId);
      const isSelf = p.userId === state.selfUserId;
      for (const b of p.balls) {
        seen.add(b.ballId);
        all.push({ key: b.ballId, mass: b.mass });
        let e = this.balls.get(b.ballId);
        if (!e) {
          const node = this.ballPool.get();
          node.removeAllChildren();
          this.playerLayer.addChild(node);
          e = {
            key: b.ballId,
            node,
            body: null,
            nameLabel: null,
            massLabel: null,
            tx: b.x, ty: b.y, tr: b.radius,
            dx: b.x, dy: b.y, dr: b.radius,
            mass: b.mass,
          };
          this.balls.set(b.ballId, e);
        }
        e.tx = b.x;
        e.ty = b.y;
        e.tr = b.radius;
        e.mass = b.mass;

        // 自身体=白描边 + 身体圆；他人=单圆
        const d = b.radius * 2;
        const rootSprite = e.node.getComponent(Sprite)!;
        if (isSelf) {
          rootSprite.color = theme.self;
          e.node.getComponent(UITransform)!.setContentSize(d + 8, d + 8);
          if (!e.body) {
            e.body = newCircleNode(color, d);
            e.node.addChild(e.body);
          }
          e.body.getComponent(Sprite)!.color = color;
          e.body.getComponent(UITransform)!.setContentSize(d, d);
        } else {
          rootSprite.color = color;
          e.node.getComponent(UITransform)!.setContentSize(d, d);
        }

        // 昵称/质量标签（懒创建，大球才显示）
        if (!e.nameLabel) {
          e.nameLabel = newLabelNode(p.nickname, 20, new Color(0, 0, 0, 217));
          e.node.addChild(e.nameLabel);
        }
        if (!e.massLabel) {
          e.massLabel = newLabelNode(String(Math.round(b.mass)), 16, new Color(0, 0, 0, 217));
          e.node.addChild(e.massLabel);
        }
        const nl = e.nameLabel.getComponent(Label)!;
        if (nl.string !== p.nickname) nl.string = p.nickname;
        const ml = e.massLabel.getComponent(Label)!;
        const ms = String(Math.round(b.mass));
        if (ml.string !== ms) ml.string = ms;
      }
    }
    for (const [key, e] of this.balls) {
      if (!seen.has(key)) {
        e.node.removeFromParent();
        this.ballPool.put(e.node);
        this.balls.delete(key);
      }
    }

    // 按质量升序排渲染顺序（只在顺序变化时更新 sibling index）
    all.sort((a, b) => a.mass - b.mass);
    const newOrder = all.map((x) => x.key);
    const changed = newOrder.length !== this.sortedBallKeys.length
      || newOrder.some((k, i) => k !== this.sortedBallKeys[i]);
    if (changed) {
      this.sortedBallKeys = newOrder;
      newOrder.forEach((key, i) => {
        const e = this.balls.get(key);
        if (e) e.node.setSiblingIndex(i);
      });
    }

    // ---- 食物 ----
    const seenFoods = new Set<string>();
    for (const f of state.foods.values()) {
      seenFoods.add(f.foodId);
      let e = this.foods.get(f.foodId);
      if (!e) {
        const node = this.simplePool.get();
        this.foodLayer.addChild(node);
        e = { key: f.foodId, node, tx: f.x, ty: f.y, radius: 5 };
        this.foods.set(f.foodId, e);
      }
      e.tx = f.x;
      e.ty = f.y;
      e.node.getComponent(Sprite)!.color = parseFoodColor(f.color);
    }
    for (const [key, e] of this.foods) {
      if (!seenFoods.has(key)) {
        e.node.removeFromParent();
        this.simplePool.put(e.node);
        this.foods.delete(key);
      }
    }

    // ---- 吐出物 ----
    const seenEjects = new Set<string>();
    for (const ej of state.ejected.values()) {
      seenEjects.add(ej.ejectId);
      let e = this.ejects.get(ej.ejectId);
      if (!e) {
        const node = this.simplePool.get();
        this.ejectedLayer.addChild(node);
        e = { key: ej.ejectId, node, tx: ej.x, ty: ej.y, radius: ej.radius };
        this.ejects.set(ej.ejectId, e);
      }
      e.tx = ej.x;
      e.ty = ej.y;
      e.radius = ej.radius;
      e.node.getComponent(Sprite)!.color = colorForUserId(ej.ownerId);
    }
    for (const [key, e] of this.ejects) {
      if (!seenEjects.has(key)) {
        e.node.removeFromParent();
        this.simplePool.put(e.node);
        this.ejects.delete(key);
      }
    }
  }

  /** 每帧：位置/半径插值 + 标签布局 + 视野裁剪。 */
  update(dt: number, camera: WorldCamera, visW: number, visH: number): void {
    const rect = camera.visibleRect(visW, visH);

    for (const e of this.balls.values()) {
      const f = frameRateAdjusted(0.3, dt);
      e.dx += (e.tx - e.dx) * f;
      e.dy += (e.ty - e.dy) * f;
      e.dr += (e.tr - e.dr) * f;
      const visible = e.dx >= rect.x0 && e.dx <= rect.x1 && e.dy >= rect.y0 && e.dy <= rect.y1;
      e.node.active = visible;
      if (!visible) continue;

      e.node.setPosition(e.dx, config.worldHeight - e.dy, 0);

      // 标签：按显示半径决定字号与显隐（世界单位，随相机缩放）
      const showLabels = e.dr * camera.scale > 14;
      if (e.nameLabel) {
        e.nameLabel.active = showLabels;
        if (showLabels) {
          const fs = Math.max(12, Math.min(e.dr * 0.5, 44));
          const nl = e.nameLabel.getComponent(Label)!;
          nl.fontSize = fs;
          nl.lineHeight = Math.ceil(fs * 1.2);
          e.nameLabel.setPosition(0, e.dr * 0.15, 0);
        }
      }
      if (e.massLabel) {
        e.massLabel.active = showLabels;
        if (showLabels) {
          const fs = Math.max(10, Math.min(e.dr * 0.4, 36));
          const ml = e.massLabel.getComponent(Label)!;
          ml.fontSize = fs;
          ml.lineHeight = Math.ceil(fs * 1.2);
          e.massLabel.setPosition(0, -e.dr * 0.4, 0);
        }
      }
    }

    for (const e of this.foods.values()) {
      const visible = e.tx >= rect.x0 && e.tx <= rect.x1 && e.ty >= rect.y0 && e.ty <= rect.y1;
      e.node.active = visible;
      if (!visible) continue;
      e.node.setPosition(e.tx, config.worldHeight - e.ty, 0);
      e.node.getComponent(UITransform)!.setContentSize(10, 10);
    }

    for (const e of this.ejects.values()) {
      const visible = e.tx >= rect.x0 && e.tx <= rect.x1 && e.ty >= rect.y0 && e.ty <= rect.y1;
      e.node.active = visible;
      if (!visible) continue;
      e.node.setPosition(e.tx, config.worldHeight - e.ty, 0);
      const d = Math.max(6, e.radius * 2);
      e.node.getComponent(UITransform)!.setContentSize(d, d);
    }
  }

  /** 断线重连/重开时全量重建。 */
  reset(): void {
    for (const e of this.balls.values()) {
      e.node.removeFromParent();
      this.ballPool.put(e.node);
    }
    this.balls.clear();
    this.sortedBallKeys = [];
    for (const e of this.foods.values()) {
      e.node.removeFromParent();
      this.simplePool.put(e.node);
    }
    this.foods.clear();
    for (const e of this.ejects.values()) {
      e.node.removeFromParent();
      this.simplePool.put(e.node);
    }
    this.ejects.clear();
  }
}
