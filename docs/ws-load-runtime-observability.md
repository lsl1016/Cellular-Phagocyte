# WebSocket 压测服务端 Runtime 观测方案

## 1. 目标

现有 `server/cmd/wsload` 已能从客户端视角统计登录、匹配、WebSocket 连接、AOI FULL/DELTA、FULL_SYNC、断线重连、消息丢弃、快照滞后和重连延迟。

本方案补充服务端视角的运行时指标，并以固定 1 秒粒度写入 time-series CSV，使压测结果不仅能回答“协议是否还能工作”，还能进一步观察服务端资源是否出现持续增长、CPU 饱和、GC 抖动或连接数量异常。

压测观测分为两层：

1. **客户端协议指标**：由 `wsload` 原有 report 统计。
2. **服务端 Runtime 指标**：由 `/debug/runtime` 暴露，`wsload` 定时采样并写 CSV。

两层数据必须结合分析。

---

## 2. 服务端 Runtime Endpoint

### 2.1 开启方式

Runtime endpoint 默认关闭，避免在普通生产部署中无意暴露进程信息。

压测环境显式设置：

```bash
ENABLE_RUNTIME_METRICS=true
```

开启后：

```text
GET /debug/runtime
```

返回 JSON 快照。

### 2.2 采集内容

#### Process

| 字段 | 含义 |
| --- | --- |
| `cpuSeconds` | 当前进程累计真正获得 CPU 调度的时间，Linux 下读取 `/proc/self/schedstat` |
| `rssBytes` | Resident Set Size，Linux 下读取 `/proc/self/statm` |
| `openFDs` | `/proc/self/fd` 当前打开文件描述符数量 |
| `cpuAvailable` | 当前平台是否能读取 CPU 进程统计 |
| `rssAvailable` | 当前平台是否能读取 RSS |
| `fdAvailable` | 当前平台是否能读取 FD |

Linux/GitHub Actions runner 可获得上述三类进程指标。非 Linux 环境无法读取 `/proc` 时，对应 `Available` 字段为 `false`。

#### Go Runtime

| 字段 | 含义 |
| --- | --- |
| `gomaxprocs` | `runtime.GOMAXPROCS(0)` |
| `numCPU` | Go 看到的逻辑 CPU 数 |
| `goroutines` | 当前 Goroutine 数 |
| `heapAllocBytes` | 当前 Heap 已分配字节 |
| `heapInuseBytes` | Heap In-use 字节 |
| `heapObjects` | 当前 Heap Object 数量 |
| `stackInuseBytes` | Goroutine Stack 使用量 |
| `sysBytes` | Go Runtime 向 OS 获取的总内存 |
| `totalAllocBytes` | 进程生命周期累计分配字节 |
| `mallocs` / `frees` | 累计分配/释放对象次数 |
| `numGC` | GC 次数 |
| `gcPauseTotalMs` | 进程启动以来 GC STW pause 累计时间 |

#### Game Cardinality

| 字段 | 含义 |
| --- | --- |
| `rooms` | 当前 Manager 持有的房间总数 |
| `runningRooms` | 当前 RUNNING 房间数 |
| `players` | 房间内玩家总数，包含 Bot |
| `humanPlayers` | 真人玩家数 |
| `connectedHumans` | 当前仍绑定 WebSocket 的真人数 |
| `balls` | 当前球体总数 |
| `foods` | 当前食物总数 |
| `ejectedMass` | 当前吐出物总数 |

这些值用于判断测试实际负载是否符合预期。例如 300 客户端测试若只观测到 200 个 `connectedHumans`，即使 CPU 很低，也不能得出“300 并发资源充足”的结论。

---

## 3. wsload 每秒 Time-series CSV

### 3.1 参数

`wsload` 新增：

```text
-server-metrics-url <url>
-timeseries-csv <path>
-metrics-interval <duration>
```

示例：

```bash
/tmp/cellular-wsload \
  -base-url http://127.0.0.1:18083 \
  -ws-url ws://127.0.0.1:18083/ws \
  -clients 300 \
  -duration 300s \
  -server-metrics-url http://127.0.0.1:18083/debug/runtime \
  -metrics-interval 1s \
  -timeseries-csv /tmp/ws-soak-timeseries.csv
```

只有显式设置 `-timeseries-csv` 时才启动服务端采样，不影响原有短测试。

如果没有显式设置 `-server-metrics-url`，则默认使用：

```text
<base-url>/debug/runtime
```

### 3.2 CSV 字段

CSV 每个采样周期写一行，核心字段如下：

```text
timestamp
elapsed_seconds
sample_error
server_uptime_seconds
process_cpu_seconds
process_cpu_percent
rss_bytes
open_fds
gomaxprocs
num_cpu
goroutines
heap_alloc_bytes
heap_inuse_bytes
heap_objects
stack_inuse_bytes
sys_bytes
total_alloc_bytes
mallocs
frees
num_gc
gc_pause_total_ms
gc_pause_delta_ms
rooms
running_rooms
players
human_players
connected_humans
balls
foods
ejected_mass
```

HTTP 采样失败时仍写一行，并将错误放入 `sample_error`，这样观测空洞不会被静默忽略。

---

## 4. 派生指标计算方式

### 4.1 Process CPU Percent

`/debug/runtime` 提供的是累计 CPU 时间。`wsload` 对相邻两次采样计算：

```text
CPU% = (current.cpuSeconds - previous.cpuSeconds)
       / wallClockDeltaSeconds
       * 100
```

这里：

```text
100% = 持续占满 1 个 CPU Core
```

因此多核情况下 `process_cpu_percent` 可以大于 100%。例如 250% 约等于持续使用 2.5 个 Core。

不能把 100% 直接理解为“整台多核机器已经满载”。

### 4.2 GC Pause Delta

每秒计算：

```text
gc_pause_delta_ms =
  current.gcPauseTotalMs - previous.gcPauseTotalMs
```

它表示该采样窗口内累计发生了多少毫秒 GC pause。

重点关注：

- 是否随客户端数显著增长；
- 是否在 reconnect storm 时突然出现尖峰；
- 压测持续运行时是否呈逐步恶化趋势。

---

## 5. GitHub Actions 测试入口

### 5.1 `WebSocket Load Chaos`

文件：

```text
.github/workflows/ws-load-chaos.yml
```

用途：

- 50 / 100 / 300 客户端；
- Linux `tc netem` delay / jitter / loss；
- reconnect storm；
- random churn；
- AOI snapshot/MOVE application drop；
- protocol recovery 验证。

现在 artifact 固定包含：

```text
ws-load-chaos-report.txt
ws-load-chaos-timeseries.csv
cellular-load-server.log
```

### 5.2 `WebSocket Soak Capacity`

文件：

```text
.github/workflows/ws-soak-capacity.yml
```

用途：

- 100 / 300 / 500 客户端；
- 默认 300 秒持续负载；
- 多轮 reconnect storm；
- 可选真实 `tc netem`；
- 每秒 Runtime CSV；
- 初步观察资源曲线和长期稳定性。

该 workflow 与普通 PR CI 分开，避免 5 分钟压测拖慢所有提交。

---

## 6. 结果分析原则

一次压测至少同时检查以下四组指标。

### 6.1 协议正确性

- `gameStarts`
- `snapshotHealthyRate`
- `sequenceMismatches`
- `FULL_SYNC requests`
- `unexpectedReadErrors`
- `writeErrors`

### 6.2 重连恢复

- forced disconnects
- reconnect attempts
- reconnect success/failure
- reconnect P50/P95/P99

### 6.3 网络体验

- snapshot lag P50/P95/P99
- bytes received
- TCP netem 条件
- application snapshot/MOVE drop 条件

### 6.4 服务端资源

重点画时间曲线：

```text
process_cpu_percent
rss_bytes
heap_alloc_bytes
heap_inuse_bytes
goroutines
open_fds
gc_pause_delta_ms
connected_humans
rooms
balls
foods
ejected_mass
```

尤其关注以下异常模式：

1. 客户端数量稳定后，RSS/Heap/Goroutine 仍持续单调增长；
2. 每轮 reconnect storm 后资源不能回落；
3. FD 数持续增长，暗示连接/文件描述符泄漏；
4. CPU 已持续高位时 Snapshot Lag 同步恶化；
5. GC pause 与 Heap 增长形成周期性尖峰；
6. `connectedHumans` 明显小于测试客户端数，但客户端 report 未正确失败。

---

## 7. 当前边界

本阶段 Runtime Metrics 已覆盖进程、Go Runtime 和业务基数，但尚未直接统计：

- Room Tick P50/P95/P99；
- Snapshot build/serialization P50/P95/P99；
- WebSocket reliable queue / latest-only queue 深度；
- 每秒服务端实际出站字节数；
- FULL_SYNC 从请求到恢复完成的服务端耗时；
- 单 Room CPU 时间。

因此当前 CSV 已足以观察资源是否健康和是否存在持续增长，但如果要精确找到 Tick 饱和点，后续仍应增加 Tick/Snapshot latency histogram。

---

## 8. 安全约束

`/debug/runtime` 会暴露进程和业务运行状态，因此：

- 默认不开启；
- 仅压测/诊断环境设置 `ENABLE_RUNTIME_METRICS=true`；
- 不应直接暴露到公网生产入口；
- 如未来生产需要长期监控，应迁移到受认证/网络隔离的 Prometheus metrics endpoint。
