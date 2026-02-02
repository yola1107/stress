# Scripts 目录

本目录包含项目的基础设施部署和管理脚本。

## 目录结构

```
scripts/
├── ci-tool/                # CI/CD 工具配置
├── db-cleaner/             # 数据库清理工具
├── mysql-compose/          # MySQL Docker Compose 配置
├── prometheus/             # 监控系统配置
├── rabbitmq-cluster/       # RabbitMQ 集群配置
├── rabbitmq-compose/       # RabbitMQ 单机配置
├── redis-cluster/          # Redis 集群配置
└── redis-compose/          # Redis 单机配置
```

## 快速开始

### MySQL 压测

```bash
# 1. 安装工具
apt install sysbench -y

# 2. 清理和创建数据库
mysql -h192.168.10.83 -P3306 -uroot -p'Aa12345!@#' -e "
DROP DATABASE IF EXISTS sbtest;
CREATE DATABASE sbtest;
"

# 3. 准备测试数据
sysbench oltp_read_write \
  --mysql-host=192.168.10.83 \
  --mysql-port=3306 \
  --mysql-user=root \
  --mysql-password='Aa12345!@#' \
  --mysql-db=sbtest \
  --tables=16 \
  --table-size=200000 \
  prepare

# 4. 运行压测
sysbench oltp_read_write \
  --mysql-host=192.168.10.83 \
  --mysql-port=3306 \
  --mysql-user=root \
  --mysql-password='Aa12345!@#' \
  --mysql-db=sbtest \
  --tables=16 \
  --threads=32 \
  --time=60 \
  run

# 5. 清理测试数据
mysql -h192.168.10.83 -P3306 -uroot -p'Aa12345!@#' -e "DROP DATABASE sbtest;"
```

### Redis 压测

```bash
# 1. 安装工具
apt install -y redis-tools

# 2. 集群压测
redis-benchmark \
  -h 192.168.10.83 \
  -p 7000 \
  -a 'A12345!' \
  --cluster \
  -c 100 \
  -n 100000 \
  -t set,get

# 3. 单机压测（去掉 --cluster）
redis-benchmark \
  -h 192.168.10.83 \
  -p 6379 \
  -a 'A12345!' \
  -c 100 \
  -n 100000 \
  -t set,get

# 4. 安静模式（只看吞吐量）
redis-benchmark \
  -h 192.168.10.83 \
  -p 7000 \
  -a 'A12345!' \
  --cluster \
  -c 200 \
  -n 1000000 \
  -q

**参数说明：**
- `-h` 和 `-p` 指定Redis服务器地址和端口
- `-a` 指定密码
- `--cluster` 指定Cluster集群模式 (单机测试去掉--cluster)
- `-c 200` 表示200个并发连接
- `-n 1000000` 表示总共执行100万次操作
- `-q` 安静模式，只输出吞吐量结果，不显示详细过程
- `-t set,get` 指定要测试的命令类型（可选，默认测试多种命令）
```
