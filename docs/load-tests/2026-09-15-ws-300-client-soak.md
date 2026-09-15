# WebSocket 300 客户端 5 分钟 Capacity Soak 测试报告

> 日期：2026-09-15  
> 分支：`feature/ws-soak-capacity`  
> Commit：`f7f7845e2c88ef18b568f900cd540877b63a32dd`  
> Workflow：`WebSocket Soak Qualification` run #3 / `34983561506`  
> Artifact：`ws-soak-qualification-300-clients-300s` / `10403091431`  
> 结论：**通过**

## 1. 测试目标

验证单 Go Server 进程同时承载 300 个真实 WebSocket 客户端、3 个 100 人房间持续运行 300 秒时，多 Room Tick/AOI、持续 MOVE、随机丢帧、random churn 和集中重连风暴下的稳定性和资源曲线。

本场景为容量基线，关闭食物，避免玩法淘汰干扰 reconnect / snapshot health。

## 2. 测试场景

- clients：300；
- rooms：3；
- duration：300 s；
- MOVE interval：200 ms；
- incoming snapshot drop：0.5%；
- outgoing MOVE drop：0.5%；
- reconnect storm：5 次，每次 90 clients；
- random churn：配置 0.005/秒；当前向上取整算法下实际约每秒 2 clients；
- reconnect jitter：0～1000 ms；
- tc netem：未开启；
- Bot：0；
- initial food / max food：0 / 0；
- AOI：开启；FULL 校正周期 5 s；
- runner：Ubuntu 24.04，4 vCPU AMD EPYC 9V74，约 16.8 GB RAM。

## 3. 观测方式

客户端由 `server/cmd/wsload` 通过真实 HTTP / WebSocket 采集协议指标。服务端开启 `/debug/runtime`，每秒写入 Runtime CSV，统计 CPU、RSS、Heap、Goroutine、FD、GC pause、房间与玩家状态。

## 4. 测试结果

### 4.1 协议与重连

| 指标 | 结果 | 判定 |
| --- | ---: | --- |
| GAME_START | 300/300 | 通过 |
| Snapshot Healthy | 298/300 = 99.33% | 通过，门槛 98% |
| Forced Disconnects | 1,048 | 场景输入 |
| Reconnect | 1,048/1,048 = 100% | 通过 |
| Reconnect Failures | 0 | 通过 |
| Reconnect P50 / P95 / P99 | 0.939 / 16.964 / 26.520 ms | 通过 |
| Snapshot Lag P50 / P95 / P99 | 11 / 24 / 29 ms | 通过 |
| Incoming snapshots dropped | 4,496 | 故障注入 |
| Sequence mismatches | 5,685 | 触发恢复路径 |
| FULL_SYNC requests | 4,371 | 触发恢复路径 |
| Unexpected Read Errors | 0 | 通过 |
| Write Errors | 9 | 见异常说明 |
| Bytes Received | 4,326,951,240 | 观测值 |

`Snapshot Healthy=99.33%` 是测试结束瞬间的 baseline 状态。场景在整个测试期间持续启用 0.5% application snapshot drop，因此最后一次 FULL_SYNC/周期 FULL 也可能被压测客户端故意忽略。重连本身为 100% 成功，且 Runtime 最终连接恢复为 300。

### 4.2 服务端 Runtime

| 指标 | Mean | P95 | P99 | Max |
| --- | ---: | ---: | ---: | ---: |
| Process CPU % | 7.319 | 11.728 | 12.957 | 14.777 |
| RSS | 51.50 MiB | 53.44 MiB | 53.81 MiB | 53.83 MiB |
| Heap Alloc | 22.07 MiB | 28.66 MiB | 29.62 MiB | 30.47 MiB |
| Heap In-use | 25.84 MiB | 31.61 MiB | 32.63 MiB | 33.27 MiB |
| Goroutines | 598.47 | 606 | 607 | 611 |
| Open FDs | 305.81 | 309 | 310 | 311 |
| GC Pause Delta | 3.565 ms | 7.399 ms | 10.889 ms | 11.166 ms |

### 4.3 业务基数与时间趋势

稳定运行阶段：

- `aliveHumans`：300；
- `deadHumans` Max：0；
- `exitedHumans` Max：0；
- `balls`：300；
- `foods`：0；
- 最终 `connectedHumans`：300；
- rooms / running rooms：3 / 3。

前后稳态窗口对比：

- RSS：约 51.37 -> 51.79 MiB，+0.42 MiB；
- Heap Alloc：约 22.31 -> 21.98 MiB，-0.33 MiB；
- Heap In-use：约 25.74 -> 26.09 MiB，+0.35 MiB；
- Goroutine：约 600.3 -> 600.0，基本不变。

没有观察到随时间单调上涨的 Heap/Goroutine/FD 泄漏特征。

## 5. 异常与说明

- 9 次客户端 write error 被如实保留。重连 1,048/1,048 成功、unexpected read error 为 0，最终连接为 300，现象更符合主动重连期间 MOVE 写旧 socket 的竞争窗口，而非服务器持续失联。
- 最终有 2 个客户端处于 `needsFull` 状态，导致 snapshot health 为 99.33%。持续应用层丢快照会使测试结束瞬时值带随机性，因此该指标按 98% qualification 门槛判定，而不写成 100% 收敛。

## 6. 结论

300 客户端 / 3 房间 / 5 分钟容量基线 **通过**。

在当前 4 vCPU runner 上，CPU P99 约 12.96%，RSS Max 约 53.83 MiB；所有 1,048 次重连均成功，所有玩家保持 alive，最终 300 个 WebSocket 连接全部恢复。当前规模未出现明显资源饱和点。

本结果不能代表生产硬件的容量上限，也不能替代带真实 `tc netem` 的弱网测试。
