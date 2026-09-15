# WebSocket 100/300/500 客户端 5 分钟容量曲线总结

> 日期：2026-09-15  
> 分支：`feature/ws-soak-capacity`  
> Workflow：`WebSocket Soak Qualification` run #3 / `34983561506`  
> Commit：`f7f7845e2c88ef18b568f900cd540877b63a32dd`  
> 总体结论：**100 / 300 / 500 客户端容量基线均通过；500 客户端尚未出现明确资源拐点**

## 1. 背景

第一轮 5 分钟 qualification 保留默认食物和战斗逻辑，导致玩家在长时间运行中发生真实吞噬淘汰。Chaos 随后继续断开已死亡玩家，使服务端按规则拒绝其重连，最终把业务死亡混入 reconnect failure 和 snapshot health。

第一轮失败报告：

```text
docs/load-tests/2026-09-15-ws-soak-qualification-v1-gameplay-mixed.md
```

第二轮容量基线修正为：

```text
GAME_INITIAL_FOOD_COUNT=0
GAME_MAX_FOOD_COUNT=0
```

并新增：

```text
aliveHumans
deadHumans
exitedHumans
```

从而把“连接/协议容量”与“玩法淘汰”分离。

## 2. 统一测试场景

| 参数 | 值 |
| --- | --- |
| Duration | 300 s |
| Clients | 100 / 300 / 500 |
| Rooms | 1 / 3 / 5 |
| Room Max Players | 100 |
| MOVE Interval | 200 ms |
| Incoming Snapshot Drop | 0.5% |
| Outgoing MOVE Drop | 0.5% |
| Reconnect Storms | 5 |
| Storm Fraction | 30% |
| Random Churn | 0.005 / s，按 `ceil` 选取客户端 |
| Reconnect Jitter | 0～1000 ms |
| Food | 0 |
| Bot | 0 |
| AOI | Enabled |
| tc netem | Disabled |
| Runner | Ubuntu 24.04 / 4 vCPU AMD EPYC 9V74 / ~16.8 GB RAM |

三档分别在独立 runner 中执行，因此互不争抢同一服务进程资源。

## 3. 协议结果对比

| 指标 | 100 | 300 | 500 |
| --- | ---: | ---: | ---: |
| GAME_START | 100/100 | 300/300 | 500/500 |
| Snapshot Healthy | 100.00% | 99.33% | 99.60% |
| Forced Disconnects | 450 | 1,048 | 1,646 |
| Reconnect Success | 450/450 | 1,048/1,048 | 1,646/1,646 |
| Reconnect Rate | 100% | 100% | 100% |
| Reconnect Failures | 0 | 0 | 0 |
| Reconnect P95 | 7.174 ms | 16.964 ms | 17.294 ms |
| Reconnect P99 | 13.366 ms | 26.520 ms | 23.766 ms |
| Snapshot Lag P95 | 16 ms | 24 ms | 24 ms |
| Snapshot Lag P99 | 18 ms | 29 ms | 29 ms |
| Unexpected Read Errors | 0 | 0 | 0 |
| Write Errors | 2 | 9 | 1 |
| Bytes Received | 1.485 GB | 4.327 GB | 7.103 GB |

三档重连全部成功。300/500 的最终 Snapshot Healthy 低于 100% 是结束瞬间仍持续启用 0.5% application snapshot drop 的随机状态，均高于 98% qualification 门槛。

## 4. 服务端资源对比

### 4.1 CPU

| Clients | Mean | P95 | P99 | Max |
| ---: | ---: | ---: | ---: | ---: |
| 100 | 1.719% | 4.728% | 5.110% | 5.890% |
| 300 | 7.319% | 11.728% | 12.957% | 14.777% |
| 500 | 8.535% | 14.819% | 15.987% | 17.227% |

CPU% 定义为 `100% = 1 Core`。当前 4 vCPU runner 下，即使 500 clients，P99 仍未接近单核满载。

### 4.2 RSS / Heap

| Clients | RSS Max | Heap Alloc Max | Heap In-use Max |
| ---: | ---: | ---: | ---: |
| 100 | 26.77 MiB | 10.16 MiB | 11.91 MiB |
| 300 | 53.83 MiB | 30.47 MiB | 33.27 MiB |
| 500 | 78.38 MiB | 53.30 MiB | 56.23 MiB |

RSS 和 Heap 随客户端规模近似增长，但离 runner 内存上限仍很远。

### 4.3 Goroutine / FD / GC

| Clients | Goroutine Max | FD Max | GC Pause P99 | GC Pause Max |
| ---: | ---: | ---: | ---: | ---: |
| 100 | 207 | 111 | 5.076 ms | 6.987 ms |
| 300 | 611 | 311 | 10.889 ms | 11.166 ms |
| 500 | 1,015 | 511 | 10.112 ms | 12.660 ms |

Goroutine 约为 `2 × clients + 常驻开销`，FD 约为 `clients + 常驻开销`，在当前范围内呈可解释的线性关系，没有观察到随时间不断累积的泄漏。

## 5. 长时间趋势

稳态前 60 秒与后 60 秒：

| Clients | RSS 变化 | Heap Alloc 变化 | Heap In-use 变化 | Goroutine 变化 |
| ---: | ---: | ---: | ---: | ---: |
| 100 | +0.26 MiB | -0.01 MiB | +0.13 MiB | -0.95 |
| 300 | +0.42 MiB | -0.33 MiB | +0.35 MiB | -0.30 |
| 500 | +0.74 MiB | -0.26 MiB | +1.21 MiB | -0.72 |

五分钟窗口内没有看到 Heap Alloc、Goroutine 或 FD 的单调增长。RSS/Heap In-use 有小幅增长，但幅度较小，不能据此判定泄漏；更长的 30～60 分钟 Soak 才适合确认慢性内存增长。

## 6. 业务状态校验

第二轮稳定运行阶段：

| Clients | Alive | Dead Max | Exited Max | Balls End | Connected End |
| ---: | ---: | ---: | ---: | ---: | ---: |
| 100 | 100 | 0 | 0 | 100 | 100 |
| 300 | 300 | 0 | 0 | 300 | 300 |
| 500 | 500 | 0 | 0 | 500 | 500 |

说明本轮 reconnect 数据未被玩法淘汰污染。

## 7. 结论

### 7.1 可以得出的结论

1. 单进程在当前测试环境下能够完成 500 个真实 WebSocket 客户端、5 个 100 人房间的 5 分钟容量基线；
2. 500 客户端下 1,646 次强制断线全部重连成功；
3. AOI DELTA 丢帧后 FULL_SYNC 路径持续被真实触发；
4. 500 客户端 CPU、RSS、Heap、Goroutine、FD 和 GC 尚未出现明显饱和或泄漏迹象；
5. 第一轮失败是重要的测试设计发现：容量测试必须区分业务终态与连接失败。

### 7.2 不能得出的结论

不能写成：

```text
单机最大容量 = 500 clients
```

原因是 500 clients 仍没有找到明确的 CPU、内存或协议退化拐点，而且当前还缺少 Room Tick / Snapshot build latency histogram。

## 8. 下一阶段

建议下一阶段按顺序进行：

1. 在服务端补 Room Tick duration / over-budget 和 Snapshot build duration 指标；
2. 容量阶梯继续到 750 / 1000 / 1500 clients，直到出现稳定 SLO 拐点；
3. 对临界点至少重复 3 次，避免 GitHub runner 抖动造成偶然结论；
4. 增加 30～60 分钟 Soak 验证慢性 Heap/RSS/FD/Goroutine 增长；
5. 另建“真实玩法战斗压力”场景，恢复 food / 玩家淘汰，但使用业务终态感知的判定标准；
6. 弱网容量测试继续使用 `ws-load-chaos.yml` + `tc netem`，不要和纯容量基线混为一个结论。

## 9. 原始资产

Workflow run：`34983561506`

Artifacts：

- 100 clients：`10402646706`
- 300 clients：`10403091431`
- 500 clients：`10402229706`

单档正式报告：

- `docs/load-tests/2026-09-15-ws-100-client-soak.md`
- `docs/load-tests/2026-09-15-ws-300-client-soak.md`
- `docs/load-tests/2026-09-15-ws-500-client-soak.md`
