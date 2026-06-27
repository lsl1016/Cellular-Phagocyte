# 《吞噬细胞》全服排行榜系统 PRD + 技术设计文档

## 1. 文档信息

| 项目     | 内容                                                         |
| -------- | ------------------------------------------------------------ |
| 文档名称 | 全服排行榜系统 PRD + 技术设计文档                            |
| 所属产品 | 吞噬细胞                                                     |
| 所属模块 | 全服排行榜系统                                               |
| 上游模块 | 结算系统、战绩系统                                           |
| 下游模块 | 大厅页、排行榜页、个人主页、赛季系统，后续支持               |
| 客户端   | Cocos Creator 3.x + TypeScript                               |
| 服务端   | Go                                                           |
| 通信方式 | HTTP                                                         |
| 存储依赖 | Redis、MySQL                                                 |
| 文档定位 | 产品需求 + 技术概要设计                                      |
| 详细程度 | 中等详细，后续可继续拆分榜单规则、Redis 结构、榜单快照、赛季榜等详细设计 |

------

# 第一部分：全服排行榜系统 PRD

## 2. 模块背景

《吞噬细胞》除了单局内的实时排名，还需要一个跨对局、跨玩家的全服排行榜，用于展示长期竞争关系。

局内排行榜解决的是：

```text
这一局谁领先
```

全服排行榜解决的是：

```text
全服玩家中谁表现最好
```

全服排行榜可以增强玩家持续游玩的动力，让玩家在大厅、排行榜页、个人主页中看到自己与其他玩家的差距，并形成长期目标。

全服排行榜主要依赖结算系统产生的数据，例如：

1. 单局最终得分。
2. 单局最终排名。
3. 金币奖励。
4. 经验奖励。
5. 吞噬玩家数。
6. 吞噬食物数。
7. 胜场次数。
8. Top 10 次数。
9. 历史最高分。
10. 累计积分。

------

## 3. 模块目标

### 3.1 产品目标

| 目标         | 说明                               |
| ------------ | ---------------------------------- |
| 展示全服竞争 | 玩家可以查看全服排名               |
| 提供长期目标 | 玩家可以追求更高分数、更高积分     |
| 支持多种榜单 | 支持日榜、周榜、总榜、最高分榜等   |
| 展示我的排名 | 玩家能看到自己当前排名和分数       |
| 支持榜单刷新 | 结算后排行榜能及时更新             |
| 控制展示范围 | 客户端只展示 Top N 和我的排名      |
| 支持后续赛季 | 为赛季榜、段位榜预留能力           |
| 防止异常刷榜 | 服务端校验来源，避免客户端直接写榜 |

### 3.2 技术目标

| 目标           | 说明                               |
| -------------- | ---------------------------------- |
| 高性能读写     | 榜单查询和更新需要高效             |
| Redis 实时榜   | 使用 Redis Sorted Set 支撑实时排名 |
| MySQL 持久化   | 榜单关键数据需要落库或可恢复       |
| 结算驱动更新   | 榜单由结算结果触发更新             |
| 支持多榜单类型 | 日榜、周榜、总榜、最高分榜配置化   |
| 支持我的排名   | 可快速查询指定玩家排名和分数       |
| 支持榜单快照   | 定期保存榜单结果，便于历史查询     |
| 幂等更新       | 同一局结算重复触发时不能重复累计   |

------

## 4. 模块定位

全服排行榜系统处于结算系统之后。

```text
Game Server
  ↓
结算系统
  ↓
战绩数据 / 奖励数据
  ↓
全服排行榜系统
  ↓
Redis Sorted Set / MySQL
  ↓
客户端排行榜页
```

全服排行榜系统不参与实时对战，也不参与局内排名计算。
它只处理结算后的长期排名数据。

------

## 5. 核心设计原则

## 5.1 榜单由服务端更新

客户端只能查询榜单，不能写入榜单。

客户端不能提交：

```text
我的全服分数是 999999
我要进入第一名
我这一局加 10000 积分
```

榜单更新只能由结算系统或内部可信服务触发。

------

## 5.2 结算完成后更新榜单

推荐链路：

```text
结算成功
  ↓
生成 SettlementResult
  ↓
更新战绩
  ↓
更新全服排行榜
```

只有结算成功的数据才允许进入全服榜。

如果结算失败，不应更新榜单。

------

## 5.3 Redis 做实时查询，MySQL 做持久化

Redis 适合：

1. 实时排名。
2. Top N 查询。
3. 我的排名查询。
4. 分数更新。

MySQL 适合：

1. 榜单快照。
2. 用户榜单统计。
3. 异常恢复。
4. 历史榜单查询。
5. 审计追踪。

------

## 5.4 不同榜单使用不同分数口径

不同榜单的 score 含义不同。

例如：

| 榜单     | score 含义     |
| -------- | -------------- |
| 日榜     | 当天累计积分   |
| 周榜     | 本周累计积分   |
| 总榜     | 历史累计积分   |
| 最高分榜 | 历史单局最高分 |
| 胜场榜   | 第 1 名次数    |
| 吞噬榜   | 累计吞噬玩家数 |

------

# 6. 用户场景

## 6.1 查看排行榜

### 场景描述

玩家从大厅点击“排行榜”，进入排行榜页查看全服排名。

### 用户流程

```text
进入大厅
  ↓
点击排行榜
  ↓
选择榜单类型
  ↓
请求排行榜数据
  ↓
展示 Top N
  ↓
展示我的排名
```

------

## 6.2 查看我的排名

### 场景描述

玩家不在 Top 100 内，但仍希望知道自己的排名。

### 展示内容

1. 我的名次。
2. 我的榜单分数。
3. 距离上一名的差距，后续支持。
4. 是否未上榜。

示例：

```text
我的排名：第 3,256 名
我的积分：12,800
```

------

## 6.3 结算后排行榜提升

### 场景描述

玩家完成一局游戏，获得高分或积分，全服榜排名上升。

### 产品表现

1. 结算页可以提示“全服排名提升”。
2. 大厅排行榜入口可展示变化。
3. 排行榜页中我的排名更新。
4. 后续可展示排名上升动画。

------

## 6.4 查看不同榜单

### 场景描述

玩家切换日榜、周榜、总榜、最高分榜。

### 产品表现

1. 标签页切换榜单类型。
2. 榜单数据刷新。
3. 我的排名跟随榜单切换。
4. 不同榜单展示不同分数单位。

------

## 6.5 榜单重置

### 场景描述

日榜每天重置，周榜每周重置。

### 产品表现

1. 日榜显示今日排名。
2. 周榜显示本周排名。
3. 榜单页展示剩余刷新时间。
4. 总榜和最高分榜不重置。

------

# 7. 榜单类型设计

## 7.1 MVP 榜单

MVP 阶段建议支持 3 个榜单：

| 榜单     | 说明           | 分数口径       |
| -------- | -------------- | -------------- |
| 日榜     | 今日累计积分   | 今日 rankPoint |
| 周榜     | 本周累计积分   | 本周 rankPoint |
| 最高分榜 | 历史单局最高分 | bestScore      |

原因：

1. 日榜反馈快。
2. 周榜提供中期目标。
3. 最高分榜直观，适合休闲竞技游戏。

------

## 7.2 完整版本榜单

后续可扩展：

| 榜单   | 说明             |
| ------ | ---------------- |
| 总榜   | 历史累计积分     |
| 胜场榜 | 历史第一名次数   |
| 吞噬榜 | 累计吞噬玩家数   |
| 生存榜 | 单局最长存活时间 |
| 赛季榜 | 当前赛季积分榜   |
| 段位榜 | 排位段位积分榜   |
| 好友榜 | 好友之间排名     |

------

## 7.3 榜单展示字段

| 字段      | 说明                       |
| --------- | -------------------------- |
| rank      | 排名                       |
| userId    | 用户 ID，客户端可不展示    |
| nickname  | 昵称                       |
| avatar    | 头像，后续支持             |
| score     | 榜单分数                   |
| extraText | 附加展示，如最高分、胜场数 |
| self      | 是否当前玩家               |

------

# 8. 榜单分数设计

## 8.1 RankPoint 积分

日榜、周榜、总榜推荐使用 rankPoint，而不是直接使用金币或经验。

rankPoint 表示玩家在对局中的竞技表现。

推荐公式：

```text
rankPoint = rankBasePoint
          + rankBonus
          + finalScoreBonus
          + eatPlayerBonus
          + aliveBonus
```

------

## 8.2 MVP 积分公式

MVP 可以先简化：

```text
rankPoint = finalScore + eatPlayerCount * 50 + rankBonus
```

rankBonus 示例：

```json
{
  "1": 1000,
  "2": 700,
  "3": 500,
  "4-10": 300,
  "11-30": 100,
  "31-100": 0
}
```

------

## 8.3 最高分榜

最高分榜不累计，只保留历史最高单局分数。

规则：

```text
如果本局 finalScore > 历史 bestScore
则更新最高分榜
否则不更新
```

------

## 8.4 日榜 / 周榜

日榜、周榜使用累计积分。

```text
新分数 = 当前榜单分数 + 本局 rankPoint
```

但必须保证同一局不重复累计。

------

# 9. 页面需求

## 9.1 排行榜入口

位置：

```text
游戏大厅 → 排行榜按钮
```

入口可展示：

1. 今日榜前 1 名昵称。
2. 我的今日排名，后续支持。
3. 红点或活动标识，后续支持。

------

## 9.2 排行榜页

页面区域：

```text
顶部：榜单标签
中部：Top 3 特殊展示
下方：排行榜列表
底部：我的排名固定栏
```

榜单标签：

1. 日榜。
2. 周榜。
3. 最高分榜。
4. 总榜，后续支持。

------

## 9.3 Top 3 展示

Top 3 可以使用更醒目的样式：

1. 第 1 名居中。
2. 第 2 名和第 3 名两侧展示。
3. 展示头像、昵称、分数。
4. 后续支持称号和皮肤展示。

MVP 可以先使用普通列表样式。

------

## 9.4 我的排名固定栏

无论当前玩家是否在 Top N，都展示我的排名。

示例：

```text
我的排名：第 256 名
我的积分：18,600
```

如果未上榜：

```text
暂未上榜，完成一局游戏后参与排名
```

------

## 9.5 榜单刷新时间

日榜展示：

```text
今日榜将在 03:00 刷新
```

周榜展示：

```text
本周榜将在下周一 03:00 刷新
```

MVP 阶段可以不展示精确刷新时间，只展示榜单名称。

------

# 10. 功能范围

## 10.1 MVP 版本功能范围

MVP 实现：

1. 日榜。
2. 周榜。
3. 最高分榜。
4. 查询 Top N。
5. 查询我的排名。
6. 结算后更新榜单。
7. Redis Sorted Set 存储实时榜。
8. MySQL 保存用户榜单统计。
9. 防止重复结算重复加分。
10. 排行榜页展示。

------

## 10.2 完整版本功能范围

完整版本扩展：

1. 总榜。
2. 胜场榜。
3. 吞噬榜。
4. 赛季榜。
5. 好友榜。
6. 榜单快照。
7. 历史榜单查询。
8. 榜单奖励。
9. 排名变化提示。
10. 风控异常榜单处理。
11. 榜单管理后台。
12. 榜单重算任务。

------

## 10.3 暂不包含范围

当前文档不详细展开：

1. 好友系统。
2. 赛季系统。
3. 段位系统。
4. 榜单奖励发放。
5. 运营活动榜。
6. 反作弊风控模型。
7. 管理后台封禁和清榜。
8. 全球多区服合榜。

------

# 第二部分：全服排行榜技术设计

## 11. 总体架构

## 11.1 模块关系

```text
Settlement Service
  ↓ SettlementResult
Rank Service
  ↓
RankPointCalculator
  ↓
Redis Sorted Set
  ↓
MySQL Rank Stats
  ↓
API Server
  ↓
Cocos Client
```

------

## 11.2 服务端模块

```text
Rank Service
├── RankController
├── RankQueryService
├── RankUpdateService
├── RankPointCalculator
├── RankStorage
├── RankSnapshotService
├── RankIdempotencyManager
├── RankConfig
└── RankMetrics
```

模块说明：

| 模块                   | 职责                     |
| ---------------------- | ------------------------ |
| RankController         | 提供排行榜查询接口       |
| RankQueryService       | 查询 Top N 和我的排名    |
| RankUpdateService      | 处理结算后的榜单更新     |
| RankPointCalculator    | 计算本局榜单积分         |
| RankStorage            | 封装 Redis 和 MySQL 操作 |
| RankSnapshotService    | 保存榜单快照，后续支持   |
| RankIdempotencyManager | 防止同一局重复更新       |
| RankConfig             | 管理榜单配置和规则       |
| RankMetrics            | 指标监控                 |

------

# 12. Redis 数据设计

## 12.1 Redis Sorted Set 说明

Redis Sorted Set 适合排行榜。

特点：

1. member 存 userId。
2. score 存榜单分数。
3. 支持按分数排序。
4. 支持查询 Top N。
5. 支持查询某个用户排名。
6. 支持对用户分数累加。

------

## 12.2 日榜 Key

```text
rank:daily:{yyyyMMdd}
```

示例：

```text
rank:daily:20260628
```

类型：

```text
Sorted Set
```

member：

```text
userId
```

score：

```text
当天累计 rankPoint
```

------

## 12.3 周榜 Key

```text
rank:weekly:{yyyyWeek}
```

示例：

```text
rank:weekly:2026W26
```

score：

```text
本周累计 rankPoint
```

------

## 12.4 最高分榜 Key

```text
rank:best_score
```

score：

```text
历史单局最高 finalScore
```

更新规则：

```text
只有本局 finalScore 大于历史值时才更新
```

------

## 12.5 总榜 Key，后续支持

```text
rank:total
```

score：

```text
历史累计 rankPoint
```

------

## 12.6 幂等 Key

为了防止重复更新榜单：

```text
rank:update:{roomId}:{userId}
```

或者按整局维度：

```text
rank:update:{roomId}
```

推荐玩家级：

```text
rank:update:{roomId}:{userId}
```

原因：

1. 支持单个玩家补偿重试。
2. 避免部分成功后无法重试。
3. 和结算记录一致。

value 示例：

```json
{
  "roomId": "r_90001",
  "userId": "10001",
  "rankPoint": 4680,
  "status": "SUCCESS",
  "updatedAt": 1710000301000
}
```

------

# 13. MySQL 数据设计

## 13.1 user_rank_stats

用户排行榜统计表。

| 字段                   | 类型     | 说明             |
| ---------------------- | -------- | ---------------- |
| id                     | bigint   | 主键             |
| user_id                | varchar  | 用户 ID          |
| best_score             | int      | 历史最高单局分   |
| total_rank_point       | bigint   | 历史累计积分     |
| daily_rank_point       | bigint   | 今日积分，非必需 |
| weekly_rank_point      | bigint   | 本周积分，非必需 |
| first_place_count      | int      | 第一名次数       |
| top3_count             | int      | Top 3 次数       |
| top10_count            | int      | Top 10 次数      |
| total_games            | int      | 总局数           |
| total_eat_player_count | bigint   | 累计吞噬玩家     |
| total_eat_food_count   | bigint   | 累计吞噬食物     |
| updated_at             | datetime | 更新时间         |

唯一索引：

```text
uniq_user_id(user_id)
```

说明：

1. Redis 用于实时排名。
2. MySQL 用于用户累计统计和恢复。
3. daily_rank_point、weekly_rank_point 可以不落 MySQL，依赖 Redis 及快照。

------

## 13.2 rank_update_logs

榜单更新日志表。

| 字段         | 类型     | 说明             |
| ------------ | -------- | ---------------- |
| id           | bigint   | 主键             |
| room_id      | varchar  | 房间 ID          |
| user_id      | varchar  | 用户 ID          |
| rank_type    | varchar  | 榜单类型         |
| rank_point   | int      | 本次增加积分     |
| final_score  | int      | 本局最终分       |
| before_score | bigint   | 更新前榜单分数   |
| after_score  | bigint   | 更新后榜单分数   |
| status       | varchar  | SUCCESS / FAILED |
| created_at   | datetime | 创建时间         |

唯一索引：

```text
uniq_room_user_rank_type(room_id, user_id, rank_type)
```

用途：

1. 防重复更新。
2. 问题排查。
3. 后续补偿。
4. 审计追踪。

------

## 13.3 rank_snapshots，后续支持

榜单快照表。

| 字段          | 类型     | 说明     |
| ------------- | -------- | -------- |
| id            | bigint   | 主键     |
| rank_type     | varchar  | 榜单类型 |
| period_key    | varchar  | 周期标识 |
| rank          | int      | 排名     |
| user_id       | varchar  | 用户 ID  |
| nickname      | varchar  | 昵称快照 |
| score         | bigint   | 分数     |
| snapshot_time | datetime | 快照时间 |

索引：

```text
idx_rank_period(rank_type, period_key)
idx_user_rank(user_id, rank_type)
```

------

# 14. 榜单更新流程

## 14.1 结算后更新流程

```text
1. 结算系统完成玩家结算
2. 发送 SettlementResult 给 Rank Service
3. Rank Service 校验结算状态是否 SUCCESS
4. 根据结算结果计算 rankPoint
5. 检查 rank:update:{roomId}:{userId} 幂等 Key
6. 如果已更新，直接返回成功
7. 更新日榜 Sorted Set
8. 更新周榜 Sorted Set
9. 更新最高分榜
10. 更新 MySQL user_rank_stats
11. 写入 rank_update_logs
12. 标记幂等 Key 为 SUCCESS
```

------

## 14.2 日榜更新

```text
ZINCRBY rank:daily:{yyyyMMdd} rankPoint userId
```

------

## 14.3 周榜更新

```text
ZINCRBY rank:weekly:{yyyyWeek} rankPoint userId
```

------

## 14.4 总榜更新，后续支持

```text
ZINCRBY rank:total rankPoint userId
```

------

## 14.5 最高分榜更新

最高分榜不能直接累加，需要比较旧值。

逻辑：

```text
oldBest = ZSCORE rank:best_score userId
if finalScore > oldBest:
    ZADD rank:best_score finalScore userId
```

如果 Redis 无旧值，可从 MySQL `user_rank_stats.best_score` 恢复。

------

# 15. 查询接口设计

## 15.1 查询排行榜接口

### 请求

```http
GET /api/ranks?rankType=daily&page=1&pageSize=50
```

### 请求参数

| 参数      | 类型   | 必填 | 说明                                |
| --------- | ------ | ---- | ----------------------------------- |
| rankType  | string | 是   | daily / weekly / best_score / total |
| page      | int    | 否   | 页码                                |
| pageSize  | int    | 否   | 每页数量                            |
| periodKey | string | 否   | 指定周期，后续支持                  |

### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "rankType": "daily",
    "periodKey": "20260628",
    "page": 1,
    "pageSize": 50,
    "list": [
      {
        "rank": 1,
        "userId": "10001",
        "nickname": "吞噬细胞",
        "avatar": "",
        "score": 18600,
        "self": true
      }
    ],
    "selfRank": {
      "rank": 1,
      "score": 18600,
      "onRank": true
    },
    "refreshText": "今日榜"
  }
}
```

------

## 15.2 查询我的排名接口

### 请求

```http
GET /api/ranks/me?rankType=daily
```

### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "rankType": "daily",
    "rank": 256,
    "score": 12800,
    "onRank": true
  }
}
```

------

## 15.3 查询榜单配置接口

### 请求

```http
GET /api/ranks/config
```

### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "rankTypes": [
      {
        "rankType": "daily",
        "name": "日榜",
        "enabled": true
      },
      {
        "rankType": "weekly",
        "name": "周榜",
        "enabled": true
      },
      {
        "rankType": "best_score",
        "name": "最高分榜",
        "enabled": true
      }
    ]
  }
}
```

------

# 16. 查询流程设计

## 16.1 Top N 查询流程

```text
1. 客户端请求排行榜
2. API Server 校验登录态，可选
3. Rank Service 根据 rankType 生成 Redis Key
4. 使用 ZREVRANGE 查询 Top N
5. 查询用户昵称和头像
6. 查询当前玩家自身排名
7. 组装响应
8. 返回客户端
```

------

## 16.2 我的排名查询流程

```text
1. 根据 userId 查询 Redis ZREVRANK
2. 查询 Redis ZSCORE
3. 如果不存在，返回未上榜
4. 如果存在，rank = redisRank + 1
5. 返回我的排名和分数
```

------

## 16.3 用户信息补全

Redis 中只保存 userId 和 score，不保存昵称头像。

排行榜返回时需要补全：

1. 昵称。
2. 头像。
3. 等级，后续支持。
4. 称号，后续支持。

可选方案：

| 方案               | 说明           |
| ------------------ | -------------- |
| 查询用户表         | 简单直接       |
| Redis 缓存用户摘要 | 性能更好       |
| 榜单快照保存昵称   | 历史榜单更稳定 |

MVP 推荐：

```text
查询用户摘要缓存，缓存未命中再查 MySQL
```

------

# 17. 周期与重置设计

## 17.1 日榜周期

日榜 Key 按日期生成：

```text
rank:daily:yyyyMMdd
```

新的一天自动使用新 Key，不需要清空旧 Key。

旧 Key 可设置过期时间：

```text
保留 7 ~ 30 天
```

------

## 17.2 周榜周期

周榜 Key 按年份和周数生成：

```text
rank:weekly:yyyyWww
```

旧 Key 可设置过期时间：

```text
保留 8 ~ 12 周
```

------

## 17.3 最高分榜

最高分榜长期存在，不自动重置。

```text
rank:best_score
```

------

## 17.4 榜单快照

后续可在日榜、周榜结束时保存快照：

```text
每日 00:00 或 03:00
  ↓
读取 rank:daily:{date} Top N
  ↓
写入 rank_snapshots
```

------

# 18. 幂等设计

## 18.1 幂等问题

结算系统可能重复通知 Rank Service。

如果没有幂等，会导致：

1. 日榜重复加分。
2. 周榜重复加分。
3. 总榜重复加分。
4. 更新日志重复。
5. 排名异常。

------

## 18.2 幂等策略

推荐使用 MySQL 唯一索引 + Redis 幂等 Key 双重保证。

Redis Key：

```text
rank:update:{roomId}:{userId}
```

MySQL 唯一索引：

```text
uniq_room_user_rank_type(room_id, user_id, rank_type)
```

处理方式：

```text
1. 先检查 Redis 幂等 Key
2. 如果不存在，执行更新
3. 写入 rank_update_logs
4. 设置 Redis 幂等 Key
5. 如果 Redis 丢失，MySQL 唯一索引兜底
```

------

## 18.3 部分更新失败

例如日榜更新成功，周榜更新失败。

处理建议：

1. 每个 rankType 单独写 rank_update_logs。
2. 每个 rankType 单独幂等。
3. 失败榜单可单独重试。
4. 不要因为单个榜单失败导致所有榜单重复加分。

------

# 19. MySQL 与 Redis 一致性设计

## 19.1 写入顺序

推荐：

```text
1. 写 MySQL rank_update_logs，PROCESSING
2. 更新 Redis 榜单
3. 更新 MySQL user_rank_stats
4. 更新 rank_update_logs 为 SUCCESS
```

或者 MVP 简化：

```text
1. 更新 Redis 榜单
2. 更新 MySQL user_rank_stats
3. 写 rank_update_logs
```

MVP 可以先简化，但需要保留重试和幂等思路。

------

## 19.2 Redis 丢失恢复

如果 Redis 数据丢失，可以从 MySQL 恢复：

1. 从 `user_rank_stats` 恢复最高分榜和总榜。
2. 日榜、周榜如果没有周期统计表，可能无法完整恢复。
3. 后续可通过 `rank_update_logs` 按时间范围重放恢复日榜和周榜。

建议：

```text
MVP 阶段允许日榜、周榜短期依赖 Redis
完整版本通过 rank_update_logs 支持重建
```

------

# 20. 缓存设计

## 20.1 用户摘要缓存

Key：

```text
user:brief:{userId}
```

Value：

```json
{
  "userId": "10001",
  "nickname": "吞噬细胞",
  "avatar": ""
}
```

用途：

1. 排行榜昵称展示。
2. 减少用户表查询。
3. 支持 Top N 批量查询。

------

## 20.2 排行榜响应缓存

对于访问量大的榜单页，可以缓存 Top N 结果。

Key：

```text
rank:cache:{rankType}:{periodKey}:top:{page}:{pageSize}
```

过期时间：

```text
5 ~ 30 秒
```

说明：

1. 榜单不需要毫秒级实时。
2. 短缓存可以显著降低 Redis 和用户信息查询压力。
3. 我的排名仍可单独实时查询。

------

# 21. 榜单展示安全

## 21.1 隐藏敏感信息

排行榜只展示：

1. 昵称。
2. 头像。
3. 排名。
4. 分数。
5. 简单称号，后续支持。

不展示：

1. 用户真实 ID。
2. 登录信息。
3. 资产明细。
4. 设备信息。
5. IP 信息。

------

## 21.2 昵称快照与实时昵称

MVP 可以展示当前昵称。

完整版本建议榜单快照保存昵称：

1. 历史榜单不受用户改名影响。
2. 榜单快照更稳定。
3. 便于运营回看。

------

# 22. 异常处理

## 22.1 榜单为空

处理：

1. 返回空列表。
2. 客户端展示空状态。
3. 文案：“暂无上榜玩家，完成一局后参与排名”。

------

## 22.2 用户未上榜

返回：

```json
{
  "rank": null,
  "score": 0,
  "onRank": false
}
```

客户端展示：

```text
暂未上榜
```

------

## 22.3 Redis 异常

处理：

1. 返回服务繁忙。
2. 可降级读取 MySQL 快照，后续支持。
3. 记录错误日志。
4. 不允许客户端自行计算榜单。

------

## 22.4 更新失败

处理：

1. 写失败日志。
2. 写 FAILED 状态。
3. 后续重试。
4. 不影响结算主流程展示，但可能提示“排行榜稍后更新”。

------

# 23. 错误码设计

| 错误码 | 含义           | 客户端处理           |
| ------ | -------------- | -------------------- |
| 0      | 成功           | 正常展示             |
| 48001  | 榜单类型不存在 | 隐藏该榜单           |
| 48002  | 榜单未开启     | 隐藏该榜单           |
| 48003  | 榜单为空       | 展示空状态           |
| 48004  | 用户未上榜     | 展示未上榜           |
| 48005  | Redis 查询失败 | 提示稍后重试         |
| 48006  | 榜单更新失败   | 不影响结算，稍后刷新 |
| 48007  | 榜单数据异常   | 使用缓存或提示重试   |
| 50000  | 系统异常       | 稍后重试             |

------

# 24. 日志设计

## 24.1 关键日志点

| 日志点                | 说明             |
| --------------------- | ---------------- |
| rank_query            | 查询排行榜       |
| rank_me_query         | 查询我的排名     |
| rank_update_request   | 收到榜单更新请求 |
| rank_update_success   | 榜单更新成功     |
| rank_update_failed    | 榜单更新失败     |
| rank_idempotent_hit   | 命中幂等         |
| rank_score_calculate  | 计算榜单积分     |
| rank_snapshot_save    | 保存榜单快照     |
| rank_redis_error      | Redis 异常       |
| rank_user_brief_query | 查询用户摘要     |

------

## 24.2 日志字段示例

```json
{
  "level": "info",
  "traceId": "trace_xxx",
  "roomId": "r_90001",
  "userId": "10001",
  "rankType": "daily",
  "rankPoint": 4680,
  "message": "rank_update_success",
  "durationMs": 6,
  "timestamp": "2026-06-28T10:00:00.000Z"
}
```

------

# 25. 监控指标

| 指标                             | 说明               |
| -------------------------------- | ------------------ |
| rank_query_total                 | 榜单查询次数       |
| rank_query_duration_ms           | 榜单查询耗时       |
| rank_update_total                | 榜单更新次数       |
| rank_update_success_total        | 榜单更新成功次数   |
| rank_update_failed_total         | 榜单更新失败次数   |
| rank_idempotent_hit_total        | 幂等命中次数       |
| rank_redis_error_total           | Redis 异常次数     |
| rank_top_query_total             | Top N 查询次数     |
| rank_me_query_total              | 我的排名查询次数   |
| rank_score_calculate_duration_ms | 积分计算耗时       |
| rank_cache_hit_total             | 榜单缓存命中次数   |
| rank_cache_miss_total            | 榜单缓存未命中次数 |

------

# 26. 安全与校验

## 26.1 查询校验

需要校验：

1. rankType 是否支持。
2. page 是否合法。
3. pageSize 是否超过上限。
4. periodKey 是否合法，后续支持。
5. 用户登录态，可选。未登录也可查看 Top N，但不能查看我的排名。

------

## 26.2 更新校验

更新榜单前需要校验：

1. 请求来源是否为内部服务。
2. settlementStatus 是否为 SUCCESS。
3. roomId 是否存在。
4. userId 是否存在。
5. finalScore 是否非负。
6. rank 是否合法。
7. rankPoint 是否非负。
8. 是否已经更新过该房间该玩家榜单。

------

## 26.3 反作弊原则

| 风险                 | 防护                            |
| -------------------- | ------------------------------- |
| 客户端伪造榜单分数   | 客户端没有写榜接口              |
| 重复结算导致重复加分 | roomId + userId + rankType 幂等 |
| 异常高分刷榜         | 奖励和分数上限校验              |
| Redis 被异常写入     | 后台定期与 MySQL 审计           |
| 昵称违规展示         | 昵称审核或敏感词过滤，后续支持  |

------

# 27. 配置设计

```json
{
  "rank": {
    "enabled": true,
    "pageSizeLimit": 100,
    "defaultPageSize": 50,
    "rankTypes": {
      "daily": {
        "enabled": true,
        "name": "日榜",
        "redisKeyPattern": "rank:daily:{yyyyMMdd}",
        "expireDays": 30,
        "scoreMode": "RANK_POINT"
      },
      "weekly": {
        "enabled": true,
        "name": "周榜",
        "redisKeyPattern": "rank:weekly:{yyyyWeek}",
        "expireWeeks": 12,
        "scoreMode": "RANK_POINT"
      },
      "best_score": {
        "enabled": true,
        "name": "最高分榜",
        "redisKeyPattern": "rank:best_score",
        "scoreMode": "BEST_SCORE"
      }
    },
    "rankPoint": {
      "eatPlayerPoint": 50,
      "rankBonus": {
        "1": 1000,
        "2": 700,
        "3": 500,
        "4-10": 300,
        "11-30": 100,
        "31-100": 0
      }
    }
  }
}
```

------

# 28. MVP 开发任务拆分

## 28.1 客户端任务

| 任务       | 说明                     |
| ---------- | ------------------------ |
| 排行榜入口 | 大厅增加排行榜入口       |
| 排行榜页   | 展示榜单页面             |
| 榜单标签   | 日榜、周榜、最高分榜切换 |
| Top N 列表 | 展示排名、昵称、分数     |
| 我的排名   | 展示当前用户排名和分数   |
| 空状态     | 无榜单数据时展示         |
| 未上榜状态 | 当前玩家未上榜时展示     |
| 错误提示   | 榜单查询失败时提示       |

------

## 28.2 服务端任务

| 任务                | 说明                     |
| ------------------- | ------------------------ |
| 榜单查询接口        | `/api/ranks`             |
| 我的排名接口        | `/api/ranks/me`          |
| 榜单配置接口        | `/api/ranks/config`      |
| 结算后更新榜单      | SettlementResult 触发    |
| RankPointCalculator | 计算本局榜单积分         |
| Redis Sorted Set    | 存储日榜、周榜、最高分榜 |
| 最高分更新逻辑      | 仅更高分时更新           |
| 幂等控制            | 防重复加分               |
| 用户摘要查询        | 补全昵称头像             |
| 日志监控            | 查询和更新指标           |

------

# 29. 后续详细设计拆分建议

全服排行榜系统后续可以继续拆成：

1. RankPoint 积分规则详细设计。
2. Redis Sorted Set 存储详细设计。
3. 榜单更新幂等详细设计。
4. 榜单查询接口详细设计。
5. 榜单快照设计。
6. 日榜 / 周榜周期切换设计。
7. 最高分榜设计。
8. 总榜和赛季榜设计。
9. 榜单缓存设计。
10. 榜单风控与异常处理设计。
11. 排行榜页面 UI 交互设计。
12. 排行榜压测设计。

------

# 30. 总结

全服排行榜系统是《吞噬细胞》中承接结算结果、提供长期竞技目标的核心模块。它把单局表现转化为跨对局、跨玩家的长期竞争数据。

核心链路如下：

```text
结算成功
  ↓
计算 rankPoint
  ↓
更新 Redis 榜单
  ↓
写入 MySQL 统计和更新日志
  ↓
客户端查询 Top N
  ↓
客户端展示我的排名
```

MVP 阶段应优先保证：

1. 日榜可以查询和更新。
2. 周榜可以查询和更新。
3. 最高分榜可以查询和更新。
4. 结算成功后自动更新榜单。
5. 同一局不会重复加分。
6. 客户端能看到 Top N 和我的排名。
7. Redis 用于实时榜，MySQL 用于统计和审计。

当前阶段不要过早加入赛季、段位、好友榜和榜单奖励。先把“结算后更新全服榜、玩家能查榜、我的排名准确、不重复加分”这条链路做稳定，全服排行榜就能支撑游戏的长期竞争体验。