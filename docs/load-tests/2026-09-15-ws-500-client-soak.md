# WebSocket 500 客户端 5 分钟 Capacity Soak 测试报告

> 日期：2026-09-15  
> 分支：`feature/ws-soak-capacity`  
> Commit：`f7f7845e2c88ef18b568f900cd540877b63a32dd`  
> Workflow：`WebSocket Soak Qualification` run #3 / `34983561506`  
> Artifact：`ws-soak-qualification-500-clients-300s` / `10402229706`  
> 结论：**通过**

## 1. 测试目标

验证单 Go Server 进程同时承载 500 个真实 WebSocket 客户端、5 个 100 人房间持续运行 300 秒时，多 Room Tick/AOI、持续 MOVE、随机丢帧、random churn 和集中重连风暴下的稳定性与资源趋势。

容量基线关闭食物和玩法淘汰，使 reconnect / snapshot health 只反映连接、协议恢复和服务端运行时状态。

## 2. 测试场景

- clients：500；
- rooms：5；
- duration：300 s；
- MOVE interval：200 ms；
- incoming snapshot drop：0.5%；
- outgoing MOVE drop：0.5%；
- reconnect storm：5 次，每次 150 clients；
- random churn：配置 0.005/秒；当前向上取整算法下实际约每秒 3 clients；
- reconnect jitter：0～1000 ms；
- tc netem：未开启；
- Bot：0；
- initial food / max food：0 / 0；
- AOI：开启；FULL 校正周期 5 s；
- runner：Ubuntu 24.04，4 vCPU AMD EPYC 9V74，约 16.8 GB RAM。

## 3. 观测方式

客户端由 `server/cmd/wsload` 通过真实 HTTP / WebSocket 统计协议与重连指标。服务端开启 `/debug/runtime`，以 1 秒粒度写 `ws-soak-timeseries.csv`，记录 CPU、RSS、Heap、Goroutine、FD、GC pause 以及 alive/dead/exited 等业务基数。

## 4. 测试结果

### 4.1 协议与重连

| 指标 | 结果 | 判定 |
| --- | ---: | --- |
| GAME_START | 500/500 | 通过 |
| Snapshot Healthy | 498/500 = 99.60% | 通过，门槛 98% |
| Forced Disconnects | 1,646 | 场景输入 |
| Reconnect | 1,646/1,646 = 100% | 通过 |
| Reconnect Failures | 0 | 通过 |
| Reconnect P50 / P95 / P99 | 0.787 / 17.294 / 23.766 ms | 通过 |
| Snapshot Lag P50 / P95 / P99 | 8 / 24 / 29 ms | 通过 |
| Incoming snapshots dropped | 7,511 | 故障注入 |
| Sequence mismatches | 9,572 | 触发恢复路径 |
| FULL_SYNC requests | 7,284 | 触发恢复路径 |
| Unexpected Read Errors | 0 | 通过 |
| Write Errors | 1 | 见异常说明 |
| Bytes Received | 7,103,177,054 | 观测值 |

### 4.2 服务端 Runtime

| 指标 | Mean | P95 | P99 | Max |
| --- | ---: | ---: | ---: | ---: |
| Process CPU % | 8.535 | 14.819 | 15.987 | 17.227 |
| RSS | 75.23 MiB | 78.02 MiB | 78.38 MiB | 78.38 MiB |
| Heap Alloc | 39.34 MiB | 50.51 MiB | 51.97 MiB | 53.30 MiB |
| Heap In-use | 44.30 MiB | 54.14 MiB | 55.62 MiB | 56.23 MiB |
| Goroutines | 1002.61 | 1010 | 1015 | 1015 |
| Open FDs | 506.90 | 511 | 511 | 511 |
| GC Pause Delta | 3.805 ms | 8.522 ms | 10.112 ms | 12.660 ms |

CPU 100% 表示一个 CPU Core 满载。当前 4 vCPU runner 上 500 客户端的 CPU P99 仍约为 15.99%。

### 4.3 业务基数与时间趋势

稳定运行阶段：

- `aliveHumans`：500；
- `deadHumans` Max：0；
- `exitedHumans` Max：0；
- `balls`：500；
- `foods`：0；
- 最终 `connectedHumans`：500；
- rooms / running rooms：5 / 5。

前后稳态窗口对比：

- RSS：约 75.24 -> 75.98 MiB，+0.74 MiB；
- Heap Alloc：约 40.46 -> 40.21 MiB，-0.26 MiB；
- Heap In-use：约 44.57 -> 45.78 MiB，+1.21 MiB；
- Goroutine：约 1009.4 -> 1008.6，基本不变。

Heap Alloc 没有随时间持续抬升，Goroutine/FD 也未出现累积泄漏特征。RSS 与 Heap In-use 有小幅增长，但在 5 分钟窗口内没有呈现失控式上涨。

## 5. 异常与说明

- 客户端记录 1 次 write error，但 1,646 次重连全部成功、unexpected read error 为 0，最终 500 个连接全部恢复。
- 最终 `Snapshot Healthy=498/500`。测试持续主动丢弃 0.5% incoming snapshot，结束瞬间仍可能恰好有少量客户端等待 FULL 修复，所以按 qualification 门槛而不是要求瞬时 100%。
- 本测试没有启用 `tc netem`，因此本报告是容量/长稳结论，不是公网弱网结论。

## 6. 结论

500 客户端 / 5 房间 / 5 分钟容量基线 **通过**。

在该环境下：

- 500/500 开局成功；
- 1,646/1,646 重连成功；
- 最终 500 个连接全部恢复；
- 无玩家死亡/退出；
- CPU P99 约 15.99%，RSS Max 约 78.38 MiB；
- 未观察到明显 Heap、Goroutine 或 FD 泄漏。

**500 客户端仍未出现明确容量拐点，因此不能把 500 视为单机上限。** 下一阶段容量曲线应继续扩大至 750 / 1000+，并补充 Room Tick / Snapshot build latency histogram 后再寻找真正的 SLO 饱和点。
