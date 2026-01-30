# internal/biz 业务层说明

## 目录结构

```
biz/
├── biz.go          # Wire ProviderSet
├── usecase.go      # UseCase 构造、DataRepo 接口、成员加载、Order 查询透传
├── scheduler.go    # 调度与任务生命周期（Schedule / CreateTask / DeleteTask / CleanTestEnvironment 等）
├── metrics.go      # Prometheus 指标定义与 ReportTaskMetrics
├── game/           # 游戏抽象与注册
│   ├── base/       # IGame 接口与默认实现
│   ├── g18890/     # 具体游戏实现
│   ├── g18912/
│   ├── g18923/
│   ├── pool.go     # 游戏池（按 ID 获取/列表）
│   └── registry.go # 游戏实例注册
├── member/         # 成员池（空闲/已分配）
│   └── pool.go
├── task/           # 任务与任务池
│   ├── task.go     # Task 实体、进度/统计/Monitor
│   └── task_pool.go
└── user/           # 压测会话与 API 客户端
    ├── session.go  # Session 状态机与执行
    └── client.go   # HTTP 客户端、Launch/Login/BetOrder/BetBonus
```

## 分层与职责

| 层 | 职责 |
|----|------|
| **biz** | 业务编排：UseCase 持有一个 DataRepo 接口 + 领域池（Game/Task/Member），只做编排，不碰 Redis/DB 实现 |
| **data** | 数据访问：实现 DataRepo（成员/订单/清理 + 任务ID计数器 Redis Hash），所有 I/O 在此层 |
| **task** | 领域层：Task 实体 + TaskPool 队列，纯内存、无 I/O |
| **game / member / user** | 领域或支撑：游戏注册、成员池、会话与 HTTP 客户端 |
| **metrics** | 监控：只上报基础数据，计算在 Prometheus/Grafana |

## 数据流

- **成员**：DataRepo.BatchUpsertMembers → MemberPool.AddIdle；Schedule 时 Allocate → runTaskSessions 用完后 Release。
- **订单/RTP**：写入由下游 API 落库；biz 层通过 DataRepo.GetDetailedOrderAmounts/GetGameOrderCount 查库，供 Prometheus 与清理逻辑使用。
- **任务**：CreateTask → TaskPool.Add + Schedule；DequeuePending → 分配成员 → t.Start → runTaskSessions；结束时 CleanTestEnvironment、TaskPool 仍保留任务记录供 API 查询。

## Task 生命周期（runTaskSessions 内严格顺序）

1. **执行阶段开始**：创建 `runCtx = WithCancel(t.Context())`，启动 `Monitor(runCtx)`、`ReportTaskMetrics(runCtx, t, repo)`，二者仅依赖 runCtx。
2. **跑 Session**：`t.Submit(session)` 使用 `t.Context()` 与 `t.pool`，`wg.Wait()` 等待全部结束。
3. **结束附属**：`stopRun()` 取消 runCtx → Monitor 与 ReportTaskMetrics 立即退出，不再打进度、不再上报 Prometheus。
4. **结束任务**：`t.Stop()` 取消 task context、释放 ants 池。
5. **收尾**：`Release(meta.ID)`、`SetStatus(COMPLETED)`、`<-CleanTestEnvironment(meta)`、`Schedule()`。

CancelTask/DeleteTask 会取消或 Stop 任务，runCtx 随 t.ctx 取消而取消，附属 goroutine 同步退出。

## 已做优化（本次）

1. **game.Pool**：移除未赋值的 `usePB` 字段，`RequireProtobuf(gameID)` 改为根据 `IGame.GetProtobufConverter() != nil` 判断。
2. **member.Pool**：删除未被调用的 `Init` 方法。
3. **task.Pool**：`List()` 按任务创建时间倒序排序，与注释及 API 约定一致。
4. **task.Pool**：计数器改为 Redis 存储，使用 Hash 结构，key 为 `stress-pool:count:YYYY-MM-DD`，field 为 gameID，过期时间为明天凌晨0点。

## 依赖关系

- biz 只定义 DataRepo 接口，data 层实现（含 NextTaskID 的 Redis 逻辑）；task 为纯领域，无 data 依赖。
- metrics 只上报基础数据，计算在 Prometheus/Grafana。
