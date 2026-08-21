# Canvas 2D → Cocos Creator 迁移计划与实现情况

| 项目 | 内容 |
| --- | --- |
| 分支 | `cocos` |
| 日期 | 2026-08-22 |
| 源实现 | `../client`（原生 TypeScript + Canvas 2D，P0+P1+P2） |
| 目标实现 | 本工程（Cocos Creator 3.8.8 + TypeScript） |
| 迁移目标 | 功能对齐旧 Canvas 客户端全部能力，渲染/UI/输入层换成 Cocos 节点系统 |

------

# 1. 迁移前关键决策

| 决策点 | 采用方案 | 理由 |
| --- | --- | --- |
| Cocos 版本 | Cocos Creator **3.8.8**（Empty 2D 模板） | 项目文档按 3.x 设计；3.8 为当前 LTS |
| 工程位置 | 新建 `client-cocos/`，旧 `client/` 暂时保留对照 | 迁移期间随时 diff、回退 |
| 实体节点创建 | **首版代码动态创建 + 单张白色圆纹理 tint 上色** | 不依赖编辑器手摆资源；颜色逻辑与旧版 `colorFor(userId)` 一致 |
| 构建目标 | 先只保证 **Web Mobile H5** | 与现有联调方式一致（服务端 `:8080`，已开 CORS） |
| 屏幕组织 | **单场景 + 面板式导航**（不使用 `director.loadScene`） | 与旧版 SceneManager 行为一致；避免多场景/编辑器元数据依赖 |

------

# 2. 代码复用映射（迁移依据）

| 旧 client 文件 | 迁移策略 | 新位置 |
| --- | --- | --- |
| `protocol/messages.ts`、`http-models.ts` | ✅ 原样迁移（去 ESM `.js` 后缀） | `core/protocol/` |
| `state/game-state.ts` | ✅ 原样迁移 | `core/state/` |
| `common/logger.ts` | ✅ 原样迁移 | `core/logger.ts` |
| `app/config.ts` | 🔧 小改：原生端无 `location`，读全局覆盖兜底 | `core/config.ts` |
| `common/storage.ts` | 🔧 小改：KV 适配器注入（`sys.localStorage`） | `core/storage.ts` |
| `network/http.ts` | 🔧 双传输：优先 `fetch`，无 fetch 回退 XHR | `net/http.ts` |
| `network/api.ts` | ✅ 基本原样 | `net/api.ts` |
| `network/ws.ts` | 🔧 去掉 DOM 类型依赖（自建 WSLike 接口） | `net/ws.ts` |
| `battle/camera.ts`（zoomForMass） | 🔅 拆为纯函数 | `core/math.ts` |
| `battle/input.ts`（方向纯函数） | 🔅 拆为纯函数（Y 翻转语义） | `core/math.ts` |
| `battle/renderer.ts` | ❌ 废弃 → 重写 | `battle/entity-manager.ts` |
| `battle/battle-manager.ts` | 🔧 保留协议/生命周期骨架，渲染输入换 Cocos | `battle/battle-manager.ts` |
| `scenes/*`、`ui/dom.ts`、`main.ts` | ❌ DOM → Cocos 重建 | `screens/*`、`ui/*`、`Boot.ts` |

------

# 3. 工程结构

```
client-cocos/
├── assets/
│   ├── scenes/Main.scene      # 唯一场景：Canvas + Boot 组件
│   └── scripts/
│       ├── core/              # 引擎无关层（Node 可测）
│       │   ├── config / logger / storage / math(纯函数)
│       │   ├── protocol/      # WS + HTTP 协议模型
│       │   └── state/         # GameState
│       ├── net/               # HttpClient(双传输) / ApiService / WsClient
│       ├── app/               # ScreenCtx / SceneManager
│       ├── ui/                # theme / texgen / builder / toast
│       ├── battle/            # battle-manager / entity-manager / object-pool / camera / input
│       ├── screens/           # login / lobby / match / game / settlement / records / rank
│       └── Boot.ts            # 启动组件（构建 ScreenRoot → 注册屏幕 → 进登录）
├── test/smoke.mjs             # 联调冒烟测试（复用 core/net 编译产物）
├── tsconfig.json              # Cocos 编译配置
└── tsconfig.test.json         # core+net → CJS（供 Node 测试）
```

**GameScene 世界节点树**（对战屏挂载）：

```
game-screen
├── WorldRoot（相机作用对象：position/scale 受 WorldCamera 控制）
│   ├── Grid（Graphics：网格 + 地图边界，静态一次绘制）
│   ├── FoodLayer / EjectedLayer / PlayerLayer
└── UIRoot（HUD：质量/倒计时/排行榜、技能按钮、状态遮罩）
```

------

# 4. 里程碑计划与完成情况

## M1：工程脚手架 + 核心层迁移 ✅ 完成

| 计划项 | 状态 | 说明 |
| --- | --- | --- |
| 处理脏文件、新建工程 | ✅ | 移除 Dashboard 自动创建的嵌套 `.git` |
| 横屏 1280×720 适配 | ✅ | `settings/v2/packages/project.json` fitHeight |
| 启用 websocket 引擎模块 | ✅ | `engine.json` 默认关闭，需手动开启 |
| 迁移 core + net | ✅ | 9 个文件，去 ESM 后缀 |
| Cocos 内编译通过 | ✅ | `tsc --noEmit` 零错误；编辑器导入无报错 |
| 验收：游客登录 HTTP | ✅ | 冒烟测试断言 `guestLogin` |

## M2：应用框架 + 登录/大厅/匹配 ✅ 完成

| 计划项 | 状态 | 说明 |
| --- | --- | --- |
| AppContext（会话/token/导航） | ✅ | `app/context.ts` |
| SceneManager 封装 | ✅ | 面板式 mount/unmount，逐帧驱动 |
| Login → Lobby → Match | ✅ | 匹配轮询 1s、取消匹配、资产刷新 |
| 验收：预览到匹配成功 | ✅ | 编辑器浏览器预览人工确认 |

## M3：对战场景核心 ✅ 完成

| 计划项 | 状态 | 说明 |
| --- | --- | --- |
| 分层节点树 | ✅ | WorldRoot(Grid/Food/Ejected/Player) + HUD |
| 坐标系适配 | ✅ | 服务端 Y 向下 vs Cocos Y 向上，统一 `y'=H−y`，只在相机 apply 与实体布点两处翻转 |
| EntityManager 快照 diff + 节点池 | ✅ | 池按类型（球/简单实体）复用，回收清状态 |
| 插值平滑 | ✅ | 球体 0.3 / 相机 0.15 @60fps，`frameRateAdjusted(dt)` 帧率无关 |
| 相机跟随 + zoomForMass + lerp | ✅ | `zoomForMass` 沿用旧版公式；基准 scale 1.6（设计宽 1280 可见 800 世界单位） |
| 相机地图边界 clamp | ✅ | `clampCameraToMap`：边缘不越界；可见范围大于整图时居中（含单测） |
| 视野裁剪 | ✅ | visibleRect + margin，出视野 `active=false` |
| HUD | ✅ | 质量/得分/倒计时/局内排行（500ms 刷新） |
| 输入系统 | ✅ | 指针/触摸方向 + Space/W + 技能按钮；切后台停发 |
| 按质量排序渲染 | ✅ | 顺序变化时才更新 siblingIndex |
| 验收：完整对局可玩 | ✅ | 预览人工确认 |

## M4：对局生命周期 + 断线重连 ✅ 完成

| 计划项 | 状态 | 说明 |
| --- | --- | --- |
| BattleManager 迁移 | ✅ | 入房/准备/倒计时/开始/结束/结算回调全保留 |
| 退避重连 + recoverToken | ✅ | 退避数组与旧版一致；**机器验证** |
| 恢复快照全量重建 | ✅ | `firstFrame=true` 重置 + EntityManager diff 重建；**机器验证** |
| 切后台停输入 | ✅ | `Game.EVENT_HIDE` → active=false |

## M5：外围页面 ✅ 完成

| 页面 | 状态 | 功能 |
| --- | --- | --- |
| Settlement | ✅ | 名次/得分/质量/吞噬/奖励 + 再来一局/返回大厅 |
| Records | ✅ | 分页 8 条/页 + 统计概览 |
| Rank | ✅ | 日榜/周榜/最高分 Tab 切换 + 自身名次 |

## M6：测试与收尾 🔶 部分完成

| 计划项 | 状态 | 说明 |
| --- | --- | --- |
| 冒烟测试改造（复用 Cocos core） | ✅ | `npm test` 全绿（见下） |
| README 更新 | ✅ | 运行/操作/测试说明 |
| 里程碑 commit | ✅ | 9 个（见提交记录） |
| 手动回归清单 | ⬜ 待做 | 需覆盖 7 个屏幕逐项打勾 |
| 性能压测 | ⬜ 待做 | 假玩家/池命中率/同屏 Label/帧率 |
| 删除旧 `client/` | ⬜ 保留中 | 等手动回归确认后再删 |

------

# 5. 验证情况

## 5.1 机器验证（`npm test`，全部通过）

```
• 纯函数断言通过（方向 / 缩放 / 帧率换算 / 相机边界）
• 闭环+技能通过 快照 60 最大分身 2 金币+350
• 资产校验通过 金币 350 经验 231
• 战绩校验通过 总场次 1 本局名次 1
• 排行榜校验通过 日榜分 1100 名次 1
• 断线重连通过 恢复快照=true 结算名次 1/4
✅ Cocos 客户端核心层冒烟测试全部通过
```

测试链路：`core+net → tsc(CJS) → dist-test` → 拉起真实 Go 服务端
（快配置：6 秒对局 / 3 机器人 / 初始质量 100）→ 用 Cocos 客户端自身的
`ApiService`/`WsClient`/`GameState` 断言。

## 5.2 人工验证（编辑器预览）

登录 → 大厅 → 匹配 → 对战（移动/吃食物/分裂/吐球/排行榜/HUD）→ 结算 →
断线重连，均已在预览中确认。

## 5.3 验证边界

游戏内 UI/渲染/输入层依赖 Cocos 引擎，**不在 Node 冒烟范围**，由编辑器
预览手动回归覆盖。

------

# 6. 与旧 Canvas 版的实现差异（要点）

| 维度 | 旧 Canvas 2D | 新 Cocos |
| --- | --- | --- |
| 渲染模型 | 立即模式，每帧清屏手动重画 | 保留模式节点树，快照只更新 transform |
| 实体渲染 | `ctx.arc` 每帧画圆 | 单张白色圆纹理 + Sprite tint，自动合批 |
| 纹理 | 无 | 运行时程序化生成（`texgen.ts`），零美术资源 |
| 对象池 | 无（README 标注未实现） | NodePool 按类型复用 |
| UI | HTML DOM + CSS | 代码构建 Cocos UI（builder.ts） |
| 场景管理 | 自研 SceneManager | 自研 SceneManager（面板式，行为一致） |
| 相机 | 手写 worldToScreen 数学 | WorldRoot 节点 position/scale（数学等价） |
| 相机边界 clamp | 无 | 有（本次迁移新增） |
| 输入 | DOM pointer/key 事件 | Cocos `input`/`Game.EVENT_HIDE` |
| 渲染循环 | `requestAnimationFrame` | Boot 组件 `update(dt)` 驱动 |
| 插值 | 每帧固定 0.3/0.15 | 相同语义，帧率无关换算 |

------

# 7. 迁移过程中踩过的坑（重要经验）

1. **场景自定义组件必须用压缩 UUID 序列化**
   场景 JSON 中自定义组件的 `__type__` 不是类名，而是脚本资产 UUID 的
   压缩格式（前 5 位 hex + 27 hex → 18 个 base64 字符，共 23 字符）。
   写成 `"Boot"` 会报 `Missing class`，组件被丢弃，预览黑屏。

2. **运行时内存纹理必须禁用动态合图**
   `Uint8Array` 生成的纹理被 DynamicAtlas 打包时，
   `texSubImage2D` 抛 `Overload resolution failed`。
   生成 SpriteFrame 后设 `sf.packable = false`。
   所有实体本就共享同纹理+同材质，不参与动态合图也能自动合批。

3. **Y 轴方向**
   服务端世界 Y 向下（屏幕习惯），Cocos 节点 Y 向上。必须集中翻转
   （`y' = H − y`），绝不散落在各 View。输入方向同理：UI 坐标求
   `atan2` 时对 dy 取负。

4. **Widget 对齐用 `isAlign*` 布尔 setter**
   直接赋 `alignFlags` 位掩码在 3.8 类型上不可用；设置 `isAlignTop` 等
   会自动更新 flags。

5. **引擎模块默认配置**
   Empty 2D 模板的 `websocket` 模块默认关闭，需在 `engine.json` 开启
   （H5 浏览器不受影响，原生端必需）。

6. **Cocos TS 不支持 ESM `.js` import 后缀**
   迁移时统一去掉（这也是 test 需单独 CJS 编译的原因之一）。

------

# 8. 遗留事项

- [ ] 手动回归清单（7 屏逐项）
- [ ] 性能压测（假玩家、池命中率、同屏 Label 数、帧率）
- [ ] 回归确认后删除旧 `client/`（或长期保留为参考实现）
- [ ] 合并 `cocos` → `main`
- [ ] 后续增强（来自旧版"暂未实现"清单）：SnapshotBuffer 完整插值、
      小地图、Protobuf

------

# 9. 提交记录

```
656290c feat: 相机 clamp 到地图边界（补齐 M3 计划项）
d6bec1c chore: 修复 .gitignore 追加换行；提交 go build 维护的 go.sum
9db662b chore: 忽略测试编译产物 dist-test
c93cf2d test: 冒烟测试改为复用 Cocos 工程核心层
b94ffb7 fix: 运行时生成的纹理禁用动态合图
bdb487d fix: 场景中 Boot 组件改用压缩 UUID 序列化
f2a1105 feat: 初始化 Cocos Creator 客户端（M1-M5 全量迁移）
```
