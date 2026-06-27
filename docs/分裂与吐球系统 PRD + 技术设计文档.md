# 《吞噬细胞》分裂与吐球系统 PRD + 技术设计文档

## 1. 文档信息

| 项目     | 内容                                                         |
| -------- | ------------------------------------------------------------ |
| 文档名称 | 分裂与吐球系统 PRD + 技术设计文档                            |
| 所属产品 | 吞噬细胞                                                     |
| 所属模块 | 分裂与吐球系统                                               |
| 上游模块 | Game Server / Tick 系统、输入系统、移动系统                  |
| 关联模块 | 碰撞与吞噬系统、状态同步系统、AOI 系统、排行榜系统、结算系统 |
| 客户端   | Cocos Creator 3.x + TypeScript                               |
| 服务端   | Go                                                           |
| 通信方式 | WebSocket                                                    |
| 初期协议 | JSON                                                         |
| 后续协议 | Protobuf                                                     |
| 文档定位 | 产品需求 + 技术概要设计                                      |
| 详细程度 | 中等详细，后续可继续拆分多球体控制、合体、技能冷却、吐出物运动等详细设计 |

------

# 第一部分：分裂与吐球系统 PRD

## 2. 模块背景

《吞噬细胞》的核心玩法不仅是移动和吞噬，还需要给玩家提供主动操作空间。分裂和吐球是球球类游戏中最重要的两个主动操作能力。

分裂用于：

1. 快速追击小球。
2. 快速扩大攻击范围。
3. 躲避大球。
4. 在关键时刻进行爆发操作。
5. 增加多球体控制难度和策略性。

吐球用于：

1. 主动降低自身质量。
2. 调整移动速度。
3. 给自己的其他球体补充质量。
4. 喂养队友，后续组队模式支持。
5. 喂刺球，后续刺球机制支持。
6. 制造战术空间。

本系统需要在保持操作简单的前提下，增加对局深度和操作技巧。

------

## 3. 模块目标

### 3.1 产品目标

| 目标           | 说明                                     |
| -------------- | ---------------------------------------- |
| 增加主动操作   | 玩家不只是移动，还能主动分裂和吐球       |
| 提升竞技深度   | 分裂追击、吐球调整、合体等待形成战术选择 |
| 操作反馈清晰   | 点击分裂或吐球后，客户端需要有明确反馈   |
| 规则易理解     | 质量足够才能分裂或吐球，操作失败要有提示 |
| 保持公平性     | 分裂与吐球结果由服务端判定               |
| 支持多球体玩法 | 分裂后玩家可控制多个球体                 |
| 支持后续扩展   | 为合体、刺球、组队、机器人策略预留能力   |

### 3.2 技术目标

| 目标           | 说明                                         |
| -------------- | -------------------------------------------- |
| 服务端权威     | 客户端只能发送操作意图，不能直接生成球体     |
| 技能条件校验   | 服务端校验质量、冷却、数量上限、房间状态     |
| 多球体状态管理 | 玩家可拥有多个 PlayerBall                    |
| 事件可同步     | 分裂、吐球、失败原因需要同步给客户端         |
| 与碰撞系统联动 | 新球体和吐出物需要参与碰撞与吞噬             |
| 与 AOI 联动    | 分裂球体、吐出物需要进入 AOI 同步            |
| 支持可配置     | 质量消耗、冷却时间、初速度、合体时间等可配置 |

------

## 4. 模块定位

分裂与吐球系统位于 Game Server 的输入处理和游戏逻辑之间。

```text
客户端点击技能按钮
  ↓
发送 SPLIT / EJECT 输入
  ↓
Game Server 校验操作条件
  ↓
SkillSystem 执行分裂 / 吐球
  ↓
更新玩家球体 / 吐出物
  ↓
写入事件缓冲
  ↓
状态同步给客户端
```

本模块不直接负责最终结算，但会影响：

1. 玩家质量。
2. 玩家移动速度。
3. 玩家吞噬能力。
4. 玩家排名。
5. 吞噬统计。
6. 对局表现。

------

## 5. 核心设计原则

## 5.1 客户端只发送操作意图

客户端发送：

```text
我要分裂
我要向某个方向分裂
我要吐球
我要向某个方向吐球
```

客户端不能发送：

```text
我已经分裂成两个球
我生成了一个吐出物
我的质量减少了多少
我的新球体坐标是多少
```

最终结果由服务端计算。

------

## 5.2 操作失败必须有明确原因

分裂或吐球失败时，客户端需要知道失败原因，避免玩家觉得按钮失灵。

常见失败原因：

1. 质量不足。
2. 冷却中。
3. 分身数量达到上限。
4. 房间未开始或已结束。
5. 玩家已死亡。
6. 输入方向异常。
7. 操作频率过快。

------

## 5.3 分裂后增加收益，也增加风险

分裂可以快速追击，但会让玩家质量分散。

产品效果：

1. 更容易吃到远处小球。
2. 更容易被更大的玩家反吃。
3. 多球体控制更复杂。
4. 合体前存在风险窗口。

------

## 5.4 吐球是质量转移能力

吐球不是直接攻击技能，而是质量转移和战术调整能力。

产品效果：

1. 降低自身质量。
2. 提升移动速度，间接提高机动性。
3. 可喂给自己的其他球体。
4. 后续可喂队友或刺球。
5. 需要频率限制，避免刷屏。

------

# 6. 用户场景

## 6.1 分裂追击

### 场景描述

玩家发现前方有一个比自己小的玩家，点击“分裂”按钮后，自己的球体向移动方向快速冲出一个分身，用于追击并吞噬目标。

### 用户流程

```text
玩家移动追击
  ↓
点击分裂按钮
  ↓
客户端发送 SPLIT
  ↓
服务端校验质量和冷却
  ↓
服务端生成新球体
  ↓
新球体获得初速度
  ↓
服务端同步 PLAYER_SPLIT 事件
  ↓
客户端播放分裂表现
```

------

## 6.2 分裂逃跑

### 场景描述

玩家遇到大球追击，可以通过分裂改变质量分布或方向，尝试逃离危险区域。

### 产品要求

1. 分裂方向应该跟随玩家输入方向。
2. 分裂后多个球体都属于同一玩家。
3. 分裂后短时间不能立即合体。
4. 分裂不应绕过服务端速度限制。

------

## 6.3 吐球喂自己

### 场景描述

玩家通过吐球将质量释放到前方，自己的其他球体可以吞噬这些吐出物，从而调整质量分布。

### 用户流程

```text
玩家点击吐球
  ↓
客户端发送 EJECT
  ↓
服务端校验质量和频率
  ↓
服务端从玩家球体扣除质量
  ↓
服务端生成吐出物
  ↓
吐出物向指定方向移动
  ↓
后续被玩家或其他玩家吞噬
```

------

## 6.4 吐球失败

### 场景描述

玩家质量不足时点击吐球。

### 产品表现

1. 按钮短暂抖动或变灰。
2. 弹出提示“质量不足，无法吐球”。
3. 不生成吐出物。
4. 服务端返回失败结果或错误事件。

------

## 6.5 分身数量达到上限

### 场景描述

玩家已经分裂出多个球体，继续点击分裂。

### 产品表现

1. 分裂按钮不可用或提示。
2. 服务端拒绝操作。
3. 客户端提示“分身数量已达上限”。

------

# 7. 功能范围

## 7.1 MVP 版本功能范围

MVP 阶段建议实现：

1. 分裂按钮。
2. 吐球按钮。
3. 客户端发送 SPLIT / EJECT 输入。
4. 服务端校验质量。
5. 服务端校验冷却。
6. 服务端校验分身数量上限。
7. 服务端生成分裂球体。
8. 服务端生成吐出物。
9. 分裂球体参与移动和吞噬。
10. 吐出物参与移动和被吞噬。
11. 分裂事件同步。
12. 吐球事件同步。
13. 操作失败提示。

------

## 7.2 完整版本功能范围

完整版本可扩展：

1. 多球体合体。
2. 分裂保护。
3. 吐出物保护期。
4. 队友喂球。
5. 吐球喂刺球。
6. 分裂数量动态上限。
7. 技能按钮冷却动效。
8. 多球体镜头缩放。
9. 多球体输入优化。
10. 机器人分裂和吐球策略。
11. 新手引导。
12. 技能操作回放。

------

## 7.3 暂不包含范围

当前文档不详细展开：

1. 完整合体算法。
2. 刺球触发分裂算法。
3. 队友协作模式。
4. 复杂技能系统。
5. 客户端预测回滚。
6. 多球体高级操控。
7. 机器人 AI 行为树。

------

# 8. 分裂规则设计

## 8.1 分裂触发条件

玩家点击“分裂”按钮后，服务端校验：

| 条件     | 说明                         |
| -------- | ---------------------------- |
| 房间状态 | 必须为 RUNNING               |
| 玩家状态 | 必须为 PLAYING               |
| 质量阈值 | 球体质量必须达到最小分裂质量 |
| 分身上限 | 玩家当前球体数量不能超过上限 |
| 冷却时间 | 距离上次分裂必须超过冷却时间 |
| 输入方向 | direction 必须合法           |
| 球体状态 | 当前球体不能处于不可分裂状态 |

------

## 8.2 分裂对象选择

玩家可能拥有多个球体。

MVP 阶段推荐：

```text
只对当前质量最大的球体执行分裂
```

原因：

1. 实现简单。
2. 玩家容易理解。
3. 避免一次操作产生过多球体。

完整版本可扩展为：

```text
所有满足条件的球体同时分裂
```

或者：

```text
根据输入焦点选择某个球体分裂
```

------

## 8.3 分裂质量分配

推荐 MVP 规则：

```text
原球体质量减少一半
新球体获得一半质量
```

即：

```text
newBall.mass = sourceBall.mass * splitMassRatio
sourceBall.mass = sourceBall.mass - newBall.mass
```

推荐配置：

```text
splitMassRatio = 0.5
```

------

## 8.4 分裂位置

新球体生成在原球体朝向方向的前方。

```text
newBall.x = sourceBall.x + cos(direction) * spawnOffset
newBall.y = sourceBall.y + sin(direction) * spawnOffset
```

spawnOffset 可以与原球体半径相关：

```text
spawnOffset = sourceBall.radius * 0.8
```

------

## 8.5 分裂初速度

新球体获得一个短时间冲刺速度。

```text
newBall.vx = cos(direction) * splitBoostSpeed
newBall.vy = sin(direction) * splitBoostSpeed
```

冲刺速度会随时间衰减。

```text
splitBoostDurationMs = 500
```

MVP 阶段可简化为：

1. 新球体向指定方向快速位移。
2. 持续若干 Tick 后恢复普通移动速度。
3. 速度计算仍由服务端控制。

------

## 8.6 分裂冷却

玩家分裂后进入冷却。

推荐配置：

```text
splitCooldownMs = 1000
```

冷却期间：

1. 分裂按钮置灰。
2. 客户端可以展示倒计时。
3. 服务端拒绝新的 SPLIT 输入。

------

## 8.7 分裂后合体限制

分裂后的球体不能立刻合体。

推荐配置：

```text
mergeDelaySeconds = 10
```

MVP 阶段可以暂不实现合体，只保留字段：

```text
canMergeAt
```

后续合体系统单独设计。

------

# 9. 吐球规则设计

## 9.1 吐球触发条件

玩家点击“吐球”按钮后，服务端校验：

| 条件     | 说明                         |
| -------- | ---------------------------- |
| 房间状态 | 必须为 RUNNING               |
| 玩家状态 | 必须为 PLAYING               |
| 质量阈值 | 球体质量必须达到最小吐球质量 |
| 操作频率 | 距离上次吐球必须超过最小间隔 |
| 输入方向 | direction 必须合法           |
| 球体状态 | 当前球体可以吐球             |

------

## 9.2 吐球对象选择

MVP 阶段推荐：

```text
质量最大的球体吐球
```

完整版本可以扩展为：

```text
所有满足条件的球体同时吐球
```

或：

```text
距离输入方向最近的球体吐球
```

------

## 9.3 吐球质量消耗

推荐规则：

```text
sourceBall.mass -= ejectMass
```

生成吐出物：

```text
ejectedMass.mass = ejectMass * ejectMassRatio
```

MVP 可设置：

```text
ejectMass = 5
ejectMassRatio = 1.0
```

如果需要减少质量膨胀，也可设置：

```text
ejectMassRatio = 0.9
```

------

## 9.4 吐出物生成位置

吐出物生成在球体前方。

```text
eject.x = sourceBall.x + cos(direction) * spawnOffset
eject.y = sourceBall.y + sin(direction) * spawnOffset
```

推荐：

```text
spawnOffset = sourceBall.radius + eject.radius
```

------

## 9.5 吐出物初速度

吐出物会沿方向飞出一段距离。

```text
eject.vx = cos(direction) * ejectSpeed
eject.vy = sin(direction) * ejectSpeed
```

推荐配置：

```text
ejectSpeed = 400
ejectMoveDurationMs = 300
```

之后吐出物停留在地图上，成为可吞噬对象。

------

## 9.6 吐球频率限制

吐球比分裂更容易高频触发，因此需要更严格限制。

推荐配置：

```text
ejectIntervalMs = 150
```

同时可限制：

```text
每秒最多吐球次数
```

例如：

```text
maxEjectPerSecond = 5
```

------

## 9.7 吐出物保护期

为了避免吐出的球被原球体立即吃回，可以设置短暂保护期。

推荐：

```text
ejectProtectMs = 300
```

保护期内：

1. 原 owner 不能立即吞噬该吐出物。
2. 其他玩家是否能吞噬可配置。
3. MVP 可先不做保护期，只通过生成位置避免立即吃回。

------

# 10. 客户端交互设计

## 10.1 按钮布局

对战页右下角展示两个核心按钮：

| 按钮 | 说明       |
| ---- | ---------- |
| 分裂 | 触发 SPLIT |
| 吐球 | 触发 EJECT |

推荐：

```text
分裂按钮更大、更醒目
吐球按钮稍小、支持连续点击
```

------

## 10.2 按钮状态

| 状态       | 分裂按钮           | 吐球按钮                 |
| ---------- | ------------------ | ------------------------ |
| 可用       | 正常高亮           | 正常高亮                 |
| 质量不足   | 置灰               | 置灰                     |
| 冷却中     | 显示冷却进度       | 显示冷却进度或短间隔禁用 |
| 玩家死亡   | 不可用             | 不可用                   |
| 房间未开始 | 不可用             | 不可用                   |
| 网络异常   | 可点击但提示或置灰 | 可点击但提示或置灰       |

------

## 10.3 失败提示

| 失败原因 | 文案                   |
| -------- | ---------------------- |
| 质量不足 | 质量不足，无法操作     |
| 分身上限 | 分身数量已达上限       |
| 冷却中   | 技能冷却中             |
| 操作过快 | 操作太频繁了           |
| 玩家死亡 | 当前已被吞噬，无法操作 |
| 房间结束 | 对局已结束             |

------

## 10.4 客户端表现

分裂表现：

1. 原球体快速分成两个球。
2. 新球体向前冲刺。
3. 播放分裂特效。
4. 球体大小根据服务端质量更新。
5. 镜头根据多个球体位置调整，后续支持。

吐球表现：

1. 球体前方喷出小球。
2. 原球体略微变小。
3. 吐出物沿方向飞出。
4. 播放吐球音效。
5. 吐出物停留并参与后续同步。

------

# 第二部分：分裂与吐球技术设计

## 11. 总体架构

## 11.1 模块关系

```text
Client Input
  ↓
WebSocket
  ↓
InputBuffer
  ↓
SkillSystem
  ↓
SplitSystem / EjectSystem
  ↓
PlayerBallManager / EntityManager
  ↓
CollisionSystem
  ↓
SnapshotSystem
```

------

## 11.2 服务端模块

```text
Game Server
├── InputBuffer
├── SkillSystem
├── SplitSystem
├── EjectSystem
├── PlayerBallManager
├── EntityManager
├── CooldownManager
├── EventBuffer
├── SnapshotSystem
└── SkillMetrics
```

模块说明：

| 模块              | 职责                       |
| ----------------- | -------------------------- |
| InputBuffer       | 接收 SPLIT / EJECT 输入    |
| SkillSystem       | 统一处理技能类操作         |
| SplitSystem       | 处理分裂逻辑               |
| EjectSystem       | 处理吐球逻辑               |
| PlayerBallManager | 管理玩家球体列表           |
| EntityManager     | 管理吐出物等实体           |
| CooldownManager   | 管理技能冷却与频率限制     |
| EventBuffer       | 存储分裂、吐球、失败事件   |
| SnapshotSystem    | 同步新球体、吐出物、事件   |
| SkillMetrics      | 统计技能使用次数、失败次数 |

------

# 12. 输入协议设计

## 12.1 SPLIT 输入

客户端发送：

```json
{
  "type": "SPLIT",
  "seq": 1201,
  "clientTime": 1710000000000,
  "data": {
    "roomId": "r_90001",
    "direction": 1.57
  }
}
```

字段说明：

| 字段       | 说明             |
| ---------- | ---------------- |
| type       | 消息类型         |
| seq        | 客户端输入序号   |
| clientTime | 客户端发送时间   |
| roomId     | 房间 ID          |
| direction  | 操作方向，弧度制 |

------

## 12.2 EJECT 输入

客户端发送：

```json
{
  "type": "EJECT",
  "seq": 1202,
  "clientTime": 1710000000100,
  "data": {
    "roomId": "r_90001",
    "direction": 1.57
  }
}
```

------

## 12.3 技能失败消息

服务端返回：

```json
{
  "type": "SKILL_FAILED",
  "seq": 1201,
  "serverTime": 1710000000050,
  "data": {
    "skillType": "SPLIT",
    "reason": "MASS_NOT_ENOUGH",
    "message": "质量不足，无法分裂"
  }
}
```

------

# 13. 分裂技术设计

## 13.1 分裂处理流程

```text
1. 从 InputBuffer 读取 SPLIT 输入
2. 校验房间状态是否 RUNNING
3. 校验玩家状态是否 PLAYING
4. 校验 direction 是否合法
5. 选择待分裂球体
6. 校验球体质量是否足够
7. 校验玩家球体数量是否超过上限
8. 校验分裂冷却
9. 计算新旧球体质量
10. 计算新球体位置
11. 计算新球体初速度
12. 更新原球体质量、半径
13. 创建新球体
14. 设置分裂冷却和合体时间
15. 写入 PLAYER_SPLIT 事件
16. 状态同步给客户端
```

------

## 13.2 分裂目标球选择

MVP：

```text
sourceBall = 玩家当前质量最大的球体
```

如果该球体不满足分裂条件，则分裂失败。

完整版本可选择所有满足条件的球体。

------

## 13.3 分裂条件校验

```text
room.status == RUNNING
player.status == PLAYING
sourceBall.mass >= minSplitMass
player.ballCount < maxSplitBalls
now >= player.nextSplitTime
direction 合法
```

推荐配置：

```json
{
  "minSplitMass": 40,
  "maxSplitBalls": 8,
  "splitCooldownMs": 1000,
  "splitMassRatio": 0.5,
  "splitBoostSpeed": 600,
  "splitBoostDurationMs": 500,
  "mergeDelaySeconds": 10
}
```

------

## 13.4 分裂结果数据

```json
{
  "oldBall": {
    "ballId": "b_10001_1",
    "mass": 640,
    "radius": 63
  },
  "newBall": {
    "ballId": "b_10001_2",
    "userId": "10001",
    "x": 1260.5,
    "y": 800.2,
    "mass": 640,
    "radius": 63,
    "vx": 600,
    "vy": 0,
    "status": "SPLIT_BOOST",
    "canMergeAt": 1710000010000
  }
}
```

------

## 13.5 PLAYER_SPLIT 事件

```json
{
  "eventId": "e_20001",
  "type": "PLAYER_SPLIT",
  "serverTime": 1710000000000,
  "data": {
    "userId": "10001",
    "sourceBallId": "b_10001_1",
    "newBallId": "b_10001_2",
    "direction": 1.57,
    "sourceMass": 640,
    "newMass": 640
  }
}
```

------

# 14. 吐球技术设计

## 14.1 吐球处理流程

```text
1. 从 InputBuffer 读取 EJECT 输入
2. 校验房间状态是否 RUNNING
3. 校验玩家状态是否 PLAYING
4. 校验 direction 是否合法
5. 选择吐球球体
6. 校验球体质量是否足够
7. 校验吐球频率限制
8. 从球体扣除质量
9. 重新计算球体半径和速度
10. 创建吐出物对象
11. 设置吐出物初速度和保护期
12. 写入 PLAYER_EJECT 事件
13. 状态同步给客户端
```

------

## 14.2 吐球目标球选择

MVP：

```text
sourceBall = 玩家当前质量最大的球体
```

完整版本：

```text
每个满足条件的球体都可以吐球
```

------

## 14.3 吐球条件校验

```text
room.status == RUNNING
player.status == PLAYING
sourceBall.mass >= minEjectMass
now >= player.nextEjectTime
direction 合法
```

推荐配置：

```json
{
  "minEjectMass": 25,
  "ejectMass": 5,
  "ejectIntervalMs": 150,
  "maxEjectPerSecond": 5,
  "ejectSpeed": 400,
  "ejectMoveDurationMs": 300,
  "ejectProtectMs": 300,
  "ejectMassRatio": 1.0
}
```

------

## 14.4 吐出物数据结构

```json
{
  "ejectId": "ej_10001_1",
  "ownerId": "10001",
  "x": 1300.5,
  "y": 800.2,
  "radius": 8,
  "mass": 5,
  "vx": 400,
  "vy": 0,
  "status": "MOVING",
  "createTime": 1710000000000,
  "protectUntil": 1710000000300
}
```

------

## 14.5 PLAYER_EJECT 事件

```json
{
  "eventId": "e_20002",
  "type": "PLAYER_EJECT",
  "serverTime": 1710000000000,
  "data": {
    "userId": "10001",
    "sourceBallId": "b_10001_1",
    "ejectId": "ej_10001_1",
    "direction": 1.57,
    "ejectMass": 5,
    "sourceNewMass": 1235
  }
}
```

------

# 15. 多球体控制设计

## 15.1 多球体移动

玩家拥有多个球体时，所有球体共享同一个移动方向。

```text
客户端发送一个 MOVE direction
服务端将该 direction 应用于该玩家的所有球体
```

不同球体根据自己的质量计算速度。

------

## 15.2 多球体视野

MVP：

```text
以所有球体中心平均值作为视野中心
```

完整版本：

```text
多个球体视野范围取并集
```

------

## 15.3 多球体排行榜

玩家得分和排名按玩家总质量或总分计算。

```text
player.totalMass = sum(player.balls.mass)
player.score = totalMass + 其他加分项
```

------

## 15.4 多球体死亡

当玩家所有球体都被吞噬时，玩家死亡。

```text
if len(player.balls) == 0:
    player.status = DEAD
```

------

# 16. 合体概要设计

合体属于分裂系统的后续扩展能力。

## 16.1 合体条件

1. 两个球体属于同一玩家。
2. 两个球体均超过 canMergeAt。
3. 两个球体距离足够近。
4. 房间处于 RUNNING。
5. 玩家处于 PLAYING。

------

## 16.2 合体结果

```text
ballA.mass += ballB.mass
移除 ballB
重新计算 ballA.radius
生成 PLAYER_MERGE 事件
```

MVP 阶段可以暂不实现，只保留：

```text
canMergeAt 字段
```

------

# 17. 与碰撞系统的关系

## 17.1 分裂后参与碰撞

分裂生成的新球体需要立即加入：

1. PlayerBallManager。
2. EntityManager。
3. SpatialGrid。
4. CollisionSystem。
5. AOI 系统。
6. SnapshotSystem。

但是可配置：

```text
分裂后短时间内不能被同玩家球体合体
```

------

## 17.2 吐出物参与碰撞

吐出物需要加入：

1. EntityManager。
2. SpatialGrid。
3. CollisionSystem。
4. AOI 系统。
5. SnapshotSystem。

吐出物可被：

1. 自己吞噬，保护期后。
2. 其他玩家吞噬。
3. 刺球吸收，后续支持。

------

# 18. 与状态同步系统的关系

分裂与吐球不直接给客户端发最终状态，而是通过事件和快照同步。

```text
SkillSystem
  ↓
更新 RoomState
  ↓
写入 EventBuffer
  ↓
SnapshotSystem 生成快照
  ↓
客户端表现
```

同步内容包括：

| 操作 | 同步内容                             |
| ---- | ------------------------------------ |
| 分裂 | 新球体、原球体质量变化、分裂事件     |
| 吐球 | 吐出物新增、原球体质量变化、吐球事件 |
| 失败 | SKILL_FAILED 消息                    |
| 合体 | 球体减少、质量合并、合体事件         |

------

# 19. 与 AOI 系统的关系

## 19.1 新球体进入 AOI

分裂生成的新球体需要：

1. 更新空间网格。
2. 进入周围玩家的 AOI 可见集合。
3. 在对应玩家快照中以 entered 对象同步。

------

## 19.2 吐出物进入 AOI

吐出物生成后：

1. 进入空间网格。
2. 进入附近玩家 AOI。
3. 被 AOI 快照下发。
4. 后续移动或停止时继续同步。

------

## 19.3 离开 AOI

分裂球体和吐出物离开某玩家视野后：

1. 从该玩家可见集合中移除。
2. 下发 left 对象。
3. 客户端隐藏或回收。

------

# 20. 冷却与频率控制

## 20.1 CooldownState

```json
{
  "userId": "10001",
  "nextSplitTime": 1710000001000,
  "nextEjectTime": 1710000000150,
  "ejectCountInWindow": 2,
  "windowStartTime": 1710000000000
}
```

------

## 20.2 分裂冷却

```text
if now < nextSplitTime:
    reject SPLIT
```

成功分裂后：

```text
nextSplitTime = now + splitCooldownMs
```

------

## 20.3 吐球频率

吐球同时限制：

1. 最小间隔。
2. 每秒最大次数。

```text
if now < nextEjectTime:
    reject EJECT

if ejectCountInWindow >= maxEjectPerSecond:
    reject EJECT
```

------

# 21. 技能失败原因设计

| reason              | 说明         | 客户端文案           |
| ------------------- | ------------ | -------------------- |
| ROOM_NOT_RUNNING    | 房间未运行   | 对局未开始           |
| PLAYER_NOT_PLAYING  | 玩家不可操作 | 当前无法操作         |
| MASS_NOT_ENOUGH     | 质量不足     | 质量不足，无法操作   |
| SPLIT_LIMIT_REACHED | 分身达到上限 | 分身数量已达上限     |
| SPLIT_COOLDOWN      | 分裂冷却中   | 分裂冷却中           |
| EJECT_COOLDOWN      | 吐球冷却中   | 吐球太频繁了         |
| INVALID_DIRECTION   | 方向异常     | 操作方向异常         |
| PLAYER_DEAD         | 玩家已死亡   | 已被吞噬，无法操作   |
| SYSTEM_ERROR        | 系统异常     | 操作失败，请稍后重试 |

------

# 22. 错误码设计

| 错误码 | 含义             | 客户端处理       |
| ------ | ---------------- | ---------------- |
| 0      | 成功             | 正常处理         |
| 44001  | 房间未运行       | 提示无法操作     |
| 44002  | 玩家不可操作     | 提示当前无法操作 |
| 44003  | 质量不足         | 置灰按钮或提示   |
| 44004  | 分身数量达到上限 | 提示上限         |
| 44005  | 分裂冷却中       | 显示冷却         |
| 44006  | 吐球冷却中       | 显示冷却         |
| 44007  | 方向非法         | 忽略本次操作     |
| 44008  | 吐球频率过高     | 限制连续点击     |
| 44009  | 球体不存在       | 请求状态恢复     |
| 50000  | 系统异常         | 稍后重试         |

------

# 23. 日志设计

## 23.1 关键日志点

| 日志点                | 说明                   |
| --------------------- | ---------------------- |
| split_request         | 收到分裂请求           |
| split_success         | 分裂成功               |
| split_failed          | 分裂失败               |
| eject_request         | 收到吐球请求           |
| eject_success         | 吐球成功               |
| eject_failed          | 吐球失败               |
| skill_cooldown_reject | 冷却拒绝               |
| skill_mass_not_enough | 质量不足               |
| player_ball_created   | 创建新球体             |
| ejected_mass_created  | 创建吐出物             |
| player_merge          | 玩家球体合体，后续支持 |

------

## 23.2 日志字段示例

```json
{
  "level": "info",
  "traceId": "trace_xxx",
  "roomId": "r_90001",
  "userId": "10001",
  "ballId": "b_10001_1",
  "skillType": "SPLIT",
  "result": "success",
  "direction": 1.57,
  "message": "split_success",
  "timestamp": "2026-06-28T10:00:00.000Z"
}
```

------

# 24. 监控指标

| 指标                               | 说明             |
| ---------------------------------- | ---------------- |
| skill_split_total                  | 分裂请求总数     |
| skill_split_success_total          | 分裂成功数       |
| skill_split_failed_total           | 分裂失败数       |
| skill_eject_total                  | 吐球请求总数     |
| skill_eject_success_total          | 吐球成功数       |
| skill_eject_failed_total           | 吐球失败数       |
| skill_reject_mass_not_enough_total | 质量不足拒绝次数 |
| skill_reject_cooldown_total        | 冷却拒绝次数     |
| player_ball_count                  | 玩家当前球体数量 |
| ejected_mass_count                 | 房间内吐出物数量 |
| skill_process_duration_ms          | 技能处理耗时     |

------

# 25. 安全与反作弊

## 25.1 作弊风险

| 风险             | 防护                                     |
| ---------------- | ---------------------------------------- |
| 客户端伪造新球体 | 新球体只由服务端创建                     |
| 客户端伪造吐出物 | 吐出物只由服务端创建                     |
| 客户端绕过冷却   | 服务端记录 nextSplitTime / nextEjectTime |
| 客户端伪造质量   | 服务端校验质量并计算扣减                 |
| 客户端高频刷包   | 服务端限频并记录异常                     |
| 客户端伪造方向   | 服务端校验 direction 范围                |
| 客户端重复 seq   | 服务端丢弃旧输入或重复输入               |

------

## 25.2 服务端校验原则

每次技能操作必须校验：

1. 房间状态。
2. 玩家状态。
3. 玩家是否属于房间。
4. 玩家是否有可操作球体。
5. 操作方向是否合法。
6. 质量是否满足。
7. 冷却是否结束。
8. 频率是否过高。
9. 分身数量是否超过限制。
10. 是否存在重复或过期输入。

------

# 26. 配置设计

```json
{
  "skill": {
    "split": {
      "minSplitMass": 40,
      "maxSplitBalls": 8,
      "splitCooldownMs": 1000,
      "splitMassRatio": 0.5,
      "splitBoostSpeed": 600,
      "splitBoostDurationMs": 500,
      "mergeDelaySeconds": 10
    },
    "eject": {
      "minEjectMass": 25,
      "ejectMass": 5,
      "ejectMassRatio": 1.0,
      "ejectIntervalMs": 150,
      "maxEjectPerSecond": 5,
      "ejectSpeed": 400,
      "ejectMoveDurationMs": 300,
      "ejectProtectMs": 300
    }
  }
}
```

------

# 27. MVP 开发任务拆分

## 27.1 客户端任务

| 任务       | 说明                             |
| ---------- | -------------------------------- |
| 分裂按钮   | 对战页展示分裂按钮               |
| 吐球按钮   | 对战页展示吐球按钮               |
| SPLIT 输入 | 点击分裂发送 SPLIT 消息          |
| EJECT 输入 | 点击吐球发送 EJECT 消息          |
| 冷却表现   | 根据服务端结果或本地预测展示冷却 |
| 失败提示   | 处理 SKILL_FAILED                |
| 分裂表现   | 收到 PLAYER_SPLIT 后播放表现     |
| 吐球表现   | 收到 PLAYER_EJECT 后播放表现     |
| 多球体渲染 | 渲染同玩家多个球体               |
| 吐出物渲染 | 渲染地图上的吐出物               |

------

## 27.2 服务端任务

| 任务                 | 说明                             |
| -------------------- | -------------------------------- |
| SPLIT 输入处理       | 从 InputBuffer 读取分裂请求      |
| EJECT 输入处理       | 从 InputBuffer 读取吐球请求      |
| 技能条件校验         | 校验房间、玩家、质量、冷却       |
| SplitSystem          | 创建新球体并更新原球体           |
| EjectSystem          | 创建吐出物并扣除质量             |
| CooldownManager      | 维护分裂和吐球冷却               |
| PlayerBallManager    | 支持一个玩家多个球体             |
| EntityManager        | 管理吐出物对象                   |
| EventBuffer          | 写入 PLAYER_SPLIT / PLAYER_EJECT |
| SnapshotSystem       | 同步新球体和吐出物               |
| CollisionSystem 接入 | 新球体和吐出物参与碰撞           |
| AOI 接入             | 新对象进入空间索引和视野同步     |

------

# 28. 后续详细设计拆分建议

分裂与吐球系统后续可以继续拆成：

1. SplitSystem 详细设计。
2. EjectSystem 详细设计。
3. 多球体控制详细设计。
4. 同玩家合体系统设计。
5. 吐出物运动与保护期设计。
6. 技能冷却与限频设计。
7. 客户端技能按钮交互设计。
8. 分裂追击数值设计。
9. 吐球战术与质量平衡设计。
10. 分裂与碰撞系统联动设计。
11. 技能系统压测设计。

------

# 29. 总结

分裂与吐球系统是《吞噬细胞》中提升操作深度和竞技性的核心玩法模块。

核心链路如下：

```text
客户端点击技能
  ↓
发送 SPLIT / EJECT
  ↓
服务端校验条件
  ↓
执行分裂 / 吐球
  ↓
更新房间状态
  ↓
生成事件
  ↓
状态同步
  ↓
客户端表现
```

当前阶段应优先保证：

1. 服务端权威创建新球体和吐出物。
2. 分裂和吐球条件清晰。
3. 质量扣减和质量分配正确。
4. 多球体能正常参与移动和吞噬。
5. 吐出物能正常移动、停留和被吞噬。
6. 客户端有明确按钮状态和失败提示。
7. 后续能继续扩展合体、刺球、组队喂球等能力。

MVP 阶段不需要一次性实现复杂合体和高级多球体操控。先实现分裂、吐球、质量变化、事件同步和客户端表现