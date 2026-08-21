// 输入：把指针位置换算成移动方向，并按节流频率发送 MOVE；
// 同时支持分裂(Space)与吐球(W)，方向取当前指针方向。

import { game, Game, input, Input, KeyCode, view } from 'cc';
import { config } from '../core/config';
import { directionFromScreenCenter } from '../core/math';

export interface InputCallbacks {
  move: (direction: number) => void;
  split: (direction: number) => void;
  eject: (direction: number) => void;
}

interface UIPoint {
  getUILocation(): { x: number; y: number };
}

export class InputController {
  private pointerX = 0;
  private pointerY = 0;
  private active = false;
  private timer: ReturnType<typeof setInterval> | null = null;
  private cb: InputCallbacks;

  private onMouseOrTouch = (e: UIPoint) => {
    const p = e.getUILocation();
    this.pointerX = p.x;
    this.pointerY = p.y;
    this.active = true;
  };

  private onKey = (e: { keyCode: KeyCode }) => {
    if (e.keyCode === KeyCode.SPACE) {
      this.cb.split(this.currentDirection());
    } else if (e.keyCode === KeyCode.KEY_W) {
      this.cb.eject(this.currentDirection());
    }
  };

  private onHide = () => {
    // 切后台停止输入，避免旧方向持续发送
    this.active = false;
  };

  constructor(cb: InputCallbacks) {
    this.cb = cb;
    const vs = view.getVisibleSize();
    this.pointerX = vs.width / 2 + 1;
    this.pointerY = vs.height / 2;
  }

  /** 当前指针对应的移动方向（供分裂/吐球按钮复用）。 */
  currentDirection(): number {
    const vs = view.getVisibleSize();
    return directionFromScreenCenter(vs.width, vs.height, this.pointerX, this.pointerY);
  }

  start(): void {
    input.on(Input.EventType.MOUSE_MOVE, this.onMouseOrTouch);
    input.on(Input.EventType.TOUCH_START, this.onMouseOrTouch);
    input.on(Input.EventType.TOUCH_MOVE, this.onMouseOrTouch);
    input.on(Input.EventType.KEY_DOWN, this.onKey);
    game.on(Game.EVENT_HIDE, this.onHide);
    this.timer = setInterval(() => {
      if (!this.active) return;
      this.cb.move(this.currentDirection());
    }, config.inputSendIntervalMs);
  }

  stop(): void {
    input.off(Input.EventType.MOUSE_MOVE, this.onMouseOrTouch);
    input.off(Input.EventType.TOUCH_START, this.onMouseOrTouch);
    input.off(Input.EventType.TOUCH_MOVE, this.onMouseOrTouch);
    input.off(Input.EventType.KEY_DOWN, this.onKey);
    game.off(Game.EVENT_HIDE, this.onHide);
    if (this.timer !== null) {
      clearInterval(this.timer);
      this.timer = null;
    }
  }
}
