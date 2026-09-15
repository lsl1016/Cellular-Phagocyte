# WebSocket Runtime Observability Smoke 测试报告

> 日期：2026-09-15  
> Branch：`feature/ws-soak-capacity`  
> PR：#16  
> Commit：`f4914723863ecf2d86494da79d4b096d7b02e989`  
> GitHub Actions CI Run：`34980405539`  
> Artifact：`ws-chaos-smoke-runtime`（Artifact ID `10400817871`）  
> 执行环境：GitHub Actions `ubuntu-24.04` / Go 1.24.10

## 1. 测试目标

本次测试的主要目标不是重新评估 50 客户端容量，而是验证新增的服务端 Runtime 观测链路是否在真实 WebSocket 压测中可用：

```text
Game Server
    -> GET /debug/runtime
    -> wsload 每秒采样
    -> time-series CSV
    -> GitHub Actions artifact
```

同时继续使用原有 50-client Chaos Smoke，确认新增观测代码没有破坏登录、匹配、AOI Snapshot、MOVE、断线重连等既有路径。

本次测试不用于推导单机容量上限，也不用于判断 5 分钟以上的内存/Goroutine 泄漏。

---

## 2. 测试场景

### 2.1 客户端与游戏配置

```text
clients                 = 50
duration                = 10s
rooms                   = 1
move interval           = 200ms
application latency     = 15ms
application jitter      = +/-20ms
incoming snapshot drop  = 0%
outgoing MOVE drop      = 3%
reconnect storm         = 30% clients x 2 rounds
random churn            = 1% clients / second
reconnect jitter        = <=300ms
```

服务端：

```text
MATCH_MIN_PLAYERS               = 50
GAME_BOTS                       = 0
GAME_INIT_MASS                  = 20
GAME_AOI_ENABLED                = true
GAME_AOI_FULL_INTERVAL_SECONDS  = 5
ENABLE_RUNTIME_METRICS          = true
```

Runtime 采样：

```text
GET http://127.0.0.1:18081/debug/runtime
interval = 1s
CSV      = /tmp/ws-ci-timeseries.csv
```

---

## 3. 观测方式

### 3.1 客户端协议观测

`server/cmd/wsload` 继续统计：

- `GAME_START`；
- AOI FULL / DELTA / RECOVER；
- DELTA sequence mismatch；
- FULL_SYNC；
- MOVE sent / dropped；
- forced disconnect；
- reconnect attempts / success / failure；
- reconnect P50/P95/P99；
- snapshot receive lag P50/P95/P99；
- WebSocket unexpected read / write errors；
- received bytes。

### 3.2 服务端 Runtime 观测

`/debug/runtime` 每秒返回：

- Process CPU accumulated seconds；
- RSS；
- open file descriptors；
- GOMAXPROCS / NumCPU；
- Goroutines；
- Heap Alloc / Heap In-use / Heap Objects；
- Stack In-use；
- Go Sys；
- Total Alloc / Mallocs / Frees；
- GC count / cumulative pause；
- rooms / running rooms；
- players / human players / connected humans；
- balls / foods / ejected mass。

`wsload` 根据相邻样本进一步计算：

```text
process_cpu_percent
gc_pause_delta_ms
```

其中：

```text
process_cpu_percent = 100%
```

表示约占满 1 个 CPU Core，而不是整台多核机器 100%。

### 3.3 原始资产

CI Artifact 包含：

```text
ws-ci-timeseries.csv
runtime-initial.json
cellular-wsload-server.log
```

客户端汇总结果保存在 GitHub Actions job log 中。

---

## 4. 测试结果

### 4.1 协议与重连

| 指标 | 结果 | 判定 |
| --- | ---: | --- |
| Game Start | 50 / 50 | 通过 |
| Final Snapshot Healthy | 50 / 50 = 100% | 通过 |
| Forced Disconnects | 40 | - |
| Reconnect | 40 / 40 = 100% | 通过 |
| Reconnect Failures | 0 | 通过 |
| MOVE Sent | 2363 | - |
| MOVE Dropped | 81 | 符合 Chaos 配置 |
| FULL Snapshots | 131 | - |
| DELTA Snapshots | 5905 | - |
| RECOVER Snapshots | 40 | - |
| Sequence Mismatches | 0 | 正常 |
| FULL_SYNC Requests | 0 | 正常 |
| Unexpected Read Errors | 0 | 通过 |
| Write Errors | 0 | 通过 |
| Bytes Received | 18,558,884 | - |

延迟：

```text
Reconnect P50 / P95 / P99
0.909 / 1.380 / 2.929 ms

Snapshot Lag P50 / P95 / P99
18 / 35 / 37 ms
```

最终：

```text
WS_LOAD_CHAOS PASS
```

### 4.2 Runtime CSV 完整性

CSV 成功生成并被 CI 强制检查：

```text
header + 13 samples
sample interval ~= 1 second
sample_error 全部为空
```

第一条采样发生在客户端尚未进入房间时：

```text
rooms             = 0
players           = 0
connectedHumans   = 0
```

进入运行态后稳定观察到：

```text
rooms             = 1
runningRooms      = 1
players           = 50
humanPlayers      = 50
connectedHumans   = 50
balls             = 50
```

说明 Runtime endpoint 观测到的业务基数与压测实际客户端规模一致。

### 4.3 服务端资源结果

以下统计仅使用 `players=50` 的运行态样本，排除启动前的首个空载样本。

| 指标 | 运行态观测结果 |
| --- | ---: |
| Process CPU | 平均约 1.45%，Peak 2.323% |
| RSS | 约 20.3 - 21.7 MiB，Peak 22,724,608 bytes |
| Open FDs | 60 - 61 |
| Goroutines | 106 - 107 |
| Heap Alloc | 约 3.37 - 5.41 MiB，Peak 5,669,328 bytes |
| Heap In-use | 约 5.54 - 7.17 MiB，Peak 7,520,256 bytes |
| Go Sys | 约 15.7 - 16.0 MiB |
| GC Pause Delta | 单个 1s 窗口 0.124 - 1.078 ms |
| GC Pause Total | 测试结束约 5.659 ms |

CPU 的 1.45% 是“单核百分比”定义。该 GitHub runner 的 Go Runtime 观测到：

```text
GOMAXPROCS = 4
NumCPU     = 4
```

因此本次 50-client smoke 的 CPU 压力很低。

### 4.4 时间趋势

10 秒运行窗口内：

- `connectedHumans` 保持 50；
- Goroutine 基本稳定在 106-107；
- FD 基本稳定在 60-61；
- Heap Alloc 在 GC 周期内上下波动，没有在这 10 秒内表现为持续单调增长；
- RSS 从进入负载后的约 20 MiB 上升到约 21-22 MiB 后基本稳定；
- GC pause delta 未出现大于约 1.1 ms 的窗口；
- 两轮 reconnect storm 未造成明显资源数量失控。

但 10 秒窗口过短，不能据此排除慢速内存泄漏、Goroutine 泄漏或 FD 泄漏。

---

## 5. 异常与问题

本次测试未出现协议错误或 Runtime 采样错误。

需要保留的限制：

1. 本次只有 50 clients / 10 seconds；
2. 未注入 Linux `tc netem`；
3. Runtime CSV 当前没有 Room Tick/Snapshot build latency histogram；
4. Process CPU 使用 GitHub Linux runner 的 `/proc/self/schedstat`；
5. 仅一次短测试不能构成容量结论。

---

## 6. 结论

**结论：通过。**

本次验证可以确认：

1. `/debug/runtime` 在显式开启后可被真实 Server 进程访问；
2. wsload 能以 1 秒粒度持续采样服务端 Runtime；
3. CSV 能正确记录 CPU、RSS、FD、Goroutine、Heap、GC 与游戏业务基数；
4. `process_cpu_percent` 与 `gc_pause_delta_ms` 可以由相邻采样稳定计算；
5. CSV、初始 JSON 和 Server Log 能作为 GitHub Actions artifact 留档；
6. 新增观测代码没有破坏现有 50-client Chaos Smoke。

本次测试**不能**得出“单机容量是 50/300/500”之类的容量上限结论。

---

## 7. 后续动作

下一阶段使用新建的 `WebSocket Soak Capacity` workflow 执行：

```text
100 clients / 5 min
300 clients / 5 min
500 clients / 5 min
```

重点分析：

```text
CPU curve
RSS / Heap slope
Goroutine slope
FD slope
GC pause delta
Reconnect storm 前后资源回落情况
Snapshot lag 与 CPU/GC 的相关性
```

每次正式 Soak/容量测试均按 `docs/load-test-report-standard.md` 生成独立 Markdown 报告。
