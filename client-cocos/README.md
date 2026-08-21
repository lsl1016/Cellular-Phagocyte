# 吞噬细胞 · Cocos Creator 客户端

Cocos Creator 3.8.x + TypeScript 实现，从 Canvas 2D 客户端（`../client`）升级而来。
协议/状态/网络层与旧实现保持一致，渲染/UI/输入层全部重写为 Cocos 节点系统。

> 迁移计划、里程碑完成情况、踩坑记录见 [MIGRATION.md](./MIGRATION.md)。

## 架构

```
assets/scripts/
├── core/          # 引擎无关层（可被 Node 测试复用）
│   ├── config / logger / storage
│   ├── protocol/  # WS + HTTP 协议模型（与旧客户端同源）
│   └── state/     # GameState（服务端快照为准）
├── net/           # HttpClient(fetch优先/XHR兜底) / ApiService / WsClient
├── app/           # ScreenCtx / SceneManager（单场景面板式导航）
├── ui/            # 主题 / 程序化纹理 / UI 构建器 / Toast
├── battle/        # BattleManager / EntityManager / 节点池 / 相机 / 输入
├── screens/       # login / lobby / match / game / settlement / records / rank
└── Boot.ts        # 挂在 Main.scene Canvas 上的启动组件
```

## 关键设计

- **单场景 + 面板导航**：Main.scene 只有一个 Boot 组件，所有屏幕由 SceneManager
  以 mount/unmount 面板方式切换，不直接使用 `director.loadScene`；
- **程序化实体渲染**：球/食物/吐出物共享一张运行时生成的白色圆纹理，
  Sprite tint 染色，可自动合批；无外部美术资源依赖；
- **对象池**：EntityManager 维护节点池，快照 diff 创建/回收；
- **坐标转换**：服务端世界 Y 向下、Cocos Y 向上，统一在
  `WorldCamera.apply` 与实体布点处做 `y' = H - y` 翻转；
- **插值**：与旧版相同的 lerp 语义（球体 0.3 / 相机 0.15 @60fps），
  按 `dt` 做帧率无关换算；
- **裁剪**：可见矩形外的实体 `active=false`；
- **服务端权威**：客户端不做任何碰撞/吞噬/排名判定。

## 运行

1. 启动服务端：
```bash
cd ../server
go run ./cmd/server   # :8080
```

2. 用 Cocos Creator 3.8.8 打开本工程，打开 `assets/scenes/Main.scene`，
   点击预览（浏览器）。

服务端地址默认 `http://localhost:8080`（H5 预览页 query 可用 `?api=...` 覆盖）。

## 操作

- 移动：鼠标/手指，球体朝指针方向前进；
- 分裂：`Space` 或右下按钮；
- 吐球：`W` 或右下按钮；
- 断线自动重连（带退避与遮罩提示）。

## 类型检查

```bash
npx tsc -p tsconfig.json --noEmit
```

## 冒烟测试

```bash
npm test
```

自动执行：
1. 把引擎无关核心层（`core/` + `net/`）编译为 CommonJS（`dist-test/`）；
2. 构建并拉起 Go 服务端（快配置：6 秒对局、3 个机器人）；
3. 用 Cocos 客户端自身的 `ApiService` / `WsClient` / `GameState` 跑通：
   - **纯函数断言**：方向换算（Y 翻转语义）/ 质量缩放 / 帧率无关插值系数；
   - **闭环 + 技能**：入房、快照、食物、分裂（分身 ≥2）、吐出物、结算、
     金币入账、战绩写入、排行榜更新；
   - **断线重连**：断开后用 reconnectToken 重连、收到恢复快照、正常结算。

> 游戏内 UI/渲染/输入层依赖 Cocos 引擎，不在 Node 冒烟范围；由编辑器预览
> 手动回归（登录 → 大厅 → 匹配 → 对战 → 结算 → 战绩/排行榜）。
