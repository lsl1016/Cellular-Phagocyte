# 压力测试文档规范

## 1. 适用范围

从本规范开始，所有 WebSocket、弱网、Soak、容量、极限并发和稳定性测试都必须保留一份 Markdown 测试报告。

报告不能只写“通过/失败”，至少必须说明：

1. 测试场景；
2. 测试环境与参数；
3. 观测方式；
4. 原始数据/关键结果；
5. 异常与风险；
6. 结论；
7. 下一步。

建议路径：

```text
docs/load-tests/YYYY-MM-DD-<scenario>.md
```

例如：

```text
docs/load-tests/2026-09-15-ws-300-client-soak.md
```

---

## 2. 强制保留的原始资产

每次测试至少保留：

```text
wsload report
server runtime time-series CSV
server log
GitHub Actions run / 本地执行命令
```

弱网测试还必须记录：

```text
tc netem delay
jitter
packet loss
application snapshot drop
application MOVE drop
```

不能只保留整理后的表格而丢失原始报告。

---

## 3. 报告模板

以下结构为最低要求。

```markdown
# <测试名称>

> 日期：YYYY-MM-DD
> Commit / Branch：...
> 执行环境：...
> 原始 Artifact：...

## 1. 测试目标

说明本次要验证什么，以及不验证什么。

## 2. 测试场景

### 2.1 客户端规模

- clients：
- rooms：
- duration：
- MOVE interval：

### 2.2 Chaos / 弱网参数

- netem delay：
- netem jitter：
- netem packet loss：
- incoming snapshot drop：
- outgoing MOVE drop：
- reconnect storm：
- random churn：

### 2.3 服务端关键配置

- TickRate：
- SnapshotRate：
- MATCH_MIN_PLAYERS：
- GAME_BOTS：
- AOI：
- FULL interval：

## 3. 观测方式

### 3.1 客户端协议观测

说明 wsload 如何统计：

- GAME_START
- FULL / DELTA / RECOVER
- sequence mismatch
- FULL_SYNC
- reconnect
- snapshot lag
- socket errors

### 3.2 服务端 Runtime 观测

说明：

- CPU
- RSS
- Heap
- Goroutine
- FD
- GC pause
- rooms / players / connected humans

并注明 time-series CSV 路径。

## 4. 测试结果

### 4.1 协议与重连

| 指标 | 结果 | 门槛 | 判定 |
| --- | ---: | ---: | --- |
| Game Start | | | |
| Snapshot Healthy | | | |
| Reconnect Success | | | |
| Reconnect P95 | | | |
| Snapshot Lag P95 | | | |
| Read Errors | | | |
| Write Errors | | | |

### 4.2 服务端资源

| 指标 | Peak / P95 / End | 说明 |
| --- | ---: | --- |
| CPU % | | |
| RSS | | |
| Heap Alloc | | |
| Goroutines | | |
| Open FDs | | |
| GC Pause Delta | | |

### 4.3 时间趋势

说明是否观察到：

- CPU 持续高位；
- Heap/RSS 单调增长；
- Goroutine/FD 泄漏；
- reconnect storm 后无法回落；
- Snapshot Lag 与 CPU/GC 尖峰相关；
- connectedHumans 与预期客户端数不一致。

## 5. 异常与问题

逐条记录真实异常，不得把“通过门槛”写成“零错误”。

## 6. 结论

明确写：

- 通过 / 条件通过 / 失败；
- 本次结论适用的客户端规模与环境；
- 不能从本次测试推出的结论。

## 7. 后续动作

列出需要修复、复测或继续扩大容量的项目。
```

---

## 4. 判定原则

### 4.1 不允许用单一指标下结论

例如：

```text
300 clients reconnect success = 99.7%
```

不能单独推出“300 客户端稳定”。还必须结合：

```text
snapshotHealthyRate
snapshot lag
CPU
RSS / Heap
Goroutine
FD
GC pause
读写错误
持续时间
```

### 4.2 必须区分瞬时值、累计值与分位数

示例：

- `RSS` 是瞬时采样；
- `TotalAlloc` 是累计值；
- `Reconnect P95` 是事件样本分位数；
- `process_cpu_percent` 是相邻采样窗口的派生值。

报告中不得混用。

### 4.3 必须区分协议稳定性与容量上限

短时间 300 客户端通过说明：

> 当前场景下协议和服务未达到失败门槛。

它不等于：

> 单机生产容量上限是 300。

容量上限需要持续增加客户端数，直到 CPU、内存、Tick/Snapshot latency 或协议 SLO 出现稳定退化，并且需要固定规格机器进行重复测试。

### 4.4 异常必须原样记录

如果：

```text
335 reconnect attempts
334 success
1 failure
```

报告必须写 1 次失败，不能只写“99.7%，通过”。

---

## 5. 建议的测试层级

### L1 - CI Smoke

```text
50 clients
10-20 seconds
```

目标：防止明显回归。

### L2 - Weak Network Qualification

```text
100 clients
真实 tc netem
reconnect storm
snapshot drop
```

目标：验证协议恢复和弱网容错。

### L3 - Multi-room Stress

```text
300 clients
3+ rooms
reconnect storm
```

目标：验证多房间并发。

### L4 - Soak

```text
100 / 300 / 500 clients
5-30 minutes
```

目标：发现 Heap、Goroutine、FD、GC 等随时间累积的问题。

### L5 - Capacity Curve

```text
100 -> 300 -> 500 -> 1000+
固定机器规格
固定场景
```

目标：寻找资源/延迟拐点和容量上限。

---

## 6. 当前约定

后续新增或执行任何正式压测时，应同时提交或更新对应 Markdown 报告，并在报告中引用原始 artifact / workflow run。

如果只是开发阶段临时 smoke，可不单独建报告；一旦测试数据被用于性能结论、容量判断、技术评审或版本准入，就必须按本规范留档。
