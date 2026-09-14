# 服务端性能基准

本项目的游戏服务默认以 **20 Hz** 驱动房间 Tick，即每个 Tick 的绝对时间窗口是 **50 ms**。50 ms 不是建议目标，而是不能长期触碰的上限：Tick 还需要给 Go 调度、GC、同进程其他房间、HTTP/WS 处理以及机器抖动留下余量。

因此性能优化应以可重复 benchmark 的趋势为依据，而不是只看一次本地体感。

## 1. 基准场景

`server/internal/game/benchmark_test.go` 固定覆盖三档规模：

| 场景 | 玩家 | 食物 | 吐出物 |
| --- | ---: | ---: | ---: |
| Small | 10 | 500 | 200 |
| Medium | 50 | 500 | 200 |
| Large | 100 | 500 | 200 |

玩家、食物和吐出物被放在互不发生吞噬的区域，玩家移动也被冻结。这样 benchmark 每轮的数据规模保持不变，不会因为对象逐渐被吃掉而让后续迭代虚假变快。

这些场景主要回答三个问题：

1. `BenchmarkRoomTickSimulationOnly`：碰撞、食物、吐出物、状态统计等**纯模拟 Tick**需要多少 CPU / 分配。
2. `BenchmarkRoomTickProductionCadence`：按照默认 **20 Hz Tick / 10 Hz Snapshot / 1 Hz Rank** 节奏运行时，房间主循环整体成本是多少。
3. `BenchmarkRoomSnapshotAssembly`：快照随玩家规模增长时的**对象构建、JSON 编码与 payload**成本是多少。

从 AOI 上线开始，后两类 benchmark 同时保留两条模式：

- `Full_*`：关闭 AOI，保留原始全房间快照历史基线。
- `AOI_*`：开启 AOI，测量按玩家生成个性化可见快照后的成本。

这很重要：不能因为默认配置切换到 AOI，就让原来 #10 建立的 Full benchmark 悄悄改变语义。Full 与 AOI 必须长期并存，才能区分“算法真的变快/变慢”和“测试场景换了”这两件事。

Snapshot benchmark 不包含真实 socket 写出。WebSocket 写出由独立 write pump 执行，不应阻塞权威 Tick；这里测的是 Tick 内必须支付的快照组装和序列化成本。

此外 benchmark 会输出：

```text
snapshot-data-bytes/op
```

它表示每次 benchmark operation 向所有连接累计生成的 `ROOM_SNAPSHOT.data` JSON 字节数，用来观察 AOI 对 payload 的趋势影响。它**不包含** WebSocket frame、Envelope 外层字段和 TLS/TCP 开销，因此不能直接当成线上 wire bytes。

## 2. 本地运行

在 `server/` 目录执行：

```bash
go test ./internal/game -run '^$' -bench BenchmarkRoomTick -benchmem -count=5
```

单独观察 Full / AOI 快照：

```bash
go test ./internal/game -run '^$' -bench BenchmarkRoomSnapshotAssembly -benchmem -count=5
```

输出会包含类似：

```text
BenchmarkRoomSnapshotAssembly/Full_P100_F500_E200
BenchmarkRoomSnapshotAssembly/AOI_P100_F500_E200
```

需要更稳定地对比优化前后时，建议分别保存结果，再用 `benchstat` 比较：

```bash
go test ./internal/game -run '^$' -bench BenchmarkRoomTick -benchmem -count=10 > before.txt
# 切换到优化后的提交
go test ./internal/game -run '^$' -bench BenchmarkRoomTick -benchmem -count=10 > after.txt
benchstat before.txt after.txt
```

关注至少四个指标：

- `ns/op`：单 Tick / 单次快照耗时趋势。
- `B/op`：每次操作产生的堆分配字节数。
- `allocs/op`：每次操作的分配次数，通常直接影响 GC 压力。
- `snapshot-data-bytes/op`：向所有玩家生成的快照 data 总量趋势。

AOI 不应只看“包变小了”。个性化快照会增加筛选和多次 JSON 序列化成本，所以必须同时观察 CPU、分配和 payload，避免把网络压力简单转移成 Tick CPU 压力。

## 3. 如何解读 50 ms Tick 预算

默认 TickRate=20，因此理论硬窗口为：

```text
1000 ms / 20 = 50 ms / Tick
```

但不要把 `49 ms` 理解为“达标”。单房间平均 Tick 如果已经接近 50 ms，多房间并行、GC 或短时抖动都会导致连续掉 Tick。

当前阶段更合理的原则是：

- 让 **100 玩家场景的纯模拟 Tick** 与 50 ms 保持明显数量级余量；
- 优先优化随玩家数呈平方增长、或随 `玩家 × 世界对象数` 增长的路径；
- 观察 `B/op` 与 `allocs/op`，避免只降低 CPU 却制造更高 GC 压力；
- Snapshot / AOI 优化要同时看 CPU 和最终网络 payload，二者不能互相替代；
- Full 与 AOI 两条 benchmark 必须使用相同实体布局和玩家规模，才能直接比较。

具体硬阈值应在部署机器规格、目标同时在线房间数和压测模型确定后再制定。

## 4. CI 中的 benchmark smoke

CI 使用固定 `-benchtime=10x` 跑 `BenchmarkRoomTick*`，目的只是：

- 确认 benchmark 代码始终可执行；
- 在 Actions 日志中留下同一 runner 环境下的趋势数据；
- 同时留下 Full / AOI production cadence 的 `snapshot-data-bytes/op`；
- 提前暴露数量级级别的退化。

**CI 暂时不根据 ns/op 设置硬失败阈值。** GitHub Hosted Runner 存在机器型号和邻居负载差异，用微基准绝对时间直接卡 PR 容易产生误报。真正的性能门槛应放到固定规格的压测环境。

## 5. 后续指标

当前 benchmark 是进程内微基准。后续 AOI / 多房间阶段还应补：

- 1 / 10 / 50 个并发 Room 的 Tick 延迟分布；
- Tick p50 / p95 / p99 与 missed-tick 次数；
- 每玩家每秒 Snapshot 字节数与 Full/AOI 降幅；
- AOI 可见玩家/球体/食物/吐出物数量分布；
- AOI filter 与 JSON encode 分阶段耗时；
- WS 慢客户端下的可靠队列长度、snapshot 覆盖次数、断连次数；
- GC pause、heap、goroutine 数；
- 100 玩家机器人压测下的 CPU / RSS / 网络吞吐。

微基准负责发现算法与分配趋势，端到端压测负责验证真实容量，两者需要同时保留。
