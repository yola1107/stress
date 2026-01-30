# Stress 快速开始指南

本文档将指导您快速上手 Stress 压力测试服务，包括游戏添加、参数配置、服务启动和结果查看等完整流程。

## 🚀 快速体验

### 1. 环境准备

```bash
# 确保安装以下依赖
go version  # >= 1.21
docker --version
docker-compose --version
```

### 2. 启动基础服务

```bash
# 启动 MySQL 数据库
cd scripts/mysql-compose
docker-compose up -d

# 启动 Redis 缓存
cd ../redis-compose
docker-compose up -d

# 启动监控系统（可选）
cd ../prometheus
./start.sh
```

### 3. 启动 Stress 服务

```bash
# 返回项目根目录
cd ../../

# 构建并启动服务
make build
./bin/server -conf ./configs/config.yaml
```

服务启动后，您将看到类似输出：
```
INFO msg=Starting server. Name="stress" Version="dev"
INFO msg=[HTTP] server listening on: [::]:8000
INFO msg=[gRPC] server listening on: [::]:9000
```

## 🎮 快速添加新游戏

### 1. 创建游戏实现

在 `internal/biz/game/` 目录下创建新的游戏包：

```bash
mkdir internal/biz/game/g12345
```

创建 `internal/biz/game/g12345/set.go`：

```go
package g12345

import (
	"fmt"
	"stress/internal/biz/game/base"
)

type Game struct {
	base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(12345, "新游戏名称")}
}

// 自定义 Spin 结束判断逻辑
func (*Game) IsSpinOver(data map[string]any) bool {
	// 根据游戏返回数据判断是否结束
	// 示例：检查 free 字段是否为 0
	return fmt.Sprintf("%v", data["free"]) == "0"
}
```

### 2. 注册游戏

编辑 `internal/biz/game/registry.go`：

```go
var gameInstances = []base.IGame{
	g18890.New(),
	g18923.New(),
	g18912.New(),
	g12345.New(), // 添加新游戏
}
```

### 3. 重新编译

```bash
make all
```

## ⚙️ 参数详解

### 任务配置参数

```json
{
  "config": {
    "game_id": 18890,           // 游戏ID（必填）
    "member_count": 100,        // 并发用户数（必填）
    "target": 10000,            // 目标回合数（必填）
    "bet_order": {              // 下注配置
      "delay_ms": 100,          // 请求间隔毫秒数
      "base_money": 0.1,        // 基础投注金额
      "random_base": false      // 是否随机基础金额
    },
    "bet_bonus": {              // 奖励配置（可选）
      "enabled": true,          // 是否启用奖励模式
      "delay_ms": 200           // 奖励请求间隔
    }
  }
}
```

### 配置文件参数

`configs/config.yaml` 主要配置项：

```yaml
server:
  http:
    addr: 0.0.0.0:8000        # HTTP 服务端口
  grpc:
    addr: 0.0.0.0:9000        # GRPC 服务端口

data:
  database:
    dsn: "数据库连接字符串"
  redis:
    addr: "redis地址:端口"
    password: "密码"

stress:
  launch:
    merchant: "商户标识"
    url: "游戏API地址"
    sign_required: true       # 是否需要签名
  chart:
    generate_local: false     # 是否生成本地图表
  notify:
    webhook_url: "飞书Webhook地址"
```

## 🚀 启动压测

### 方法一：HTTP API

```bash
# 创建压测任务
curl -X POST http://localhost:8000/stress/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "config": {
      "game_id": 18890,
      "member_count": 50,
      "target": 5000,
      "bet_order": {
        "delay_ms": 100,
        "base_money": 100
      }
    }
  }'

# 响应示例
{
  "task_id": "20240101-18890-1",
  "status": "TASK_PENDING"
}
```

### 方法二：GRPC 客户端

```bash
# 使用 grpcurl 工具
grpcurl -plaintext -d '{
  "config": {
    "game_id": 18890,
    "member_count": 50,
    "target": 5000,
    "bet_order": {
      "delay_ms": 100,
      "base_money": 100
    }
  }
}' localhost:9000 stress.v1.StressService/CreateTask
```

### 方法三：Python 客户端

```python
import requests
import json

response = requests.post(
    'http://localhost:8000/stress/tasks',
    headers={'Content-Type': 'application/json'},
    data=json.dumps({
        "config": {
            "game_id": 18890,
            "member_count": 50,
            "target": 5000,
            "bet_order": {
                "delay_ms": 100,
                "base_money": 100
            }
        }
    })
)

task_info = response.json()
print(f"任务ID: {task_info['task_id']}")
```

## 📊 实时监控

### 1. 命令行查看

```bash
# 查看任务列表
curl http://localhost:8000/stress/tasks

# 查看特定任务详情
curl http://localhost:8000/stress/tasks/{task_id}

# 实时监控（每秒刷新）
watch -n 1 'curl -s http://localhost:8000/stress/tasks/{task_id}'
```

### 2. Grafana 监控面板

访问地址：`http://localhost:3000`
默认账号：`admin/admin`

主要监控面板：
- **任务进度**: 当前完成进度百分比
- **实时QPS**: 每秒请求数
- **RTP曲线**: 平台回报率趋势
- **成功率**: 请求成功比例
- **系统资源**: CPU、内存使用情况

### 3. Prometheus 指标

访问地址：`http://localhost:9090`

关键指标：
```promql
# 当前任务进度
stress_task_progress_pct{task_id="20240101-18890-1"}

# 实时QPS
stress_task_qps{task_id="20240101-18890-1"}

# RTP百分比
stress_task_rtp_pct{task_id="20240101-18890-1"}

# 活跃成员数
stress_task_active_members{task_id="20240101-18890-1"}
```

## 📈 查看压测结果

### 1. 任务完成状态

```bash
# 查看任务最终状态
curl http://localhost:8000/stress/tasks/{task_id}

# 响应示例
{
  "task": {
    "id": "20240101-18890-1",
    "status": "TASK_COMPLETED",
    "game_id": 18890,
    "game_name": "战火西岐",
    "member_count": 50,
    "target": 5000,
    "process": 5000,
    "progress_pct": 100,
    "duration": "2m30s",
    "qps": 33.33,
    "avg_latency": "30ms",
    "rtp_pct": 95.5
  }
}
```

### 2. 详细统计报告

```bash
# 获取完整统计报告
curl http://localhost:8000/stress/records/{task_id}

# 响应包含：
# - RTP 趋势图表（HTML格式）
# - 详细统计数据
# - 时间序列信息
```

### 3. 图表查看

任务完成后，系统会生成 HTML 报告，可通过以下方式访问：

```bash
# 如果配置了 S3 存储
# 报告会上传到 S3，返回可访问的 URL

# 如果配置了本地生成
# 报告保存在本地，可通过文件系统访问
```

## 🛠️ 常用操作命令

### 任务管理

```bash
# 列出所有任务
curl http://localhost:8000/stress/tasks

# 获取任务详情
curl http://localhost:8000/stress/tasks/{task_id}

# 取消运行中的任务
curl -X POST http://localhost:8000/stress/tasks/{task_id}/cancel

# 删除已完成的任务
curl -X DELETE http://localhost:8000/stress/tasks/{task_id}
```

### 系统维护

```bash
# 清理 Redis 缓存
# （系统会在启动时自动清理）

# 查看系统状态
curl http://localhost:8000/stress/ping/health

# 重新加载配置
kill -HUP {进程ID}
```

## 🎯 性能调优建议

### 1. 并发用户数设置

```bash
# 小规模测试（验证功能）
member_count: 10-50

# 中等规模测试（性能评估）
member_count: 100-500

# 大规模测试（压力测试）
member_count: 1000+
```

### 2. 延迟参数调整

```json
{
  "bet_order": {
    "delay_ms": 50    // 高并发时可适当减小
  },
  "bet_bonus": {
    "delay_ms": 100   // 奖励请求可设置较大延迟
  }
}
```

### 3. 监控告警设置

在 Grafana 中设置告警规则：
- QPS 异常下降
- RTP 异常波动
- 错误率超过阈值
- 系统资源使用过高

## 🔧 故障排除

### 常见问题

1. **服务无法启动**
   ```bash
   # 检查端口占用
   netstat -tlnp | grep 8000
   
   # 检查依赖服务状态
   docker ps
   ```

2. **数据库连接失败**
   ```bash
   # 测试数据库连接
   mysql -h localhost -P 3306 -u user -p
   
   # 检查配置文件中的 DSN
   ```

3. **Redis 连接问题**
   ```bash
   # 测试 Redis 连接
   redis-cli -h localhost -p 6379 ping
   ```

4. **任务执行异常**
   ```bash
   # 查看详细日志
   tail -f logs/stress.log
   
   # 检查游戏 API 可达性
   curl -v https://game-api.example.com/health
   ```

### 日志查看

```bash
# 应用日志
tail -f logs/stress.log

# Docker 容器日志
docker-compose logs -f mysql
docker-compose logs -f redis
```

## 📚 进阶使用

### 1. 批量任务执行

```bash
#!/bin/bash
# batch_test.sh

games=(18890 18912 18923)
members=(50 100 200)

for game in "${games[@]}"; do
  for member in "${members[@]}"; do
    curl -X POST http://localhost:8000/stress/tasks \
      -H "Content-Type: application/json" \
      -d "{
        \"config\": {
          \"game_id\": $game,
          \"member_count\": $member,
          \"target\": 10000,
          \"bet_order\": {
            \"delay_ms\": 100,
            \"base_money\": 100
          }
        }
      }"
    sleep 10
  done
done
```

### 2. 自定义监控面板

在 Grafana 中导入自定义仪表板：
- RTP 实时监控面板
- 用户行为分析面板
- 系统性能综合面板

### 3. 集成 CI/CD

```yaml
# .gitlab-ci.yml 片段
stress_test:
  stage: test
  script:
    - make build
    - ./bin/server -conf configs/test.yaml &
    - sleep 5
    - python3 scripts/test_client.py
  only:
    - merge_requests
```

---

🎉 恭喜！您已经掌握了 Stress 压力测试服务的基本使用方法。如需更深入的功能，请参考完整文档或联系技术支持。