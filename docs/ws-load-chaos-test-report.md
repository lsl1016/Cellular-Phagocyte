# Cellular-Phagocyte WebSocket 压力与弱网测试报告

> 测试日期：2026-09-15  
> 测试分支：`feature/ws-load-chaos`  
> 关联 PR：#14 `feature/real-e2e`、#15 `feature/ws-load-chaos`  
> 测试对象：Go Server、HTTP API、WebSocket 实时链路、AOI FULL/DELTA、重连恢复与 FULL_SYNC 机制

---

## 1. 测试背景

`Cellular-Phagocyte` 已具备真实服务进程 E2E、WebSocket 可靠消息、AOI FULL/DELTA、断线重连和快照恢复能力。为了验证这些机制在多客户端并发、网络抖动、消息丢弃和集中重连场景下是否仍能工作，本次增加了一套真实 WebSocket 压力与弱网测试体系。

本次测试不是直接调用 `Room` 内部方法，也不是基于 `httptest.NewServer` 的进程内测试。压力客户端均通过真实 HTTP API 和真实 WebSocket 网关完成登录、匹配、入房、开局、移动、断线、重连和快照恢复。

主要验证以下能力：

1. 数十至数百 WebSocket 客户端同时在线时，服务端能否稳定完成登录、匹配、入房和开局。
2. 多客户端持续发送 `MOVE` 时，实时链路是否出现异常读写、明显阻塞或崩溃。
3. AOI FULL / DELTA 在客户端漏帧后能否通过 `FULL_SYNC` 正确恢复 baseline。
4. 大规模集中断线后，服务端能否处理重连风暴。
5. 在 TCP 真实延迟、抖动和丢包条件下，连接和快照链能否继续工作。
6. 300 客户端、多房间场景下的初步容量与稳定性表现。

---

## 2. 测试范围

### 2.1 本次覆盖

本次压力测试覆盖以下真实链路：

```text
Guest Login
    -> Match Start
    -> Match Status / MATCHED
    -> WebSocket Dial
    -> ENTER_ROOM
    -> READY
    -> GAME_START
    -> AOI_FULL
    -> AOI_DELTA
    -> MOVE traffic
    -> intentional disconnect
    -> fresh TCP/WebSocket connection
    -> RECONNECT
    -> RECONNECT_RESULT
    -> AOI_FULL_RECOVER
    -> resume AOI_DELTA
    -> FULL_SYNC when baseline mismatch occurs
```

### 2.2 本次未覆盖

本次测试暂未覆盖：

- Cocos Creator 图形客户端真实渲染性能；
- 多服务器节点横向扩容；
- Redis Matcher 多实例竞争；
- 真实公网跨地域弱网；
- 5 分钟以上持续 Soak Test；
- 服务端 CPU / Memory / Goroutine / GC / Tick latency 的完整容量曲线；
- 500 / 1000+ 并发 WebSocket 的极限容量测试。

因此本报告结论应理解为：**当前单机服务端在已执行的 50 / 100 / 300 客户端测试条件下表现符合设定门槛，但不能直接等价为生产容量上限。**

---

## 3. 测试工具与实现

### 3.1 压测程序

新增独立压测程序：

```text
server/cmd/wsload
```

该程序是一个独立可执行客户端，通过真实 HTTP 和 WebSocket 与另外一个独立运行的游戏 Server 进程通信。

支持配置：

- 客户端数量；
- 运行时长；
- MOVE 发送频率；
- 应用层延迟；
- jitter；
- 入站 Snapshot 丢弃；
- 出站 MOVE 丢弃；
- reconnect storm 比例；
- reconnect storm 次数；
- 随机 churn；
- 重连 jitter；
- setup concurrency；
- 匹配超时；
- 重连成功率门槛；
- Snapshot baseline 健康率门槛；
- deterministic random seed。

### 3.2 每个客户端维护的状态

每个 load client 独立维护：

- `userId`
- `accessToken`
- `matchId`
- `roomId`
- `enterToken`
- `reconnectToken`
- 当前 WebSocket connection
- 客户端输入 `seq`
- `lastSnapshotSeq`
- `needsFull`
- reconnect CAS 状态

因此每个客户端都具备独立的 AOI DELTA baseline 校验能力。

### 3.3 AOI 恢复逻辑

正常快照链：

```text
AOI_FULL seq=100
    -> AOI_DELTA baseSeq=100 seq=101
    -> AOI_DELTA baseSeq=101 seq=102
```

若客户端漏掉中间 DELTA：

```text
client lastSnapshotSeq = 100
server sends DELTA baseSeq = 101
                |
                +-- mismatch
                    -> mark needsFull
                    -> send FULL_SYNC
                    -> receive AOI_FULL
                    -> rebuild baseline
```

因此测试不仅检查 WebSocket 是否仍然连接，还检查 AOI 增量协议是否能在弱网下自动恢复。

---

## 4. 弱网注入方式

本次测试使用两类弱网方式。

### 4.1 应用层 Chaos

应用层可以主动：

- 延迟处理消息；
- 增加随机 jitter；
- 忽略部分收到的 Snapshot；
- 不发送部分 MOVE；
- 主动关闭 WebSocket；
- 批量触发集中重连风暴；
- 每秒随机触发少量 churn。

这种方式适合稳定复现协议恢复逻辑。

### 4.2 Linux `tc netem`

100 客户端资格测试使用了 Linux `tc netem`：

```bash
sudo tc qdisc replace dev lo root netem \
  delay 60ms 20ms distribution normal \
  loss 1%
```

这属于 TCP / 网络层真实 packet loss。

需要注意：TCP 层丢包并不等于“随机少一条 WebSocket 消息”。TCP 会进行重传，因此其主要表现通常包括：

- RTT 增加；
- 抖动增加；
- 排队与拥塞；
- 重传导致尾延迟升高；
- 严重情况下连接超时或断开。

所以本次同时保留了 TCP 层 `netem` 和应用层 Snapshot/MOVE drop 两类故障模型。

---

## 5. 测试指标

`wsload` 最终输出人类可读报告及：

```text
WS_LOAD_CHAOS_SUMMARY {...}
```

主要指标包括：

| 指标 | 含义 |
| --- | --- |
| `clients` | 总压测客户端数 |
| `rooms` | 实际进入的房间数 |
| `gameStarts` | 收到 GAME_START 的客户端数 |
| `snapshotClients` | 成功收到 Snapshot 的客户端数 |
| `fullSnapshots` | AOI FULL 数量 |
| `deltaSnapshots` | AOI DELTA 数量 |
| `recoverSnapshots` | 重连恢复 Snapshot 数量 |
| `incomingSnapshotsDropped` | 应用层主动丢弃 Snapshot 数量 |
| `sequenceMismatches` | DELTA `baseSeq` 与本地 baseline 不一致次数 |
| `fullSyncRequests` | 客户端主动请求 FULL_SYNC 次数 |
| `movesSent` | 实际发送 MOVE 数量 |
| `movesDropped` | 应用层主动丢弃 MOVE 数量 |
| `forcedDisconnects` | 主动断线次数 |
| `reconnectAttempts` | 重连尝试次数 |
| `reconnectSuccess` | 重连成功次数 |
| `reconnectFailures` | 重连失败次数 |
| `snapshotHealthyRate` | 测试结束时 Snapshot baseline 健康率 |
| `unexpectedReadErrors` | 非预期 WebSocket 读错误 |
| `writeErrors` | WebSocket 写错误 |
| `bytesReceived` | 客户端总接收字节数 |
| Reconnect P50/P95/P99 | 重连延迟分位数 |
| Snapshot Lag P50/P95/P99 | Snapshot 接收滞后分位数 |

---

## 6. 场景一：50 客户端 CI Chaos Smoke

### 6.1 测试配置

```text
clients                 = 50
duration                = 10s
move interval           = 200ms
application latency     = 15ms
application jitter      = +/-20ms
incoming snapshot drop  = 0%
outgoing MOVE drop      = 3%
reconnect storm         = 30% clients x 2 rounds
random churn            = 1% clients / second
reconnect jitter        = <=300ms
```

服务端配置：

```text
MATCH_MIN_PLAYERS = 50
GAME_BOTS         = 0
AOI               = enabled
FULL interval     = 5s
```

50 个客户端进入同一个真人房间。

### 6.2 实测结果

```text
clients                     50
rooms                       1
gameStarts                  50/50
snapshotHealthy             50/50 = 100%
forcedDisconnects           40
reconnectSuccess            40/40 = 100%
reconnectFailures           0
movesSent                   2358
movesDropped                79
FULL snapshots              132
DELTA snapshots             5910
RECOVER snapshots           40
sequenceMismatches          1
FULL_SYNC requests          1
unexpectedReadErrors        0
writeErrors                 0
bytesReceived               17,294,230
reconnect latency P50       1.418 ms
reconnect latency P95       2.920 ms
reconnect latency P99       4.040 ms
snapshot lag P50            19 ms
snapshot lag P95            36 ms
snapshot lag P99            38 ms
```

最终输出：

```text
WS_LOAD_CHAOS PASS
```

### 6.3 结果分析

本场景下：

- 50/50 客户端全部完成开局；
- 40 次主动断线均成功重连；
- 测试结束时所有客户端 Snapshot baseline 均处于健康状态；
- 无非预期 WebSocket 读错误；
- 无 WebSocket 写错误；
- 两轮 reconnect storm 未造成服务端进程异常；
- DELTA baseline 曾出现 1 次不连续，并成功通过 1 次 `FULL_SYNC` 修复。

**结论：通过。**

该场景适合作为普通 Pull Request 的稳定硬门禁。

---

## 7. 场景二：100 客户端真实 TCP 弱网测试

### 7.1 测试配置

客户端：

```text
clients                   = 100
rooms                     = 1
reconnect storm           = 50% clients x 2 rounds
random churn              = 1% / second
application snapshot drop = 1%
application MOVE drop     = 1%
```

Linux `tc netem`：

```text
network delay  = 60ms
network jitter = 20ms
packet loss    = 1%
```

对应命令：

```bash
sudo tc qdisc replace dev lo root netem \
  delay 60ms 20ms distribution normal \
  loss 1%
```

### 7.2 实测结果

```text
gameStarts                  100/100
forcedDisconnects           112
reconnectSuccess            112/112 = 100%
reconnectFailures           0
final snapshotHealthy       99/100 = 99%
FULL snapshots              368
DELTA snapshots             14114
RECOVER snapshots           112
incoming snapshots dropped  147
sequenceMismatches          311
FULL_SYNC requests          146
movesSent                   5520
movesDropped                76
unexpectedReadErrors        0
writeErrors                 2
bytesReceived               79,965,501
reconnect latency P50       371.430 ms
reconnect latency P95       697.385 ms
reconnect latency P99       1388.344 ms
snapshot lag P50            75 ms
snapshot lag P95            119 ms
snapshot lag P99            253 ms
```

最终输出：

```text
WS_LOAD_CHAOS PASS
```

### 7.3 结果分析

本场景是本次测试中最接近真实弱网的一组，因为网络故障由 Linux `tc netem` 注入到 TCP 层。

主要观察：

1. 100 个客户端均正常进入游戏。
2. 共发生 112 次真实 WebSocket 断线重连，全部成功。
3. 1% TCP 丢包和 60ms ±20ms 网络延迟显著拉高了重连 P95/P99。
4. 产生 311 次 AOI DELTA baseline mismatch。
5. 客户端主动发送 146 次 `FULL_SYNC`，说明恢复机制在真实弱网下被实际触发。
6. 最终 99/100 客户端 baseline 健康，达到设定的 95% 资格测试门槛。
7. 记录到 2 次 WebSocket write error，需要继续在更长 soak 场景下观察其频率和根因。

**结论：通过。**

当前数据说明 AOI DELTA + FULL_SYNC 机制能够在真实 TCP 弱网下完成恢复，但高尾延迟以及少量写错误仍值得后续持续关注。

---

## 8. 场景三：300 客户端多房间重连风暴

### 8.1 测试配置

```text
clients                   = 300
room capacity              = 100
actual rooms               = 3
application latency        = 20ms
application jitter         = +/-30ms
incoming snapshot drop     = 2%
outgoing MOVE drop         = 2%
reconnect storm            = 50% clients x 2 rounds
simultaneous disconnects   = 150 clients / storm
random churn               = 1% / second
reconnect jitter           = <=1000ms
```

300 个用户最终分配为：

```text
Room 1: 100
Room 2: 100
Room 3: 100
```

### 8.2 实测结果

```text
gameStarts                  300/300
forcedDisconnects           335
reconnectSuccess            334/335 = 99.70%
reconnectFailures           1
final snapshotHealthy       287/300 = 95.67%
FULL snapshots              1391
DELTA snapshots             42428
RECOVER snapshots           334
incoming snapshots dropped  904
sequenceMismatches          1268
FULL_SYNC requests          857
movesSent                   13324
movesDropped                302
unexpectedReadErrors        0
writeErrors                 0
bytesReceived               219,154,520
reconnect latency P50       1.035 ms
reconnect latency P95       4.989 ms
reconnect latency P99       9.879 ms
snapshot lag P50            27 ms
snapshot lag P95            54 ms
snapshot lag P99            60 ms
```

最终输出：

```text
WS_LOAD_CHAOS PASS
```

### 8.3 结果分析

300 客户端场景下的主要结果：

- 300/300 客户端全部成功开局；
- 共发生 335 次主动断线；
- 334 次首次重连成功；
- 1 次首次重连失败；
- 重连首次成功率 99.70%；
- 最终 Snapshot baseline 健康率 95.67%；
- 产生 1268 次 baseline mismatch；
- 触发 857 次 FULL_SYNC；
- 无非预期 read error；
- 无 write error。

该场景达到了预先设定的 95% 资格测试门槛。

但需要强调：**该结果不应描述为“300 客户端完全零错误稳定运行”。**

原因包括：

1. 出现了 1 次首次重连失败；
2. 测试结束时仍有 13 个客户端处于未完全恢复 baseline 的状态；
3. 测试持续时间较短，还不能证明长时间运行时不存在资源泄漏或延迟恶化；
4. 当前没有同时采集服务端 CPU、Memory、GC、Goroutine 和 Tick latency，尚不能得出单机最大容量结论。

**结论：通过资格门槛，但存在容量与恢复边界，需要继续进行 Soak Test 和服务端资源监控。**

---

## 9. 三档结果汇总

| 场景 | 客户端 | 房间 | 重连成功率 | 最终 Snapshot 健康率 | 主要结果 | 判定 |
| --- | ---: | ---: | ---: | ---: | --- | --- |
| PR Chaos Smoke | 50 | 1 | 100% | 100% | 40/40 重连成功，0 非预期读写错误 | 通过 |
| TCP 真弱网 | 100 | 1 | 100% | 99% | 1% TCP 丢包下 112/112 重连成功 | 通过 |
| 多房间压力 | 300 | 3 | 99.70% | 95.67% | 334/335 首次重连成功，1 次失败 | 通过资格门槛 |

---

## 10. AOI / FULL_SYNC 恢复能力分析

本次测试的重要价值之一，是实际验证了 AOI DELTA baseline 恢复机制。

三档测试中的 mismatch / FULL_SYNC：

| 场景 | Sequence Mismatch | FULL_SYNC |
| --- | ---: | ---: |
| 50 客户端 | 1 | 1 |
| 100 客户端真实弱网 | 311 | 146 |
| 300 客户端 | 1268 | 857 |

在 100 和 300 客户端场景中，弱网和故意漏帧产生了大量 DELTA baseline 不连续，客户端没有继续盲目应用错误 DELTA，而是主动进入恢复流程：

```text
DELTA base mismatch
    -> needsFull = true
    -> FULL_SYNC
    -> AOI_FULL
    -> reset baseline
    -> resume DELTA
```

这说明当前 AOI 增量协议已经具备基本的故障恢复闭环。

---

## 11. 重连风暴分析

一次 reconnect storm 会执行：

```text
selected clients
    -> Close old WebSocket
    -> random reconnect jitter
    -> open fresh TCP connection
    -> WebSocket handshake
    -> RECONNECT seq=1
    -> RECONNECT_RESULT
    -> AOI_FULL_RECOVER
    -> reset client input sequence space
    -> resume MOVE / DELTA
```

300 客户端场景每轮同时选择约 150 个客户端断开，两轮 storm 叠加随机 churn 后，总断线次数达到 335。

结果显示：

- 服务进程未崩溃；
- 房间仍继续 Tick；
- 大部分连接能够快速恢复；
- 未出现大面积 read/write error；
- 99.70% 首次重连成功。

同时，1 次首次重连失败说明后续应该把测试指标进一步拆分为：

```text
first reconnect success rate
retry-to-recovery success rate
recovery time P95/P99
```

这样可以更准确地区分“首次尝试偶发失败”和“最终无法恢复”。

---

## 12. CI 集成结果

最终分支 CI Run #60：

```text
Run ID: 34962416911
```

全部通过：

```text
Go server tests / build / benchmark smoke   PASS
Real process E2E                            PASS
WebSocket 50-client chaos smoke            PASS
Cocos core smoke                            PASS
Legacy H5 smoke                             PASS
```

其中 50-client Chaos Smoke 已作为普通 PR CI 的稳定门禁。

日常 PR 不持续启用随机 incoming Snapshot drop，因为资格测试阶段发现，如果在测试结束前仍持续随机漏 Snapshot，最终健康率会受“采样瞬间客户端恰好等待 FULL_SYNC”影响，形成无意义的 flaky gate。

因此当前策略为：

### 日常 PR

```text
50 clients
application latency / jitter
MOVE drop
reconnect storm
random churn
98% reconnect gate
98% snapshot health gate
```

### 合并前 / 发版前手动测试

```text
100 / 300 clients
Linux tc netem
real TCP packet loss
snapshot drop
MOVE drop
large reconnect storm
```

这种分层可以兼顾 CI 稳定性和真实弱网覆盖。

---

## 13. 当前已发现的问题与风险

### 13.1 300 客户端出现 1 次首次重连失败

严重程度：中。

当前数据：

```text
334 / 335 = 99.70%
```

建议：

- 为 load client 增加有限次数自动 retry；
- 分离 first-attempt 与 final-recovery 指标；
- 记录失败类型：dial / handshake / RECONNECT reject / recover timeout；
- 增加 5-10 分钟 soak 后重新统计错误率。

### 13.2 100 客户端真实弱网出现 2 次 write error

严重程度：低到中。

当前没有造成重连失败或大面积连接异常，但建议后续记录具体 WebSocket error 类型及客户端 ID，区分：

- intentional reconnect 窗口；
- TCP 网络抖动；
- deadline；
- closed pipe；
- 服务端主动关闭。

### 13.3 300 客户端最终 Snapshot 健康率 95.67%

严重程度：中。

虽然达到资格测试 95% 门槛，但不适合作为生产目标。

建议最终产品 SLO 至少拆成：

- 正常网络 baseline healthy >= 99.9%；
- 弱网故障后 N 秒内恢复比例；
- FULL_SYNC recovery P95 / P99；
- 持续 mismatch 客户端比例。

### 13.4 暂未采集服务端资源数据

当前测试主要回答“协议是否还能工作”，尚未完整回答“单机还剩多少容量”。

后续至少应同时采集：

- CPU 使用率；
- RSS / Heap；
- Goroutine；
- GC pause；
- allocation rate；
- Tick duration P50/P95/P99；
- Snapshot encode duration；
- WebSocket outbound queue depth；
- dropped latest-only snapshots；
- 房间数；
- 在线连接数。

---

## 14. 建议的下一阶段测试

### 14.1 300 客户端 5-10 分钟 Soak Test

建议场景：

```text
clients          = 300
duration         = 5-10 min
rooms            = 3
MOVE interval    = 200-250ms
network delay    = 50-100ms
jitter           = 20-50ms
packet loss      = 1-3%
reconnect storm  = 50% x multiple rounds
random churn     = 1% / second
```

重点观察：

- Goroutine 是否持续增长；
- Heap 是否不能回落；
- GC 是否恶化；
- Tick P99 是否逐步变高；
- FULL_SYNC 是否形成持续风暴；
- reconnect failure 是否随时间增加。

### 14.2 容量阶梯测试

建议逐级执行：

```text
50
100
200
300
500
800
1000
```

每档保持固定房间人数和固定 MOVE 频率，并记录服务端 CPU / Memory / Tick P99，最终得到单机容量曲线，而不是仅凭连接数判断容量。

### 14.3 更极端弱网档位

建议增加：

```text
100ms latency + 50ms jitter + 3% packet loss
200ms latency + 100ms jitter + 5% packet loss
short burst loss
temporary 1-3s blackhole
reconnect storm during packet loss
```

其中“短时间完全断网再恢复”比单纯随机 packet loss 更接近移动端切网、地铁、电梯等真实场景。

---

## 15. 最终结论

本次压力与弱网测试完成了从普通功能测试向真实多客户端运行时验证的进一步推进。

当前可以确认：

1. 50 个真实 WebSocket 客户端在持续 MOVE、延迟、jitter、MOVE 丢弃、随机 churn 和两轮 reconnect storm 下可以稳定运行，40/40 次重连成功，最终 Snapshot 健康率 100%。
2. 100 个客户端在 Linux `tc netem` 真实 60ms ±20ms、1% TCP packet loss 环境下，112/112 次重连成功，最终 Snapshot 健康率 99%。
3. AOI DELTA baseline mismatch 在真实弱网下会实际发生，客户端可以通过 `FULL_SYNC` 恢复，而不是继续应用错误增量。
4. 300 客户端可以在三个 100 人房间中完成开局和持续实时通信，两轮每次约 150 人集中断线后，首次重连成功率为 99.70%。
5. 300 客户端测试中存在 1 次首次重连失败，最终 Snapshot 健康率为 95.67%，说明当前系统已经进入需要进一步关注容量、恢复时延和长时间稳定性的阶段。
6. 当前测试尚不能给出生产单机最大容量，应继续结合 5-10 分钟 Soak Test 和 CPU / Memory / Goroutine / GC / Tick latency 指标进行容量评估。

综合判定：

```text
50-client PR Chaos Smoke        PASS
100-client real weak network    PASS
300-client multi-room stress    PASS WITH OBSERVED RISK
```

总体结论：**当前 WebSocket、AOI DELTA、FULL_SYNC 和断线重连机制已经通过本阶段真实并发与弱网资格测试，可以进入下一阶段的长时间 Soak Test 和服务端容量评估。**

---

## 16. 相关资产

| 文件 | 作用 |
| --- | --- |
| `server/cmd/wsload` | 多客户端真实 WebSocket 压测/Chaos 驱动 |
| `.github/workflows/ci.yml` | 50-client 日常 PR Chaos Smoke |
| `.github/workflows/ws-load-chaos.yml` | 手动 50/100/300 客户端 + `tc netem` 弱网测试 |
| `docs/ws-load-chaos.md` | 压测工具使用说明与参数说明 |
| `docs/ws-load-chaos-test-report.md` | 本测试报告 |

### PR 合并顺序

当前 PR #15 依赖 PR #14 的真实进程 E2E 和重连输入序号修复，推荐顺序：

```text
PR #14 feature/real-e2e
    -> PR #15 feature/ws-load-chaos
```

在 PR #14 合并前，不建议先独立合并 PR #15。
