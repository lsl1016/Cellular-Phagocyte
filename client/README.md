# 吞噬细胞 · H5 Canvas 客户端（P0 最小闭环）

原生 TypeScript + Canvas 2D 实现，不依赖 Cocos，可直接在浏览器与 `../server` 联调。
覆盖完整闭环：**游客登录 → 大厅 → 匹配 → 入房/准备 → 摇杆移动 → 接收快照渲染玩家与食物 → 局内排行榜 → 结算 → 返回大厅**。

## 目录结构

```
client/
├── public/
│   ├── index.html        # 入口页面（加载 dist/main.js）
│   ├── style.css         # UI 样式
│   └── dist/             # tsc 编译产物（构建后生成）
├── src/
│   ├── app/              # config 运行时配置 / context 场景上下文
│   ├── common/           # logger / storage / events
│   ├── protocol/         # messages(WS) / http-models（与服务端对齐）
│   ├── network/          # HttpClient / ApiService / WsClient（含分发+心跳）
│   ├── state/            # GameState（以服务端快照为准）
│   ├── battle/           # input / camera / renderer / battle-manager
│   ├── scenes/           # login / lobby / match / game / settlement + scene-manager
│   └── main.ts           # 入口装配
└── test/smoke.mjs        # Node 联调冒烟测试（拉起服务端跑通全流程）
```

## 运行

### 1. 启动服务端
```bash
cd ../server
go run ./cmd/server          # 监听 :8080
```

### 2. 构建并启动客户端
```bash
cd client
npm install                  # 安装 typescript
npm run build                # tsc 编译到 public/dist
npm run serve                # 静态服务 public/ 于 http://localhost:5173
```
浏览器打开 <http://localhost:5173>，点击「游客登录」即可开始。

> 客户端默认调用 `http://<当前host>:8080`。服务端已开启 CORS，跨端口无碍。
> 如需指定服务端地址：`http://localhost:5173/?api=http://192.168.1.10:8080`。

开发时可用 `npm run watch` 让 tsc 持续编译。

## 操作

- **移动**：鼠标/手指移动，球体朝指针方向前进（方向 = 屏幕中心 → 指针）。
- **分裂**：`Space` 或右下角「分裂」按钮（朝当前指针方向）。
- **吐球**：`W` 或右下角「吐球」按钮。
- 右上角为局内排行榜，顶部中间为剩余时间，左上角为质量/得分。
- 断线会自动重连（带退避与「正在重连」遮罩），恢复后继续对局。
- 对局结束自动进入结算页，可「再来一局」或「返回大厅」。

## 验证

```bash
npm run smoke
```
该脚本会自动 `go build` 并以快配置拉起服务端，用客户端自身的
`ApiService` / `WsClient` / `GameState` 跑通三类场景并断言：
- **闭环 + 技能**：收到快照、食物非空、分裂后分身数≥2、出现吐出物、结算成功、奖励入账；
- **断线重连**：断开后用 `reconnectToken` 重连、收到恢复快照、重连后正常结算；
- 纯函数（方向计算、摄像机缩放）断言。

类型检查：`npx tsc --noEmit`。

## 设计要点

- **客户端不做权威判定**：位置、质量、吞噬、排名、奖励全部以服务端快照/结算为准；客户端只采集输入、做表现。
- **网络/协议/状态层无 DOM 依赖**，因此能在 Node 下被冒烟测试直接复用。
- 渲染对球体位置做轻量插值（向快照目标 lerp），缓解 10Hz 快照跳变；摄像机随自身质量缩放视野。

## 已实现 / 暂未实现

已实现：P0 最小闭环 + P1（**分裂/吐球**、多分身渲染、吐出物、**断线自动重连**）+ P2（**战绩页**分页 + 统计、**排行榜页**日榜/周榜/最高分切换，大厅入口）。

暂未实现：SnapshotBuffer 完整插值、小地图、对象池、Protobuf。
