# WebSocket 100/300/500 客户端 5 分钟 Soak Qualification V1 测试报告

> 日期：2026-09-15  
> 分支：`feature/ws-soak-capacity`  
> Workflow：`WebSocket Soak Qualification` run #1 / `34981873913`  
> Qualification Commit：`65851dcdd155f8ec4a98c37aa96f329da852c87a`  
> 测试判定：**失败；发现测试场景将玩法淘汰混入重连容量指标，不能作为纯容量上限结论**

## 1. 测试目标

本轮目标是验证单个 Go Server 进程在 100 / 300 / 500 个真实 WebSocket 客户端持续运行 300 秒时的协议稳定性、重连恢复和服务端资源趋势。

测试重点观测：

- GAME_START 与快照链是否保持有效；
- 多轮集中重连风暴与持续随机 churn 下的重连成功率；
- AOI DELTA 丢帧后 FULL_SYNC 的修复情况；
- CPU、RSS、Heap、Goroutine、FD、GC pause；
- 多房间并行 Tick / AOI 的资源增长趋势。

本轮最终未用于给出生产容量上限，因为测试过程中出现了明显的玩法淘汰干扰，详见第 6 节。

## 2. 测试环境

三个规模分别运行在独立 GitHub Actions runner 上，互不共享服务进程资源。

共同环境：

- OS：Ubuntu 24.04；
- Go：1.24.10；
- CPU：4 vCPU，AMD EPYC 9V74；
- Memory：约 16.8 GB；
- Storage：memory；
- TickRate：20 Hz；
- SnapshotRate：10 Hz；
- AOI：开启；
- AOI FULL 校正周期：5 秒；
- Bot：0；
- PlayerInitialMass：20；
- 测试时长：300 秒。

## 3. 测试场景

### 3.1 并发规模

| 客户端 | 房间数 | 单房间上限 |
| ---: | ---: | ---: |
| 100 | 1 | 100 |
| 300 | 3 | 100 |
| 500 | 5 | 100 |

### 3.2 Chaos 参数

- MOVE interval：200 ms；
- incoming snapshot drop：0.5%；
- outgoing MOVE drop：0.5%；
- reconnect storm：5 次；
- 每次 storm：随机断开 30% 客户端；
- random churn：`0.005` / 秒；
- reconnect jitter：0～1000 ms；
- 内核 netem：未开启，本轮重点为容量与长时间稳定性。

需要注意：当前 `selectClients` 对 `N × fraction` 使用 `ceil`。因此 `churn-fraction=0.005` 实际表现为：

- 100 clients：每秒 1 个客户端；
- 300 clients：每秒 2 个客户端；
- 500 clients：每秒 3 个客户端。

因此 300 秒内 churn 强度高于把 0.5% 理解为可取小数的直觉值。

## 4. 观测方式

### 4.1 客户端协议指标

`server/cmd/wsload` 使用真实 HTTP + WebSocket 链路统计：

- GAME_START；
- AOI FULL / DELTA / RECOVER；
- sequence mismatch；
- FULL_SYNC；
- forced disconnect；
- reconnect attempt / success / failure；
- reconnect P50/P95/P99；
- snapshot receive lag P50/P95/P99；
- read/write errors；
- bytes received。

### 4.2 服务端 Runtime 指标

开启：

```text
ENABLE_RUNTIME_METRICS=true
GET /debug/runtime
```

`wsload` 每秒采样到 CSV，统计 CPU、RSS、Heap、Goroutine、FD、GC pause 和游戏对象数量。

原始 Artifact：

- 100 clients：`10401567996`
- 300 clients：`10402127566`
- 500 clients：`10402192097`

每个 Artifact 均包含 `ws-soak-report.txt`、`ws-soak-timeseries.csv`、runtime summary、runner environment 和 server log。

## 5. 测试结果

### 5.1 协议与重连

| 指标 | 100 clients | 300 clients | 500 clients |
| --- | ---: | ---: | ---: |
| GAME_START | 100/100 | 300/300 | 500/500 |
| Snapshot Healthy | 65.00% | 80.33% | 70.60% |
| Forced Disconnects | 450 | 1049 | 1646 |
| Reconnect | 338/450 | 888/1049 | 1282/1646 |
| Reconnect Success | 75.11% | 84.65% | 77.89% |
| Reconnect Failures | 112 | 161 | 364 |
| Reconnect P95 | 4.456 ms | 26.972 ms | 35.474 ms |
| Reconnect P99 | 8.131 ms | 35.861 ms | 65.193 ms |
| Snapshot Lag P95 | 10 ms | 40 ms | 50 ms |
| Snapshot Lag P99 | 12 ms | 49 ms | 73 ms |
| Unexpected Read Errors | 0 | 0 | 0 |
| Write Errors | 3 | 0 | 8 |

三档均未达到本轮 98% reconnect / snapshot health qualification 门槛。

### 5.2 AOI 恢复行为

| 指标 | 100 | 300 | 500 |
| --- | ---: | ---: | ---: |
| Incoming snapshots dropped | 1,248 | 4,136 | 6,456 |
| Sequence mismatches | 1,539 | 5,049 | 8,125 |
| FULL_SYNC requests | 1,212 | 4,018 | 6,251 |

说明长期丢帧场景持续触发了 DELTA baseline 修复路径。

### 5.3 服务端资源

| 指标 | 100 | 300 | 500 |
| --- | ---: | ---: | ---: |
| CPU Mean | 1.768% | 8.637% | 9.430% |
| CPU P95 | 3.418% | 14.358% | 21.106% |
| CPU P99 | 4.220% | 15.749% | 23.792% |
| CPU Max | 4.532% | 18.267% | 24.788% |
| RSS Max | 30.62 MB | 62.56 MB | 92.72 MB |
| Goroutine Max | 207 | 611 | 1015 |
| Heap Alloc Max | 11.44 MB | 36.06 MB | 60.60 MB |
| Heap In-use Max | 13.44 MB | 39.76 MB | 64.76 MB |
| GC Pause Delta P99 | 3.269 ms | 9.976 ms | 12.793 ms |
| GC Pause Delta Max | 3.890 ms | 12.143 ms | 20.337 ms |
| Open FD Max | 111 | 311 | 511 |

CPU 的定义为 `100% = 1 个 CPU core`。三档测试均运行在 4 vCPU runner 上。

从资源数据看，本轮失败时 CPU、RSS、Heap 和 FD 均没有接近机器资源上限，因此不能把失败归因于 CPU 或内存饱和。

## 6. 关键异常与根因分析

### 6.1 玩家球体数量持续下降

Runtime CSV 中 `balls` 随时间明显下降：

| 时间 | 100 clients | 300 clients | 500 clients |
| ---: | ---: | ---: | ---: |
| ~10s | 100 | 300 | 499 |
| ~50s | 93 | 286 | 466 |
| ~100s | 84 | 268 | 415 |
| ~150s | 72 | 257 | 384 |
| ~200s | 66 | 246 | 362 |
| ~250s | 60 | 237 | 339 |
| ~300s | 54 | 230 | 315 |

当前默认每个房间生成 400 个食物、最多补充到 500。玩家持续吃食物后质量出现差异，随后会发生玩家之间的真实吞噬淘汰。

### 6.2 玩法死亡被统计成重连失败

服务端 `Room.Reconnect` 不允许已死亡或已退出玩家恢复对局。

但 `wsload` 的 storm/churn 仍然从所有客户端中随机选择目标。因此当一个已经在玩法上死亡的玩家再次被 Chaos 选中并断开时，它的后续 RECONNECT 会被服务端正常拒绝，而压测程序会把该结果计为 `reconnectFailures`。

这会进一步造成该客户端最终不再有有效快照 baseline，从而降低 `snapshotHealthyRate`。

因此本轮 reconnect/snapshot health 指标混合了两种含义：

1. 真正的连接/重连失败；
2. 按游戏规则已经死亡、理论上本来就不应该重连的玩家。

这种混合使它不能作为纯容量 Soak 的通过/失败依据。

### 6.3 Reconnect Token 过期不是本轮原因

Reconnect token 默认 TTL 为 600000 ms（10 分钟），本轮只运行约 5 分钟，因此 token 生命周期不足不是主要根因。

## 7. 结论

本轮判定：**失败，但失败主要暴露的是测试场景设计问题，而不是服务端已经达到 100/300/500 客户端资源容量上限。**

可以得出的结论：

1. 100 / 300 / 500 客户端均能完成真实登录、匹配、WebSocket 入房和开局；
2. 五分钟窗口内 Runtime CSV 稳定产生，观测链路有效；
3. 资源随客户端规模增长，但当前 4 vCPU runner 上 CPU 和内存仍有较大余量；
4. 长时间测试首次暴露了“玩法死亡污染重连容量指标”的问题；
5. 本轮不能作为单机生产容量上限结论。

不能得出的结论：

- 100 客户端只能达到 75% 重连成功率；
- 500 客户端已经超过 CPU/内存容量；
- 当前单机最大只能承载 500 客户端。

这些判断都需要在消除玩法淘汰干扰后重新测试。

## 8. 修正方案与复测计划

下一版容量基线采用：

```text
GAME_INITIAL_FOOD_COUNT=0
GAME_MAX_FOOD_COUNT=0
```

所有玩家保持相同初始质量，无法互相吞噬，从而使 reconnect / snapshot health 只反映连接、协议和服务端运行时问题。

同时 Runtime 指标新增：

- `aliveHumans`
- `deadHumans`
- `exitedHumans`

复测仍执行：

```text
100 / 300 / 500 clients
300 seconds
5 reconnect storms
0.5% incoming snapshot drop
0.5% outgoing MOVE drop
0.5%/second configured churn
```

玩法战斗压力测试后续单独建立场景，保留食物和玩家淘汰，但采用与容量基线不同的成功判定标准。
