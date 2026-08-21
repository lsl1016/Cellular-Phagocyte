// 联调冒烟测试：复用 Cocos 客户端的引擎无关核心层(core/net)，对真实服务端
// 跑通三类场景：① 完整对局闭环+分裂/吐球 ② 断线重连 ③ 纯函数断言。
//
// 用法：npm test（自动编译 core/net 到 dist-test、构建并拉起 Go 服务端）

import { spawnSync, spawn } from 'node:child_process';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';
import assert from 'node:assert/strict';

const require = createRequire(import.meta.url);
const __dirname = dirname(fileURLToPath(import.meta.url));
const serverDir = resolve(__dirname, '../../server');
const bin = '/tmp/cp-smoke-server-cocos';
const PORT = 18091;
const API = `http://localhost:${PORT}`;
const WS = `ws://localhost:${PORT}/ws`;

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const log = (...a) => console.log('•', ...a);

function buildServer() {
  const r = spawnSync('go', ['build', '-o', bin, './cmd/server'], { cwd: serverDir, encoding: 'utf8' });
  if (r.status !== 0) {
    console.error(r.stderr || r.stdout);
    throw new Error('go build failed');
  }
}

async function waitHealthy(timeoutMs = 10000) {
  const end = Date.now() + timeoutMs;
  while (Date.now() < end) {
    try {
      const r = await fetch(`${API}/healthz`);
      if (r.ok) return;
    } catch {
      /* not up yet */
    }
    await sleep(150);
  }
  throw new Error('server did not become healthy');
}

// 客户端模块（在设置好全局配置后再加载，config.ts 在 import 时解析地址）
let mods;
async function loginAndMatch(deviceId) {
  const api = new mods.ApiService();
  const login = await api.guestLogin(deviceId);
  assert.ok(login.accessToken, '应返回 accessToken');
  api.setToken(login.accessToken);
  const userId = login.user.userId;

  const start = await api.matchStart('classic');
  let matched = null;
  for (let i = 0; i < 30 && !matched; i++) {
    const st = await api.matchStatus(start.matchId);
    if (st.status === 'MATCHED') matched = st;
    else await sleep(200);
  }
  assert.ok(matched && matched.roomId && matched.enterToken, '应匹配成功');
  return { api, userId, roomId: matched.roomId, enterToken: matched.enterToken };
}

async function testFullLoopAndSkills() {
  const { api, userId, roomId, enterToken } = await loginAndMatch('cocos-smoke-skills');
  const { WsClient, GameState, C2S, S2C } = mods;

  const state = new GameState();
  state.reset(userId, roomId);
  const ws = new WsClient();

  let started = false;
  let didSkills = false;
  let snapshots = 0;
  let maxSelfBalls = 1;
  let sawEjected = false;
  let settlement = null;

  const done = new Promise((res, rej) => {
    const timer = setTimeout(() => rej(new Error('技能场景超时未结算')), 25000);
    ws.on(S2C.ENTER_ROOM_RESULT, (d) => {
      assert.ok(d.success, '入房应成功');
      ws.send(C2S.READY, { roomId, userId });
    });
    ws.on(S2C.GAME_START, () => (started = true));
    ws.on(S2C.ROOM_SNAPSHOT, (d) => {
      snapshots++;
      state.applySnapshot(d);
      const self = state.players.get(userId);
      if (self) maxSelfBalls = Math.max(maxSelfBalls, self.balls.length);
      if (state.ejected.size > 0) sawEjected = true;
      if (started && !didSkills) {
        didSkills = true;
        ws.send(C2S.SPLIT, { direction: 0 });
        ws.send(C2S.EJECT, { direction: Math.PI });
      }
    });
    ws.on(S2C.SETTLEMENT_RESULT, (d) => {
      settlement = d;
      clearTimeout(timer);
      res();
    });
  });

  await ws.connect(WS);
  ws.send(C2S.ENTER_ROOM, { roomId, userId, enterToken });
  await done;
  ws.close();

  assert.ok(snapshots > 0, '应收到快照');
  assert.ok(state.foods.size > 0, '快照应含食物');
  assert.ok(maxSelfBalls >= 2, `分裂后分身数应>=2, got ${maxSelfBalls}`);
  assert.ok(sawEjected, '应观察到吐出物');
  assert.equal(settlement.status, 'SUCCESS', '结算应成功');
  assert.ok(settlement.coinReward > 0, '应有金币奖励');
  log('闭环+技能通过', `快照 ${snapshots}`, `最大分身 ${maxSelfBalls}`, `金币+${settlement.coinReward}`);

  // 资产入账校验
  const assets = await api.getAssets();
  assert.equal(assets.coin, settlement.coinReward, '金币应入账');
  log('资产校验通过', `金币 ${assets.coin}`, `经验 ${assets.exp}`);

  // 战绩写入校验
  const records = await api.records(1, 10);
  assert.ok(records.total >= 1, '应至少有一条战绩');
  assert.equal(records.list[0].roomId, roomId, '最近战绩应为本局');
  assert.equal(records.list[0].finalScore, settlement.finalScore, '战绩分数应与结算一致');
  const summary = await api.recordSummary();
  assert.ok(summary.totalGames >= 1, '统计总场次应>=1');
  log('战绩校验通过', `总场次 ${summary.totalGames}`, `本局名次 ${records.list[0].rank}`);

  // 排行榜更新校验
  const board = await api.ranks('daily', 1, 50);
  const mine = board.list.find((e) => e.userId === userId);
  assert.ok(mine && mine.score > 0, '日榜应包含本人且分值>0');
  const rankMe = await api.rankMe('daily');
  assert.ok(rankMe.onRank && rankMe.rank !== null, '本人应已上日榜');
  log('排行榜校验通过', `日榜分 ${mine.score}`, `名次 ${rankMe.rank}`);
}

async function testReconnect() {
  const { userId, roomId, enterToken } = await loginAndMatch('cocos-smoke-recon');
  const { WsClient, C2S, S2C } = mods;

  const ws1 = new WsClient();
  let reconToken = '';
  const startedP = new Promise((res) => {
    ws1.on(S2C.ENTER_ROOM_RESULT, (d) => {
      assert.ok(d.success, '入房应成功');
      reconToken = d.reconnectToken;
      ws1.send(C2S.READY, { roomId, userId });
    });
    ws1.on(S2C.GAME_START, () => res());
  });
  await ws1.connect(WS);
  ws1.send(C2S.ENTER_ROOM, { roomId, userId, enterToken });
  await startedP;
  assert.ok(reconToken, '应下发 reconnectToken');

  await sleep(300);
  ws1.close(); // 模拟断线
  await sleep(400);

  // 用 reconnectToken 重连
  const ws2 = new WsClient();
  let reconOk = false;
  let recovered = false;
  let settlement = null;
  const recon = new Promise((res, rej) => {
    const timer = setTimeout(() => rej(new Error('重连后超时未结算')), 20000);
    ws2.on(S2C.RECONNECT_RESULT, (d) => {
      reconOk = d.success;
      if (!d.success) {
        clearTimeout(timer);
        rej(new Error('重连失败: ' + d.message));
      }
    });
    ws2.on(S2C.ROOM_RECOVER_SNAPSHOT, () => (recovered = true));
    ws2.on(S2C.SETTLEMENT_RESULT, (d) => {
      settlement = d;
      clearTimeout(timer);
      res();
    });
  });
  await ws2.connect(WS);
  ws2.send(C2S.RECONNECT, { roomId, userId, reconnectToken: reconToken });
  await recon;
  ws2.close();

  assert.ok(reconOk, '重连应成功');
  assert.ok(recovered, '应收到 ROOM_RECOVER_SNAPSHOT');
  assert.ok(settlement && settlement.status === 'SUCCESS', '重连后应能正常结算');
  log('断线重连通过', `恢复快照=${recovered}`, `结算名次 ${settlement.rank}/${settlement.totalPlayers}`);
}

async function run() {
  buildServer();
  const srv = spawn(bin, [], {
    env: {
      ...process.env,
      HTTP_ADDR: `:${PORT}`,
      GAME_BATTLE_SECONDS: '6',
      GAME_COUNTDOWN_SECONDS: '0',
      GAME_BOTS: '3',
      GAME_INIT_MASS: '100',
      MATCH_MIN_PLAYERS: '1',
      MATCH_MAX_WAIT_SECONDS: '1',
    },
    stdio: 'ignore',
  });

  try {
    await waitHealthy();
    globalThis.CP_API_BASE = API;
    globalThis.CP_WS_URL = WS;

    // 加载编译产物（CJS）：引擎无关核心层
    const api = require('../dist-test/net/api.js');
    const ws = require('../dist-test/net/ws.js');
    const state = require('../dist-test/core/state/game-state.js');
    const msg = require('../dist-test/core/protocol/messages.js');
    const math = require('../dist-test/core/math.js');
    mods = {
      ApiService: api.ApiService,
      WsClient: ws.WsClient,
      GameState: state.GameState,
      C2S: msg.C2S,
      S2C: msg.S2C,
    };

    // 纯函数断言（Cocos UI 坐标 Y 向上，已翻转语义：+PI/2=屏幕下方/服务端 +Y）
    const d = math.directionFromScreenCenter;
    assert.ok(Math.abs(d(100, 100, 50, 0) - Math.PI / 2) < 1e-6, '指针在屏幕底端(y=0)应为 +PI/2');
    assert.ok(Math.abs(d(100, 100, 50, 100) + Math.PI / 2) < 1e-6, '指针在屏幕顶端应为 -PI/2');
    assert.ok(Math.abs(d(100, 100, 100, 50)) < 1e-6, '指针在右侧应为 0');
    assert.ok(math.zoomForMass(80, 20, 4) < math.zoomForMass(20, 20, 4), '质量越大缩放应越小');
    assert.ok(Math.abs(math.frameRateAdjusted(0.3, 1 / 60) - 0.3) < 1e-9, '60fps 下插值系数应等于基准值');
    const c = math.clampCameraToMap;
    assert.deepEqual(c(-500, 2000, 1, 1280, 720, 4000, 4000), [640, 2000], '左边界应 clamp 到 halfW');
    assert.deepEqual(c(2000, -500, 1, 1280, 720, 4000, 4000), [2000, 360], '下边界应 clamp 到 halfH');
    assert.deepEqual(c(2000, 2000, 0.1, 1280, 720, 4000, 4000), [2000, 2000], '可见范围大于地图时应居中');
    log('纯函数断言通过（方向 / 缩放 / 帧率换算 / 相机边界）');

    await testFullLoopAndSkills();
    await testReconnect();

    console.log('\n✅ Cocos 客户端核心层冒烟测试全部通过（闭环 / 分裂吐球 / 断线重连）');
  } finally {
    srv.kill('SIGKILL');
  }
}

run().catch((e) => {
  console.error('\n❌ 测试失败:', e.message);
  process.exit(1);
});
