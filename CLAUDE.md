# CLAUDE.md

本文件为 Claude Code (claude.ai/code) 在处理此仓库代码时提供指导。所有交流必须使用中文，文档内容本身也必须使用中文编写。

## Commands

```bash
make init    # 初始化
make api     # 生成 proto 代码
make all     # 生成所有代码
make build   # 构建
go test ./... # 测试
```

## Architecture

```
cmd/server/         # 入口
internal/
├── service/        # HTTP/GRPC 接口层
├── biz/            # 业务逻辑
│   ├── game/       # 游戏多态实现
│   │   ├── base/   # IGame 接口 + Default 默认实现
│   │   ├── gXXXXX/ # 各游戏覆盖（目前 45 款）
│   │   └── registry.go  # 显式注册表（编译期 ID 冲突检测）
│   ├── task/       # 任务生命周期
│   │   ├── task.go       # Task 实体 + Stats 统计
│   │   ├── task_exec.go  # Execute 主流程 + TaskRepo 接口 + ExecDeps
│   │   ├── task_pool.go  # 任务池 + 过期清理
│   │   ├── client.go     # APIClient + SessionEnv
│   │   └── session.go    # Session 状态机
│   ├── member/     # 玩家池分配/归还
│   ├── chart/      # Plotly.js 图表生成 + Chrome PNG
│   ├── metrics/    # Prometheus 指标
│   ├── notify/     # 飞书通知
│   └── scheduler.go  # 调度器（限流、队列、成员分配）
├── data/           # MySQL(xorm)/Redis/S3 数据层
│   ├── order.go    # 订单读取（SQL 分桶聚合）
│   └── cleanup.go  # 环境清理
└── conf/           # 配置定义
api/stress/v1/      # protobuf API 定义
```

## 新游戏

1. `internal/biz/game/g{gameID}/game.go` — 嵌入 `*base.Default`
2. 按需覆盖方法（详见下方 IGame 接口说明）
3. `registry.go` 添加一行注册
4. `make all && make build`

## 配置

- 端口: 8001 (HTTP) / 9001 (gRPC)
- 配置: `configs/config.yaml`
- 监控: `stress_task_*` (Prometheus)
- CI/CD: `.gitlab-ci.yml`

## IGame 接口

```go
type IGame interface {
    GameID() int64
    Name() string
    BetSize() []float64
    SetBetSize(betSize []float64)
    ValidBetMoney(money float64) bool
    IsSpinOver(data map[string]any) bool      // 判断一局是否结束
    NeedBetBonus(data map[string]any) bool     // 是否需要选奖（opt-in）
    BonusNextState(data map[string]any) bool   // 多轮 bonus 是否继续
    PickBonusNum() int64                       // 选取 bonus 编号
    GetProtobufConverter() ProtobufConverter   // protobuf 解码（nil 则用 JSON）
}
```

**Default 默认行为**：`IsSpinOver → true`，`NeedBetBonus → false`，`BonusNextState → false`，`PickBonusNum → rand(1-10)`

**Bonus 是 opt-in 的**：Default 不参与 bonus。需要 bonus 的游戏自行覆盖 `NeedBetBonus`（如 g18902/g18920/g18931/g18946），按需覆盖 `BonusNextState` 和 `PickBonusNum`。

**常见覆盖模式**：
- 仅覆盖 `IsSpinOver`：大多数普通游戏
- 覆盖 `NeedBetBonus`：有宝箱/选奖玩法的游戏
- 覆盖 `NeedBetBonus` + `BonusNextState`：多轮 bonus 游戏（如 g18902）
- 覆盖 `GetProtobufConverter`：使用 protobuf 协议的游戏（如 g18964）

## 任务执行流程

```
CreateTask → PENDING → scheduleLoop → RUNNING → Execute:
  1. BindSessionEnv（构建 SessionEnv：持有 game + task 引用）
  2. Monitor（1s 日志输出）
  3. startReporter（15s 周期 reportMetrics）
  4. runSessions（ants 并发池，每成员一个 Session 状态机）
  5. Stop（取消 context，Session 退出）
  6. SetFinishAt
  7. waitOrderWrite（轮询直到异步订单落库）
  8. stopReporter → finalize（图表上传 + 通知 + 环境清理 + 状态转换）
  9. cleanup（归还成员、关闭连接）
```

## Session 状态机

```
Idle → Launching → LoggingIn → Betting ⇄ BonusSelect → Completed/Failed
```

- `Betting`: 调 BetOrder API，成功后判断 `game.IsSpinOver` + `game.NeedBetBonus`
- `BonusSelect`: 调 BetBonus API（`game.PickBonusNum`），`game.BonusNextState` 决定是否继续
- 错误处理: relaunch / relogin / 重试（最大 5 次）

## 依赖注入

**TaskRepo 接口**（定义在 task 包，方法名与 biz.DataRepo 对齐）：
```go
type TaskRepo interface {
    GetGameOrderCount(ctx) (int64, error)
    GetDetailedOrderAmounts(ctx, scope) (...)
    QueryGameOrderPoints(ctx, scope) ([]chart.Point, error)
    UploadBytes(ctx, bucket, key, contentType string, data []byte) (string, error)
    CleanRedisBySites(ctx, sites []string) error
    CleanGameOrderTable(ctx) error
}
```

`scheduler.go` 直接传 `uc.repo`（DataRepo 隐式满足 TaskRepo），无需适配器。

**SessionEnv**（5 个字段）：
- `ctx` / `cfg` / `game` (base.IGame) / `task` (*Task) / `protobuf` (ProtobufConverter)
- Session 通过 `env.game.XXX()` 和 `env.task.XXX()` 直接调用，无 func 回调

## 游戏注册策略

采用**显式注册**（`registry.go` 中 map 字面量）：
- 编译期检测 ID 重复（map key 重复直接编译报错）
- 一屏可见全部游戏，便于维护
- 游戏数量可控（<100），不需要插件化

## 订单数据采样

`QueryGameOrderPoints` 使用 **SQL 分桶聚合**：
- 将 ID 范围等分为 ≤5000 个桶
- MySQL 一次 GROUP BY 返回 ~5000 行（而非 2000 万行）
- Go 侧前缀累加 cumBet/cumWin，数学等价于逐行扫描
- 性能：20M 订单从 30 分钟降至 1-3 分钟

## 原则

**文件作用域模式**:
- 默认只分析当前修改的单款游戏 (gXXXXX/)
- 涉及通用逻辑时，分析 base/ 和 registry.go
- 仅在明确指定"全仓分析"时扫描所有游戏

**修改优先级**:
1. 优先修改单款游戏的独立逻辑
2. 如需修改 base/ 通用逻辑，需确保向后兼容
3. 禁止修改 registry.go 以外的其他游戏代码
4. 架构调整前必须与用户确认

**代码质量**:
- 代码精简，避免冗余，不过分设计
- 优先使用组合而非继承 (如 `*base.Default`)
- 接口隔离: DataRepo/UseCase/TaskRepo 分层清晰
- 资源管理: defer 释放/超时控制/重试机制

**测试要求**:
- 新增功能必须附带单元测试
- 修改前运行现有测试确保通过
- 保持测试覆盖率不降低
