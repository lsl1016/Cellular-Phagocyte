# 房间系统 PRD + 技术设计文档

## 1. 文档信息

| 项目     | 内容                                                         |
| -------- | ------------------------------------------------------------ |
| 文档名称 | 房间系统 PRD + 技术设计文档                                  |
| 所属产品 | 吞噬细胞                                                     |
| 所属模块 | 房间系统                                                     |
| 上游模块 | 匹配系统                                                     |
| 下游模块 | Game Server、实时对战、结算系统                              |
| 客户端   | Cocos Creator 3.x + TypeScript                               |
| 服务端   | Go                                                           |
| 通信方式 | HTTP + WebSocket                                             |
| 存储依赖 | Redis、MySQL                                                 |
| 文档定位 | 产品需求 + 技术概要设计                                      |
| 详细程度 | 中等详细，后续可继续拆分 Game Server、Tick、同步、结算等详细设计 |

------

# 第一部分：房间系统 PRD

## 2. 模块背景

房间系统负责承接匹配系统的匹配结果，将一批玩家组织成一局游戏，并为该局游戏分配具体的 Game Server 承载对战过程。

在《吞噬细胞》中，房间是一次对局的运行容器。玩家匹配成功后，并不是直接开始实时对战，而是先进入房间。房间系统需要完成以下工作：

1. 根据匹配结果创建房间。
2. 为房间分配 Game Server。
3. 生成房间 ID 和入房凭证。
4. 管理房间状态流转。
5. 等待玩家建立 WebSocket 连接。
6. 等待玩家加载完成并发送 Ready。
7. 在满足开始条件后启动对局。
8. 在对局结束后释放房间资源。
9. 处理玩家入房失败、断线、退出等异常情况。

------

## 3. 模块目标

### 3.1 产品目标

| 目标                 | 说明                                                 |
| -------------------- | ---------------------------------------------------- |
| 稳定承接匹配结果     | 匹配成功后能稳定创建房间                             |
| 快速进入对局         | 玩家拿到房间信息后能尽快连接 Game Server             |
| 状态清晰             | 客户端能明确知道当前处于入房、加载、等待、对战等状态 |
| 支持多人同房         | 目标支持单局最多 100 人                              |
| 支持入房失败处理     | 玩家连接失败、超时、取消等情况有明确处理             |
| 支持房间生命周期管理 | 房间从创建到销毁状态可控                             |
| 支持对局结束衔接     | 对局结束后可以进入结算系统                           |

### 3.2 技术目标

| 目标                 | 说明                                                      |
| -------------------- | --------------------------------------------------------- |
| 房间状态可追踪       | 房间状态需要可查询、可恢复、可排查                        |
| Game Server 分配可控 | 根据可用 Game Server 进行房间分配                         |
| 入房认证安全         | 玩家必须携带有效 enterToken 才能入房                      |
| 房间资源可释放       | 对局结束后及时释放内存和连接资源                          |
| 支持异常恢复         | 玩家未及时入房、断线、Game Server 异常时有处理策略        |
| 与实时对战逻辑解耦   | 房间系统负责房间生命周期，具体战斗逻辑由 Game Server 执行 |

------

## 4. 房间系统定位

房间系统处于匹配系统和实时对战系统之间。

```text
匹配系统
  ↓
房间系统
  ↓
Game Server
  ↓
实时对战
  ↓
结算系统
```

房间系统主要解决“玩家如何从匹配成功过渡到正式对局”的问题。

------

## 5. 用户场景

## 5.1 正常入房开始对局

### 场景描述

玩家匹配成功后，服务端创建房间并分配 Game Server。客户端拿到入房信息后建立 WebSocket 连接，发送入房认证和 Ready 消息。所有必要玩家准备完成后，房间进入对局状态。

### 用户流程

```text
匹配成功
  ↓
服务端创建房间
  ↓
服务端分配 Game Server
  ↓
客户端收到 roomId、wsUrl、enterToken
  ↓
客户端建立 WebSocket 连接
  ↓
客户端发送入房认证
  ↓
客户端加载游戏资源
  ↓
客户端发送 Ready
  ↓
房间进入 RUNNING 状态
  ↓
对局开始
```

------

## 5.2 玩家入房超时

### 场景描述

玩家匹配成功后，由于网络异常、客户端退出、连接失败等原因，未在规定时间内进入房间。

### 处理规则

1. 房间系统为玩家保留短时间入房资格。
2. 超过入房有效期后，玩家状态变更为 ENTER_TIMEOUT。
3. 如果房间人数仍满足最低开局人数，可以继续开局。
4. 如果房间人数不足，则房间取消或补入机器人，后续支持。
5. 客户端再次请求时，提示“入房超时，请重新匹配”。

------

## 5.3 玩家 Ready 超时

### 场景描述

玩家已建立 WebSocket 连接，但迟迟未加载完成或未发送 Ready。

### 处理规则

1. 房间状态保持 LOADING。
2. 服务端记录玩家加载状态。
3. 超过 Ready 超时时间后，将该玩家标记为 NOT_READY_TIMEOUT。
4. 房间可以根据剩余 Ready 玩家数量决定是否开始。
5. 如果 Ready 人数不足最低开局人数，则房间失败并通知已连接玩家。

------

## 5.4 玩家主动退出房间

### 场景描述

玩家入房后，在对局开始前主动退出。

### 处理规则

1. 如果房间尚未进入 RUNNING，可允许退出。
2. 服务端将玩家从房间候选列表中移除。
3. 房间人数重新计算。
4. 如果剩余人数不足最低开局人数，可以等待、取消房间或补位。
5. 如果已经进入 RUNNING，则按对局中退出规则处理，不再属于房间系统主要职责。

------

## 5.5 Game Server 分配失败

### 场景描述

匹配成功后，Room Coordinator 未找到可用 Game Server。

### 处理规则

1. 返回房间创建失败。
2. 匹配系统将玩家状态更新为 FAILED。
3. 客户端展示“服务器繁忙，请稍后重试”。
4. 记录错误日志和监控指标。
5. 后续可支持重新选择 Game Server 或延迟重试。

------

## 6. 功能范围

## 6.1 MVP 版本功能范围

MVP 版本需要实现：

1. 创建房间。
2. 分配 Game Server。
3. 生成 roomId。
4. 生成 enterToken。
5. 返回 wsUrl。
6. 玩家 WebSocket 入房认证。
7. 玩家 Ready 状态管理。
8. 房间状态流转。
9. 开始对局。
10. 房间结束。
11. 房间销毁。
12. 入房超时处理。
13. 基础异常提示。

## 6.2 完整版本功能范围

完整版本可扩展：

1. 多 Game Server 负载分配。
2. Game Server 健康检查。
3. 房间人数动态补位。
4. 机器人补位。
5. 房间恢复。
6. 房间迁移，后续高阶能力。
7. 自定义房间，后续玩法。
8. 好友组队房间，后续玩法。
9. 观战房间，后续玩法。
10. 房间内阶段管理，如开局倒计时、阶段结算。

## 6.3 暂不包含范围

当前房间系统不包含：

1. 游戏 Tick 详细逻辑。
2. 玩家移动同步细节。
3. 碰撞检测细节。
4. AOI 视野同步细节。
5. 局内排行榜计算细节。
6. 战斗数值规则。
7. 最终奖励结算规则。
8. 全服排行榜更新逻辑。

这些内容后续按模块单独设计。

------

## 7. 房间状态设计

## 7.1 房间状态定义

| 状态        | 说明                           |
| ----------- | ------------------------------ |
| CREATED     | 房间已创建，但尚未接收玩家连接 |
| WAITING     | 等待玩家入房                   |
| LOADING     | 玩家正在加载资源或等待 Ready   |
| READY_CHECK | 房间检查玩家 Ready 状态        |
| RUNNING     | 对局进行中                     |
| SETTLING    | 对局结束，正在结算             |
| FINISHED    | 对局已完成                     |
| DESTROYED   | 房间已销毁                     |
| FAILED      | 房间创建或启动失败             |

------

## 7.2 房间状态流转

正常流转：

```text
CREATED
  ↓
WAITING
  ↓
LOADING
  ↓
READY_CHECK
  ↓
RUNNING
  ↓
SETTLING
  ↓
FINISHED
  ↓
DESTROYED
```

异常流转：

```text
CREATED / WAITING / LOADING
  ↓ Game Server 不可用 / 入房人数不足 / Ready 超时
FAILED
  ↓
DESTROYED
```

------

## 8. 玩家房间状态设计

## 8.1 玩家状态定义

| 状态          | 说明                   |
| ------------- | ---------------------- |
| MATCHED       | 已匹配成功，等待入房   |
| CONNECTING    | 正在建立 WebSocket     |
| AUTHENTICATED | WebSocket 入房认证成功 |
| LOADING       | 客户端加载游戏资源     |
| READY         | 客户端已准备完成       |
| PLAYING       | 已进入对局             |
| DISCONNECTED  | 已断线                 |
| EXITED        | 已退出                 |
| ENTER_TIMEOUT | 入房超时               |
| READY_TIMEOUT | Ready 超时             |

------

## 8.2 玩家状态流转

```text
MATCHED
  ↓ 建立 WebSocket
CONNECTING
  ↓ 入房认证成功
AUTHENTICATED
  ↓ 客户端加载资源
LOADING
  ↓ 发送 Ready
READY
  ↓ 房间开始
PLAYING
```

异常流转：

```text
MATCHED
  ↓ 超过入房有效期
ENTER_TIMEOUT
LOADING
  ↓ 超过 Ready 时间
READY_TIMEOUT
AUTHENTICATED / LOADING / READY / PLAYING
  ↓ 连接断开
DISCONNECTED
```

------

## 9. 房间人数规则

## 9.1 基础人数配置

| 配置项                  | 建议值 | 说明           |
| ----------------------- | ------ | -------------- |
| maxPlayers              | 100    | 单局最大人数   |
| minStartPlayers         | 10     | 最低开局人数   |
| standardStartPlayers    | 30     | 标准开局人数   |
| enterRoomTimeoutSeconds | 30     | 入房超时时间   |
| readyTimeoutSeconds     | 20     | Ready 超时时间 |
| startCountdownSeconds   | 3      | 开局倒计时     |

------

## 9.2 开局规则

MVP 阶段建议：

```text
Ready 玩家数 >= minStartPlayers
并且
等待 Ready 时间达到指定条件
即可开局
```

如果房间由匹配系统一次性分配了一批玩家，可以采用：

```text
达到预期 Ready 人数
或
Ready 超时后剩余人数仍 >= minStartPlayers
```

------

## 9.3 人数不足处理

| 场景                 | 处理方式               |
| -------------------- | ---------------------- |
| 入房人数不足         | 等待入房超时           |
| Ready 人数不足       | 继续等待或房间失败     |
| 房间启动前玩家退出   | 重新计算人数           |
| 人数不足最低开局人数 | 房间失败，玩家返回大厅 |
| 后续支持机器人       | 可补入机器人后继续开局 |

------

## 10. 客户端页面需求

## 10.1 入房加载页

匹配成功后，客户端进入入房加载页。

页面展示内容：

1. 当前模式。
2. 房间 ID，开发环境可显示，正式环境可隐藏。
3. 正在连接游戏服务器。
4. 资源加载进度。
5. 当前入房状态。
6. 网络状态。
7. 失败重试按钮，异常时展示。

状态文案：

| 状态     | 文案                     |
| -------- | ------------------------ |
| 正在连接 | 正在连接游戏服务器...    |
| 认证中   | 正在验证入房信息...      |
| 加载中   | 正在加载对战资源...      |
| 等待玩家 | 正在等待其他玩家准备...  |
| 即将开始 | 对局即将开始...          |
| 入房失败 | 进入房间失败，请重新匹配 |

------

## 10.2 开局倒计时

当房间满足开始条件后，客户端展示倒计时。

文案：

```text
3
2
1
开始！
```

倒计时期间：

1. 禁止玩家提前移动。
2. 可以展示玩家出生点。
3. 可以展示地图环境。
4. 服务端在倒计时结束后正式进入 RUNNING。

------

## 10.3 入房失败页面

当入房失败时，客户端展示：

1. 失败原因。
2. 返回大厅按钮。
3. 重新匹配按钮。

常见失败原因：

| 原因            | 文案                       |
| --------------- | -------------------------- |
| enterToken 过期 | 入房凭证已过期，请重新匹配 |
| 房间不存在      | 房间已失效，请重新匹配     |
| 服务器不可用    | 游戏服务器繁忙，请稍后再试 |
| 网络异常        | 网络异常，请检查连接       |
| 房间已开始      | 对局已开始，请重新匹配     |

------

## 11. 产品验收标准

## 11.1 MVP 验收标准

| 验收项           | 标准                                             |
| ---------------- | ------------------------------------------------ |
| 创建房间         | 匹配成功后可以创建房间                           |
| 分配 Game Server | 房间可以被分配到可用 Game Server                 |
| 返回入房信息     | 客户端可拿到 roomId、serverId、wsUrl、enterToken |
| WebSocket 入房   | 客户端可通过 WebSocket 进入房间                  |
| 入房认证         | 服务端能校验 enterToken                          |
| Ready 管理       | 客户端发送 Ready 后服务端记录状态                |
| 房间开始         | Ready 人数满足条件后进入 RUNNING                 |
| 入房超时         | 玩家未连接时会超时处理                           |
| Ready 超时       | 玩家连接但未 Ready 时会超时处理                  |
| 房间结束         | 对局结束后房间进入 FINISHED                      |
| 房间销毁         | 房间资源可释放                                   |
| 异常提示         | 入房失败、房间失效等有明确提示                   |

------

# 第二部分：房间系统技术设计

## 12. 总体架构

## 12.1 模块关系

```text
Match Server
  ↓ 创建房间请求
Room Coordinator
  ↓ 分配房间
Game Server
  ↓ WebSocket 入房
Cocos Client
```

涉及模块：

| 模块             | 职责                                         |
| ---------------- | -------------------------------------------- |
| Match Server     | 根据匹配结果请求创建房间                     |
| Room Coordinator | 创建房间记录、选择 Game Server、生成入房信息 |
| Game Server      | 创建房间实例、管理玩家连接、启动对局         |
| Redis            | 存储房间路由、入房凭证、临时状态             |
| MySQL            | 存储房间基础记录和最终对局记录，MVP 可选     |
| Cocos Client     | 根据入房信息建立 WebSocket 连接并发送 Ready  |

------

## 12.2 房间系统边界

房间系统负责：

```text
1. 创建房间
2. 分配 Game Server
3. 生成 roomId
4. 生成 enterToken
5. 维护房间状态
6. 维护玩家入房状态
7. 处理入房认证
8. 管理 Ready 状态
9. 触发对局开始
10. 触发房间结束
11. 销毁房间资源
```

房间系统不负责：

```text
1. 匹配规则计算
2. 移动同步细节
3. 碰撞检测细节
4. 吞噬判定细节
5. 局内排行榜详细计算
6. 结算奖励计算
7. 全服排行榜更新
```

------

## 13. 核心流程设计

## 13.1 创建房间流程

```text
1. Match Server 匹配出一批玩家
2. Match Server 调用 Room Coordinator 创建房间
3. Room Coordinator 校验玩家列表
4. Room Coordinator 选择可用 Game Server
5. Room Coordinator 生成 roomId
6. Room Coordinator 生成每个玩家的 enterToken
7. Room Coordinator 请求 Game Server 创建房间实例
8. Game Server 初始化房间内存状态
9. Game Server 返回创建成功
10. Room Coordinator 保存房间路由和入房凭证
11. Room Coordinator 返回 roomId、serverId、wsUrl、enterToken
```

------

## 13.2 客户端入房流程

```text
1. 客户端从匹配状态接口拿到 roomId、wsUrl、enterToken
2. 客户端建立 WebSocket 连接
3. 客户端发送 ENTER_ROOM 消息
4. Game Server 校验 enterToken
5. Game Server 校验 userId 是否属于该房间
6. Game Server 将 WebSocket 连接绑定到玩家
7. Game Server 返回 ENTER_ROOM_RESULT
8. 客户端加载游戏资源
9. 客户端发送 READY
10. Game Server 更新玩家 Ready 状态
```

------

## 13.3 房间开始流程

```text
1. Game Server 周期检查房间玩家 Ready 状态
2. 判断 Ready 玩家数是否满足最低开局人数
3. 判断是否达到预期 Ready 人数或 Ready 超时
4. 满足条件后房间进入 READY_CHECK
5. Game Server 广播 START_COUNTDOWN
6. 倒计时结束后房间进入 RUNNING
7. Game Server 开始 Tick 循环
8. 客户端进入正式对战
```

------

## 13.4 房间结束流程

```text
1. Game Server 判断对局达到结束条件
2. 房间状态从 RUNNING 变为 SETTLING
3. Game Server 冻结房间状态
4. Game Server 生成本局基础结果
5. Game Server 通知结算服务
6. 结算服务返回结算结果
7. Game Server 广播 SETTLEMENT_RESULT
8. 房间状态变为 FINISHED
9. 延迟释放连接和内存资源
10. 房间状态变为 DESTROYED
```

------

## 13.5 房间销毁流程

```text
1. 房间进入 FINISHED 或 FAILED
2. Game Server 停止房间 Tick
3. 清理玩家连接绑定
4. 清理房间内存对象
5. 清理 Redis 房间临时状态
6. 上报房间结束日志和指标
7. 从 Game Server 房间管理器中移除 roomId
```

------

## 14. 接口设计

## 14.1 创建房间接口

调用方：

```text
Match Server → Room Coordinator
```

### 请求

```http
POST /internal/rooms
```

### 请求参数

```json
{
  "matchId": "m_10001",
  "mode": "classic",
  "players": [
    {
      "userId": "10001",
      "nickname": "吞噬细胞",
      "level": 25,
      "rank": "黄金 III"
    },
    {
      "userId": "10002",
      "nickname": "绿巨人",
      "level": 18,
      "rank": "白银 I"
    }
  ],
  "config": {
    "maxPlayers": 100,
    "minStartPlayers": 10,
    "readyTimeoutSeconds": 20,
    "battleDurationSeconds": 300
  }
}
```

### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "roomId": "r_90001",
    "serverId": "gs_01",
    "wsUrl": "wss://game.example.com/ws",
    "enterTokens": {
      "10001": "enter_token_10001",
      "10002": "enter_token_10002"
    },
    "expireAt": 1710000040000
  }
}
```

------

## 14.2 查询房间状态接口

调用方：

```text
内部服务 / 管理后台 / 调试工具
```

### 请求

```http
GET /internal/rooms/{roomId}
```

### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "roomId": "r_90001",
    "matchId": "m_10001",
    "mode": "classic",
    "serverId": "gs_01",
    "status": "LOADING",
    "playerCount": 28,
    "readyCount": 21,
    "createdAt": 1710000000000,
    "startedAt": null
  }
}
```

------

## 14.3 Game Server 创建房间接口

调用方：

```text
Room Coordinator → Game Server
```

### 请求

```http
POST /internal/game/rooms
```

### 请求参数

```json
{
  "roomId": "r_90001",
  "matchId": "m_10001",
  "mode": "classic",
  "players": ["10001", "10002", "10003"],
  "config": {
    "maxPlayers": 100,
    "minStartPlayers": 10,
    "readyTimeoutSeconds": 20,
    "battleDurationSeconds": 300
  }
}
```

### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "roomId": "r_90001",
    "serverId": "gs_01",
    "status": "CREATED"
  }
}
```

------

## 15. WebSocket 消息设计

## 15.1 入房认证消息

客户端发送：

```json
{
  "type": "ENTER_ROOM",
  "seq": 1001,
  "data": {
    "roomId": "r_90001",
    "userId": "10001",
    "enterToken": "enter_token_10001"
  }
}
```

服务端返回：

```json
{
  "type": "ENTER_ROOM_RESULT",
  "seq": 1001,
  "data": {
    "success": true,
    "roomId": "r_90001",
    "status": "LOADING",
    "serverTime": 1710000000000
  }
}
```

------

## 15.2 Ready 消息

客户端发送：

```json
{
  "type": "READY",
  "seq": 1002,
  "data": {
    "roomId": "r_90001",
    "userId": "10001",
    "clientLoadCostMs": 1230
  }
}
```

服务端广播：

```json
{
  "type": "PLAYER_READY",
  "data": {
    "roomId": "r_90001",
    "userId": "10001",
    "readyCount": 21,
    "playerCount": 28
  }
}
```

------

## 15.3 开局倒计时消息

```json
{
  "type": "START_COUNTDOWN",
  "data": {
    "roomId": "r_90001",
    "countdownSeconds": 3,
    "serverStartTime": 1710000003000
  }
}
```

------

## 15.4 对局开始消息

```json
{
  "type": "GAME_START",
  "data": {
    "roomId": "r_90001",
    "serverTime": 1710000003000,
    "battleDurationSeconds": 300
  }
}
```

------

## 15.5 入房失败消息

```json
{
  "type": "ENTER_ROOM_RESULT",
  "seq": 1001,
  "data": {
    "success": false,
    "errorCode": 30011,
    "message": "入房凭证已过期，请重新匹配"
  }
}
```

------

## 16. Redis 数据设计

## 16.1 房间路由

### Key

```text
room:route:{roomId}
```

### 类型

```text
Hash / String
```

### 示例

```json
{
  "roomId": "r_90001",
  "serverId": "gs_01",
  "wsUrl": "wss://game.example.com/ws",
  "status": "LOADING",
  "createdAt": 1710000000000,
  "expireAt": 1710000040000
}
```

### 用途

1. 根据 roomId 找到对应 Game Server。
2. 支持客户端重连时定位房间。
3. 支持内部服务查询房间位置。

------

## 16.2 入房令牌

### Key

```text
room:enter_token:{token}
```

### 类型

```text
Hash / String
```

### 示例

```json
{
  "token": "enter_token_10001",
  "roomId": "r_90001",
  "userId": "10001",
  "serverId": "gs_01",
  "status": "UNUSED",
  "expireAt": 1710000040000
}
```

### 用途

1. WebSocket 入房认证。
2. 防止非匹配玩家进入房间。
3. 控制入房有效期。
4. 防止 token 被重复滥用。

------

## 16.3 房间玩家列表

### Key

```text
room:players:{roomId}
```

### 类型

```text
Set / Hash
```

### 示例

```json
{
  "10001": {
    "status": "READY",
    "nickname": "吞噬细胞",
    "joinTime": 1710000001000
  },
  "10002": {
    "status": "LOADING",
    "nickname": "绿巨人",
    "joinTime": 1710000002000
  }
}
```

------

## 16.4 Game Server 信息

### Key

```text
game_server:{serverId}
```

### 类型

```text
Hash
```

### 示例

```json
{
  "serverId": "gs_01",
  "host": "10.0.0.11",
  "wsUrl": "wss://game-01.example.com/ws",
  "status": "ONLINE",
  "roomCount": 12,
  "playerCount": 856,
  "lastHeartbeat": 1710000000000
}
```

### 用途

1. Room Coordinator 选择 Game Server。
2. 判断 Game Server 是否可用。
3. 统计当前负载。
4. 支持后续多 Game Server 扩展。

------

## 17. MySQL 数据设计

MVP 阶段房间状态可以主要存 Redis 和 Game Server 内存。MySQL 可只保存最终对局记录。

如果需要保存房间生命周期记录，可设计如下表。

## 17.1 room_records

| 字段         | 类型     | 说明           |
| ------------ | -------- | -------------- |
| id           | bigint   | 主键           |
| room_id      | varchar  | 房间 ID        |
| match_id     | varchar  | 匹配 ID        |
| mode         | varchar  | 游戏模式       |
| server_id    | varchar  | Game Server ID |
| status       | varchar  | 房间最终状态   |
| player_count | int      | 玩家数量       |
| ready_count  | int      | Ready 数量     |
| started_at   | datetime | 开始时间       |
| finished_at  | datetime | 结束时间       |
| created_at   | datetime | 创建时间       |
| updated_at   | datetime | 更新时间       |

------

## 17.2 room_player_records

| 字段         | 类型     | 说明       |
| ------------ | -------- | ---------- |
| id           | bigint   | 主键       |
| room_id      | varchar  | 房间 ID    |
| user_id      | varchar  | 用户 ID    |
| nickname     | varchar  | 昵称       |
| enter_status | varchar  | 入房状态   |
| ready_status | varchar  | Ready 状态 |
| join_time    | datetime | 入房时间   |
| ready_time   | datetime | Ready 时间 |
| created_at   | datetime | 创建时间   |
| updated_at   | datetime | 更新时间   |

------

## 18. Game Server 分配策略

## 18.1 MVP 策略

MVP 阶段采用简单策略：

```text
选择 ONLINE 状态下 roomCount 最少的 Game Server
```

处理流程：

```text
1. 查询可用 Game Server 列表
2. 过滤状态不为 ONLINE 的节点
3. 按 roomCount 升序排序
4. 选择房间数最少的节点
5. 创建房间
```

------

## 18.2 完整版本策略

后续可综合以下因素：

1. 当前房间数。
2. 当前玩家数。
3. CPU 使用率。
4. 内存使用率。
5. 网络延迟。
6. 区域。
7. 是否正在维护。
8. 是否达到最大房间数。
9. 是否达到最大连接数。

综合评分示例：

```text
score = roomWeight + playerWeight + cpuWeight + memoryWeight
```

选择 score 最低的 Game Server。

------

## 19. 幂等与并发控制

## 19.1 创建房间幂等

Match Server 可能因为网络超时重复请求创建房间，因此需要根据 matchId 做幂等。

处理规则：

1. Room Coordinator 收到创建请求后，先查询 matchId 是否已创建房间。
2. 如果已创建，直接返回已有 roomId 和入房信息。
3. 如果未创建，则创建新房间。
4. 保证同一个 matchId 不会创建多个房间。

幂等 Key：

```text
room:match:{matchId}
```

------

## 19.2 入房认证幂等

同一玩家可能因重试多次发送 ENTER_ROOM。

处理规则：

1. 如果玩家未入房，则正常绑定连接。
2. 如果玩家已入房且连接有效，则返回已入房。
3. 如果玩家已入房但旧连接断开，则允许替换连接。
4. 如果 enterToken 过期，则拒绝。

------

## 19.3 Ready 幂等

同一客户端可能多次发送 READY。

处理规则：

1. 第一次 READY 更新玩家状态。
2. 后续重复 READY 直接返回当前状态。
3. readyCount 不重复增加。

------

## 20. 异常处理

## 20.1 房间创建失败

原因：

1. 没有可用 Game Server。
2. Game Server 创建房间失败。
3. Redis 写入失败。
4. 内部服务超时。

处理：

1. 返回创建失败。
2. 通知 Match Server。
3. 玩家匹配状态变更为 FAILED。
4. 客户端提示服务器繁忙。
5. 记录日志和监控指标。

------

## 20.2 入房凭证过期

处理：

1. 拒绝 WebSocket 入房。
2. 返回 ENTER_ROOM_RESULT success=false。
3. 客户端展示“入房凭证已过期，请重新匹配”。
4. 玩家返回大厅。

------

## 20.3 房间不存在

处理：

1. 查询 roomId 失败。
2. 返回房间不存在。
3. 客户端重新匹配。
4. 记录异常日志。

------

## 20.4 Game Server 断开

MVP 阶段处理：

1. 房间失败。
2. 断开玩家连接。
3. 客户端返回大厅。
4. 记录异常日志。

完整版本后续可支持：

1. 房间状态恢复。
2. 房间迁移。
3. 玩家重连到新 Game Server。

------

## 21. 错误码设计

| 错误码 | 含义                     | 客户端处理     |
| ------ | ------------------------ | -------------- |
| 0      | 成功                     | 正常处理       |
| 30001  | 房间创建失败             | 提示服务器繁忙 |
| 30002  | 无可用 Game Server       | 提示稍后重试   |
| 30003  | Game Server 创建房间失败 | 返回大厅       |
| 30010  | 房间不存在               | 重新匹配       |
| 30011  | 入房凭证过期             | 重新匹配       |
| 30012  | 用户不属于该房间         | 返回大厅       |
| 30013  | 房间人数已满             | 重新匹配       |
| 30014  | 房间已开始               | 重新匹配       |
| 30015  | Ready 超时               | 返回大厅       |
| 30016  | 入房超时                 | 重新匹配       |
| 50000  | 系统异常                 | 稍后重试       |

------

## 22. 日志设计

## 22.1 关键日志点

| 日志点               | 说明              |
| -------------------- | ----------------- |
| room_create_request  | 收到创建房间请求  |
| room_create_success  | 房间创建成功      |
| room_create_failed   | 房间创建失败      |
| game_server_selected | 选择 Game Server  |
| player_enter_room    | 玩家入房          |
| player_enter_failed  | 玩家入房失败      |
| player_ready         | 玩家 Ready        |
| room_start_countdown | 房间开始倒计时    |
| room_running         | 房间进入 RUNNING  |
| room_settling        | 房间进入 SETTLING |
| room_finished        | 房间完成          |
| room_destroyed       | 房间销毁          |

------

## 22.2 日志字段

```json
{
  "level": "info",
  "traceId": "trace_xxx",
  "matchId": "m_10001",
  "roomId": "r_90001",
  "serverId": "gs_01",
  "userId": "10001",
  "status": "LOADING",
  "message": "player_ready",
  "timestamp": "2026-06-28T10:00:00.000Z"
}
```

------

## 23. 监控指标

| 指标                            | 说明                        |
| ------------------------------- | --------------------------- |
| room_create_total               | 创建房间次数                |
| room_create_failed_total        | 房间创建失败次数            |
| room_running_total              | 成功进入 RUNNING 的房间数   |
| room_destroy_total              | 销毁房间次数                |
| room_player_enter_total         | 玩家入房次数                |
| room_player_enter_failed_total  | 玩家入房失败次数            |
| room_ready_timeout_total        | Ready 超时次数              |
| room_enter_timeout_total        | 入房超时次数                |
| room_count_by_server            | 每个 Game Server 当前房间数 |
| room_player_count_by_server     | 每个 Game Server 当前玩家数 |
| room_lifecycle_duration_seconds | 房间生命周期耗时            |

------

## 24. 安全与校验

## 24.1 创建房间校验

Room Coordinator 创建房间前需要校验：

1. matchId 是否存在。
2. 玩家列表是否为空。
3. 玩家数量是否超过 maxPlayers。
4. 游戏模式是否支持。
5. 是否已有同 matchId 房间。
6. 是否存在可用 Game Server。

------

## 24.2 入房认证校验

Game Server 处理 ENTER_ROOM 时需要校验：

1. roomId 是否存在。
2. userId 是否属于该房间。
3. enterToken 是否存在。
4. enterToken 是否过期。
5. enterToken 与 userId、roomId 是否匹配。
6. 房间状态是否允许入房。
7. 玩家是否重复连接。

------

## 24.3 Ready 校验

Game Server 处理 READY 时需要校验：

1. 玩家是否已通过入房认证。
2. 玩家是否属于当前房间。
3. 房间状态是否为 LOADING 或 READY_CHECK。
4. 是否已经 Ready。
5. 客户端版本是否兼容，后续支持。

------

## 25. MVP 开发任务拆分

## 25.1 客户端任务

| 任务               | 说明                             |
| ------------------ | -------------------------------- |
| 入房加载页         | 展示连接、认证、加载、Ready 状态 |
| WebSocket 入房认证 | 发送 ENTER_ROOM 消息             |
| Ready 上报         | 加载完成后发送 READY             |
| 开局倒计时展示     | 接收 START_COUNTDOWN 并展示      |
| 入房失败处理       | 根据错误码展示提示               |
| 对局开始跳转       | 收到 GAME_START 后进入对战页     |

------

## 25.2 服务端任务

| 任务               | 说明                         |
| ------------------ | ---------------------------- |
| Room Coordinator   | 提供创建房间能力             |
| Game Server 注册   | 维护可用 Game Server 列表    |
| Game Server 分配   | 选择可用 Game Server         |
| 房间创建接口       | Game Server 创建房间实例     |
| enterToken 生成    | 为玩家生成入房凭证           |
| Redis 房间路由     | 保存 roomId 与 serverId 映射 |
| WebSocket 入房认证 | 校验 ENTER_ROOM              |
| Ready 状态管理     | 记录玩家 Ready               |
| 房间状态流转       | CREATED 到 RUNNING           |
| 房间销毁           | 对局结束后释放资源           |

------

## 26. 后续详细设计拆分建议

房间系统后续可继续拆成以下详细设计：

1. Room Coordinator 详细设计。
2. Game Server 注册与负载分配设计。
3. WebSocket 入房认证设计。
4. 房间状态机设计。
5. 房间内玩家状态机设计。
6. Ready 与开局倒计时设计。
7. 房间销毁与资源释放设计。
8. Game Server 异常恢复设计。
9. 房间监控与日志设计。

------

## 27. 总结

房间系统是《吞噬细胞》中连接匹配系统和实时对战系统的关键模块。它的核心职责是：

```text
创建房间
  ↓
分配 Game Server
  ↓
生成入房信息
  ↓
玩家 WebSocket 入房
  ↓
玩家 Ready
  ↓
房间开始
  ↓
对局结束
  ↓
房间销毁
```

MVP 阶段需要优先保证房间链路稳定，而不是过早引入复杂房间迁移、机器人补位和高可用恢复能力。

当前阶段的关键设计原则：

1. 房间由服务端创建。
2. 玩家必须通过 enterToken 入房。
3. 房间状态必须明确。
4. Ready 状态必须可追踪。
5. 房间开始条件必须可配置。
6. 房间结束后必须释放资源。
7. Game Server 分配策略先简单，后续再优化。

后续可以继续围绕 Game Server Tick、状态同步、AOI、排行榜和结算系统进行详细设计。