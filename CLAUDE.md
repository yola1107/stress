# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
make init    # 初始化
make all     # 生成代码
make build   # 构建
go test ./... # 测试
```

## Architecture

```
cmd/server/      # 入口
internal/
├── service/     # HTTP/GRPC
├── biz/         # 业务逻辑
│   ├── game/    # 游戏 (gXXXXX)
│   ├── task/    # 任务
│   ├── member/  # 玩家池
│   └── ...      # scheduler/chart/metrics/notify
└── data/        # MySQL/Redis/S3
```

## 新游戏

1. `internal/biz/game/g{gameID}/game.go`
2. `registry.go` 注册
3. `make all && make build`

## 配置

- 端口: 8001 (HTTP) / 9001 (gRPC)
- 配置: `configs/config.yaml`
- 监控: `stress_task_*` (Prometheus)
- CI/CD: `.gitlab-ci.yml`

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
- 接口隔离:DataRepo/Usecase 分层清晰
- 资源管理:defer 释放/超时控制/重试机制 (如 S3 上传)

**测试要求**:
- 新增功能必须附带单元测试
- 修改前运行现有测试确保通过
- 保持测试覆盖率不降低

**数据兼容**:
- 数据结构变更需检查数据库兼容性
- 协议变更需保证向前兼容
- 配置变更需提供迁移方案
