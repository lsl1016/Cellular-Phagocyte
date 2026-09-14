import assert from 'node:assert/strict';
import { SnapshotBuffer } from '../dist-test/core/state/snapshot-buffer.js';

function snapshot(serverTime, balls = [], ejected = []) {
  return {
    roomId: 'r1',
    snapshotType: 'FULL',
    tickSeq: serverTime,
    serverTime,
    players: [{
      userId: 'u2',
      nickname: 'remote',
      status: 'PLAYING',
      score: 0,
      mass: balls.reduce((sum, b) => sum + b.mass, 0),
      balls,
    }],
    foods: [],
    ejected,
    events: [],
  };
}

function ball(ballId, x, y, radius = 10, mass = 20) {
  return { ballId, x, y, radius, mass };
}

{
  const buf = new SnapshotBuffer(150, 8);
  assert.equal(buf.push(snapshot(1000, [ball('b1', 0, 0, 10, 20)]), 2000), true);
  assert.equal(buf.push(snapshot(1100, [ball('b1', 100, 50, 20, 40)]), 2100), true);

  // latest server=1100, elapsed=100, delay=150 => render target=1050 => 两帧正中间。
  const sample = buf.sample(2200);
  assert.ok(sample, '应能采样时间轴');
  assert.equal(sample.alpha, 0.5);
  const m = sample.ball('b1');
  assert.ok(m, '远端球应存在');
  assert.equal(m.x, 50);
  assert.equal(m.y, 25);
  assert.equal(m.radius, 15);
  assert.equal(m.mass, 30);
}

{
  const buf = new SnapshotBuffer(150, 8);
  buf.push(snapshot(1000, [ball('old', 0, 0)]), 1000);
  buf.push(snapshot(1100, [ball('old', 100, 0), ball('new', 10, 10)]), 1100);
  const sample = buf.sample(1200); // target=1050
  assert.ok(sample);
  assert.equal(sample.ball('new'), undefined, '新实体不能在其服务端帧时间之前提前出现');
}

{
  const buf = new SnapshotBuffer(100, 8);
  buf.push(snapshot(1000, [ball('gone', 25, 30)]), 1000);
  buf.push(snapshot(1100, []), 1100);

  const beforeLeave = buf.sample(1150); // target=1050，仍在上一帧与离开帧之间
  assert.ok(beforeLeave);
  assert.ok(beforeLeave.ball('gone'), '远端实体不能在渲染时间轴到达离开帧前提前消失');

  const afterLeave = buf.sample(1200); // target=1100，正式跨过离开帧
  assert.ok(afterLeave);
  assert.equal(afterLeave.ball('gone'), undefined, '渲染时间轴到达离开帧后应移除实体');
}

{
  const buf = new SnapshotBuffer(150, 8);
  buf.push(snapshot(1000, [ball('b1', 0, 0)]), 1000);
  buf.push(snapshot(1100, [ball('b1', 100, 0)]), 1100);
  assert.equal(buf.push(snapshot(1050, [ball('b1', 999, 0)]), 1150), false, '乱序快照应丢弃');
  assert.equal(buf.size, 2);

  // 长时间没有新包时不做速度外推，停在最新权威位置。
  const sample = buf.sample(5000);
  assert.ok(sample);
  assert.equal(sample.ball('b1')?.x, 100);
}

{
  const buf = new SnapshotBuffer(100, 8);
  const e1 = [{ ejectId: 'e1', ownerId: 'u2', x: 0, y: 0, radius: 5, mass: 5 }];
  const e2 = [{ ejectId: 'e1', ownerId: 'u2', x: 40, y: 20, radius: 7, mass: 5 }];
  buf.push(snapshot(1000, [], e1), 1000);
  buf.push(snapshot(1100, [], e2), 1100);
  const sample = buf.sample(1150); // target=1050
  assert.ok(sample);
  const e = sample.ejected('e1');
  assert.ok(e);
  assert.equal(e.x, 20);
  assert.equal(e.y, 10);
  assert.equal(e.radius, 6);

  buf.reset();
  assert.equal(buf.size, 0);
  assert.equal(buf.sample(2000), null);
}

{
  const buf = new SnapshotBuffer(100, 8);
  const e = [{ ejectId: 'gone-e', ownerId: 'u2', x: 5, y: 8, radius: 5, mass: 5 }];
  buf.push(snapshot(1000, [], e), 1000);
  buf.push(snapshot(1100, [], []), 1100);
  assert.ok(buf.sample(1150)?.ejected('gone-e'), '吐出物也应保留到其离开帧时间');
  assert.equal(buf.sample(1200)?.ejected('gone-e'), undefined, '吐出物跨过离开帧后应消失');
}

console.log('✅ SnapshotBuffer 时间轴插值测试通过');
