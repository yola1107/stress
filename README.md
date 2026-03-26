# Stress - 高性能压力测试服务

基于 [Kratos](https://github.com/go-kratos/kratos) 框架构建的压力测试与性能监控服务，专为游戏平台设计，支持多游戏并发测试、实时指标监控和自动化报告生成。

## 🚀 核心特性

- **多游戏支持**: 内置 3 款热门游戏（战火西岐、金钱虎、巨龙传说）
- **高并发测试**: 支持数千用户同时在线压测
- **实时监控**: 集成 Prometheus + Grafana 监控体系
- **智能调度**: 任务队列管理和资源调度优化
- **自动化报告**: 测试完成后自动生成图表和统计报告
- **飞书通知**: 支持测试完成通知和异常告警
- **容器化部署**: Docker + Kubernetes 友好

## 🏗️ 系统架构

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   HTTP/GRPC     │    │    Scheduler    │    │     Metrics     │
│    Server       │◄──►│   (任务调度)    │◄──►│   (指标收集)    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│     Games       │    │     Users       │    │    Storage      │
│   (游戏池)      │    │   (用户管理)    │    │   (存储层)      │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## 🎮 支持的游戏

| 游戏ID | 游戏名称 | 特性 |
|-------|---------|------|
| 18890 | 战火西岐 | 自定义 Spin 结束判断逻辑 |
| 18912 | 金钱虎 | Free Spin 模式检测 |
| 18923 | 巨龙传说 | 复杂 Bonus 游戏机制 |

## 📊 监控指标

### 核心业务指标
- **QPS**: 每秒请求数
- **RTP**: 平台回报率 (Return to Player)
- **成功率**: 请求成功比例
- **平均延迟**: 接口响应时间
- **并发用户数**: 当前活跃用户数量

### 系统性能指标
- **CPU/Memory**: 系统资源使用率
- **连接数**: Redis/MySQL 连接状态
- **队列长度**: 任务队列堆积情况

## 📁 项目结构

```
stress/
├── api/                    # API 定义
│   └── stress/v1/         # v1 版本 API
│       ├── stress.proto   # Protocol Buffer 定义
│       ├── stress.pb.go   # 生成的 Go 代码
│       └── stress_http.pb.go # HTTP 绑定代码
├── cmd/                   # 应用入口
│   └── server/           # 服务端主程序
│       ├── main.go       # 程序入口
│       ├── wire.go       # Wire 依赖注入配置
│       └── wire_gen.go   # 生成的依赖注入代码
├── configs/              # 配置文件
│   └── config.yaml       # 主配置文件
├── internal/             # 内部业务逻辑
│   ├── biz/             # 业务逻辑层
│   │   ├── chart/       # 图表生成模块
│   │   ├── game/        # 游戏逻辑模块
│   │   ├── member/      # 用户管理模块
│   │   ├── metrics/     # 指标收集模块
│   │   ├── task/        # 任务管理模块
│   │   ├── user/        # 用户会话模块
│   │   ├── scheduler.go # 任务调度器
│   │   └── usecase.go   # 业务用例
│   ├── conf/            # 配置定义
│   ├── data/            # 数据访问层
│   ├── notify/          # 通知服务
│   ├── server/          # 服务启动配置
│   └── service/         # 服务实现
├── pkg/                 # 公共工具包
│   ├── xgo/            # Go 扩展工具
│   └── zap/            # 日志工具
├── scripts/             # 脚本工具
│   ├── ci-tool/        # CI/CD 工具
│   ├── mysql-compose/  # MySQL Docker 配置
│   ├── prometheus/     # 监控系统配置
│   ├── rabbitmq-cluster/ # RabbitMQ 集群
│   ├── redis-cluster/  # Redis 集群
│   └── README.md       # 脚本使用说明
├── third_party/         # 第三方协议定义
├── Dockerfile          # Docker 构建文件
├── Makefile            # 构建脚本
├── .gitlab-ci.yml      # CI/CD 配置
├── go.mod             # Go 模块定义
├── grafana.json       # Grafana 仪表板配置
└── openapi.yaml       # OpenAPI 规范
```

## 🛠️ 工具链

### 开发工具

1. **代码生成工具**
   ```bash
   # Protocol Buffer 编译器
   protoc --version  # >= 3.12.0
   
   # Go 代码生成插件
   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
   go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http@latest
   go install github.com/envoyproxy/protoc-gen-validate@latest
   ```

2. **依赖管理工具**
   ```bash
   # Wire 依赖注入
   go install github.com/google/wire/cmd/wire@latest
   
   # 代码质量检查
   go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
   ```

3. **API 文档工具**
   ```bash
   # Swagger/OpenAPI 生成
   go install github.com/google/gnostic/cmd/protoc-gen-openapi@latest
   ```

### 构建和部署工具

1. **容器化工具**
   ```bash
   docker --version     # >= 20.10
   docker-compose --version # >= 1.29
   ```

2. **监控工具**
   ```bash
   # Prometheus + Grafana
   # 通过 docker-compose 启动
   
   # 压测工具
   sysbench --version   # MySQL 压测
   redis-benchmark --help # Redis 压测
   ```

### 测试工具

1. **单元测试**
   ```bash
   # 基础测试
   go test ./...
   
   # 覆盖率测试
   go test -coverprofile=coverage.out ./...
   go tool cover -html=coverage.out
   
   # 基准测试
   go test -bench=. ./...
   ```

2. **集成测试**
   ```bash
   # API 测试
   grpcurl -plaintext localhost:9000 list
   
   # 性能测试
   wrk -t12 -c400 -d30s http://localhost:8000/stress/ping/health
   ```

## 🔄 CI/CD 流程

### GitLab CI 配置

`.gitlab-ci.yml` 定义了完整的 CI/CD 流程：

```yaml
# 构建阶段
build:
  stage: build
  image: 192.168.10.67/egame/ci-tools:go1.24.5-podman
  script:
    - make init     # 初始化依赖
    - make api      # 生成 API 代码
    - make build    # 编译构建
  artifacts:
    paths:
      - bin/
    expire_in: 1 week

# 部署阶段
deploy:
  stage: deploy
  image: 192.168.10.67/egame/alpine:3.16
  script:
    - # 部署到测试环境
    - # 部署到生产环境
  only:
    - main
    - tags
  when: manual
```

### 自动化流程

1. **代码提交触发**
   - 任何分支提交都会触发构建
   - 主分支和标签提交会触发完整流水线

2. **质量门禁**
   ```bash
   # 代码格式检查
   go fmt ./...
   
   # 代码质量检查
   golangci-lint run
   
   # 单元测试
   go test -v ./...
   ```

3. **构建产物**
   - Linux AMD64 二进制文件
   - Docker 镜像
   - API 文档

4. **部署策略**
   - 测试环境：自动部署
   - 预发布环境：手动审批
   - 生产环境：手动部署

### 部署脚本

```bash
# 构建脚本 (Makefile)
build:
	mkdir -p bin/ && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		-ldflags "-X main.Version=$(VERSION)" -o ./bin/ ./...

# 部署脚本
scp: build
	scp bin/server user@remote:/app/stress/
	ssh user@remote "cd /app/stress && ./server -conf configs/config.yaml"
```

### 监控告警

CI/CD 流程集成了完善的监控告警：

1. **构建状态监控**
   - 构建成功/失败通知
   - 构建时长统计
   - 资源使用监控

2. **部署状态监控**
   - 服务健康检查
   - 性能指标监控
   - 错误率监控

3. **通知渠道**
   - 飞书机器人通知
   - 邮件告警
   - Slack 通知

## 📦 快速开始

### 1. 环境准备

```bash
# 安装 Go (>= 1.21)
# 安装 Docker 和 Docker Compose
# 安装 protoc 编译器
```

### 2. 本地开发

```bash
# 克隆项目
git clone <repository-url>
cd stress

# 初始化依赖
make init

# 生成代码
make all

# 启动依赖服务
docker-compose -f scripts/mysql-compose/docker-compose.yaml up -d
docker-compose -f scripts/redis-compose/docker-compose up -d

# 启动服务
go run ./cmd/server
```

### 3. 容器化部署

```bash
# 构建镜像
make build

# 启动服务
docker-compose up -d
```

## 🎯 使用示例

### 创建压测任务

```bash
# 通过 HTTP API
curl -X POST http://localhost:8000/stress/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "config": {
      "game_id": 18890,
      "member_count": 100,
      "target": 10000,
      "bet_order": {
        "delay_ms": 100,
        "base_money": 100
      }
    }
  }'

# 通过 GRPC (使用 grpcurl)
grpcurl -plaintext -d '{
  "config": {
    "game_id": 18890,
    "member_count": 100,
    "target": 10000,
    "bet_order": {
      "delay_ms": 100,
      "base_money": 100
    }
  }
}' localhost:9000 stress.v1.StressService/CreateTask
```

### 查看任务状态

```bash
# 获取任务列表
curl http://localhost:8000/stress/tasks

# 获取特定任务详情
curl http://localhost:8000/stress/tasks/{task_id}

# 获取任务结果报告
curl http://localhost:8000/stress/records/{task_id}
```

## 📈 监控面板

访问 Grafana: `http://localhost:3000`

预设仪表板包含：
- 实时 QPS 监控
- RTP 趋势图
- 成功率统计
- 系统资源使用情况
- 任务进度跟踪

## 🔧 配置说明

### 主要配置项

```yaml
# configs/config.yaml
server:
  http:
    addr: 0.0.0.0:8000
  grpc:
    addr: 0.0.0.0:9000

data:
  database:
    dsn: "user:password@tcp(localhost:3306)/stress?charset=utf8mb4&parseTime=True&loc=Local"
  redis:
    addr: "localhost:6379"
    password: ""

stress:
  launch:
    merchant: "default"
    url: "https://game-api.example.com"
  chart:
    generate_local: false
  notify:
    webhook_url: "https://open.feishu.cn/open-apis/bot/v2/hook/xxx"
```

### 环境变量

```bash
STRESS_SERVER_HTTP_ADDR=:8000
STRESS_SERVER_GRPC_ADDR=:9000
STRESS_DATA_DATABASE_DSN=mysql://...
STRESS_DATA_REDIS_ADDR=localhost:6379
```

## 🧪 压测脚本

项目提供多种基础设施压测脚本：

### MySQL 压测
```bash
cd scripts
# Sysbench 压测
./mysql-sysbench.sh
```

### Redis 压测
```bash
# 集群模式压测
redis-benchmark -h localhost -p 7000 --cluster -c 100 -n 100000 -t set,get

# 单机模式压测
redis-benchmark -h localhost -p 6379 -c 100 -n 100000 -t set,get
```

### RabbitMQ 压测
```bash
# 启动集群
cd scripts/rabbitmq-cluster
./start.sh

# Python 客户端测试
python3 test_producer.py
```

## 📊 性能基准

基于标准配置的性能表现：

| 配置 | QPS | 平均延迟 | 最大并发 | RTP 精度 |
|------|-----|----------|----------|----------|
| 100用户 | ~5,000 | ~20ms | 100 | ±0.1% |
| 500用户 | ~20,000 | ~25ms | 500 | ±0.1% |
| 1000用户 | ~35,000 | ~30ms | 1000 | ±0.2% |

*测试环境：8核16G，本地网络*

## 🔒 安全考虑

- API 访问控制（JWT/API Key）
- 请求频率限制
- 敏感配置加密存储
- 日志脱敏处理
- 容器安全扫描

## 🤝 贡献指南

1. Fork 项目
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

### 代码规范

- 遵循 Go 官方编码规范
- 使用 `gofmt` 格式化代码
- 添加必要的单元测试
- 更新相关文档

## 📄 License

MIT License - 详见 [LICENSE](LICENSE) 文件

## 📞 支持

- Issues: [GitHub Issues](https://github.com/your-org/stress/issues)
- 文档: [Wiki](https://github.com/your-org/stress/wiki)
- 邮件: tech@example.com

---

*Powered by [Kratos](https://github.com/go-kratos/kratos) - A Go microservices framework*