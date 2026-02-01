# RabbitMQ 集群

基于Docker的RabbitMQ高可用集群配置，采用3节点架构 + HAProxy负载均衡。

## 快速开始

### 一键部署 (推荐)
```bash
./start.sh
```
该脚本会自动清理旧进程、创建数据目录、启动集群并配置HAProxy负载均衡。

### 手动部署
```bash
# 1. 创建数据目录
mkdir -p data/rabbitmq{1,2,3}
chmod -R 755 data/

# 2. 启动RabbitMQ节点
docker-compose up -d rabbitmq1 rabbitmq2 rabbitmq3

# 3. 组建集群
docker exec rabbitmq2 rabbitmqctl stop_app
docker exec rabbitmq2 rabbitmqctl join_cluster rabbit@rabbitmq1
docker exec rabbitmq2 rabbitmqctl start_app

docker exec rabbitmq3 rabbitmqctl stop_app
docker exec rabbitmq3 rabbitmqctl join_cluster rabbit@rabbitmq1
docker exec rabbitmq3 rabbitmqctl start_app

# 4. 启动HAProxy
docker-compose up -d haproxy
```

## 访问集群

```bash
# 连接AMQP (通过HAProxy负载均衡)
# 地址: localhost:5672
# 用户名: admin
# 密码: admin

# 管理界面访问
# 地址: http://localhost:15672
# 用户名: admin
# 密码: admin
# 注意: 管理界面已映射到 rabbitmq1 节点，可直接访问

# 集群状态检查
docker exec rabbitmq1 rabbitmqctl cluster_status
docker exec rabbitmq1 rabbitmqctl list_nodes

# 队列和消息统计
docker exec rabbitmq1 rabbitmqctl list_queues
docker exec rabbitmq1 rabbitmqctl list_exchanges
```

## 架构组件

- **rabbitmq1/2/3**: 3个RabbitMQ节点 (rabbitmq1为主节点)
- **haproxy**: HAProxy负载均衡器 (5672端口)
- **专用网络**: rabbitmq-net 集群内部通信

## 使用示例

### Python客户端连接
```python
import pika

# 连接到HAProxy负载均衡器
connection = pika.BlockingConnection(
    pika.ConnectionParameters(
        host='localhost',
        port=5672,
        credentials=pika.PlainCredentials('guest', 'guest')
    )
)
channel = connection.channel()

# 声明队列
channel.queue_declare(queue='hello', durable=True)

# 发送消息
channel.basic_publish(
    exchange='',
    routing_key='hello',
    body='Hello RabbitMQ Cluster!',
    properties=pika.BasicProperties(delivery_mode=2)  # 持久化消息
)

print("消息已发送到RabbitMQ集群")
connection.close()
```

### 命令行测试
```bash
# 安装rabbitmqadmin (可选)
# curl -o rabbitmqadmin http://localhost:15672/cli/rabbitmqadmin
# chmod +x rabbitmqadmin

# 或使用Python发送测试消息
python3 -c "
import pika
conn = pika.BlockingConnection(pika.ConnectionParameters('localhost', 5672, 'guest', 'guest'))
ch = conn.channel()
ch.queue_declare('test_queue', durable=True)
ch.basic_publish('', 'test_queue', 'Test Message', pika.BasicProperties(delivery_mode=2))
print('Message sent')
conn.close()
"
```

## 集群特性

- **高可用**: 3节点集群，自动故障转移
- **负载均衡**: HAProxy轮询调度
- **数据持久化**: 消息和配置持久化到磁盘
- **镜像队列**: 支持跨节点队列镜像
- **联邦插件**: 支持跨集群消息路由

## 故障排除

```bash
# 查看服务状态
docker-compose ps

# 查看日志
docker-compose logs -f haproxy
docker-compose logs -f rabbitmq1

# 重启集群
docker-compose restart

# 重置集群 (清除所有数据)
docker-compose down -v
./start.sh

# 检查集群状态
docker exec rabbitmq1 rabbitmqctl cluster_status
docker exec rabbitmq1 rabbitmqctl list_nodes

# 检查节点健康
docker exec rabbitmq1 rabbitmq-diagnostics ping
docker exec rabbitmq1 rabbitmq-diagnostics memory_breakdown

# 查看连接和通道
docker exec rabbitmq1 rabbitmqctl list_connections
docker exec rabbitmq1 rabbitmqctl list_channels

# 性能监控
docker exec rabbitmq1 rabbitmqctl report > cluster_report.txt
```

## 生产环境配置

### 安全性增强
```bash
# 修改默认密码
docker exec rabbitmq1 rabbitmqctl change_password guest 'your_strong_password'

# 创建新用户并删除guest
docker exec rabbitmq1 rabbitmqctl add_user admin 'admin_password'
docker exec rabbitmq1 rabbitmqctl set_user_tags admin administrator
docker exec rabbitmq1 rabbitmqctl delete_user guest

# 配置权限
docker exec rabbitmq1 rabbitmqctl set_permissions -p / admin ".*" ".*" ".*"
```

### 性能优化
```bash
# 启用必要插件
docker exec rabbitmq1 rabbitmq-plugins enable rabbitmq_management
docker exec rabbitmq1 rabbitmq-plugins enable rabbitmq_prometheus

# 配置内存和磁盘告警
docker exec rabbitmq1 rabbitmqctl set_vm_memory_high_watermark 0.8
docker exec rabbitmq1 rabbitmqctl set_disk_free_limit 2GB
```

### 监控告警
```bash
# Prometheus指标 (需要启用插件)
curl http://localhost:15692/metrics

# 健康检查端点
curl http://localhost:15672/api/healthchecks/node

# 集群分区处理
docker exec rabbitmq1 rabbitmqctl set_cluster_partition_handling pause_minority
```

## 注意事项

- **Erlang Cookie**: 确保所有节点使用相同的.erlang.cookie文件
- **时钟同步**: 集群节点间时钟必须同步 (NTP)
- **网络连接**: 确保节点间网络连通性良好
- **磁盘空间**: 监控数据目录磁盘使用率
- **内存使用**: 设置合理的内存水位线
- **备份策略**: 定期备份配置和数据

## 端口说明

- **5672**: AMQP协议端口 (通过HAProxy负载均衡)
- **15672**: 管理界面端口 (已映射到rabbitmq1，可直接访问)
- **25672**: 集群通信端口 (内部使用，无需映射)

## 扩展集群

添加新节点：
```bash
# 1. 添加新服务到docker-compose.yml
# 2. 启动新节点
docker-compose up -d rabbitmq4

# 3. 加入集群
docker exec rabbitmq4 rabbitmqctl stop_app
docker exec rabbitmq4 rabbitmqctl join_cluster rabbit@rabbitmq1
docker exec rabbitmq4 rabbitmqctl start_app

# 4. 更新HAProxy配置
# 添加新节点到haproxy.cfg
# 重启HAProxy
docker-compose restart haproxy
```
