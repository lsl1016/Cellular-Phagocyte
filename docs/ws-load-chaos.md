# WebSocket 压测与弱网 / Chaos 测试

本项目提供 `server/cmd/wsload`，用于对真实运行中的 Go Server 发起几十到数百个真实 WebSocket 客户端，并持续执行登录、匹配、入房、READY、移动、AOI 快照、FULL_SYNC、断线与重连。

该工具不是直接调用 Room 方法，也不是 `httptest`。所有客户端均通过真实 HTTP API 与真实 WebSocket 网关工作。

## 1. 测试层次

### 1.1 CI：50 客户端确定性 Chaos smoke

`.github/workflows/ci.yml` 中的 `WebSocket 50-client chaos smoke` 每次 PR 都执行：

- 50 个真人客户端；
- 真实 HTTP 游客登录；
- 真实匹配；
- 真实 WebSocket `ENTER_ROOM`；
- 全部进入后再统一 `READY`；
- 持续发送 `MOVE`；
- 应用层延迟与抖动；
- 随机丢弃部分客户端发送的 MOVE；
- 随机忽略部分客户端收到的 ROOM_SNAPSHOT；
- AOI DELTA base 不连续时自动发 `FULL_SYNC`；
- 2 轮集中断线 / 重连风暴；
- 每秒少量随机连接 churn；
- 最终检查重连成功率以及快照链健康率。

CI 中的应用层丢弃是为了稳定、可重复地验证恢复逻辑，不等价于 TCP 数据包丢失。

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

当前单房默认最大真人数为 100，因此 300 客户端应按 100 人门槛匹配成多个房间：

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
2% incoming snapshot drop
3% MOVE drop
30% reconnect storm x2
1% client churn / second
```

目标是快速发现协议恢复、并发、死锁、重连和 AOI baseline 回归。

### 合并前 / 发版前

```text
100 clients
20-30s
50-100ms delay
20-50ms jitter
1-3% TCP packet loss
50% reconnect storm x2~3
```

### 压力探索

```text
300 clients
30-60s
100 clients / room
100ms+ delay
3-5% TCP packet loss
50-80% reconnect storm
```

这个档位不建议作为每次 PR 的硬门禁，因为 GitHub hosted runner 的 CPU 和网络调度会产生较大波动，更适合作为手动基准测试。

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

延迟 P95 / P99 当前先作为观测指标，不作为固定失败门槛，因为单机 CI runner 性能会影响绝对延迟。后续积累基线后，可以再增加 P95/P99 SLO gate。
