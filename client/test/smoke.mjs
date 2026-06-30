// 联调冒烟测试：用客户端自身的网络层(api/ws)+状态层(GameState)，对真实服务端
// 跑通三类场景：① 完整对局闭环 ② 分裂/吐球 ③ 断线重连。
//
// 用法：node test/smoke.mjs（自动编译并拉起 Go 服务端）

import { spawnSync, spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';
import assert from 'node:assert/strict';

const __dirname = dirname(fileURLToPath(import.meta.url));
const serverDir = resolve(__dirname, '../../server');
const bin = '/tmp/cp-smoke-server';
const PORT = 18090;
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

// 动态导入的客户端模块（在设置好全局配置后再加载）
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
  const { api, userId, roomId, enterToken } = await loginAndMatch('smoke-skills');
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
  const { userId, roomId, enterToken } = await loginAndMatch('smoke-recon');
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

    const [api, ws, state, msg, input, camera] = await Promise.all([
      import('../public/dist/network/api.js'),
      import('../public/dist/network/ws.js'),
      import('../public/dist/state/game-state.js'),
      import('../public/dist/protocol/messages.js'),
      import('../public/dist/battle/input.js'),
      import('../public/dist/battle/camera.js'),
    ]);
    mods = {
      ApiService: api.ApiService,
      WsClient: ws.WsClient,
      GameState: state.GameState,
      C2S: msg.C2S,
      S2C: msg.S2C,
    };

    // 纯函数断言
    assert.ok(Math.abs(input.directionFromScreenCenter(100, 100, 50, 0) + Math.PI / 2) < 1e-6, '向上应为 -PI/2');
    assert.ok(camera.zoomForMass(80, 20, 4) < camera.zoomForMass(20, 20, 4), '质量越大缩放应越小');

    await testFullLoopAndSkills();
    await testReconnect();

    console.log('\n✅ 全部联调冒烟测试通过（闭环 / 分裂吐球 / 断线重连）');
  } finally {
    srv.kill('SIGKILL');
  }
}

run().catch((e) => {
  console.error('\n❌ 测试失败:', e.message);
  process.exit(1);
});
