# 《吞噬细胞》断线重连系统 PRD + 技术设计文档

## 1. 文档信息

| 项目     | 内容                                                         |
| -------- | ------------------------------------------------------------ |
| 文档名称 | 断线重连系统 PRD + 技术设计文档                              |
| 所属产品 | 吞噬细胞                                                     |
| 所属模块 | 断线重连系统                                                 |
| 上游模块 | 房间系统、Game Server、状态同步系统                          |
| 关联模块 | 结算系统、战绩系统、全服排行榜系统                           |
| 客户端   | Cocos Creator 3.x + TypeScript                               |
| 服务端   | Go                                                           |
| 通信方式 | WebSocket + HTTP                                             |
| 存储依赖 | Redis、MySQL                                                 |
| 文档定位 | 产品需求 + 技术概要设计                                      |
| 详细程度 | 中等详细，后续可继续拆分心跳、重连协议、快照恢复、弱网策略等详细设计 |

------

# 第一部分：断线重连系统 PRD

## 2. 模块背景

《吞噬细胞》是一款实时多人对战游戏，客户端与 Game Server 之间主要通过 WebSocket 保持长连接。移动、分裂、吐球、状态快照、排行榜、结算等消息都依赖该连接。

在真实网络环境中，玩家可能出现：

1. 网络短暂波动。
2. Wi-Fi 和移动网络切换。
3. App 切后台。
4. 浏览器刷新。
5. 设备息屏。
6. WebSocket 异常断开。
7. 客户端崩溃后重新进入。
8. 游戏结束后错过结算结果。

断线重连系统需要保证：
玩家短时间断线后，可以尽可能恢复当前对局；如果无法恢复，也要给出明确结果，避免玩家不知道自己处于什么状态。

------

## 3. 模块目标

### 3.1 产品目标

| 目标             | 说明                               |
| ---------------- | ---------------------------------- |
| 弱网体验可接受   | 短时间网络波动不应直接踢出对局     |
| 支持短线重连     | 玩家在重连窗口内可以恢复对局       |
| 状态恢复清晰     | 重连成功后客户端能恢复当前房间画面 |
| 失败提示明确     | 重连失败时给出明确原因             |
| 支持结算找回     | 对局已结束时可以查看结算结果       |
| 避免重复连接混乱 | 同一玩家只能有一个有效连接         |
| 保护游戏公平     | 断线玩家不能利用重连作弊           |
| 降低误伤         | 短暂网络波动不应立即判定玩家退出   |

### 3.2 技术目标

| 目标           | 说明                                     |
| -------------- | ---------------------------------------- |
| 心跳检测       | 通过 Ping/Pong 或服务端心跳判断连接状态  |
| 连接状态可追踪 | 服务端记录玩家在线、断线、重连状态       |
| 重连窗口可配置 | 允许一定时间内恢复连接                   |
| 房间路由可恢复 | 客户端可以根据 roomId 找到原 Game Server |
| 快照恢复       | 重连成功后下发完整恢复快照               |
| 连接替换       | 新连接成功后替换旧连接                   |
| 幂等处理       | 重复重连请求不造成状态异常               |
| 与结算联动     | 对局已结束时返回结算状态或结果           |

------

## 4. 模块定位

断线重连系统横跨客户端、接入层、房间系统、Game Server、状态同步和结算系统。

```text
Cocos Client
  ↓ WebSocket 断开
Reconnect Manager
  ↓
API Server / Room Route
  ↓
Game Server
  ↓
Room Instance
  ↓
Recover Snapshot / Settlement Result
```

断线重连系统不是独立玩法模块，而是实时对战链路中的异常恢复能力。

------

## 5. 核心设计原则

## 5.1 短断线可恢复，长断线按退出处理

推荐规则：

```text
断线 0 ~ 30 秒：允许重连
超过 30 秒：视为超时退出或死亡
```

重连窗口可配置，MVP 推荐：

```text
reconnectWindowSeconds = 30
```

------

## 5.2 服务端保留房间权威状态

玩家断线后，服务端不会依赖客户端保存状态。

服务端继续维护：

1. 玩家对象。
2. 玩家质量。
3. 玩家位置。
4. 玩家排名。
5. 玩家死亡状态。
6. 对局进度。
7. 结算结果。

客户端重连后，必须从服务端恢复状态。

------

## 5.3 重连后下发全量恢复快照

重连成功后，不建议继续使用客户端旧状态。

推荐流程：

```text
客户端重连成功
  ↓
服务端下发 ROOM_RECOVER_SNAPSHOT
  ↓
客户端清空本地房间状态
  ↓
客户端基于恢复快照重建画面
```

这样可以避免客户端本地状态与服务端状态不一致。

------

## 5.4 同一玩家只允许一个有效连接

玩家重连成功后，新 WebSocket 连接应替换旧连接。

处理规则：

1. 如果旧连接仍存在但不可用，关闭旧连接。
2. 如果旧连接仍在线，按重复登录处理。
3. 以最新认证成功连接为准。
4. 防止一个玩家多个连接同时发送输入。

------

## 5.5 对局已结束时进入结算恢复

如果玩家重连时房间已经结束：

```text
返回结算结果
```

如果结算还在处理中：

```text
返回结算处理中
```

如果结算失败：

```text
提示稍后在战绩中查看
```

------

# 6. 用户场景

## 6.1 对局中短暂断网后重连成功

### 场景描述

玩家网络短暂断开，客户端自动重连，服务端仍保留玩家对象。

### 用户流程

```text
WebSocket 断开
  ↓
客户端展示“正在重连...”
  ↓
客户端重新获取房间路由
  ↓
建立新的 WebSocket
  ↓
发送 RECONNECT
  ↓
服务端校验成功
  ↓
下发恢复快照
  ↓
客户端恢复对局
```

------

## 6.2 重连超时失败

### 场景描述

玩家断线时间超过重连窗口。

### 产品表现

1. 客户端提示“重连超时，已退出对局”。
2. 如果对局仍在进行，玩家按退出或死亡处理。
3. 如果对局已结束，进入结算查询。
4. 玩家可返回大厅或重新匹配。

------

## 6.3 房间已结束，重连后查看结算

### 场景描述

玩家断线期间，对局已经结束。

### 产品表现

1. 客户端重连后不再进入对战页。
2. 服务端返回房间已结束。
3. 客户端查询结算结果。
4. 展示结算页或“结算处理中”。

------

## 6.4 客户端刷新页面后恢复

### 场景描述

H5 客户端刷新页面，导致 WebSocket 断开，本地内存状态丢失。

### 处理流程

```text
客户端重新启动
  ↓
读取本地保存的 roomId / reconnectToken
  ↓
请求恢复房间状态
  ↓
如果仍可重连，进入对战
  ↓
否则进入大厅或结算页
```

------

## 6.5 App 切后台后恢复

### 场景描述

移动端切后台，WebSocket 可能被系统断开。

### 处理策略

1. 客户端切前台后检测连接状态。
2. 如果连接可用，继续游戏。
3. 如果连接断开，自动重连。
4. 超过重连窗口则提示失败。
5. 对局结束则展示结算结果。

------

# 7. 功能范围

## 7.1 MVP 版本功能范围

MVP 阶段实现：

1. WebSocket 心跳。
2. 服务端断线检测。
3. 玩家断线状态标记。
4. 重连窗口。
5. 客户端自动重连。
6. RECONNECT 消息。
7. reconnectToken 校验。
8. 房间路由恢复。
9. 新连接替换旧连接。
10. 重连后下发全量恢复快照。
11. 重连失败提示。
12. 房间结束后的结算查询。

------

## 7.2 完整版本功能范围

完整版本扩展：

1. 弱网状态提示。
2. 多次重连退避策略。
3. 客户端本地输入缓存。
4. 服务端输入补偿，谨慎支持。
5. 网络延迟和抖动监控。
6. 切后台特殊策略。
7. 房间迁移恢复，后续高阶能力。
8. Game Server 异常恢复。
9. 重连风控。
10. 断线期间 AI 托管，后续玩法。
11. 重连后快速状态校正。
12. 结算红点提醒。

------

## 7.3 暂不包含范围

当前文档不详细展开：

1. Game Server 房间迁移。
2. 跨服务器热恢复。
3. 客户端预测回滚。
4. 离线托管 AI。
5. 复杂弱网补偿。
6. UDP / KCP 网络方案。
7. 多端同时登录策略。
8. 战斗回放恢复。

------

# 8. 玩家连接状态设计

## 8.1 连接状态

| 状态              | 说明       |
| ----------------- | ---------- |
| CONNECTED         | 已连接     |
| AUTHENTICATED     | 已认证     |
| PLAYING           | 正常对战中 |
| DISCONNECTED      | 连接已断开 |
| RECONNECTING      | 正在重连   |
| RECONNECTED       | 重连成功   |
| RECONNECT_TIMEOUT | 重连超时   |
| EXITED            | 已退出     |
| SETTLED           | 已结算     |

------

## 8.2 状态流转

正常连接：

```text
CONNECTED
  ↓
AUTHENTICATED
  ↓
PLAYING
```

断线重连：

```text
PLAYING
  ↓ WebSocket 断开
DISCONNECTED
  ↓ 重连窗口内发起重连
RECONNECTING
  ↓ 校验成功
RECONNECTED
  ↓ 恢复快照完成
PLAYING
```

重连失败：

```text
DISCONNECTED
  ↓ 超过重连窗口
RECONNECT_TIMEOUT
  ↓
EXITED / SETTLED
```

------

# 9. 客户端交互设计

## 9.1 网络状态提示

客户端需要展示：

| 场景       | 文案                            |
| ---------- | ------------------------------- |
| 连接正常   | 不展示或展示低延迟图标          |
| 网络波动   | 网络不稳定                      |
| 连接断开   | 连接已断开，正在重连...         |
| 重连中     | 正在恢复对局...                 |
| 重连成功   | 已恢复连接                      |
| 重连失败   | 重连失败，请返回大厅            |
| 对局已结束 | 对局已结束，正在获取结算结果... |

------

## 9.2 重连遮罩

断线后展示半透明遮罩：

```text
正在重连...
请保持网络连接
```

遮罩期间：

1. 禁止发送移动、分裂、吐球等输入。
2. 可以保留当前画面。
3. 可以展示重连倒计时。
4. 重连成功后恢复操作。
5. 重连失败后跳转失败页或大厅。

------

## 9.3 重连失败页

展示内容：

1. 失败原因。
2. 返回大厅按钮。
3. 查看结算按钮，如果已结算。
4. 重新匹配按钮。

常见失败原因：

| 原因         | 文案                       |
| ------------ | -------------------------- |
| 超过重连时间 | 重连超时，已退出对局       |
| 房间不存在   | 房间已失效                 |
| 对局已结束   | 对局已结束                 |
| 认证失败     | 登录状态异常，请重新登录   |
| 服务器不可用 | 游戏服务器繁忙，请稍后重试 |

------

## 9.4 本地保存信息

客户端进入房间后，需要保存短期恢复信息：

```json
{
  "roomId": "r_90001",
  "serverId": "gs_01",
  "reconnectToken": "reconnect_token_xxx",
  "lastEnterTime": 1710000000000
}
```

用途：

1. 页面刷新后恢复。
2. App 重启后尝试找回房间。
3. WebSocket 断开后自动重连。

注意：

1. token 必须有有效期。
2. 对局结束后清除。
3. 返回大厅后清除。

------

# 第二部分：断线重连技术设计

## 10. 总体架构

## 10.1 模块关系

```text
Cocos Client
  ↓ WebSocket
Game Server ConnectionManager
  ↓
DisconnectManager
  ↓
RoomInstance
  ↓
ReconnectManager
  ↓
SnapshotSystem
  ↓
Settlement / Record Service
```

------

## 10.2 服务端模块

```text
Game Server
├── ConnectionManager
├── HeartbeatManager
├── DisconnectManager
├── ReconnectManager
├── RoomManager
├── SessionManager
├── SnapshotSystem
├── SettlementQueryClient
└── ReconnectMetrics
```

模块说明：

| 模块                  | 职责                 |
| --------------------- | -------------------- |
| ConnectionManager     | 管理 WebSocket 连接  |
| HeartbeatManager      | 心跳检测             |
| DisconnectManager     | 处理连接断开         |
| ReconnectManager      | 处理重连认证和恢复   |
| RoomManager           | 查找房间实例         |
| SessionManager        | 管理玩家会话和 token |
| SnapshotSystem        | 生成恢复快照         |
| SettlementQueryClient | 查询结算结果         |
| ReconnectMetrics      | 统计重连指标         |

------

## 10.3 客户端模块

```text
Cocos Client
├── WsClient
├── HeartbeatClient
├── ReconnectClient
├── LocalSessionStore
├── NetworkStatusPanel
├── SnapshotReceiver
└── SceneRecoveryManager
```

模块说明：

| 模块                 | 职责                     |
| -------------------- | ------------------------ |
| WsClient             | WebSocket 连接管理       |
| HeartbeatClient      | 发送 Ping，接收 Pong     |
| ReconnectClient      | 自动重连流程             |
| LocalSessionStore    | 本地保存 roomId 和 token |
| NetworkStatusPanel   | 展示网络状态             |
| SnapshotReceiver     | 处理恢复快照             |
| SceneRecoveryManager | 恢复对战场景             |

------

# 11. 心跳设计

## 11.1 心跳目标

心跳用于判断连接是否仍然可用。

心跳需要解决：

1. 客户端是否还在线。
2. 服务端连接是否可写。
3. 网络延迟是否过高。
4. 是否需要触发断线处理。

------

## 11.2 客户端心跳

客户端定时发送：

```json
{
  "type": "PING",
  "seq": 9001,
  "clientTime": 1710000000000,
  "data": {
    "roomId": "r_90001"
  }
}
```

服务端返回：

```json
{
  "type": "PONG",
  "seq": 9001,
  "serverTime": 1710000000050,
  "data": {
    "roomId": "r_90001"
  }
}
```

------

## 11.3 心跳频率

推荐配置：

| 配置                     | 建议值 |
| ------------------------ | ------ |
| heartbeatIntervalSeconds | 5      |
| heartbeatTimeoutSeconds  | 15     |
| maxMissedHeartbeat       | 3      |

MVP 可以使用：

```text
每 5 秒 Ping 一次
连续 3 次未响应判定断线
```

------

## 11.4 服务端心跳检测

服务端记录：

```text
lastActiveTime
lastPingTime
lastPongTime
```

如果超过阈值：

```text
now - lastActiveTime > heartbeatTimeout
```

则标记连接异常，并触发断线处理。

------

# 12. 会话与 Token 设计

## 12.1 enterToken 与 reconnectToken

| Token          | 用途           | 生命周期                 |
| -------------- | -------------- | ------------------------ |
| enterToken     | 首次入房认证   | 入房阶段短期有效         |
| reconnectToken | 对局中重连认证 | 对局期间有效，带重连窗口 |
| accessToken    | 用户登录态     | 按登录系统配置           |

首次进入房间使用 `enterToken`。
成功入房后，服务端下发 `reconnectToken`。

------

## 12.2 reconnectToken 数据

Redis Key：

```text
room:reconnect_token:{token}
```

Value：

```json
{
  "token": "reconnect_token_xxx",
  "userId": "10001",
  "roomId": "r_90001",
  "serverId": "gs_01",
  "expireAt": 1710000300000,
  "status": "ACTIVE"
}
```

用途：

1. 校验玩家是否有权重连。
2. 找到原房间。
3. 防止其他玩家伪造重连。
4. 支持页面刷新后恢复。

------

## 12.3 房间路由数据

Redis Key：

```text
room:route:{roomId}
```

Value：

```json
{
  "roomId": "r_90001",
  "serverId": "gs_01",
  "wsUrl": "wss://game-01.example.com/ws",
  "status": "RUNNING",
  "expireAt": 1710000400000
}
```

客户端如果丢失 wsUrl，可以通过 API 查询 room route。

------

# 13. 服务端断线处理流程

## 13.1 WebSocket 异常断开

```text
1. WebSocket 读写异常
2. ConnectionManager 捕获断开事件
3. 查询连接绑定的 userId 和 roomId
4. 标记玩家连接状态为 DISCONNECTED
5. 记录 disconnectTime
6. 保留玩家房间状态
7. 设置 reconnectDeadline
8. 广播或不广播玩家断线事件，按配置
```

------

## 13.2 玩家断线后的房间状态

断线后，玩家对象如何处理可配置。

MVP 推荐：

```text
玩家对象继续留在地图中
但不再接收输入
保持最后移动方向或停止移动
```

可选策略：

| 策略                | 说明                   |
| ------------------- | ---------------------- |
| STOP                | 断线后停止移动         |
| KEEP_LAST_DIRECTION | 保持最后方向短时间移动 |
| AI_TAKEOVER         | AI 托管，后续支持      |
| IMMEDIATE_DEAD      | 立即死亡，不推荐       |

MVP 推荐：

```text
STOP
```

原因：

1. 实现简单。
2. 不容易被利用。
3. 玩家断线后存在风险，符合实时对战直觉。
4. 重连后状态容易恢复。

------

## 13.3 重连超时处理

```text
1. Game Server 定时检查断线玩家
2. 如果 now > reconnectDeadline
3. 玩家状态变为 RECONNECT_TIMEOUT
4. 根据规则处理为退出或死亡
5. 生成 PLAYER_RECONNECT_TIMEOUT 事件
6. 进入后续结算统计
```

MVP 推荐：

```text
断线超时后视为死亡或退出
```

如果游戏玩法希望更友好，可配置为：

```text
断线超时后保留死亡时得分，等待结算
```

------

# 14. 客户端自动重连流程

## 14.1 WebSocket 断开后

```text
1. WsClient 监听到 close / error
2. 停止发送玩家输入
3. 展示重连遮罩
4. 读取本地 roomId 和 reconnectToken
5. 开始自动重连
```

------

## 14.2 重连退避策略

MVP 推荐：

```text
第 1 次：立即重连
第 2 次：1 秒后
第 3 次：2 秒后
第 4 次：3 秒后
之后每 5 秒一次
直到超过重连窗口
```

------

## 14.3 查询房间路由

如果客户端没有可用 wsUrl：

```http
GET /api/rooms/{roomId}/route
```

返回：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "roomId": "r_90001",
    "serverId": "gs_01",
    "wsUrl": "wss://game-01.example.com/ws",
    "status": "RUNNING"
  }
}
```

------

# 15. 重连协议设计

## 15.1 RECONNECT 请求

客户端建立新 WebSocket 后发送：

```json
{
  "type": "RECONNECT",
  "seq": 9101,
  "clientTime": 1710000000000,
  "data": {
    "roomId": "r_90001",
    "userId": "10001",
    "reconnectToken": "reconnect_token_xxx",
    "lastClientSnapshotSeq": 3001
  }
}
```

字段说明：

| 字段                  | 说明                     |
| --------------------- | ------------------------ |
| roomId                | 房间 ID                  |
| userId                | 用户 ID                  |
| reconnectToken        | 重连令牌                 |
| lastClientSnapshotSeq | 客户端最后处理的快照序号 |

------

## 15.2 RECONNECT_RESULT 成功

```json
{
  "type": "RECONNECT_RESULT",
  "seq": 9101,
  "serverTime": 1710000000050,
  "data": {
    "success": true,
    "roomId": "r_90001",
    "status": "RECONNECTED",
    "message": "重连成功"
  }
}
```

------

## 15.3 恢复快照

重连成功后下发：

```json
{
  "type": "ROOM_RECOVER_SNAPSHOT",
  "seq": 9102,
  "serverTime": 1710000000060,
  "data": {
    "roomId": "r_90001",
    "tickSeq": 1520,
    "snapshotType": "FULL_RECOVER",
    "self": {},
    "players": [],
    "foods": [],
    "viruses": [],
    "ejectedMass": [],
    "rank": {},
    "events": []
  }
}
```

------

## 15.4 RECONNECT_RESULT 失败

```json
{
  "type": "RECONNECT_RESULT",
  "seq": 9101,
  "serverTime": 1710000000050,
  "data": {
    "success": false,
    "reason": "RECONNECT_TIMEOUT",
    "message": "重连超时，已退出对局"
  }
}
```

失败原因：

| reason             | 说明         |
| ------------------ | ------------ |
| TOKEN_INVALID      | 重连凭证无效 |
| TOKEN_EXPIRED      | 重连凭证过期 |
| ROOM_NOT_FOUND     | 房间不存在   |
| ROOM_FINISHED      | 房间已结束   |
| PLAYER_NOT_IN_ROOM | 玩家不在房间 |
| RECONNECT_TIMEOUT  | 超过重连窗口 |
| SERVER_ERROR       | 服务异常     |

------

# 16. 服务端重连处理流程

## 16.1 RECONNECT 处理

```text
1. Game Server 收到 RECONNECT
2. 校验 roomId 是否存在
3. 校验 reconnectToken 是否有效
4. 校验 token 中 userId 和 roomId 是否匹配
5. 查询玩家是否属于房间
6. 判断房间状态
7. 判断是否超过重连窗口
8. 关闭旧连接，绑定新连接
9. 更新玩家状态为 PLAYING
10. 生成 ROOM_RECOVER_SNAPSHOT
11. 返回 RECONNECT_RESULT
12. 下发恢复快照
```

------

## 16.2 房间状态分支

| 房间状态  | 处理             |
| --------- | ---------------- |
| RUNNING   | 允许重连         |
| SETTLING  | 返回结算处理中   |
| FINISHED  | 返回结算结果查询 |
| DESTROYED | 返回房间已失效   |
| FAILED    | 返回房间异常     |

------

## 16.3 新旧连接替换

处理规则：

```text
1. 如果旧连接存在，先标记为 replaced
2. 关闭旧连接
3. 新连接绑定 userId + roomId
4. 更新 ConnectionManager
5. 后续输入只接受新连接
```

避免：

1. 多连接重复发送 MOVE。
2. 多连接重复 SPLIT。
3. 旧连接继续收到快照。
4. 客户端状态混乱。

------

# 17. 重连后的状态恢复

## 17.1 客户端恢复流程

```text
1. 收到 RECONNECT_RESULT success=true
2. 等待 ROOM_RECOVER_SNAPSHOT
3. 清空本地 EntityManager
4. 清空 SnapshotBuffer
5. 根据恢复快照创建对象
6. 恢复排行榜 UI
7. 恢复倒计时和剩余时间
8. 关闭重连遮罩
9. 恢复输入发送
```

------

## 17.2 需要恢复的数据

恢复快照应包含：

1. 当前玩家状态。
2. 当前玩家球体。
3. 可见玩家对象。
4. 食物对象。
5. 刺球对象。
6. 吐出物对象。
7. 当前排行榜。
8. 房间剩余时间。
9. 当前 tickSeq。
10. 服务端时间。
11. 玩家是否死亡。
12. 游戏是否已结束。

------

## 17.3 AOI 场景恢复

如果已经接入 AOI：

```text
重连后下发 AOI_FULL_SNAPSHOT
```

客户端不应使用断线前的可见集合。

------

# 18. 对局结束后的恢复

## 18.1 重连时房间已结束

处理流程：

```text
1. 客户端发送 RECONNECT
2. 服务端发现房间 FINISHED 或 DESTROYED
3. 返回 ROOM_FINISHED
4. 客户端调用结算查询接口
5. 如果结算成功，展示结算页
6. 如果结算中，展示 loading
7. 如果结算失败，提示稍后在战绩中查看
```

------

## 18.2 结算结果查询

客户端调用：

```http
GET /api/settlements/{roomId}/me
```

或：

```http
GET /api/records/latest-settlement
```

------

## 18.3 大厅找回结算

如果客户端直接回到大厅：

```text
进入大厅
  ↓
请求 latest-settlement
  ↓
如果有未查看结算
  ↓
提示查看结果
```

------

# 19. Redis 数据设计

## 19.1 玩家会话

Key：

```text
room:session:{roomId}:{userId}
```

Value：

```json
{
  "roomId": "r_90001",
  "userId": "10001",
  "serverId": "gs_01",
  "status": "DISCONNECTED",
  "connectId": "conn_abc",
  "disconnectTime": 1710000000000,
  "reconnectDeadline": 1710000030000,
  "lastActiveTime": 1710000000000
}
```

------

## 19.2 重连令牌

Key：

```text
room:reconnect_token:{token}
```

Value：

```json
{
  "roomId": "r_90001",
  "userId": "10001",
  "serverId": "gs_01",
  "expireAt": 1710000300000,
  "status": "ACTIVE"
}
```

------

## 19.3 房间路由

Key：

```text
room:route:{roomId}
```

Value：

```json
{
  "roomId": "r_90001",
  "serverId": "gs_01",
  "wsUrl": "wss://game-01.example.com/ws",
  "status": "RUNNING",
  "updatedAt": 1710000000000
}
```

------

# 20. HTTP 接口设计

## 20.1 查询当前可恢复房间

用于客户端启动后判断是否需要恢复。

```http
GET /api/rooms/recoverable
```

响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "recoverable": true,
    "roomId": "r_90001",
    "serverId": "gs_01",
    "wsUrl": "wss://game-01.example.com/ws",
    "status": "RUNNING",
    "reconnectDeadline": 1710000030000
  }
}
```

------

## 20.2 查询房间路由

```http
GET /api/rooms/{roomId}/route
```

响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "roomId": "r_90001",
    "serverId": "gs_01",
    "wsUrl": "wss://game-01.example.com/ws",
    "status": "RUNNING"
  }
}
```

------

# 21. 断线策略配置

```json
{
  "reconnect": {
    "enabled": true,
    "reconnectWindowSeconds": 30,
    "heartbeatIntervalSeconds": 5,
    "heartbeatTimeoutSeconds": 15,
    "maxReconnectAttempts": 10,
    "reconnectBackoffMs": [0, 1000, 2000, 3000, 5000],
    "disconnectPlayerPolicy": "STOP",
    "clearLocalSessionOnFinish": true
  }
}
```

字段说明：

| 字段                      | 说明                   |
| ------------------------- | ---------------------- |
| reconnectWindowSeconds    | 允许重连窗口           |
| heartbeatIntervalSeconds  | 心跳间隔               |
| heartbeatTimeoutSeconds   | 心跳超时时间           |
| maxReconnectAttempts      | 最大重连次数           |
| reconnectBackoffMs        | 重连退避间隔           |
| disconnectPlayerPolicy    | 断线玩家处理策略       |
| clearLocalSessionOnFinish | 结算后是否清理本地会话 |

------

# 22. 异常处理

## 22.1 reconnectToken 无效

处理：

1. 拒绝重连。
2. 客户端返回大厅。
3. 清理本地恢复信息。
4. 提示登录或重新匹配。

------

## 22.2 房间不存在

处理：

1. 返回 ROOM_NOT_FOUND。
2. 客户端查询最近结算。
3. 如果没有结算，返回大厅。
4. 清理本地 roomId。

------

## 22.3 Game Server 不可用

MVP 处理：

1. 重连失败。
2. 客户端返回大厅。
3. 查询最近结算。
4. 提示服务器异常。

完整版本后续支持：

1. 房间迁移。
2. 状态快照持久化恢复。
3. 玩家转移到新 Game Server。

------

## 22.4 重复重连

场景：

```text
客户端短时间发送多个 RECONNECT
```

处理：

1. 使用最新连接。
2. 旧连接关闭。
3. 重复请求幂等返回当前状态。
4. 不重复下发异常状态。

------

# 23. 错误码设计

| 错误码 | 含义               | 客户端处理         |
| ------ | ------------------ | ------------------ |
| 0      | 成功               | 正常恢复           |
| 49001  | 重连凭证无效       | 返回大厅           |
| 49002  | 重连凭证过期       | 返回大厅或查结算   |
| 49003  | 房间不存在         | 查询结算或返回大厅 |
| 49004  | 玩家不在房间       | 返回大厅           |
| 49005  | 重连超时           | 返回大厅或查结算   |
| 49006  | 房间已结束         | 查询结算           |
| 49007  | 房间状态异常       | 返回大厅           |
| 49008  | Game Server 不可用 | 返回大厅           |
| 49009  | 恢复快照生成失败   | 请求重试或返回大厅 |
| 50000  | 系统异常           | 稍后重试           |

------

# 24. 日志设计

## 24.1 关键日志点

| 日志点                     | 说明               |
| -------------------------- | ------------------ |
| ws_connected               | WebSocket 连接建立 |
| ws_disconnected            | WebSocket 断开     |
| heartbeat_timeout          | 心跳超时           |
| player_disconnected        | 玩家断线           |
| reconnect_request          | 收到重连请求       |
| reconnect_success          | 重连成功           |
| reconnect_failed           | 重连失败           |
| reconnect_timeout          | 重连超时           |
| connection_replaced        | 新连接替换旧连接   |
| recover_snapshot_send      | 下发恢复快照       |
| room_finished_on_reconnect | 重连时房间已结束   |

------

## 24.2 日志字段示例

```json
{
  "level": "info",
  "traceId": "trace_xxx",
  "roomId": "r_90001",
  "userId": "10001",
  "connectId": "conn_abc",
  "status": "RECONNECTED",
  "message": "reconnect_success",
  "durationMs": 8,
  "timestamp": "2026-06-28T10:00:00.000Z"
}
```

------

# 25. 监控指标

| 指标                          | 说明                 |
| ----------------------------- | -------------------- |
| ws_connected_total            | WebSocket 连接数     |
| ws_disconnected_total         | WebSocket 断开次数   |
| heartbeat_timeout_total       | 心跳超时次数         |
| reconnect_request_total       | 重连请求数           |
| reconnect_success_total       | 重连成功数           |
| reconnect_failed_total        | 重连失败数           |
| reconnect_timeout_total       | 重连超时数           |
| reconnect_duration_ms         | 重连耗时             |
| recover_snapshot_total        | 恢复快照次数         |
| recover_snapshot_size_bytes   | 恢复快照大小         |
| connection_replaced_total     | 连接替换次数         |
| reconnect_room_finished_total | 重连时房间已结束次数 |

------

# 26. 安全与反作弊

## 26.1 风险点

| 风险                     | 防护                             |
| ------------------------ | -------------------------------- |
| 伪造 reconnectToken      | 服务端校验 token                 |
| 伪造 roomId 重连他人房间 | token 必须绑定 userId + roomId   |
| 多连接刷输入             | 新连接替换旧连接                 |
| 重连绕过死亡状态         | 服务端校验玩家状态               |
| 重连恢复旧快照作弊       | 服务端下发权威恢复快照           |
| 高频重连攻击             | 限制重连频率                     |
| 断线保命                 | 断线玩家保留在地图中或按策略处理 |

------

## 26.2 服务端校验

重连时校验：

1. accessToken 是否有效。
2. reconnectToken 是否有效。
3. reconnectToken 是否过期。
4. token 中 userId 是否与连接用户一致。
5. token 中 roomId 是否与请求 roomId 一致。
6. 房间是否存在。
7. 玩家是否属于该房间。
8. 是否超过重连窗口。
9. 玩家是否已死亡或已结算。
10. 当前连接是否允许替换。

------

# 27. MVP 开发任务拆分

## 27.1 客户端任务

| 任务               | 说明                                  |
| ------------------ | ------------------------------------- |
| WebSocket 断开监听 | 监听 close / error                    |
| 心跳发送           | 定时发送 PING                         |
| 重连遮罩           | 断线后展示重连状态                    |
| 本地会话保存       | 保存 roomId、serverId、reconnectToken |
| 自动重连           | 按退避策略重新连接                    |
| RECONNECT 消息     | 新连接建立后发送                      |
| 恢复快照处理       | 收到 ROOM_RECOVER_SNAPSHOT 后重建场景 |
| 重连失败处理       | 返回大厅或查结算                      |
| 结算找回           | 房间结束后查询结算结果                |
| 本地会话清理       | 结算完成或返回大厅后清理              |

------

## 27.2 服务端任务

| 任务           | 说明                           |
| -------------- | ------------------------------ |
| 心跳处理       | PING / PONG                    |
| 连接管理       | 绑定 userId、roomId、connectId |
| 断线检测       | WebSocket 异常和心跳超时       |
| 玩家断线状态   | 标记 DISCONNECTED              |
| reconnectToken | 生成和校验重连令牌             |
| RECONNECT 处理 | 校验并恢复玩家连接             |
| 连接替换       | 新连接替换旧连接               |
| 恢复快照       | 生成 ROOM_RECOVER_SNAPSHOT     |
| 重连超时扫描   | 超过窗口后处理退出或死亡       |
| 房间结束分支   | 返回结算查询指引               |
| Redis 会话     | 保存 session、route、token     |

------

# 28. 后续详细设计拆分建议

断线重连系统后续可以继续拆成：

1. WebSocket 心跳详细设计。
2. ReconnectToken 详细设计。
3. 客户端自动重连详细设计。
4. 服务端连接替换详细设计。
5. ROOM_RECOVER_SNAPSHOT 设计。
6. AOI 场景下重连恢复设计。
7. 对局结束后的结算找回设计。
8. 弱网状态 UI 设计。
9. Game Server 异常恢复设计。

------

# 29. 总结

断线重连系统是《吞噬细胞》实时多人对战中保证体验稳定的重要模块。它不直接改变玩法规则，但决定了玩家在真实网络环境下是否能顺利继续对局。

核心链路如下：

```text
WebSocket 断开
  ↓
客户端进入重连状态
  ↓
服务端保留玩家房间状态
  ↓
客户端重新连接 Game Server
  ↓
发送 RECONNECT
  ↓
服务端校验 token 和房间状态
  ↓
新连接替换旧连接
  ↓
下发恢复快照
  ↓
客户端恢复对局
```

MVP 阶段应优先保证：

1. WebSocket 断开能被检测。
2. 玩家断线状态能被服务端记录。
3. 短时间内可以重连。
4. 重连成功后下发完整恢复快照。
5. 重连失败时客户端有明确提示。
6. 对局结束后可以查询结算结果。
7. 同一玩家不会出现多个有效连接。
8. 重连不会破坏服务端权威状态。

当前阶段不要过早实现房间迁移、AI 托管和复杂弱网补偿。先把“短线可恢复、长线可结束、结算可找回”这条链路做稳，就能覆盖大多数真实网络异常场景。