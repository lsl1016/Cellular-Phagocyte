# WebSocket 协议处理设计文档

## 1. 文档信息

| 项目         | 内容                                                         |
| ------------ | ------------------------------------------------------------ |
| 文档名称     | WebSocket 协议处理设计文档                                   |
| 所属产品     | 吞噬细胞                                                   |
| 所属端       | Cocos Creator 3.x H5 客户端                                  |
| 服务端       | Go Game Server                                               |
| 通信方式     | WebSocket                                                    |
| MVP 协议格式 | JSON                                                         |
| 后续协议格式 | Protobuf                                                     |
| 覆盖范围     | 入房、Ready、移动输入、技能输入、状态快照、排行榜、游戏结束、结算、心跳、重连、错误处理 |
| 文档定位     | 客户端和 Game Server 实时通信协议设计                        |

------

# 2. 设计目标

WebSocket 协议用于承载实时对战通信。

它需要解决：

```text
客户端如何进入房间？
客户端如何告诉服务端自己准备好了？
客户端如何发送移动、分裂、吐球？
服务端如何下发房间状态？
服务端如何下发局内排行榜？
服务端如何通知游戏结束和结算？
客户端和服务端如何保持心跳？
断线后如何重连恢复？
协议错误如何返回？
```

MVP 阶段优先保证：

1. 协议结构统一。
2. 消息类型清晰。
3. 客户端只发送输入意图。
4. 服务端下发权威状态。
5. 支持消息序号。
6. 支持心跳。
7. 支持错误返回。
8. 支持后续 Protobuf 替换。

------

# 3. 协议设计原则

## 3.1 服务端权威

客户端不能通过 WebSocket 直接上报权威结果。

客户端不能发送：

```text
我的位置是 x=100, y=200
我吃到了某个玩家
我的分数增加了
我获得了第 1 名
我获得了金币
```

客户端只能发送：

```text
我要往哪个方向移动
我要分裂
我要吐球
我已经准备好了
我要重连
```

服务端负责：

1. 移动计算。
2. 碰撞判定。
3. 吞噬判定。
4. 质量变化。
5. 排名计算。
6. 结算计算。
7. 奖励计算。

------

## 3.2 统一消息外壳

所有 WebSocket 消息统一使用 envelope 包装。

好处：

1. 便于消息分发。
2. 便于日志排查。
3. 便于协议版本管理。
4. 便于后续接 Protobuf。
5. 便于错误处理。

------

## 3.3 业务消息按 type 分发

客户端收到消息后，根据 `type` 分发到不同模块。

```text
ROOM_SNAPSHOT → BattleManager
RANK_UPDATE → RankPanel
SETTLEMENT_RESULT → SettlementManager
ERROR → NetworkErrorHandler
```

------

## 3.4 高频消息和低频消息分离

高频消息：

1. MOVE。
2. ROOM_SNAPSHOT。
3. ROOM_DELTA，后续支持。

低频消息：

1. ENTER_ROOM。
2. READY。
3. RANK_UPDATE。
4. GAME_END。
5. SETTLEMENT_RESULT。
6. ERROR。
7. PING / PONG。

高频消息需要控制频率和包体大小。

------

# 4. WebSocket 生命周期

## 4.1 连接生命周期

```text
IDLE
  ↓ connect
CONNECTING
  ↓ onOpen
CONNECTED
  ↓ ENTER_ROOM
AUTHENTICATING
  ↓ ENTER_ROOM_RESULT success
AUTHENTICATED
  ↓ READY
READY
  ↓ GAME_START
PLAYING
  ↓ GAME_END
ENDING
  ↓ SETTLEMENT_RESULT
SETTLED
  ↓ close
CLOSED
```

------

## 4.2 异常生命周期

```text
PLAYING
  ↓ ws close / heartbeat timeout
DISCONNECTED
  ↓ reconnect
RECONNECTING
  ↓ RECONNECT_RESULT success
RECOVERING
  ↓ ROOM_RECOVER_SNAPSHOT
PLAYING
```

重连失败：

```text
RECONNECTING
  ↓ RECONNECT_RESULT failed
RECONNECT_FAILED
  ↓
LobbyScene / SettlementScene
```

------

# 5. 通用消息结构

## 5.1 通用 Envelope

MVP 使用 JSON。

```json
{
  "type": "MOVE",
  "seq": 1001,
  "traceId": "trace_xxx",
  "clientTime": 1710000000000,
  "serverTime": 0,
  "roomId": "r_90001",
  "data": {}
}
```

------

## 5.2 字段说明

| 字段       | 类型   | 必填             | 说明             |
| ---------- | ------ | ---------------- | ---------------- |
| type       | string | 是               | 消息类型         |
| seq        | number | 是               | 消息序号         |
| traceId    | string | 否               | 链路 ID          |
| clientTime | number | 客户端发出时必填 | 客户端毫秒时间戳 |
| serverTime | number | 服务端下发时必填 | 服务端毫秒时间戳 |
| roomId     | string | 房间内消息必填   | 房间 ID          |
| data       | object | 是               | 业务数据         |

------

## 5.3 客户端消息示例

```json
{
  "type": "MOVE",
  "seq": 1001,
  "traceId": "trace_abc",
  "clientTime": 1710000000000,
  "roomId": "r_90001",
  "data": {
    "direction": {
      "x": 0.86,
      "y": 0.51
    }
  }
}
```

------

## 5.4 服务端消息示例

```json
{
  "type": "ROOM_SNAPSHOT",
  "seq": 3001,
  "traceId": "trace_abc",
  "serverTime": 1710000000050,
  "roomId": "r_90001",
  "data": {
    "tickSeq": 520,
    "players": [],
    "foods": [],
    "events": []
  }
}
```

------

# 6. 消息分类

## 6.1 客户端发送消息

| 消息类型   | 说明               | 频率 |
| ---------- | ------------------ | ---- |
| ENTER_ROOM | 入房认证           | 低频 |
| READY      | 客户端资源加载完成 | 低频 |
| MOVE       | 移动方向输入       | 高频 |
| SPLIT      | 分裂输入           | 低频 |
| EJECT      | 吐球输入           | 中频 |
| PING       | 心跳               | 低频 |
| RECONNECT  | 重连请求           | 低频 |

------

## 6.2 服务端下发消息

| 消息类型              | 说明                      | 频率 |
| --------------------- | ------------------------- | ---- |
| ENTER_ROOM_RESULT     | 入房认证结果              | 低频 |
| PLAYER_READY          | 玩家 Ready 状态，后续支持 | 低频 |
| START_COUNTDOWN       | 开局倒计时                | 低频 |
| GAME_START            | 游戏开始                  | 低频 |
| ROOM_SNAPSHOT         | 房间全量快照              | 高频 |
| ROOM_DELTA            | 房间增量快照，后续支持    | 高频 |
| RANK_UPDATE           | 局内排行榜                | 低频 |
| SKILL_FAILED          | 技能失败                  | 低频 |
| GAME_END              | 游戏结束                  | 低频 |
| SETTLEMENT_PROCESSING | 结算处理中                | 低频 |
| SETTLEMENT_RESULT     | 结算结果                  | 低频 |
| SETTLEMENT_FAILED     | 结算失败                  | 低频 |
| PONG                  | 心跳响应                  | 低频 |
| RECONNECT_RESULT      | 重连结果                  | 低频 |
| ROOM_RECOVER_SNAPSHOT | 重连恢复快照              | 低频 |
| ERROR                 | 通用错误                  | 低频 |

------

# 7. 客户端发送协议

## 7.1 ENTER_ROOM

### 触发时机

客户端进入 GameScene，WebSocket 连接成功后立即发送。

### 用途

1. 认证玩家是否有权限进入房间。
2. 绑定 userId、roomId、connectId。
3. 获取 reconnectToken。
4. 进入房间会话。

### 请求

```json
{
  "type": "ENTER_ROOM",
  "seq": 1,
  "traceId": "trace_xxx",
  "clientTime": 1710000000000,
  "roomId": "r_90001",
  "data": {
    "enterToken": "enter_token_xxx",
    "clientVersion": "1.0.0"
  }
}
```

### 字段说明

| 字段          | 说明                           |
| ------------- | ------------------------------ |
| roomId        | 房间 ID                        |
| enterToken    | 匹配成功后服务端返回的入房令牌 |
| clientVersion | 客户端版本                     |

------

## 7.2 READY

### 触发时机

客户端入房成功，并且对战资源加载完成后发送。

### 请求

```json
{
  "type": "READY",
  "seq": 2,
  "traceId": "trace_xxx",
  "clientTime": 1710000000100,
  "roomId": "r_90001",
  "data": {
    "loaded": true
  }
}
```

### 说明

READY 表示客户端已经准备好接收游戏开始消息。

------

## 7.3 MOVE

### 触发时机

玩家拖动摇杆时，客户端按固定频率发送。

### 推荐频率

```text
20 次 / 秒
```

即：

```text
每 50ms 发送一次
```

### 请求

```json
{
  "type": "MOVE",
  "seq": 1001,
  "traceId": "trace_xxx",
  "clientTime": 1710000000200,
  "roomId": "r_90001",
  "data": {
    "direction": {
      "x": 0.86,
      "y": 0.51
    }
  }
}
```

### direction 规则

| 字段 | 说明    |
| ---- | ------- |
| x    | -1 到 1 |
| y    | -1 到 1 |

要求：

```text
sqrt(x*x + y*y) <= 1
```

如果摇杆未移动：

```json
{
  "x": 0,
  "y": 0
}
```

### 客户端不发送

```text
坐标
速度
质量
是否碰撞
是否吞噬
```

------

## 7.4 SPLIT

### 触发时机

玩家点击分裂按钮。

### 请求

```json
{
  "type": "SPLIT",
  "seq": 1101,
  "traceId": "trace_xxx",
  "clientTime": 1710000000300,
  "roomId": "r_90001",
  "data": {
    "direction": {
      "x": 1,
      "y": 0
    }
  }
}
```

### 说明

方向来自当前摇杆方向。
如果摇杆没有方向，可以使用玩家当前移动方向或默认朝右，具体由客户端策略决定。

服务端最终校验：

1. 质量是否足够。
2. 分身数量是否超过上限。
3. 技能冷却是否结束。
4. 玩家是否存活。
5. 房间是否运行中。

------

## 7.5 EJECT

### 触发时机

玩家点击或长按吐球按钮。

### 请求

```json
{
  "type": "EJECT",
  "seq": 1201,
  "traceId": "trace_xxx",
  "clientTime": 1710000000400,
  "roomId": "r_90001",
  "data": {
    "direction": {
      "x": 1,
      "y": 0
    }
  }
}
```

### 客户端限频

MVP 推荐：

```text
每 150ms 最多发送一次
```

服务端仍需要再次限频。

------

## 7.6 PING

### 触发时机

HeartbeatClient 定时发送。

### 请求

```json
{
  "type": "PING",
  "seq": 9001,
  "traceId": "trace_xxx",
  "clientTime": 1710000000000,
  "roomId": "r_90001",
  "data": {}
}
```

------

## 7.7 RECONNECT

### 触发时机

WebSocket 断开后，客户端重新连接成功，发送重连请求。

### 请求

```json
{
  "type": "RECONNECT",
  "seq": 9101,
  "traceId": "trace_xxx",
  "clientTime": 1710000000000,
  "roomId": "r_90001",
  "data": {
    "reconnectToken": "reconnect_token_xxx",
    "lastClientSnapshotSeq": 3001,
    "lastClientTickSeq": 520
  }
}
```

------

# 8. 服务端下发协议

## 8.1 ENTER_ROOM_RESULT

### 触发时机

服务端处理 ENTER_ROOM 后返回。

### 成功响应

```json
{
  "type": "ENTER_ROOM_RESULT",
  "seq": 1,
  "traceId": "trace_xxx",
  "serverTime": 1710000000050,
  "roomId": "r_90001",
  "data": {
    "success": true,
    "userId": "10001",
    "roomStatus": "WAITING",
    "reconnectToken": "reconnect_token_xxx"
  }
}
```

### 失败响应

```json
{
  "type": "ENTER_ROOM_RESULT",
  "seq": 1,
  "traceId": "trace_xxx",
  "serverTime": 1710000000050,
  "roomId": "r_90001",
  "data": {
    "success": false,
    "errorCode": 30011,
    "message": "入房凭证无效"
  }
}
```

------

## 8.2 START_COUNTDOWN

### 用途

通知客户端即将开始游戏。

```json
{
  "type": "START_COUNTDOWN",
  "seq": 5,
  "serverTime": 1710000000500,
  "roomId": "r_90001",
  "data": {
    "countdownSeconds": 3
  }
}
```

MVP 可以不单独实现，直接等待 GAME_START。

------

## 8.3 GAME_START

### 用途

通知客户端对局正式开始。

```json
{
  "type": "GAME_START",
  "seq": 10,
  "serverTime": 1710000001000,
  "roomId": "r_90001",
  "data": {
    "startTime": 1710000001000,
    "durationSeconds": 300,
    "map": {
      "width": 10000,
      "height": 10000
    },
    "self": {
      "userId": "10001",
      "nickname": "吞噬细胞1234"
    }
  }
}
```

客户端收到后：

1. 关闭加载页。
2. 开启输入。
3. 开始接收快照。
4. 展示 HUD。
5. 启动倒计时显示。

------

## 8.4 ROOM_SNAPSHOT

### 用途

服务端定时下发房间权威状态。

### 推荐频率

```text
10 次 / 秒
```

即：

```text
每 100ms 下发一次
```

### 消息结构

```json
{
  "type": "ROOM_SNAPSHOT",
  "seq": 3001,
  "serverTime": 1710000002000,
  "roomId": "r_90001",
  "data": {
    "tickSeq": 520,
    "snapshotType": "FULL",
    "remainingSeconds": 280,
    "selfUserId": "10001",
    "players": [
      {
        "userId": "10001",
        "nickname": "吞噬细胞1234",
        "alive": true,
        "balls": [
          {
            "ballId": "b_10001_1",
            "x": 1200,
            "y": 800,
            "radius": 36,
            "mass": 200
          }
        ]
      }
    ],
    "foods": [
      {
        "foodId": "f_1",
        "x": 1300,
        "y": 900,
        "radius": 5,
        "mass": 1,
        "color": "#ffcc00"
      }
    ],
    "ejectedMass": [],
    "viruses": [],
    "events": []
  }
}
```

------

## 8.5 快照字段说明

| 字段             | 说明                                |
| ---------------- | ----------------------------------- |
| tickSeq          | 服务端 Tick 序号                    |
| snapshotType     | FULL / DELTA / AOI_FULL / AOI_DELTA |
| remainingSeconds | 剩余时间                            |
| selfUserId       | 当前玩家 ID                         |
| players          | 玩家列表                            |
| foods            | 食物列表                            |
| ejectedMass      | 吐出物                              |
| viruses          | 刺球                                |
| events           | 本次快照中的事件                    |

------

## 8.6 玩家对象结构

```json
{
  "userId": "10001",
  "nickname": "吞噬细胞1234",
  "alive": true,
  "balls": [
    {
      "ballId": "b_10001_1",
      "x": 1200,
      "y": 800,
      "radius": 36,
      "mass": 200
    }
  ]
}
```

说明：

1. 一个玩家可以有多个球体。
2. MVP 可以先只支持一个球体。
3. 后续分裂后，一个玩家会有多个 ball。

------

## 8.7 事件结构

快照中可以携带事件。

示例：

```json
{
  "eventId": "evt_1",
  "eventType": "PLAYER_EATEN",
  "serverTime": 1710000002000,
  "data": {
    "attackerUserId": "10001",
    "targetUserId": "10002"
  }
}
```

常见事件：

| eventType       | 说明               |
| --------------- | ------------------ |
| FOOD_EATEN      | 食物被吞噬         |
| PLAYER_EATEN    | 玩家被吞噬         |
| PLAYER_DEAD     | 玩家死亡           |
| PLAYER_SPLIT    | 玩家分裂           |
| PLAYER_EJECT    | 玩家吐球           |
| VIRUS_TRIGGERED | 触发刺球，后续支持 |

------

## 8.8 RANK_UPDATE

### 用途

下发局内排行榜。

### 推荐频率

```text
1 次 / 秒
```

### 消息结构

```json
{
  "type": "RANK_UPDATE",
  "seq": 4001,
  "serverTime": 1710000003000,
  "roomId": "r_90001",
  "data": {
    "rankSeq": 12,
    "topN": [
      {
        "rank": 1,
        "userId": "10001",
        "nickname": "吞噬细胞1234",
        "score": 3680
      }
    ],
    "selfRank": {
      "rank": 1,
      "score": 3680,
      "totalPlayers": 100
    }
  }
}
```

------

## 8.9 SKILL_FAILED

### 用途

服务端拒绝分裂或吐球时返回原因。

```json
{
  "type": "SKILL_FAILED",
  "seq": 1101,
  "serverTime": 1710000003050,
  "roomId": "r_90001",
  "data": {
    "skillType": "SPLIT",
    "reason": "MASS_NOT_ENOUGH",
    "message": "质量不足，无法分裂"
  }
}
```

------

## 8.10 GAME_END

### 用途

通知客户端对局结束。

```json
{
  "type": "GAME_END",
  "seq": 5001,
  "serverTime": 1710000300000,
  "roomId": "r_90001",
  "data": {
    "reason": "TIME_END",
    "message": "游戏结束"
  }
}
```

客户端收到后：

1. 停止发送 MOVE。
2. 禁用技能按钮。
3. 展示结算 loading。
4. 等待 SETTLEMENT_RESULT。
5. 如果超时，调用 HTTP 查询结算。

------

## 8.11 SETTLEMENT_PROCESSING

```json
{
  "type": "SETTLEMENT_PROCESSING",
  "seq": 5002,
  "serverTime": 1710000300050,
  "roomId": "r_90001",
  "data": {
    "message": "正在生成结算结果"
  }
}
```

------

## 8.12 SETTLEMENT_RESULT

```json
{
  "type": "SETTLEMENT_RESULT",
  "seq": 5003,
  "serverTime": 1710000300100,
  "roomId": "r_90001",
  "data": {
    "rank": 3,
    "totalPlayers": 100,
    "finalScore": 3680,
    "maxMass": 2560,
    "eatPlayerCount": 7,
    "eatFoodCount": 236,
    "aliveSeconds": 280,
    "coinReward": 320,
    "expReward": 180,
    "isBestScore": true
  }
}
```

------

## 8.13 SETTLEMENT_FAILED

```json
{
  "type": "SETTLEMENT_FAILED",
  "seq": 5004,
  "serverTime": 1710000300100,
  "roomId": "r_90001",
  "data": {
    "errorCode": 46003,
    "message": "结算处理中，请稍后在战绩中查看"
  }
}
```

------

## 8.14 PONG

```json
{
  "type": "PONG",
  "seq": 9001,
  "serverTime": 1710000000050,
  "roomId": "r_90001",
  "data": {}
}
```

------

## 8.15 RECONNECT_RESULT

### 成功

```json
{
  "type": "RECONNECT_RESULT",
  "seq": 9101,
  "serverTime": 1710000000050,
  "roomId": "r_90001",
  "data": {
    "success": true,
    "status": "RECONNECTED",
    "message": "重连成功"
  }
}
```

### 失败

```json
{
  "type": "RECONNECT_RESULT",
  "seq": 9101,
  "serverTime": 1710000000050,
  "roomId": "r_90001",
  "data": {
    "success": false,
    "reason": "RECONNECT_TIMEOUT",
    "message": "重连超时，已退出对局"
  }
}
```

------

## 8.16 ROOM_RECOVER_SNAPSHOT

```json
{
  "type": "ROOM_RECOVER_SNAPSHOT",
  "seq": 9102,
  "serverTime": 1710000000060,
  "roomId": "r_90001",
  "data": {
    "tickSeq": 1520,
    "snapshotType": "FULL_RECOVER",
    "remainingSeconds": 120,
    "players": [],
    "foods": [],
    "ejectedMass": [],
    "viruses": [],
    "rank": {},
    "events": []
  }
}
```

客户端收到后：

1. 清空本地 BattleState。
2. 清空 EntityManager。
3. 重建所有对象。
4. 恢复排行榜。
5. 关闭重连遮罩。
6. 继续发送输入。

------

# 9. ERROR 通用错误协议

## 9.1 消息结构

```json
{
  "type": "ERROR",
  "seq": 9999,
  "serverTime": 1710000000000,
  "roomId": "r_90001",
  "data": {
    "errorCode": 49001,
    "reason": "TOKEN_INVALID",
    "message": "重连凭证无效",
    "requestType": "RECONNECT"
  }
}
```

------

## 9.2 常见错误码

| errorCode | reason               | 说明               |
| --------- | -------------------- | ------------------ |
| 30011     | ENTER_TOKEN_INVALID  | 入房凭证无效       |
| 30012     | ROOM_NOT_FOUND       | 房间不存在         |
| 30013     | ROOM_FULL            | 房间已满           |
| 30014     | ROOM_STATUS_INVALID  | 房间状态不允许入房 |
| 40001     | MESSAGE_INVALID      | 消息格式错误       |
| 40002     | MESSAGE_TYPE_UNKNOWN | 消息类型未知       |
| 40003     | MESSAGE_TOO_FREQUENT | 消息发送过于频繁   |
| 44003     | MASS_NOT_ENOUGH      | 质量不足           |
| 44005     | SPLIT_COOLDOWN       | 分裂冷却中         |
| 44006     | EJECT_COOLDOWN       | 吐球冷却中         |
| 49001     | TOKEN_INVALID        | 重连凭证无效       |
| 49005     | RECONNECT_TIMEOUT    | 重连超时           |
| 50000     | SERVER_ERROR         | 服务端异常         |

------

# 10. 客户端消息处理状态机

## 10.1 GameScene 状态

| 状态          | 可处理消息                                                   |
| ------------- | ------------------------------------------------------------ |
| INIT          | 无                                                           |
| WS_CONNECTING | 无                                                           |
| ENTERING_ROOM | ENTER_ROOM_RESULT                                            |
| LOADING       | GAME_START / ERROR                                           |
| PLAYING       | ROOM_SNAPSHOT / RANK_UPDATE / GAME_END / SKILL_FAILED        |
| ENDING        | SETTLEMENT_PROCESSING / SETTLEMENT_RESULT / SETTLEMENT_FAILED |
| RECONNECTING  | RECONNECT_RESULT / ROOM_RECOVER_SNAPSHOT                     |
| CLOSED        | 不处理对战消息                                               |

------

## 10.2 状态约束

例如：

1. 未进入 PLAYING 前，不发送 MOVE。
2. 收到 GAME_END 后，停止 MOVE。
3. CLOSED 状态下丢弃 ROOM_SNAPSHOT。
4. RECONNECTING 状态下暂停输入。
5. SETTLEMENT_RESULT 只处理一次。

------

# 11. 客户端发送频率与限流

## 11.1 MOVE 限流

```text
moveSendIntervalMs = 50
```

客户端每 50ms 发送一次最新方向。

如果方向未变化，仍可低频发送用于保持操作连续性。
MVP 可以简单每 50ms 发送。

------

## 11.2 SPLIT 限流

```text
splitCooldownMs = 1000
```

客户端本地限制用于减少无效请求。
服务端必须再次校验。

------

## 11.3 EJECT 限流

```text
ejectIntervalMs = 150
```

长按吐球时，最多每 150ms 发送一次。

------

## 11.4 PING 频率

```text
heartbeatIntervalMs = 5000
```

------

# 12. 客户端消息去重与丢弃策略

## 12.1 快照旧包丢弃

客户端保存：

```text
lastSnapshotSeq
lastTickSeq
```

如果收到：

```text
snapshot.seq <= lastSnapshotSeq
```

则丢弃。

------

## 12.2 结算结果只处理一次

客户端保存：

```text
settlementHandled = true
```

如果重复收到 SETTLEMENT_RESULT：

```text
忽略重复消息
```

------

## 12.3 GAME_END 只处理一次

收到 GAME_END 后：

1. 设置 roomStatus = ENDING。
2. 停止输入。
3. 显示结算中。
4. 后续重复 GAME_END 忽略。

------

# 13. 协议版本设计

## 13.1 版本字段

可以在 ENTER_ROOM 中携带：

```json
{
  "clientVersion": "1.0.0",
  "protocolVersion": "json-v1"
}
```

------

## 13.2 服务端兼容

服务端根据 protocolVersion 判断：

1. 是否允许连接。
2. 是否需要返回版本不兼容错误。
3. 是否启用 Protobuf。
4. 是否启用增量快照。

MVP 可以先只支持：

```text
json-v1
```

------

# 14. 协议演进规划

## 14.1 MVP：JSON 全量快照

特点：

1. 实现简单。
2. 易调试。
3. 浏览器开发方便。
4. 包体较大。
5. 性能一般。

适合 MVP。

------

## 14.2 V1：JSON + 增量快照

支持：

1. ROOM_DELTA。
2. entered / updated / left / deleted。
3. 减少包体。
4. 配合 AOI。

------

## 14.3 V2：Protobuf

支持：

1. 二进制编码。
2. 减少包体。
3. 降低解析成本。
4. 适合正式版本。

------

## 14.4 V3：压缩和分包

后续再考虑：

1. 大快照压缩。
2. 分片传输。
3. 按实体类型分批。
4. 优先同步玩家，低频同步食物。

------

# 15. 客户端模块处理关系

```text
WsClient
  ↓ 收到原始消息
MessageCodec
  ↓ 解码
WsMessageDispatcher
  ↓ 按 type 分发
BattleManager / RankPanel / SettlementManager / ReconnectClient
  ↓
GameState / UI / EntityManager
```

------

# 16. MVP 开发任务拆分

## 16.1 协议基础

| 任务                | 说明             |
| ------------------- | ---------------- |
| MessageType         | 定义所有消息类型 |
| WsEnvelope          | 定义通用消息结构 |
| MessageCodec        | JSON 编解码      |
| SeqGenerator        | 客户端消息序号   |
| WsMessageDispatcher | 消息分发         |
| ErrorMessageHandler | ERROR 消息处理   |

------

## 16.2 客户端发送消息

| 任务          | 说明               |
| ------------- | ------------------ |
| sendEnterRoom | 发送 ENTER_ROOM    |
| sendReady     | 发送 READY         |
| sendMove      | 发送 MOVE          |
| sendSplit     | 发送 SPLIT，P1     |
| sendEject     | 发送 EJECT，P1     |
| sendPing      | 发送 PING          |
| sendReconnect | 发送 RECONNECT，P1 |

------

## 16.3 服务端下发处理

| 任务                   | 说明         |
| ---------------------- | ------------ |
| handleEnterRoomResult  | 处理入房结果 |
| handleGameStart        | 处理游戏开始 |
| handleRoomSnapshot     | 处理房间快照 |
| handleRankUpdate       | 处理局内排行 |
| handleSkillFailed      | 处理技能失败 |
| handleGameEnd          | 处理游戏结束 |
| handleSettlementResult | 处理结算结果 |
| handlePong             | 处理心跳响应 |
| handleError            | 处理通用错误 |

------

# 17. 当前阶段不实现

MVP 阶段暂不实现：

```text
Protobuf
消息压缩
消息加密
增量快照
AOI 增量协议
输入 ACK
客户端预测回滚
复杂弱网补偿
跨 Game Server 重连
房间迁移恢复
```

只预留模块和字段，不进入第一阶段开发。

------

# 18. 总结

WebSocket 协议处理的核心目标是打通实时对战链路：

```text
连接 Game Server
  ↓
ENTER_ROOM 入房
  ↓
READY 准备
  ↓
GAME_START 开始
  ↓
MOVE 输入
  ↓
ROOM_SNAPSHOT 同步
  ↓
RANK_UPDATE 排名
  ↓
GAME_END 结束
  ↓
SETTLEMENT_RESULT 结算
```

MVP 阶段坚持三个原则：

1. 客户端只发送输入意图。
2. 服务端下发权威状态。
3. 协议结构统一，便于后续升级。

当前先使用 JSON 协议，方便调试和快速落地。
等核心链路稳定后，再逐步升级到增量快照、AOI 协议、Protobuf 和压缩传输。