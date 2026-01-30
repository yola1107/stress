# 监控系统

监控远程 RabbitMQ 集群、Redis 集群和 MySQL 数据库。

## 部署拓扑
- **Prometheus + Grafana + Exporters**: 192.168.10.87
- **RabbitMQ 集群**: 192.168.10.83 (scripts/rabbitmq-cluster)
- **Redis 集群**: 192.168.10.83 (scripts/redis-cluster)
- **MySQL**: 192.168.10.83

## 组件
- Prometheus: 数据收集
- Grafana: 数据可视化
- Node Exporter: 系统监控
- MySQL Exporter: MySQL 监控  
- Redis Exporter: Redis 监控
- RabbitMQ Exporter (kbudde): RabbitMQ Management API 指标

## 监控目标
- Redis 集群: 192.168.10.83:7000-7005 (redis_exporter /scrape)
- MySQL 数据库: 192.168.10.83:3306
- RabbitMQ 集群: 192.168.10.83:15692/15693/15694 (rabbitmq_prometheus 原生插件，Grafana 10991)
- RabbitMQ Management: 192.168.10.83:15672 (kbudde exporter)

## 快速开始

### 1. 部署前准备

**MySQL (192.168.10.83) 准备：**
```sql
CREATE USER 'exporter'@'%' IDENTIFIED BY 'exporter123';
GRANT PROCESS, REPLICATION CLIENT, SELECT ON *.* TO 'exporter'@'%';
FLUSH PRIVILEGES;
```

### 2. 启动服务
```bash
./start.sh
```

### 3. 访问地址
- **Prometheus**: http://localhost:9090
- **Grafana**: http://localhost:3000 (admin/admin)

## 故障排除

### MySQL Exporter 重启问题
**症状：** mysql-exporter 容器一直重启

**解决方案：**
```bash
# 1. 测试 MySQL 连接
./test-mysql.sh

# 2. 检查详细日志
docker-compose logs mysql-exporter

# 3. 运行完整诊断
./troubleshoot.sh
```

**常见原因：**
- MySQL 服务器不可达
- 用户权限不足
- 密码错误
- 防火墙阻止

### 通用故障排除
```bash
# 完整故障诊断
./troubleshoot.sh

# 检查网络连通性
./test-connectivity.sh

# 测试 MySQL 连接
./test-mysql.sh

# 测试 RabbitMQ 连接
./test-rabbitmq.sh

# 查看所有服务状态
docker-compose ps

# 查看特定服务日志
docker-compose logs [服务名]
```

### 重启服务
```bash
# 重启所有服务
docker-compose restart

# 重启特定服务
docker-compose restart mysql-exporter

# 完全重建
docker-compose down && docker-compose up -d
```

## 文件说明
- `docker-compose.yml`: 服务配置
- `prometheus.yml`: Prometheus 配置
- `start.sh`: 启动脚本（含健康检查）
- `mysql-setup.sql`: MySQL 用户创建脚本
- `test-connectivity.sh`: 网络连通性测试
- `test-mysql.sh`: MySQL 连接测试
- `test-rabbitmq.sh`: RabbitMQ 连接测试
- `troubleshoot.sh`: 完整故障诊断

## 端口说明
- 9090: Prometheus
- 3000: Grafana
- 9100: Node Exporter
- 9104: MySQL Exporter
- 9121: Redis Exporter
- 9419: RabbitMQ Exporter