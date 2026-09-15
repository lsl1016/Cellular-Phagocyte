# WebSocket 100 客户端 5 分钟 Capacity Soak 测试报告

> 日期：2026-09-15  
> 分支：`feature/ws-soak-capacity`  
> Commit：`f7f7845e2c88ef18b568f900cd540877b63a32dd`  
> Workflow：`WebSocket Soak Qualification` run #3 / `34983561506`  
> Artifact：`ws-soak-qualification-100-clients-300s` / `10402646706`  
> 结论：**通过**

## 1. 测试目标

验证单 Go Server 进程在 100 个真实 WebSocket 客户端持续运行 300 秒、持续 MOVE、随机丢帧、随机 churn 和 5 轮集中重连风暴下的协议恢复能力及服务端资源稳定性。

本场景是容量基线测试，不测试真实战斗淘汰。为避免玩家死亡污染 reconnect 指标，食物生成被显式关闭。

## 2. 测试场景

- clients：100；
- rooms：1；
- duration：300 s；
- MOVE interval：200 ms；
- incoming snapshot drop：0.5%；
- outgoing MOVE drop：0.5%；
- reconnect storm：5 次，每次 30 clients；
- random churn：配置值 0.005/秒；由于向上取整，100 clients 实际每秒选择 1 client；
- reconnect jitter：0～1000 ms；
- tc netem：未开启；
- Bot：0；
- initial food / max food：0 / 0；
- AOI：开启；FULL 校正周期 5 s；
- runner：Ubuntu 24.04，4 vCPU AMD EPYC 9V74，约 16.8 GB RAM。

## 3. 观测方式

客户端由 `server/cmd/wsload` 通过真实 HTTP + WebSocket 统计 GAME_START、FULL/DELTA/RECOVER、FULL_SYNC、重连、snapshot lag 和 socket error。

服务端开启 `ENABLE_RUNTIME_METRICS=true`，`wsload` 每秒请求 `/debug/runtime`，写入 `ws-soak-timeseries.csv`，采集 CPU、RSS、Heap、Goroutine、FD、GC pause、房间/玩家/连接和 alive/dead/exited 状态。

原始 Artifact 同时保留 wsload report、CSV、runtime summary、runner environment 和 server log。

## 4. 测试结果

### 4.1 协议与重连

| 指标 | 结果 | 判定 |
| --- | ---: | --- |
| GAME_START | 100/100 | 通过 |
| Snapshot Healthy | 100/100 = 100% | 通过 |
| Forced Disconnects | 450 | 场景输入 |
| Reconnect | 450/450 = 100% | 通过 |
| Reconnect Failures | 0 | 通过 |
| Reconnect P50 / P95 / P99 | 0.999 / 7.174 / 13.366 ms | 通过 |
| Snapshot Lag P50 / P95 / P99 | 10 / 16 / 18 ms | 通过 |
| Incoming snapshots dropped | 1,482 | 故障注入 |
| Sequence mismatches | 1,717 | 触发恢复路径 |
| FULL_SYNC requests | 1,440 | 触发恢复路径 |
| Unexpected Read Errors | 0 | 通过 |
| Write Errors | 2 | 见异常说明 |
| Bytes Received | 1,484,751,202 | 观测值 |

### 4.2 服务端 Runtime

| 指标 | Mean | P95 | P99 | Max |
| --- | ---: | ---: | ---: | ---: |
| Process CPU % | 1.719 | 4.728 | 5.110 | 5.890 |
| RSS | 25.41 MiB | 26.20 MiB | 26.73 MiB | 26.77 MiB |
| Heap Alloc | 7.51 MiB | 9.71 MiB | 10.06 MiB | 10.16 MiB |
| Heap In-use | 9.50 MiB | 11.45 MiB | 11.70 MiB | 11.91 MiB |
| Goroutines | 203.18 | 206 | 207 | 207 |
| Open FDs | 109.17 | 111 | 111 | 111 |
| GC Pause Delta | 2.132 ms | 4.425 ms | 5.076 ms | 6.987 ms |

CPU 定义：100% 表示持续占满一个 CPU Core。

### 4.3 业务基数与时间趋势

- steady-state `aliveHumans`：100；
- `deadHumans` Max：0；
- `exitedHumans` Max：0；
- `balls`：稳定 100；
- `foods`：稳定 0；
- 最终 `connectedHumans`：100。

排除启动阶段后，前 60 秒与后 60 秒对比：

- RSS：约 25.24 -> 25.50 MiB，+0.26 MiB；
- Heap Alloc：约 7.54 -> 7.53 MiB，基本不变；
- Heap In-use：约 9.45 -> 9.58 MiB，+0.13 MiB；
- Goroutine：约 203.6 -> 202.7，无持续增长。

未观察到明显 Heap/Goroutine/FD 泄漏模式。

## 5. 异常与说明

出现 2 次客户端 `writeErrors`。本轮 reconnect success 为 100%、unexpected read error 为 0，且连接最终全部恢复。结合 wsload 的并发模型，这类写错误可能来自 MOVE 写操作与主动关闭旧 socket 的竞争窗口。该指标仍按原始结果保留，不将“测试通过”等同于“零错误”。

## 6. 结论

100 客户端、1 房间、5 分钟容量基线 **通过**：

- 所有客户端开局成功；
- 450 次强制断线全部重连成功；
- 最终快照 baseline 全部健康；
- 无玩家死亡/退出污染；
- CPU、RSS、Heap、Goroutine、FD 和 GC 在本时长内无明显失控或泄漏趋势。

本报告不能推出单机容量上限为 100。该规模距离当前 runner 的资源饱和仍很远。
