# WebSocket Load Chaos CI Notes

- `ci.yml` 的 50-client job 是 PR 硬门禁，使用应用层可重复 Chaos。
- `ws-load-chaos.yml` 是手动大规模工作流，使用 Linux `tc netem` 做真实 TCP delay/jitter/loss。
- 大规模 300-client 模式会按当前 `Match.MaxPlayers=100` 分成多个房间；测试目标是同一服务进程承载多房间实时 Tick、AOI 与重连风暴。
- `WS_LOAD_CHAOS_SUMMARY` 是后续接入趋势图、基准历史或性能回归门禁的稳定机器可读输出。
