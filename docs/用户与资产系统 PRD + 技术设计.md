# 《吞噬细胞》用户与资产系统 PRD + 技术设计文档

## 1. 文档信息

| 项目     | 内容                                                         |
| -------- | ------------------------------------------------------------ |
| 文档名称 | 用户与资产系统 PRD + 技术设计文档                            |
| 所属产品 | 吞噬细胞                                                     |
| 所属模块 | 用户系统 / 资产系统                                          |
| 上游模块 | 登录入口、结算系统、任务系统，后续支持                       |
| 下游模块 | 大厅页、匹配系统、结算页、战绩系统、商城系统，后续支持       |
| 客户端   | Cocos Creator 3.x + TypeScript                               |
| 服务端   | Go                                                           |
| 通信方式 | HTTP                                                         |
| 存储依赖 | MySQL、Redis                                                 |
| 文档定位 | 产品需求 + 技术概要设计                                      |
| 详细程度 | 中等详细，后续可继续拆分登录认证、资产流水、等级系统、游客转正等详细设计 |

------

# 第一部分：用户与资产系统 PRD

## 2. 模块背景

《吞噬细胞》需要支持玩家进入游戏、开始匹配、参与对局、获得奖励、查看战绩和排行榜。为了支撑这些功能，系统需要建立基础用户体系和资产体系。

用户系统负责回答：

```text
这个玩家是谁？
他是否登录？
他的昵称、头像、等级是什么？
他是否可以进入匹配和对局？
```

资产系统负责回答：

```text
这个玩家有多少金币？
有多少经验？
本局奖励是否已经入账？
资产变化是否可追踪？
是否发生重复发奖？
```

用户与资产系统不是实时对战核心模块，但它是游戏长期成长、奖励闭环和数据可信的基础。

------

## 3. 模块目标

### 3.1 产品目标

| 目标             | 说明                             |
| ---------------- | -------------------------------- |
| 支持快速进入游戏 | 玩家可以用游客身份快速进入       |
| 支持注册用户     | 后续支持账号登录和跨设备保留数据 |
| 展示基础资料     | 大厅展示昵称、头像、等级、金币等 |
| 承接结算奖励     | 对局结束后金币、经验正确入账     |
| 支持成长反馈     | 经验增长、等级提升给玩家长期目标 |
| 支持资产查询     | 玩家可以看到当前金币、经验等资产 |
| 支持资产追踪     | 每次资产变化都有流水记录         |
| 防止重复发奖     | 同一局结算奖励不能重复入账       |

### 3.2 技术目标

| 目标         | 说明                               |
| ------------ | ---------------------------------- |
| 身份唯一     | 每个玩家有唯一 userId              |
| 登录态可校验 | 服务端接口能识别当前用户           |
| 游客可升级   | 游客账号后续可绑定为正式账号       |
| 资产强一致   | 金币、经验更新需要事务保护         |
| 流水可追踪   | 每次资产变更写入 asset_change_logs |
| 幂等可靠     | 结算奖励重复请求不重复加资产       |
| 查询高效     | 大厅用户资料和资产查询要快         |
| 可扩展       | 后续支持皮肤、道具、等级、商城     |

------

## 4. 模块定位

用户与资产系统位于基础服务层。

```text
客户端
  ↓
API Server
  ↓
User Service
  ↓
Asset Service
  ↓
MySQL / Redis
```

与其他系统关系：

```text
登录系统 → 用户系统
结算系统 → 资产系统
大厅页 → 用户资料 + 资产查询
匹配系统 → 用户身份校验
排行榜系统 → 用户昵称头像查询
战绩系统 → 用户历史数据查询
```

------

## 5. 核心设计原则

## 5.1 用户身份由服务端生成

客户端不能自己生成可信 userId。

客户端可以保存：

```text
accessToken
guestToken
本地设备标识，非可信
```

服务端负责：

```text
生成 userId
校验 token
维护用户状态
绑定游客账号和正式账号
```

------

## 5.2 游客优先，注册增强

MVP 阶段建议先支持游客登录。

原因：

1. 降低进入门槛。
2. 快速体验游戏。
3. 便于测试匹配和对局流程。
4. 后续可平滑升级为正式账号。

游客能力：

1. 可以进入大厅。
2. 可以匹配。
3. 可以对战。
4. 可以获得临时战绩和奖励。
5. 数据保存在当前设备或服务端游客账号下。

限制：

1. 换设备后可能无法恢复。
2. 不能保证长期资产安全。
3. 后续需要绑定账号才能跨设备保存。

------

## 5.3 资产只能由服务端修改

客户端不能直接修改金币、经验、等级。

资产来源包括：

1. 对局结算奖励。
2. 任务奖励，后续支持。
3. 活动奖励，后续支持。
4. 管理后台补偿，后续支持。
5. 商城消费，后续支持。

所有资产变化必须经过 Asset Service。

------

## 5.4 资产变化必须有流水

每次金币或经验变化，都必须写资产流水。

流水用于：

1. 防重复发奖。
2. 问题排查。
3. 资产对账。
4. 用户申诉。
5. 后续风控分析。

------

## 5.5 业务幂等优先

结算奖励、任务奖励、活动奖励都可能重复请求。

必须使用业务 ID 保证幂等。

示例：

```text
settlement:{roomId}:{userId}
task:{taskId}:{userId}:{date}
activity:{activityId}:{userId}
```

------

# 6. 用户场景

## 6.1 游客进入游戏

### 场景描述

新玩家第一次打开游戏，不想注册，点击“游客进入”。

### 用户流程

```text
打开游戏
  ↓
点击游客进入
  ↓
服务端创建游客用户
  ↓
返回 accessToken
  ↓
进入大厅
```

### 产品表现

1. 自动生成昵称。
2. 自动分配默认头像。
3. 初始金币和等级为默认值。
4. 可以直接开始匹配。

------

## 6.2 注册用户登录

### 场景描述

玩家希望长期保存数据，使用账号登录。

### MVP 可选

MVP 阶段可以先不实现完整账号体系，只预留字段。

完整版本支持：

1. 手机号登录。
2. 邮箱登录。
3. 第三方平台登录。
4. 游客账号绑定正式账号。
5. 跨设备登录。

------

## 6.3 大厅展示用户信息

### 场景描述

玩家进入大厅后，需要看到自己的基础信息。

### 展示内容

1. 昵称。
2. 头像。
3. 等级。
4. 当前经验。
5. 升级所需经验。
6. 金币。
7. 钻石，后续支持。
8. 今日排名，后续支持。

------

## 6.4 对局结算后资产入账

### 场景描述

玩家完成一局游戏，结算系统计算出金币和经验奖励。

### 用户流程

```text
对局结束
  ↓
结算系统计算奖励
  ↓
Asset Service 增加金币和经验
  ↓
写入资产流水
  ↓
客户端结算页展示奖励
  ↓
回到大厅后资产刷新
```

------

## 6.5 重复结算请求

### 场景描述

服务端因为网络超时重复调用资产入账。

### 处理规则

1. 根据业务 ID 查询是否已处理。
2. 如果已处理，直接返回已有结果。
3. 不重复增加金币和经验。
4. 返回幂等结果给调用方。

------

## 6.6 经验升级

### 场景描述

玩家获得经验后，经验达到升级阈值。

### 产品表现

1. 结算页展示经验增长。
2. 如果升级，展示等级提升。
3. 大厅展示新等级。
4. 后续可发放升级奖励。

MVP 阶段可先实现简单等级公式，也可以只保存经验，不做复杂升级奖励。

------

# 7. 功能范围

## 7.1 MVP 版本功能范围

MVP 阶段实现：

1. 游客登录。
2. 生成用户 ID。
3. 生成默认昵称。
4. 生成默认头像。
5. 查询用户资料。
6. 查询用户资产。
7. 金币资产。
8. 经验资产。
9. 简单等级。
10. 结算奖励入账。
11. 资产流水。
12. 资产幂等。
13. 大厅展示用户信息和资产。
14. 登录态校验。

------

## 7.2 完整版本功能范围

完整版本扩展：

1. 正式账号注册。
2. 手机号 / 邮箱登录。
3. 第三方登录。
4. 游客转正式账号。
5. 昵称修改。
6. 头像修改。
7. 皮肤资产。
8. 道具资产。
9. 钻石资产。
10. 等级奖励。
11. 商城消费。
12. 资产冻结。
13. 资产补偿。
14. 风控审计。
15. 多设备登录策略。

------

## 7.3 暂不包含范围

当前文档不详细展开：

1. 支付系统。
2. 商城系统。
3. 皮肤装扮系统。
4. 好友系统。
5. 实名认证。
6. 防沉迷系统。
7. 第三方平台接入。
8. 复杂账号安全策略。

------

# 8. 用户资料设计

## 8.1 用户类型

| 类型       | 说明                     |
| ---------- | ------------------------ |
| GUEST      | 游客用户                 |
| REGISTERED | 注册用户                 |
| BOT        | 机器人用户，后续支持     |
| SYSTEM     | 系统用户，用于后台发放等 |

------

## 8.2 用户状态

| 状态          | 说明               |
| ------------- | ------------------ |
| ACTIVE        | 正常               |
| BANNED        | 封禁               |
| DELETED       | 已注销，后续支持   |
| GUEST_EXPIRED | 游客过期，后续支持 |

------

## 8.3 用户基础资料

| 字段        | 说明         |
| ----------- | ------------ |
| userId      | 用户 ID      |
| userType    | 用户类型     |
| nickname    | 昵称         |
| avatar      | 头像         |
| level       | 等级         |
| exp         | 当前累计经验 |
| coin        | 金币         |
| status      | 用户状态     |
| createdAt   | 创建时间     |
| lastLoginAt | 最近登录时间 |

------

# 9. 资产设计

## 9.1 资产类型

MVP 阶段：

| 资产 | 说明 |
| ---- | ---- |
| COIN | 金币 |
| EXP  | 经验 |

完整版本扩展：

| 资产       | 说明     |
| ---------- | -------- |
| DIAMOND    | 钻石     |
| SKIN       | 皮肤     |
| ITEM       | 道具     |
| TITLE      | 称号     |
| RANK_POINT | 排位积分 |

------

## 9.2 金币用途

MVP 阶段金币可以先只展示，不消费。

后续用途：

1. 购买皮肤。
2. 购买表情。
3. 解锁头像框。
4. 参与活动。
5. 升级装扮。

------

## 9.3 经验用途

经验用于等级成长。

MVP 阶段：

```text
结算获得经验
经验累积
根据经验计算等级
```

后续支持：

1. 等级奖励。
2. 等级称号。
3. 等级解锁皮肤。
4. 等级排行榜。

------

## 9.4 等级计算

MVP 推荐简单等级表。

示例：

| 等级 | 所需累计经验 |
| ---- | ------------ |
| 1    | 0            |
| 2    | 100          |
| 3    | 300          |
| 4    | 600          |
| 5    | 1000         |

也可以使用公式：

```text
level = floor(sqrt(totalExp / 100)) + 1
```

MVP 推荐使用等级配置表，便于调整。

------

# 第二部分：用户与资产系统技术设计

## 10. 总体架构

## 10.1 模块关系

```text
Cocos Client
  ↓
API Server
  ↓
Auth Service
  ↓
User Service
  ↓
Asset Service
  ↓
MySQL / Redis
```

结算入账链路：

```text
Settlement Service
  ↓
Asset Service
  ↓
MySQL Transaction
  ├── user_assets
  └── asset_change_logs
```

------

## 10.2 服务端模块

```text
User Asset Service
├── AuthController
├── UserController
├── AssetController
├── GuestLoginService
├── TokenService
├── UserService
├── AssetService
├── LevelService
├── AssetFlowService
├── IdempotencyManager
├── UserCache
└── UserAssetMetrics
```

模块说明：

| 模块               | 职责                   |
| ------------------ | ---------------------- |
| AuthController     | 登录相关接口           |
| UserController     | 用户资料查询和修改     |
| AssetController    | 资产查询接口           |
| GuestLoginService  | 游客登录和创建         |
| TokenService       | accessToken 生成和校验 |
| UserService        | 用户资料管理           |
| AssetService       | 资产增减               |
| LevelService       | 经验和等级计算         |
| AssetFlowService   | 资产流水管理           |
| IdempotencyManager | 业务幂等               |
| UserCache          | 用户资料和资产缓存     |
| UserAssetMetrics   | 指标监控               |

------

# 11. 数据库设计

## 11.1 users

用户基础表。

| 字段               | 类型     | 说明                     |
| ------------------ | -------- | ------------------------ |
| id                 | bigint   | 主键                     |
| user_id            | varchar  | 用户 ID                  |
| user_type          | varchar  | GUEST / REGISTERED / BOT |
| nickname           | varchar  | 昵称                     |
| avatar             | varchar  | 头像                     |
| status             | varchar  | ACTIVE / BANNED          |
| guest_device_id    | varchar  | 游客设备 ID，后续可选    |
| registered_account | varchar  | 正式账号，后续可选       |
| created_at         | datetime | 创建时间                 |
| updated_at         | datetime | 更新时间                 |
| last_login_at      | datetime | 最近登录时间             |

索引：

```text
uniq_user_id(user_id)
idx_guest_device_id(guest_device_id)
idx_registered_account(registered_account)
```

------

## 11.2 user_assets

用户资产表。

| 字段       | 类型     | 说明           |
| ---------- | -------- | -------------- |
| id         | bigint   | 主键           |
| user_id    | varchar  | 用户 ID        |
| coin       | bigint   | 金币           |
| exp        | bigint   | 经验           |
| level      | int      | 等级           |
| diamond    | bigint   | 钻石，后续支持 |
| created_at | datetime | 创建时间       |
| updated_at | datetime | 更新时间       |

索引：

```text
uniq_user_id(user_id)
```

说明：

1. `coin` 表示当前金币余额。
2. `exp` 表示累计经验。
3. `level` 可冗余保存，便于查询。
4. 等级也可以根据 exp 动态计算，MVP 推荐冗余保存。

------

## 11.3 asset_change_logs

资产流水表。

| 字段          | 类型     | 说明                         |
| ------------- | -------- | ---------------------------- |
| id            | bigint   | 主键                         |
| user_id       | varchar  | 用户 ID                      |
| asset_type    | varchar  | COIN / EXP / DIAMOND         |
| change_type   | varchar  | 变化类型                     |
| change_amount | bigint   | 变化数量，正数增加，负数扣减 |
| before_amount | bigint   | 变化前数量                   |
| after_amount  | bigint   | 变化后数量                   |
| biz_type      | varchar  | 业务类型                     |
| biz_id        | varchar  | 业务 ID                      |
| remark        | varchar  | 备注                         |
| created_at    | datetime | 创建时间                     |

索引：

```text
idx_user_created(user_id, created_at)
uniq_biz_asset(user_id, biz_id, asset_type)
idx_biz_id(biz_id)
```

唯一索引用于防止重复发放同一业务资产。

------

## 11.4 user_login_logs

登录日志表，后续可选。

| 字段           | 类型     | 说明                           |
| -------------- | -------- | ------------------------------ |
| id             | bigint   | 主键                           |
| user_id        | varchar  | 用户 ID                        |
| login_type     | varchar  | GUEST / PASSWORD / THIRD_PARTY |
| device_id      | varchar  | 设备 ID                        |
| ip             | varchar  | IP，后续可选                   |
| client_version | varchar  | 客户端版本                     |
| created_at     | datetime | 创建时间                       |

------

## 11.5 level_configs

等级配置表，也可使用配置文件。

| 字段         | 类型     | 说明                   |
| ------------ | -------- | ---------------------- |
| id           | bigint   | 主键                   |
| level        | int      | 等级                   |
| required_exp | bigint   | 所需累计经验           |
| reward_coin  | bigint   | 升级奖励金币，后续支持 |
| created_at   | datetime | 创建时间               |
| updated_at   | datetime | 更新时间               |

MVP 可以先写在配置文件中，不一定建表。

------

# 12. Redis 设计

## 12.1 用户资料缓存

Key：

```text
user:profile:{userId}
```

Value：

```json
{
  "userId": "10001",
  "nickname": "吞噬细胞",
  "avatar": "",
  "userType": "GUEST",
  "status": "ACTIVE"
}
```

过期时间：

```text
10 ~ 30 分钟
```

------

## 12.2 用户资产缓存

Key：

```text
user:asset:{userId}
```

Value：

```json
{
  "userId": "10001",
  "coin": 1200,
  "exp": 350,
  "level": 3
}
```

资产缓存需要谨慎：

1. 资产更新后主动删除或更新缓存。
2. 资产展示可以读缓存。
3. 资产变更必须以 MySQL 为准。

------

## 12.3 登录 Token

Key：

```text
auth:token:{accessToken}
```

Value：

```json
{
  "userId": "10001",
  "userType": "GUEST",
  "expireAt": 1710000000000
}
```

过期时间：

```text
7 天或按登录策略配置
```

MVP 阶段可以使用 JWT，也可以使用 Redis Token。
如果希望支持主动登出和封禁立即生效，Redis Token 更简单。

------

# 13. 接口设计

## 13.1 游客登录接口

### 请求

```http
POST /api/auth/guest-login
```

### 请求参数

```json
{
  "deviceId": "device_xxx",
  "clientVersion": "1.0.0"
}
```

### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "accessToken": "access_token_xxx",
    "user": {
      "userId": "10001",
      "nickname": "吞噬细胞1234",
      "avatar": "",
      "userType": "GUEST",
      "level": 1,
      "coin": 0,
      "exp": 0
    }
  }
}
```

------

## 13.2 查询我的资料接口

### 请求

```http
GET /api/users/me
```

### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "userId": "10001",
    "nickname": "吞噬细胞1234",
    "avatar": "",
    "userType": "GUEST",
    "status": "ACTIVE",
    "level": 3,
    "exp": 350,
    "nextLevelExp": 600,
    "coin": 1200
  }
}
```

------

## 13.3 查询我的资产接口

### 请求

```http
GET /api/assets/me
```

### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "userId": "10001",
    "coin": 1200,
    "exp": 350,
    "level": 3,
    "nextLevelExp": 600
  }
}
```

------

## 13.4 资产入账内部接口

调用方：

```text
Settlement Service → Asset Service
```

### 请求

```http
POST /internal/assets/grant
```

### 请求参数

```json
{
  "userId": "10001",
  "bizType": "SETTLEMENT_REWARD",
  "bizId": "settlement:r_90001:10001",
  "items": [
    {
      "assetType": "COIN",
      "amount": 443
    },
    {
      "assetType": "EXP",
      "amount": 295
    }
  ],
  "remark": "对局结算奖励"
}
```

### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "userId": "10001",
    "results": [
      {
        "assetType": "COIN",
        "beforeAmount": 1200,
        "changeAmount": 443,
        "afterAmount": 1643
      },
      {
        "assetType": "EXP",
        "beforeAmount": 350,
        "changeAmount": 295,
        "afterAmount": 645
      }
    ],
    "levelChanged": true,
    "oldLevel": 3,
    "newLevel": 4
  }
}
```

------

## 13.5 查询资产流水接口，后续支持

### 请求

```http
GET /api/assets/logs?page=1&pageSize=20
```

用途：

1. 玩家查看金币来源。
2. 客服处理资产问题。
3. 后续后台审计。

MVP 客户端可以不开放该入口，但服务端保留能力。

------

# 14. 游客登录流程

## 14.1 新游客登录

```text
1. 客户端请求 guest-login
2. 服务端检查 deviceId 是否已有游客账号
3. 如果没有，创建 users 记录
4. 创建 user_assets 记录
5. 生成默认昵称和头像
6. 生成 accessToken
7. 写入登录日志
8. 返回用户资料和资产
```

------

## 14.2 老游客登录

```text
1. 客户端请求 guest-login
2. 服务端根据 deviceId 查询已有游客账号
3. 如果存在且状态正常，更新 lastLoginAt
4. 生成新的 accessToken
5. 返回用户资料和资产
```

------

## 14.3 游客数据风险

游客账号依赖设备标识或本地 token，存在风险：

1. 清除缓存后可能丢失。
2. 换设备无法恢复。
3. deviceId 不一定稳定。
4. 不适合长期资产保存。

后续需要支持：

```text
游客绑定正式账号
```

------

# 15. 登录态校验设计

## 15.1 accessToken 使用

客户端请求需要带：

```http
Authorization: Bearer access_token_xxx
```

服务端流程：

```text
1. 解析 accessToken
2. 查询 Redis 或校验 JWT
3. 获取 userId
4. 查询用户状态
5. 如果用户被封禁，拒绝请求
6. 将 userId 注入请求上下文
```

------

## 15.2 需要登录的接口

需要登录：

1. 开始匹配。
2. 进入房间。
3. 查询我的资料。
4. 查询我的资产。
5. 查询战绩。
6. 查询我的排名。
7. 查询结算结果。

可不登录：

1. 查看部分公告，后续支持。
2. 查看公开排行榜 Top N，按产品策略决定。

------

# 16. 资产入账流程

## 16.1 结算奖励入账

```text
1. 结算系统计算 coinReward 和 expReward
2. 调用 Asset Service grant 接口
3. Asset Service 校验 userId、bizId、items
4. 检查 asset_change_logs 是否已有该 bizId
5. 开启 MySQL 事务
6. 查询 user_assets 当前余额
7. 更新 coin 和 exp
8. 重新计算 level
9. 插入 asset_change_logs
10. 提交事务
11. 删除或更新 Redis 缓存
12. 返回入账结果
```

------

## 16.2 幂等处理

如果重复请求同一个 bizId：

```text
1. 查询 asset_change_logs
2. 如果已有 COIN 和 EXP 流水
3. 不重复更新 user_assets
4. 返回已有流水结果
```

唯一约束：

```text
uniq_biz_asset(user_id, biz_id, asset_type)
```

------

## 16.3 事务设计

资产入账必须在事务中完成：

```text
BEGIN
  SELECT user_assets FOR UPDATE
  UPDATE user_assets
  INSERT asset_change_logs
COMMIT
```

原因：

1. 防止并发更新丢失。
2. 保证资产余额和流水一致。
3. 避免有流水无余额变化。
4. 避免有余额变化无流水。

------

# 17. 等级计算设计

## 17.1 等级更新时机

经验变化后触发等级计算：

```text
oldExp = 当前经验
newExp = oldExp + expReward
oldLevel = 当前等级
newLevel = LevelService.calculate(newExp)
```

如果：

```text
newLevel > oldLevel
```

则返回升级结果。

------

## 17.2 等级配置表方案

推荐使用等级配置表：

```json
[
  { "level": 1, "requiredExp": 0 },
  { "level": 2, "requiredExp": 100 },
  { "level": 3, "requiredExp": 300 },
  { "level": 4, "requiredExp": 600 },
  { "level": 5, "requiredExp": 1000 }
]
```

优点：

1. 方便策划调整。
2. 可以控制升级节奏。
3. 后续可加升级奖励。
4. 不依赖固定公式。

------

## 17.3 nextLevelExp

客户端展示经验进度需要：

```text
当前等级
当前经验
下一等级所需经验
```

示例：

```json
{
  "level": 3,
  "exp": 350,
  "currentLevelExp": 300,
  "nextLevelExp": 600
}
```

------

# 18. 资产流水类型设计

## 18.1 change_type

| 类型              | 说明               |
| ----------------- | ------------------ |
| SETTLEMENT_REWARD | 对局结算奖励       |
| TASK_REWARD       | 任务奖励，后续支持 |
| ACTIVITY_REWARD   | 活动奖励，后续支持 |
| SHOP_CONSUME      | 商城消费，后续支持 |
| ADMIN_GRANT       | 后台发放，后续支持 |
| ADMIN_DEDUCT      | 后台扣减，后续支持 |
| LEVEL_REWARD      | 升级奖励，后续支持 |

------

## 18.2 biz_type

| 类型       | 说明     |
| ---------- | -------- |
| SETTLEMENT | 对局结算 |
| TASK       | 任务     |
| ACTIVITY   | 活动     |
| SHOP       | 商城     |
| ADMIN      | 后台     |

------

# 19. 缓存一致性设计

## 19.1 查询缓存

用户资料和资产可以缓存。

查询流程：

```text
1. 查 Redis
2. 命中则返回
3. 未命中查 MySQL
4. 写 Redis
5. 返回
```

------

## 19.2 更新缓存

资产更新后：

推荐策略：

```text
事务提交成功后删除缓存
```

原因：

1. 简单可靠。
2. 避免缓存旧数据。
3. 下次查询自动加载最新数据。

也可以选择：

```text
事务提交成功后主动更新缓存
```

MVP 推荐删除缓存。

------

# 20. 异常处理

## 20.1 用户不存在

处理：

1. 返回未登录或用户不存在。
2. 客户端引导重新登录。
3. 游客场景可重新创建游客账号。

------

## 20.2 用户被封禁

处理：

1. 拒绝登录或进入游戏。
2. 返回封禁提示。
3. 禁止匹配和对局。
4. 保留资产和战绩数据。

------

## 20.3 资产入账失败

处理：

1. 事务回滚。
2. 返回失败给结算系统。
3. 结算系统标记该玩家结算失败或重试。
4. 不允许出现部分资产入账。

------

## 20.4 并发资产更新

场景：

1. 结算奖励同时到账。
2. 任务奖励同时到账。
3. 后台补偿同时到账。

处理：

1. user_assets 行级锁。
2. asset_change_logs 唯一业务 ID。
3. 每种资产分别写流水。
4. 事务保证一致性。

------

# 21. 错误码设计

| 错误码 | 含义           | 客户端处理   |
| ------ | -------------- | ------------ |
| 0      | 成功           | 正常处理     |
| 50001  | 未登录         | 重新登录     |
| 50002  | Token 无效     | 重新登录     |
| 50003  | Token 过期     | 重新登录     |
| 50004  | 用户不存在     | 重新登录     |
| 50005  | 用户被封禁     | 展示封禁提示 |
| 50006  | 昵称非法       | 提示重新输入 |
| 50007  | 资产不存在     | 重新拉取     |
| 50008  | 资产不足       | 提示资产不足 |
| 50009  | 资产入账失败   | 稍后重试     |
| 50010  | 重复业务请求   | 返回幂等结果 |
| 50011  | 等级配置不存在 | 使用默认等级 |
| 50000  | 系统异常       | 稍后重试     |

------

# 22. 日志设计

## 22.1 关键日志点

| 日志点               | 说明               |
| -------------------- | ------------------ |
| guest_login_request  | 游客登录请求       |
| guest_user_created   | 创建游客用户       |
| token_issued         | 生成 accessToken   |
| user_profile_query   | 查询用户资料       |
| user_asset_query     | 查询用户资产       |
| asset_grant_request  | 资产发放请求       |
| asset_grant_success  | 资产发放成功       |
| asset_grant_failed   | 资产发放失败       |
| asset_idempotent_hit | 资产幂等命中       |
| level_up             | 用户升级           |
| user_banned_reject   | 封禁用户请求被拒绝 |

------

## 22.2 日志字段示例

```json
{
  "level": "info",
  "traceId": "trace_xxx",
  "userId": "10001",
  "bizId": "settlement:r_90001:10001",
  "assetType": "COIN",
  "changeAmount": 443,
  "beforeAmount": 1200,
  "afterAmount": 1643,
  "message": "asset_grant_success",
  "durationMs": 6,
  "timestamp": "2026-06-28T10:00:00.000Z"
}
```

------

# 23. 监控指标

| 指标                          | 说明                 |
| ----------------------------- | -------------------- |
| auth_guest_login_total        | 游客登录次数         |
| auth_token_invalid_total      | Token 无效次数       |
| user_profile_query_total      | 用户资料查询次数     |
| asset_query_total             | 资产查询次数         |
| asset_grant_total             | 资产发放次数         |
| asset_grant_success_total     | 资产发放成功次数     |
| asset_grant_failed_total      | 资产发放失败次数     |
| asset_idempotent_hit_total    | 幂等命中次数         |
| asset_transaction_duration_ms | 资产事务耗时         |
| level_up_total                | 用户升级次数         |
| user_banned_reject_total      | 封禁用户请求拒绝次数 |

------

# 24. 安全与校验

## 24.1 登录校验

需要校验：

1. accessToken 是否存在。
2. accessToken 是否过期。
3. userId 是否存在。
4. 用户状态是否正常。
5. 客户端版本是否允许，后续支持。

------

## 24.2 资产接口校验

资产发放接口需要校验：

1. 请求来源是否为内部服务。
2. userId 是否存在。
3. bizId 是否为空。
4. assetType 是否支持。
5. amount 是否为正数。
6. 是否超过单次发放上限。
7. 是否已处理过该 bizId。
8. 用户状态是否允许入账。

------

## 24.3 反作弊原则

| 风险                 | 防护                     |
| -------------------- | ------------------------ |
| 客户端伪造金币       | 客户端没有资产发放接口   |
| 客户端伪造经验       | 经验由服务端发放         |
| 重复结算刷资产       | bizId 幂等               |
| 并发请求导致余额错误 | 行级锁 + 事务            |
| 修改本地显示资产     | 每次关键页面从服务端刷新 |
| 封禁用户继续匹配     | 匹配前校验用户状态       |

------

# 25. MVP 开发任务拆分

## 25.1 客户端任务

| 任务           | 说明                     |
| -------------- | ------------------------ |
| 游客登录页     | 支持游客进入             |
| Token 保存     | 本地保存 accessToken     |
| 自动登录       | 启动时尝试使用已有 token |
| 大厅用户信息   | 展示昵称、等级、金币     |
| 我的资产查询   | 调用资产接口             |
| 结算资产刷新   | 结算后刷新金币和经验     |
| Token 过期处理 | 重新登录                 |
| 封禁提示       | 用户被封禁时展示提示     |

------

## 25.2 服务端任务

| 任务           | 说明                     |
| -------------- | ------------------------ |
| 游客登录接口   | `/api/auth/guest-login`  |
| Token 生成校验 | accessToken 管理         |
| 用户表         | users                    |
| 资产表         | user_assets              |
| 资产流水表     | asset_change_logs        |
| 我的资料接口   | `/api/users/me`          |
| 我的资产接口   | `/api/assets/me`         |
| 资产发放接口   | `/internal/assets/grant` |
| 资产事务       | 余额更新 + 流水写入      |
| 资产幂等       | bizId 防重复             |
| 等级计算       | exp → level              |
| 用户缓存       | 用户资料和资产缓存       |
| 日志监控       | 登录、查询、资产发放指标 |

------

# 26. 后续详细设计拆分建议

用户与资产系统后续可以继续拆成：

1. 游客登录详细设计。
2. Token 认证详细设计。
3. 游客转正式账号设计。
4. 用户资料修改设计。
5. 资产流水详细设计。
6. 资产幂等详细设计。
7. 等级系统详细设计。
8. 商城消费资产扣减设计。
9. 皮肤资产系统设计。
10. 用户封禁与风控设计。
11. 用户缓存一致性设计。
12. 用户与资产系统压测设计。

------

# 27. 总结

用户与资产系统是《吞噬细胞》的基础支撑模块。它不直接参与实时对战计算，但决定了玩家身份、长期数据、奖励入账和成长体系是否可靠。

核心链路如下：

```text
游客登录
  ↓
创建用户
  ↓
创建资产
  ↓
进入大厅
  ↓
参与对局
  ↓
结算奖励
  ↓
资产入账
  ↓
写入流水
  ↓
大厅展示最新资产
```

MVP 阶段应优先保证：

1. 玩家可以快速以游客身份进入游戏。
2. 每个玩家有唯一 userId。
3. 大厅可以展示基础资料和资产。
4. 结算奖励可以正确入账。
5. 金币和经验变化有流水。
6. 资产入账有事务保护。
7. 同一业务奖励不会重复发放。
8. 匹配、战绩、排行榜等系统可以基于 userId 关联数据。

当前阶段不要过早加入复杂支付、商城、皮肤和第三方账号系统。先把“用户是谁、奖励给谁、资产怎么安全变化、是否可追踪”这条链路做稳定，后续成长和商业化能力才有可靠基础。