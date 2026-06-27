# 《吞噬细胞》结算系统 PRD + 技术设计文档

## 1. 文档信息

| 项目     | 内容                                                         |
| -------- | ------------------------------------------------------------ |
| 文档名称 | 结算系统 PRD + 技术设计文档                                  |
| 所属产品 | 吞噬细胞                                                     |
| 所属模块 | 结算系统                                                     |
| 上游模块 | Game Server / Tick 系统、局内排行榜系统                      |
| 下游模块 | 战绩系统、用户资产系统、全服排行榜系统、任务系统，后续支持   |
| 客户端   | Cocos Creator 3.x + TypeScript                               |
| 服务端   | Go                                                           |
| 通信方式 | WebSocket + HTTP                                             |
| 存储依赖 | MySQL、Redis                                                 |
| 文档定位 | 产品需求 + 技术概要设计                                      |
| 详细程度 | 中等详细，后续可继续拆分奖励规则、战绩表、资产流水、全服排行榜等详细设计 |

------

# 第一部分：结算系统 PRD

## 2. 模块背景

《吞噬细胞》是一款实时多人吞噬竞技游戏。每一局游戏结束后，系统需要根据玩家在本局中的表现生成结算结果。

结算系统负责把 Game Server 中的临时对局状态转化为最终结果，包括：

1. 最终排名。
2. 最终得分。
3. 玩家表现统计。
4. 金币奖励。
5. 经验奖励。
6. 历史战绩。
7. 用户资产变化。
8. 全服排行榜更新，后续支持。
9. 结算页展示数据。

结算系统是“实时对战”和“长期成长”的分界点。
对局中的状态主要在 Game Server 内存中，结算后的结果需要进入持久化系统。

------

## 3. 模块目标

### 3.1 产品目标

| 目标             | 说明                               |
| ---------------- | ---------------------------------- |
| 明确展示本局结果 | 玩家能看到最终排名、得分和表现     |
| 正确发放奖励     | 根据排名和表现发放金币、经验等奖励 |
| 保存历史战绩     | 玩家可以在战绩页查看历史对局       |
| 支持再来一局     | 结算后可快速重新匹配               |
| 支持返回大厅     | 结算后可回到游戏大厅               |
| 保持结果可信     | 排名、得分、奖励由服务端计算       |
| 防止重复发奖     | 重连、重试、重复请求不能重复结算   |
| 支持后续成长体系 | 为等级、段位、任务、成就预留数据   |

### 3.2 技术目标

| 目标                | 说明                                     |
| ------------------- | ---------------------------------------- |
| 服务端权威结算      | 结算数据由服务端生成                     |
| 结算幂等            | 同一 roomId + userId 只能成功结算一次    |
| 数据一致性          | 战绩、资产、奖励流水需要保持一致         |
| 异常可恢复          | 结算失败可重试，不应丢失结果             |
| 与 Game Server 解耦 | Game Server 触发结算，结算服务负责持久化 |
| 支持异步扩展        | 后续支持异步更新任务、成就、全服榜       |
| 可观测              | 记录结算耗时、失败次数、重复结算次数     |

------

## 4. 模块定位

结算系统位于 Game Server 和持久化系统之间。

```text
Game Server
  ↓
最终排名 / 玩家表现
  ↓
Settlement Service
  ↓
MySQL / Redis
  ↓
结算结果
  ↓
客户端结算页
```

结算系统不负责实时对战逻辑，也不负责局内排行榜实时推送。
它只负责一局结束后的最终结果计算、保存和返回。

------

## 5. 核心设计原则

## 5.1 服务端生成结果

客户端不能提交结算结果。

客户端不能提交：

```text
我第 1 名
我获得 500 金币
我吞噬了 10 个玩家
我的最终得分是 9999
```

客户端只能接收服务端结算结果并展示。

------

## 5.2 先冻结，再结算

对局结束时，Game Server 必须先冻结房间状态，再触发结算。

```text
对局结束
  ↓
停止接收核心输入
  ↓
冻结最终房间状态
  ↓
生成 FinalRankList
  ↓
触发结算
```

避免在结算过程中继续发生移动、吞噬、分裂、吐球等影响结果的操作。

------

## 5.3 幂等优先

结算系统必须防止重复发奖。

典型重复场景：

1. Game Server 重试调用结算接口。
2. 客户端重复请求结算结果。
3. 玩家断线重连后再次拉取结算。
4. 服务端超时但实际已经写入成功。
5. 消息重复投递。

核心约束：

```text
同一个 roomId + userId 只能生成一条有效结算记录
```

------

## 5.4 结算结果可追溯

每次结算需要保存：

1. 原始对局结果。
2. 排名数据。
3. 奖励计算结果。
4. 资产变更流水。
5. 结算状态。
6. 失败原因，异常时记录。

这样方便后续排查问题和补偿。

------

# 6. 用户场景

## 6.1 正常结算

### 场景描述

一局游戏达到结束时间，服务端冻结状态并计算最终排名，玩家进入结算页。

### 用户流程

```text
对局结束
  ↓
客户端收到 GAME_END
  ↓
等待结算结果
  ↓
服务端计算奖励
  ↓
客户端收到 SETTLEMENT_RESULT
  ↓
展示结算页
  ↓
玩家选择返回大厅或再来一局
```

------

## 6.2 玩家提前死亡后等待结算

### 场景描述

玩家在局内被吞噬死亡，但本局还未结束。

### 产品规则

1. 玩家死亡后进入观战或等待结算状态。
2. 死亡时记录死亡得分和存活时间。
3. 对局结束后统一结算。
4. 结算页展示死亡玩家本局表现。

MVP 阶段可简化为：

```text
死亡后等待本局结束，再统一结算
```

------

## 6.3 玩家断线后结算

### 场景描述

玩家在游戏结束前断线，或结算过程中断线。

### 处理规则

1. 只要玩家参与了本局，就应生成结算记录。
2. 玩家重新进入游戏后可查询最近一局结算。
3. 如果结算结果已生成，客户端直接展示。
4. 不因客户端断线导致结算丢失。

------

## 6.4 客户端重复请求结算结果

### 场景描述

客户端未收到结算结果，重复调用查询接口。

### 处理规则

1. 如果结算已完成，返回已有结果。
2. 如果结算中，返回 PROCESSING。
3. 如果结算失败，返回失败原因或触发重试。
4. 不重复发放奖励。

------

## 6.5 结算失败

### 场景描述

结算服务或数据库异常，导致结算没有完成。

### 处理规则

1. 结算状态标记为 FAILED。
2. 记录失败原因。
3. 支持后续重试。
4. 客户端提示“结算处理中，请稍后查看”。
5. 不允许生成部分奖励后丢失记录。

------

# 7. 功能范围

## 7.1 MVP 版本功能范围

MVP 阶段需要实现：

1. Game Server 触发结算。
2. 接收 FinalRankList。
3. 计算最终排名。
4. 计算金币奖励。
5. 计算经验奖励。
6. 保存对局记录。
7. 保存玩家结算记录。
8. 更新用户资产。
9. 生成资产流水。
10. 返回结算结果。
11. 客户端展示结算页。
12. 支持查询结算结果。
13. 支持结算幂等。

------

## 7.2 完整版本功能范围

完整版本可扩展：

1. 段位积分变化。
2. 等级经验升级。
3. 任务进度更新。
4. 成就进度更新。
5. 活动奖励。
6. 全服排行榜更新。
7. 赛季积分。
8. 结算补偿。
9. 结算重试队列。
10. 结算审计后台。
11. 结算异常人工处理。
12. 多模式奖励配置。

------

## 7.3 暂不包含范围

当前文档不详细展开：

1. 全服排行榜详细设计。
2. 段位系统。
3. 赛季系统。
4. 付费系统。
5. 商城资产系统。
6. 任务系统详细设计。
7. 成就系统详细设计。
8. 复杂风控策略。

------

# 8. 结算触发条件

## 8.1 正常结束

MVP 阶段推荐使用固定对局时长。

```text
battleDurationSeconds 到达
  ↓
Game Server 触发 GAME_END
  ↓
进入结算
```

例如：

```text
对局时长：300 秒
```

------

## 8.2 提前结束

后续可支持：

1. 房间只剩 1 个存活玩家。
2. 某玩家达到目标分数。
3. 全部玩家退出。
4. 服务器主动关闭房间。
5. 活动玩法特殊目标完成。

------

## 8.3 异常结束

异常场景：

1. Game Server 异常。
2. 房间状态损坏。
3. 玩家数量异常。
4. 房间强制关闭。

处理规则：

1. 能获取最终状态则正常结算。
2. 不能获取最终状态则按异常房间处理。
3. 已产生的有效表现尽量保存。
4. 必要时返回补偿结果，后续支持。

------

# 9. 结算展示设计

## 9.1 结算页核心内容

结算页需要展示：

| 内容         | 说明                       |
| ------------ | -------------------------- |
| 最终排名     | 本局第几名                 |
| 总人数       | 本局参与人数               |
| 最终得分     | 本局最终分数               |
| 最大质量     | 本局达到的最大质量         |
| 吞噬玩家数   | 本局吞噬玩家数量           |
| 吞噬食物数   | 本局吞噬食物数量           |
| 存活时间     | 本局存活时长               |
| 金币奖励     | 本局获得金币               |
| 经验奖励     | 本局获得经验               |
| 是否刷新最佳 | 是否刷新个人最佳，后续支持 |
| 返回大厅     | 回到大厅                   |
| 再来一局     | 重新匹配                   |

------

## 9.2 结算页文案

| 场景     | 文案                     |
| -------- | ------------------------ |
| 第 1 名  | 本局第 1 名，统治全场！  |
| 前 3 名  | 表现出色，继续冲榜！     |
| 前 10 名 | 成功进入前十！           |
| 普通排名 | 本局表现不错，再接再厉！ |
| 死亡结算 | 本局已被吞噬，继续努力！ |
| 结算中   | 正在生成结算结果...      |
| 结算失败 | 结算处理中，请稍后查看   |

------

## 9.3 再来一局流程

```text
结算页点击再来一局
  ↓
客户端关闭结算页
  ↓
请求开始匹配
  ↓
进入匹配页
```

注意：

1. 再来一局不复用旧房间。
2. 需要重新走匹配流程。
3. 结算记录保存不应受到再来一局影响。

------

## 9.4 返回大厅流程

```text
结算页点击返回大厅
  ↓
断开或释放旧房间连接
  ↓
回到大厅页
  ↓
刷新用户资产和基础信息
```

------

# 10. 奖励规则设计

## 10.1 MVP 奖励类型

MVP 阶段建议只支持：

1. 金币。
2. 经验。

不建议一开始加入复杂奖励类型。

------

## 10.2 金币奖励

金币奖励可由排名和表现共同决定。

推荐公式：

```text
coinReward = baseCoin
           + rankCoinBonus
           + eatPlayerCount * eatPlayerCoin
           + eatFoodCount * eatFoodCoin
```

示例配置：

```json
{
  "baseCoin": 50,
  "eatPlayerCoin": 10,
  "eatFoodCoin": 0.1,
  "rankBonus": {
    "1": 300,
    "2": 200,
    "3": 150,
    "4-10": 80,
    "11-30": 30,
    "31-100": 0
  }
}
```

------

## 10.3 经验奖励

经验奖励可由排名、存活时间、吞噬数量决定。

推荐公式：

```text
expReward = baseExp
          + rankExpBonus
          + aliveSeconds * aliveExpRate
          + eatPlayerCount * eatPlayerExp
```

示例配置：

```json
{
  "baseExp": 30,
  "aliveExpRate": 0.2,
  "eatPlayerExp": 5,
  "rankExpBonus": {
    "1": 200,
    "2": 150,
    "3": 120,
    "4-10": 60,
    "11-30": 30,
    "31-100": 0
  }
}
```

------

## 10.4 奖励上限

为了避免异常数据导致奖励过高，需要设置上限。

```json
{
  "maxCoinRewardPerGame": 1000,
  "maxExpRewardPerGame": 800
}
```

处理规则：

```text
如果计算奖励超过上限
  ↓
按上限发放
  ↓
记录异常日志
```

------

# 第二部分：结算系统技术设计

## 11. 总体架构

## 11.1 模块关系

```text
Game Server
  ↓ FinalRankList / PlayerStats
Settlement Service
  ↓
Reward Calculator
  ↓
MySQL Transaction
  ├── game_records
  ├── game_player_results
  ├── settlement_records
  ├── user_assets
  └── asset_change_logs
  ↓
Redis Rank / Cache，后续支持
  ↓
SETTLEMENT_RESULT
  ↓
Client Settlement Page
```

------

## 11.2 服务端模块

```text
Settlement Service
├── SettlementController
├── SettlementProcessor
├── RewardCalculator
├── RecordWriter
├── AssetService
├── IdempotencyManager
├── RankUpdater
├── ResultQueryService
├── RetryWorker，后续支持
└── SettlementMetrics
```

模块说明：

| 模块                 | 职责                      |
| -------------------- | ------------------------- |
| SettlementController | 接收 Game Server 结算请求 |
| SettlementProcessor  | 编排结算流程              |
| RewardCalculator     | 计算金币、经验等奖励      |
| RecordWriter         | 写入对局和玩家结果        |
| AssetService         | 更新用户资产和流水        |
| IdempotencyManager   | 保证结算幂等              |
| RankUpdater          | 更新全服排行榜，后续支持  |
| ResultQueryService   | 查询结算结果              |
| RetryWorker          | 失败结算重试，后续支持    |
| SettlementMetrics    | 记录指标                  |

------

# 12. 结算流程设计

## 12.1 正常结算流程

```text
1. Game Server 判断对局结束
2. Game Server 冻结房间状态
3. RankSystem 生成 FinalRankList
4. Game Server 调用 Settlement Service
5. Settlement Service 校验 roomId 和玩家结果
6. Settlement Service 执行幂等检查
7. RewardCalculator 计算每个玩家奖励
8. 开启 MySQL 事务
9. 写入 game_records
10. 写入 game_player_results
11. 写入 settlement_records
12. 更新 user_assets
13. 写入 asset_change_logs
14. 提交事务
15. 更新 Redis 排行榜，后续支持
16. 返回结算结果给 Game Server
17. Game Server 推送 SETTLEMENT_RESULT 给客户端
18. 房间进入 FINISHED
19. Game Server 清理房间资源
```

------

## 12.2 查询结算结果流程

```text
1. 客户端请求结算结果
2. API Server 校验用户身份
3. 查询 settlement_records
4. 如果已完成，返回结算结果
5. 如果处理中，返回 PROCESSING
6. 如果失败，返回 FAILED 或触发重试
```

------

## 12.3 结算失败重试流程，后续支持

```text
1. 结算执行失败
2. 写入 FAILED 状态或重试任务
3. RetryWorker 定时扫描失败任务
4. 重新执行结算
5. 成功后更新状态为 SUCCESS
6. 客户端再次查询时返回成功结果
```

------

# 13. 接口设计

## 13.1 触发结算接口

调用方：

```text
Game Server → Settlement Service
```

### 请求

```http
POST /internal/settlements
```

### 请求参数

```json
{
  "roomId": "r_90001",
  "matchId": "m_10001",
  "mode": "classic",
  "serverId": "gs_01",
  "startTime": 1710000000000,
  "endTime": 1710000300000,
  "battleDurationSeconds": 300,
  "players": [
    {
      "userId": "10001",
      "nickname": "吞噬细胞",
      "rank": 1,
      "finalScore": 3680,
      "maxMass": 2560,
      "eatPlayerCount": 7,
      "eatFoodCount": 236,
      "aliveSeconds": 300,
      "alive": true
    },
    {
      "userId": "10002",
      "nickname": "绿巨人",
      "rank": 2,
      "finalScore": 2980,
      "maxMass": 2100,
      "eatPlayerCount": 4,
      "eatFoodCount": 188,
      "aliveSeconds": 286,
      "alive": true
    }
  ]
}
```

### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "roomId": "r_90001",
    "status": "SUCCESS",
    "results": [
      {
        "userId": "10001",
        "rank": 1,
        "finalScore": 3680,
        "coinReward": 443,
        "expReward": 295,
        "isBestScore": true
      }
    ]
  }
}
```

------

## 13.2 查询个人结算结果接口

调用方：

```text
Cocos Client → API Server
```

### 请求

```http
GET /api/settlements/{roomId}/me
```

### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "roomId": "r_90001",
    "userId": "10001",
    "rank": 1,
    "totalPlayers": 100,
    "finalScore": 3680,
    "maxMass": 2560,
    "eatPlayerCount": 7,
    "eatFoodCount": 236,
    "aliveSeconds": 300,
    "coinReward": 443,
    "expReward": 295,
    "isBestScore": true,
    "status": "SUCCESS"
  }
}
```

------

## 13.3 查询最近一局结算接口

### 请求

```http
GET /api/settlements/latest
```

### 用途

1. 玩家断线后重新进入游戏。
2. 客户端错过 WebSocket 结算推送。
3. 大厅页提醒“你有一局未查看结算”。

------

## 14. WebSocket 消息设计

## 14.1 游戏结束消息

```json
{
  "type": "GAME_END",
  "seq": 5001,
  "serverTime": 1710000300000,
  "data": {
    "roomId": "r_90001",
    "reason": "TIME_LIMIT",
    "message": "对局结束，正在结算..."
  }
}
```

------

## 14.2 结算结果消息

```json
{
  "type": "SETTLEMENT_RESULT",
  "seq": 5002,
  "serverTime": 1710000301000,
  "data": {
    "roomId": "r_90001",
    "userId": "10001",
    "rank": 1,
    "totalPlayers": 100,
    "finalScore": 3680,
    "maxMass": 2560,
    "eatPlayerCount": 7,
    "eatFoodCount": 236,
    "aliveSeconds": 300,
    "coinReward": 443,
    "expReward": 295,
    "isBestScore": true,
    "status": "SUCCESS"
  }
}
```

------

## 14.3 结算处理中消息

```json
{
  "type": "SETTLEMENT_PROCESSING",
  "seq": 5003,
  "serverTime": 1710000300500,
  "data": {
    "roomId": "r_90001",
    "message": "正在生成结算结果..."
  }
}
```

------

## 14.4 结算失败消息

```json
{
  "type": "SETTLEMENT_FAILED",
  "seq": 5004,
  "serverTime": 1710000301000,
  "data": {
    "roomId": "r_90001",
    "message": "结算处理中，请稍后查看"
  }
}
```

------

# 15. 数据库设计

## 15.1 game_records

对局主表。

| 字段             | 类型     | 说明           |
| ---------------- | -------- | -------------- |
| id               | bigint   | 主键           |
| room_id          | varchar  | 房间 ID        |
| match_id         | varchar  | 匹配 ID        |
| mode             | varchar  | 游戏模式       |
| server_id        | varchar  | Game Server ID |
| total_players    | int      | 总玩家数       |
| start_time       | datetime | 对局开始时间   |
| end_time         | datetime | 对局结束时间   |
| duration_seconds | int      | 对局时长       |
| status           | varchar  | 对局状态       |
| created_at       | datetime | 创建时间       |
| updated_at       | datetime | 更新时间       |

唯一索引：

```text
uniq_room_id(room_id)
```

------

## 15.2 game_player_results

玩家对局结果表。

| 字段             | 类型     | 说明           |
| ---------------- | -------- | -------------- |
| id               | bigint   | 主键           |
| room_id          | varchar  | 房间 ID        |
| user_id          | varchar  | 用户 ID        |
| nickname         | varchar  | 昵称快照       |
| rank             | int      | 最终排名       |
| final_score      | int      | 最终得分       |
| max_mass         | int      | 最大质量       |
| eat_player_count | int      | 吞噬玩家数     |
| eat_food_count   | int      | 吞噬食物数     |
| alive_seconds    | int      | 存活时间       |
| alive            | boolean  | 是否存活到结束 |
| created_at       | datetime | 创建时间       |
| updated_at       | datetime | 更新时间       |

唯一索引：

```text
uniq_room_user(room_id, user_id)
```

------

## 15.3 settlement_records

玩家结算记录表。

| 字段          | 类型     | 说明                          |
| ------------- | -------- | ----------------------------- |
| id            | bigint   | 主键                          |
| settlement_id | varchar  | 结算 ID                       |
| room_id       | varchar  | 房间 ID                       |
| user_id       | varchar  | 用户 ID                       |
| rank          | int      | 排名                          |
| final_score   | int      | 最终得分                      |
| coin_reward   | int      | 金币奖励                      |
| exp_reward    | int      | 经验奖励                      |
| status        | varchar  | SUCCESS / PROCESSING / FAILED |
| fail_reason   | varchar  | 失败原因                      |
| created_at    | datetime | 创建时间                      |
| updated_at    | datetime | 更新时间                      |

唯一索引：

```text
uniq_settlement_room_user(room_id, user_id)
```

------

## 15.4 user_assets

用户资产表。

| 字段       | 类型     | 说明           |
| ---------- | -------- | -------------- |
| id         | bigint   | 主键           |
| user_id    | varchar  | 用户 ID        |
| coin       | bigint   | 金币           |
| exp        | bigint   | 经验           |
| level      | int      | 等级，后续支持 |
| created_at | datetime | 创建时间       |
| updated_at | datetime | 更新时间       |

唯一索引：

```text
uniq_user_id(user_id)
```

------

## 15.5 asset_change_logs

资产变更流水表。

| 字段          | 类型     | 说明              |
| ------------- | -------- | ----------------- |
| id            | bigint   | 主键              |
| user_id       | varchar  | 用户 ID           |
| room_id       | varchar  | 房间 ID           |
| change_type   | varchar  | SETTLEMENT_REWARD |
| asset_type    | varchar  | COIN / EXP        |
| change_amount | bigint   | 变化数量          |
| before_amount | bigint   | 变化前数量        |
| after_amount  | bigint   | 变化后数量        |
| biz_id        | varchar  | 业务 ID           |
| created_at    | datetime | 创建时间          |

唯一索引：

```text
uniq_biz_asset(user_id, biz_id, asset_type)
```

biz_id 可使用：

```text
settlement:{roomId}:{userId}
```

------

# 16. 幂等设计

## 16.1 结算幂等 Key

核心幂等维度：

```text
roomId + userId
```

结算记录唯一约束：

```text
uniq_settlement_room_user(room_id, user_id)
```

资产流水唯一约束：

```text
uniq_biz_asset(user_id, biz_id, asset_type)
```

------

## 16.2 房间级幂等

同一个 roomId 只能生成一条对局记录。

```text
game_records.room_id 唯一
```

如果 Game Server 重复提交同一个 roomId 的结算请求：

1. 查询 game_records 是否存在。
2. 查询 settlement_records 是否已完成。
3. 如果已完成，直接返回已有结果。
4. 如果处理中，返回 PROCESSING。
5. 如果失败，按重试策略处理。

------

## 16.3 玩家级幂等

同一个玩家同一个房间只能结算一次。

```text
settlement_records(room_id, user_id) 唯一
```

如果重复执行：

1. 不重复插入结算记录。
2. 不重复更新资产。
3. 不重复写资产流水。
4. 返回已有结算结果。

------

## 16.4 资产流水幂等

资产变更必须有唯一业务 ID。

示例：

```text
bizId = settlement:r_90001:10001
```

金币流水：

```text
assetType = COIN
```

经验流水：

```text
assetType = EXP
```

这样即使重复执行，也不会重复发放同一类资产。

------

# 17. 事务设计

## 17.1 单玩家结算事务

对每个玩家执行：

```text
1. 插入或确认 game_player_results
2. 插入 settlement_records
3. 查询 user_assets
4. 更新 user_assets
5. 插入 asset_change_logs
6. 提交事务
```

------

## 17.2 房间整局事务

如果玩家数量较多，不建议把 100 个玩家全部放在一个巨大事务中。

推荐策略：

```text
对局主记录单独写入
每个玩家结算独立事务
最终汇总房间结算状态
```

优点：

1. 降低大事务风险。
2. 单个玩家失败可单独重试。
3. 更容易定位问题。

风险：

1. 可能出现部分玩家成功、部分玩家失败。

应对：

1. settlement_records 标记每个玩家状态。
2. 失败玩家进入重试。
3. 房间状态可为 PARTIAL_SUCCESS，后续支持。

MVP 阶段可以先使用：

```text
房间记录 + 玩家结果 + 资产更新在可控事务内完成
```

如果单局最多 100 人，事务仍可接受，但要注意超时和锁等待。

------

# 18. Redis 使用设计

## 18.1 结算状态缓存

Key：

```text
settlement:room:{roomId}
```

示例：

```json
{
  "roomId": "r_90001",
  "status": "SUCCESS",
  "totalPlayers": 100,
  "successCount": 100,
  "failedCount": 0,
  "updatedAt": 1710000301000
}
```

用途：

1. 快速查询房间结算状态。
2. 防止重复触发。
3. 支持客户端短时间轮询。
4. 支持重试任务判断。

------

## 18.2 玩家结算结果缓存

Key：

```text
settlement:player:{roomId}:{userId}
```

用途：

1. 客户端快速查询。
2. WebSocket 推送失败后可 HTTP 查询。
3. 短期缓存结算页数据。

过期时间建议：

```text
10 ~ 30 分钟
```

------

## 18.3 全服排行榜更新

后续支持。

示例：

```text
rank:daily:{date}
rank:weekly:{week}
rank:total
```

Redis 类型：

```text
Sorted Set
```

score 可使用：

```text
finalScore / bestScore / rankPoint
```

------

# 19. 奖励计算设计

## 19.1 RewardCalculator 输入

```json
{
  "rank": 1,
  "totalPlayers": 100,
  "finalScore": 3680,
  "maxMass": 2560,
  "eatPlayerCount": 7,
  "eatFoodCount": 236,
  "aliveSeconds": 300,
  "alive": true
}
```

------

## 19.2 RewardCalculator 输出

```json
{
  "coinReward": 443,
  "expReward": 295,
  "rewardDetails": {
    "baseCoin": 50,
    "rankCoinBonus": 300,
    "eatPlayerCoin": 70,
    "eatFoodCoin": 23,
    "baseExp": 30,
    "rankExpBonus": 200,
    "aliveExp": 60,
    "eatPlayerExp": 35
  }
}
```

------

## 19.3 奖励明细展示

客户端可展示：

1. 基础奖励。
2. 排名奖励。
3. 吞噬奖励。
4. 存活奖励。
5. 总奖励。

MVP 结算页可以只展示总金币和总经验，奖励明细后续支持。

------

# 20. 状态设计

## 20.1 房间结算状态

| 状态            | 说明                       |
| --------------- | -------------------------- |
| INIT            | 初始状态                   |
| PROCESSING      | 结算中                     |
| SUCCESS         | 全部结算成功               |
| PARTIAL_SUCCESS | 部分玩家结算成功，后续支持 |
| FAILED          | 整局结算失败               |
| RETRYING        | 重试中，后续支持           |

------

## 20.2 玩家结算状态

| 状态        | 说明             |
| ----------- | ---------------- |
| PROCESSING  | 处理中           |
| SUCCESS     | 成功             |
| FAILED      | 失败             |
| COMPENSATED | 已补偿，后续支持 |

------

# 21. 异常处理

## 21.1 Game Server 重复提交

处理：

1. 根据 roomId 查询已有结算。
2. 已成功则返回已有结果。
3. 处理中则返回 PROCESSING。
4. 失败则按策略允许重试。

------

## 21.2 数据库写入失败

处理：

1. 事务回滚。
2. 标记结算失败。
3. 记录失败原因。
4. 写入重试任务，后续支持。
5. 客户端提示结算处理中。

------

## 21.3 资产更新失败

处理：

1. 当前玩家事务回滚。
2. settlement_records 标记 FAILED。
3. 不写成功结算。
4. 后续重试该玩家结算。
5. 避免出现“有结算无奖励”或“有奖励无流水”。

------

## 21.4 客户端未收到推送

处理：

1. 客户端在结算页展示 loading。
2. 超过一定时间后调用查询接口。
3. 查询到 SUCCESS 则展示。
4. 查询到 PROCESSING 则继续等待。
5. 查询到 FAILED 则提示稍后查看。

------

# 22. 与其他系统关系

## 22.1 与 Game Server

Game Server 提供：

1. roomId。
2. matchId。
3. mode。
4. startTime。
5. endTime。
6. FinalRankList。
7. PlayerStats。
8. 游戏结束原因。

结算系统返回：

1. 每个玩家结算结果。
2. 结算状态。
3. 失败原因，异常时。

------

## 22.2 与局内排行榜系统

局内排行榜系统输出最终排名。

```text
RankSystem → FinalRankList → SettlementSystem
```

结算系统不重新计算实时排名，但可以校验最终排名数据完整性。

------

## 22.3 与用户资产系统

结算系统需要更新用户资产：

1. 金币增加。
2. 经验增加。
3. 等级变化，后续支持。
4. 资产流水记录。

------

## 22.4 与战绩系统

结算系统需要写入战绩数据：

1. 对局记录。
2. 玩家表现。
3. 排名。
4. 奖励。
5. 时间。

战绩系统后续通过查询接口展示历史对局。

------

## 22.5 与全服排行榜系统

后续版本：

1. 根据 finalScore 更新最高分榜。
2. 根据奖励积分更新积分榜。
3. 根据胜场更新胜场榜。
4. 根据段位积分更新排位榜。

MVP 可以暂不接入全服排行榜，只预留扩展点。

------

# 23. 安全与校验

## 23.1 结算请求校验

Settlement Service 需要校验：

1. roomId 是否为空。
2. players 是否为空。
3. rank 是否重复。
4. userId 是否重复。
5. finalScore 是否为非负数。
6. aliveSeconds 是否合理。
7. totalPlayers 是否与玩家数量一致。
8. 请求来源是否为内部服务。
9. roomId 是否已经结算过。

------

## 23.2 奖励校验

需要校验：

1. coinReward 不能为负数。
2. expReward 不能为负数。
3. 奖励不能超过配置上限。
4. 排名奖励必须匹配配置。
5. 资产更新必须有流水。
6. 同一业务 ID 不重复发奖。

------

## 23.3 反作弊原则

| 风险                     | 防护                               |
| ------------------------ | ---------------------------------- |
| 客户端伪造结算结果       | 客户端不能调用内部结算接口         |
| 客户端伪造奖励           | 奖励由服务端 RewardCalculator 计算 |
| 重复领取奖励             | roomId + userId 幂等               |
| 资产重复增加             | asset_change_logs 唯一业务 ID      |
| 异常超高得分             | 奖励上限和数据校验                 |
| Game Server 异常重复提交 | roomId 幂等                        |

------

# 24. 错误码设计

| 错误码 | 含义             | 客户端处理         |
| ------ | ---------------- | ------------------ |
| 0      | 成功             | 展示结算页         |
| 46001  | 结算处理中       | 展示 loading       |
| 46002  | 结算结果不存在   | 稍后重试或返回大厅 |
| 46003  | 结算失败         | 提示稍后查看       |
| 46004  | 房间已结算       | 返回已有结果       |
| 46005  | 玩家未参与该房间 | 返回大厅           |
| 46006  | 奖励计算失败     | 提示结算处理中     |
| 46007  | 资产更新失败     | 提示结算处理中     |
| 46008  | 结算数据异常     | 提示结算异常       |
| 50000  | 系统异常         | 稍后重试           |

------

# 25. 日志设计

## 25.1 关键日志点

| 日志点                    | 说明               |
| ------------------------- | ------------------ |
| settlement_request        | 收到结算请求       |
| settlement_idempotent_hit | 命中幂等           |
| settlement_start          | 开始结算           |
| reward_calculate          | 奖励计算           |
| game_record_write         | 写入对局记录       |
| player_result_write       | 写入玩家结果       |
| asset_update              | 更新资产           |
| asset_log_write           | 写入资产流水       |
| settlement_success        | 结算成功           |
| settlement_failed         | 结算失败           |
| settlement_query          | 查询结算结果       |
| settlement_retry          | 结算重试，后续支持 |

------

## 25.2 日志字段示例

```json
{
  "level": "info",
  "traceId": "trace_xxx",
  "roomId": "r_90001",
  "userId": "10001",
  "settlementId": "s_90001_10001",
  "rank": 1,
  "coinReward": 443,
  "expReward": 295,
  "message": "settlement_success",
  "durationMs": 12,
  "timestamp": "2026-06-28T10:00:00.000Z"
}
```

------

# 26. 监控指标

| 指标                                 | 说明                   |
| ------------------------------------ | ---------------------- |
| settlement_request_total             | 结算请求次数           |
| settlement_success_total             | 结算成功次数           |
| settlement_failed_total              | 结算失败次数           |
| settlement_idempotent_hit_total      | 幂等命中次数           |
| settlement_duration_ms               | 结算耗时               |
| settlement_player_count              | 每局结算玩家数         |
| settlement_reward_coin_total         | 发放金币总量           |
| settlement_reward_exp_total          | 发放经验总量           |
| settlement_asset_update_failed_total | 资产更新失败次数       |
| settlement_query_total               | 结算查询次数           |
| settlement_retry_total               | 结算重试次数，后续支持 |

------

# 27. MVP 开发任务拆分

## 27.1 客户端任务

| 任务                   | 说明                           |
| ---------------------- | ------------------------------ |
| GAME_END 处理          | 收到游戏结束消息后进入结算等待 |
| SETTLEMENT_RESULT 处理 | 展示结算页                     |
| 结算 loading           | 结算未返回时展示加载状态       |
| 结算查询接口           | 推送失败时查询结果             |
| 结算页 UI              | 展示排名、得分、统计、奖励     |
| 返回大厅               | 回到大厅并刷新资产             |
| 再来一局               | 重新发起匹配                   |
| 结算失败提示           | 展示稍后查看                   |

------

## 27.2 服务端任务

| 任务                     | 说明                      |
| ------------------------ | ------------------------- |
| 结算触发接口             | 接收 Game Server 结算请求 |
| 结算幂等                 | roomId + userId 防重复    |
| RewardCalculator         | 计算金币和经验            |
| game_records 写入        | 保存对局主记录            |
| game_player_results 写入 | 保存玩家表现              |
| settlement_records 写入  | 保存玩家结算              |
| user_assets 更新         | 增加金币和经验            |
| asset_change_logs 写入   | 保存资产流水              |
| 查询结算接口             | 支持客户端查询            |
| WebSocket 推送           | 推送 SETTLEMENT_RESULT    |
| 日志监控                 | 记录结算指标              |

------

# 28. 后续详细设计拆分建议

结算系统后续可以继续拆成：

1. 奖励规则详细设计。
2. 战绩系统详细设计。
3. 用户资产系统详细设计。
4. 资产流水与幂等详细设计。
5. 全服排行榜更新设计。
6. 段位积分结算设计。
7. 任务成就进度更新设计。
8. 结算失败重试设计。
9. 结算补偿后台设计。
10. 结算页 UI 交互设计。
11. 结算系统压测设计。

------

# 29. 总结

结算系统是《吞噬细胞》中将“一局实时对战”转化为“长期用户成长”的关键模块。

核心链路如下：

```text
Game Server 判断游戏结束
  ↓
冻结房间状态
  ↓
生成最终排名
  ↓
触发结算
  ↓
计算奖励
  ↓
写入战绩
  ↓
更新资产
  ↓
写入流水
  ↓
返回结算结果
  ↓
客户端展示结算页
```

MVP 阶段应优先保证：

1. 结算结果由服务端生成。
2. 最终排名和结算页一致。
3. 金币和经验奖励正确。
4. 战绩可以保存。
5. 用户资产可以更新。
6. 资产流水完整。
7. 结算具有幂等能力。
8. 客户端可通过推送或查询拿到结算结果。

当前阶段不要过早引入复杂段位、赛季、任务和活动奖励。先把“本局第几名、得了多少分、获得多少奖励、战绩是否保存、奖励是否只发一次”这条链路做稳，结算系统就能支撑游戏的基础闭环。