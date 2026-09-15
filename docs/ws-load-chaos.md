# WebSocket 压测与弱网 / Chaos 测试

本项目提供 `server/cmd/wsload`，用于对真实运行中的 Go Server 发起几十到数百个真实 WebSocket 客户端，并持续执行登录、匹配、入房、READY、移动、AOI 快照、FULL_SYNC、断线与重连。

该工具不是直接调用 Room 方法，也不是 `httptest`。所有客户端均通过真实 HTTP API 与真实 WebSocket 网关工作。

## 1. 测试层次

### 1.1 CI：50 客户端稳定 Chaos smoke

`.github/workflows/ci.yml` 中的 `WebSocket 50-client chaos smoke` 每次 PR 都执行：

- 50 个真人客户端；
- 真实 HTTP 游客登录；
- 真实匹配；
- 真实 WebSocket `ENTER_ROOM`；
- 全部进入后再统一 `READY`；
- 持续发送 `MOVE`；
- 15ms 延迟和 +/-20ms 应用层 jitter；
- 随机丢弃 3% 客户端 MOVE；
- 2 轮 30% 客户端集中断线 / 重连风暴；
- 每秒 1% 随机连接 churn；
- 最终检查重连成功率以及快照链健康率。

日常 PR 硬门禁不持续随机丢弃 `ROOM_SNAPSHOT`。原因是持续随机漏快照会让最终某一毫秒的 `snapshotHealthyRate` 取决于有多少客户端恰好处于 `FULL_SYNC` 修复窗口，从而制造无意义的 CI 抖动。

AOI DELTA 丢帧 / `FULL_SYNC` 恢复仍由以下两层覆盖：

1. 独立进程 Real E2E 中显式执行 `FULL_SYNC`；
2. 手动 `tc netem` 弱网工作流中保留少量应用层 snapshot drop，并同时注入真实 TCP packet loss。

### 1.2 手动工作流：Linux `tc netem` 真弱网

`.github/workflows/ws-load-chaos.yml` 是手动工作流，支持：

- 50 / 100 / 300 客户端；
- Linux loopback 上真正的 packet delay；
- jitter；
- packet loss；
- 多轮 reconnect storm；
- 随机 churn；
- 同时保留少量应用层 snapshot / MOVE drop，用来验证 FULL_SYNC 恢复。

例如默认配置：

```text
100 clients
80ms delay
30ms jitter
2% packet loss
50% clients per reconnect storm
2 reconnect storms
```

`tc netem` 发生在 TCP 层，因此 WebSocket 本身不会简单“少一条消息”。TCP 会重传丢失的数据包，表现为额外延迟、拥塞、超时，严重时才会断开。这和应用层故意漏处理一帧 AOI DELTA 是两种不同故障，因此两层都保留。

## 2. 本地启动 Server

示例：100 真人同房。

```bash
cd server

HTTP_ADDR=127.0.0.1:18080 \
WS_HOST=127.0.0.1:18080 \
STORAGE=memory \
GAME_BATTLE_SECONDS=90 \
GAME_COUNTDOWN_SECONDS=0 \
GAME_BOTS=0 \
GAME_INIT_MASS=20 \
GAME_AOI_ENABLED=true \
GAME_AOI_FULL_INTERVAL_SECONDS=5 \
MATCH_MIN_PLAYERS=100 \
MATCH_MAX_WAIT_SECONDS=8 \
go run ./cmd/server
```

另开终端执行：

```bash
cd server

go run ./cmd/wsload \
  -base-url http://127.0.0.1:18080 \
  -ws-url ws://127.0.0.1:18080/ws \
  -clients 100 \
  -duration 20s \
  -move-interval 200ms \
  -latency 50ms \
  -jitter 30ms \
  -incoming-drop-rate 0.02 \
  -outgoing-drop-rate 0.02 \
  -storm-fraction 0.50 \
  -storm-count 2 \
  -churn-fraction 0.01 \
  -reconnect-jitter 800ms
```

## 3. 300 客户端多房间压测

当前单房默认最大真人数为 100，因此 300 客户端按 100 人门槛匹配成多个房间：

```bash
MATCH_MIN_PLAYERS=100 MATCH_MAX_WAIT_SECONDS=8 ... go run ./cmd/server
```

然后：

```bash
go run ./cmd/wsload \
  -base-url http://127.0.0.1:18080 \
  -ws-url ws://127.0.0.1:18080/ws \
  -clients 300 \
  -duration 30s \
  -setup-concurrency 100 \
  -move-interval 250ms \
  -latency 80ms \
  -jitter 50ms \
  -incoming-drop-rate 0.03 \
  -outgoing-drop-rate 0.03 \
  -storm-fraction 0.50 \
  -storm-count 3 \
  -churn-fraction 0.01 \
  -reconnect-jitter 1200ms \
  -min-reconnect-success 0.95 \
  -min-snapshot-healthy 0.95
```

输出会显示实际创建了多少个房间。

## 4. 本地使用 `tc netem`

Linux 可直接对 loopback 注入真实网络故障：

```bash
sudo tc qdisc replace dev lo root netem \
  delay 80ms 30ms distribution normal \
  loss 2%
```

运行压测后务必恢复：

```bash
sudo tc qdisc del dev lo root
```

如果使用 macOS，可使用 Network Link Conditioner、Docker/Linux VM 或远端 Linux runner；`tc` 只适用于 Linux。

## 5. wsload 参数

| 参数 | 含义 | 默认值 |
| --- | --- | ---: |
| `-clients` | 真实 WebSocket 客户端数量 | 50 |
| `-duration` | Chaos 运行时长 | 12s |
| `-move-interval` | 每客户端 MOVE 周期 | 200ms |
| `-latency` | 应用层处理延迟 | 20ms |
| `-jitter` | 应用层延迟抖动 | 20ms |
| `-incoming-drop-rate` | 故意忽略 ROOM_SNAPSHOT 的比例 | 0.02 |
| `-outgoing-drop-rate` | 故意不发送 MOVE 的比例 | 0.02 |
| `-storm-fraction` | 每轮风暴同时断开的客户端比例 | 0.30 |
| `-storm-count` | 风暴次数 | 2 |
| `-churn-fraction` | 每秒随机断开的客户端比例 | 0.01 |
| `-reconnect-jitter` | 重连前随机等待上限 | 500ms |
| `-setup-concurrency` | 登录 / 匹配 / 建连最大并发 | 50 |
| `-match-timeout` | 匹配超时 | 12s |
| `-min-reconnect-success` | 重连成功率门槛 | 0.98 |
| `-min-snapshot-healthy` | 结束时快照链健康率门槛 | 0.98 |
| `-seed` | Chaos 随机种子 | 20260915 |

## 6. AOI / DELTA 弱网校验逻辑

每个 load client 都维护自己的 `lastSnapshotSeq`。

正常情况：

```text
AOI_FULL seq=100
  -> AOI_DELTA baseSeq=100 seq=101
  -> AOI_DELTA baseSeq=101 seq=102
```

如果客户端故意漏处理 `seq=101`：

```text
local lastSnapshotSeq=100
server AOI_DELTA baseSeq=101 seq=102
                |
                +-- mismatch
                    -> FULL_SYNC
                    -> AOI_FULL
                    -> rebuild baseline
```

因此压测不只是统计“WebSocket 还活着”，还会检查 AOI DELTA 恢复链是否工作。

## 7. 重连风暴

一次 storm 会选取配置比例的客户端，并真正执行：

```text
Close WebSocket
  -> random reconnect jitter
  -> new TCP/WebSocket connection
  -> RECONNECT
  -> RECONNECT_RESULT
  -> AOI_FULL_RECOVER
  -> restart client sequence space
  -> resume MOVE / AOI_DELTA
```

同一个客户端有 reconnect CAS 锁，随机 churn 和集中 storm 重叠时不会并发发起两次重连。

## 8. 结果指标

工具最终输出人类可读报告以及一行 JSON：

```text
WS_LOAD_CHAOS_SUMMARY {...}
```

核心指标包括：

- `initialConnections`
- `gameStarts`
- `snapshotClients`
- `fullSnapshots`
- `deltaSnapshots`
- `recoverSnapshots`
- `incomingSnapshotsDropped`
- `sequenceMismatches`
- `fullSyncRequests`
- `movesSent`
- `movesDropped`
- `forcedDisconnects`
- `reconnectAttempts`
- `reconnectSuccess`
- `reconnectSuccessRate`
- `snapshotHealthyRate`
- `unexpectedReadErrors`
- `writeErrors`
- `bytesReceived`
- reconnect latency P50 / P95 / P99
- snapshot receive lag P50 / P95 / P99

## 9. 推荐测试档位

### 日常 PR

```text
50 clients
10s
15ms +/- 20ms application delay
0% incoming snapshot drop
3% MOVE drop
30% reconnect storm x2
1% client churn / second
```

目标是稳定发现并发、死锁、WebSocket 生命周期、重连和 AOI baseline 回归，不使用随机最终时刻作为 flaky hard gate。

### 合并前 / 发版前

```text
100 clients
20-30s
50-100ms delay
20-50ms jitter
1-3% TCP packet loss
1% application snapshot drop
50% reconnect storm x2~3
```

### 压力探索

```text
300 clients
30-60s
100 clients / room
20-100ms application delay/jitter
1-3% application snapshot drop
50-80% reconnect storm
```

300-client 档位不建议作为每次 PR 的硬门禁，因为 GitHub hosted runner 的 CPU 和调度会产生较大波动，更适合作为手动基准测试。

## 10. 当前通过标准

`wsload` 默认硬门槛：

```text
GAME_START = 100%
reconnect success >= 98%
snapshot baseline healthy >= 98%
```

手动 `tc netem` 工作流在真实 packet loss 下默认放宽为：

```text
reconnect success >= 95%
snapshot baseline healthy >= 95%
```

延迟 P95 / P99 当前先作为观测指标，不作为固定失败门槛，因为单机 CI runner 性能会影响绝对延迟。积累基线后再增加 P95/P99 SLO gate。

## 11. 2026-09-15 资格测试结果

以下结果来自独立 GitHub Actions runner 上的真实 server 进程和真实 WebSocket 客户端，不是进程内模拟。

### 11.1 50 客户端日常 Chaos 基线

一次成功运行：

```text
clients                 50
rooms                   1
gameStarts              50/50
forcedDisconnects       40
reconnectSuccess        40/40 = 100%
incomingSnapshotsDrop   116
sequenceMismatches      245
FULL_SYNC requests      102
final snapshotHealthy   50/50 = 100%
unexpectedReadErrors    0
writeErrors             0
reconnect P95           2.155ms
snapshot lag P95        36ms
bytes received          18,041,756
```

这次资格运行仍开启了 2% snapshot drop，用来证明 FULL_SYNC 恢复链实际工作；最终的日常 PR 配置已改为不持续随机丢 snapshot，以避免随机采样门禁。

### 11.2 100 客户端真实 `tc netem` 弱网

网络条件：

```text
100 clients / 1 room
60ms delay
20ms jitter
1% TCP packet loss
1% application snapshot drop
1% MOVE drop
50% reconnect storm x2
```

结果：

```text
gameStarts              100/100
forcedDisconnects       112
reconnectSuccess        112/112 = 100%
final snapshotHealthy   99/100 = 99%
sequenceMismatches      311
FULL_SYNC requests      146
reconnect P50/P95/P99   371.430 / 697.385 / 1388.344 ms
snapshot lag P50/P95/P99 75 / 119 / 253 ms
unexpectedReadErrors    0
writeErrors             2
bytes received          79,965,501
```

这里的 packet loss 是 Linux `tc netem` 对 loopback 真正注入的 TCP 层丢包，不是简单跳过 WebSocket 消息处理。

### 11.3 300 客户端多房间压力

条件：

```text
300 clients
3 rooms x 100 clients
20ms application delay +/-30ms jitter
2% snapshot drop
2% MOVE drop
50% reconnect storm x2
1% random churn / second
```

结果：

```text
gameStarts              300/300
forcedDisconnects       335
reconnectSuccess        334/335 = 99.70%
reconnectFailures       1
final snapshotHealthy   287/300 = 95.67%
sequenceMismatches      1268
FULL_SYNC requests      857
reconnect P50/P95/P99   1.035 / 4.989 / 9.879 ms
snapshot lag P50/P95/P99 27 / 54 / 60 ms
unexpectedReadErrors    0
writeErrors             0
bytes received          219,154,520
```

300-client 档位通过预设的 95% 资格门槛。出现 1 次单次重连失败，因此该结果不能表述为“300 客户端零错误”；它说明在两轮各 150 客户端同时断线、再叠加随机 churn 的条件下，单次重连成功率为 99.70%。后续如果要把 300-client 变成发布 SLO，应继续做 5~10 分钟 soak test，并分别记录首尝试成功率与带重试后的最终恢复率。
