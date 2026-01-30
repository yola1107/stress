# Redis Cluster + Twemproxy 部署配置

基于Docker的Redis集群配置，包含Twemproxy代理层，3主3从高可用架构。

## 🚀 快速开始

```bash
# 部署集群
cd /data/redis-cluster
chmod +x start-cluster.sh
./start-cluster.sh

# 连接使用 - 两种方式

## 方式1: 直接连接Redis集群
redis-cli -c -h localhost -p 7000 -a "A12345!"
set test "hello"
get test

# 查看状态
cluster info
cluster nodes

## 方式2: 通过Twemproxy代理
redis-cli -h localhost -p 22121
set proxy_test "world"
get proxy_test

# 监控集群
./monitor-cluster.sh

# 实时监控
watch -n 5 ./monitor-cluster.sh

```

## ⚠️ Redis Cluster 设计理念

### 🎯 **为什么Redis官方没有Cluster Proxy？**

Redis Cluster 的设计理念是**客户端直接连接集群**，而不是通过单一代理。这提供了：

- **更好的性能**: 避免代理层的额外开销
- **更强的容错性**: 客户端可以智能处理节点故障
- **更好的扩展性**: 无单点代理瓶颈

### 📋 **当前状况说明**

**Redis官方确实没有专门的cluster proxy**，因为：

1. **协议复杂性**: MOVED/ASK重定向需要客户端智能处理
2. **性能考虑**: 代理会增加额外的网络跳数
3. **设计哲学**: 分布式系统应该让客户端参与集群管理

### 💡 **实际可行的解决方案**

#### **方案1：客户端直接连接（推荐）**
```python
# 使用支持Cluster的Redis客户端
from redis.cluster import RedisCluster

# 连接任意节点，客户端会自动发现整个集群
rc = RedisCluster(
    host='localhost',
    port=7000,
    password='A12345!',
    decode_responses=True
)

rc.set('mykey', 'myvalue')
rc.get('mykey')
```

#### **方案2：使用第三方代理**
- **Twemproxy**: 不支持Cluster
- **Codis**: 有自己的协议，不完全兼容
- **KeyDB Proxy**: 部分支持，但不是为Redis Cluster设计的

#### **方案3：应用层代理**
在应用层实现一个简单的代理，将请求路由到正确的节点。

### 🎯 **结论**

如果你坚持不修改业务代码，那么**Redis Cluster可能不是最佳选择**。建议考虑：

1. **Redis Sentinel**: 支持主从切换，但不支持数据分片
2. **Codis**: 完全兼容Redis协议，但需要迁移数据
3. **单机Redis + 分片**: 应用层实现分片逻辑

## ⚙️ 配置说明

### 集群架构
- **3主3从**: 7000/7002/7004为主节点，7001/7003/7005为从节点
- **自动故障转移**: 主节点故障时，从节点自动提升
- **数据分片**: 16384个哈希槽自动分布

### 端口映射
- 7000 → Master 1
- 7001 → Slave 1
- 7002 → Master 2
- 7003 → Slave 2
- 7004 → Master 3
- 7005 → Slave 3
- **22121 → Twemproxy 代理**

### 代理层说明
- **当前配置**: 使用HAProxy进行基本负载均衡
- **限制**: 无法处理Redis Cluster的数据分片逻辑
- **建议**: 业务代码直接使用Redis Cluster客户端

⚠️ **重要说明**: Redis Cluster的设计理念是客户端直接连接集群节点。如果需要透明代理，建议使用专门的Redis Cluster代理（如官方cluster proxy）或修改业务代码使用支持Cluster的客户端库。

### 基本配置
- **内存限制**: 每个节点2GB（LRU淘汰）
- **访问认证**: 密码 "A12345!"
- **数据持久化**: AOF模式

### 基本操作
```bash
# Redis Cluster 正确的使用方式：
# 必须使用支持Cluster的客户端

# 方式1: redis-cli (需要-c参数)
redis-cli -c -h localhost -p 7000 -a "A12345!"
set user:1 "Alice"
get user:1
cluster keyslot user:1  # 查看键分布

# 方式2: 编程语言客户端
# Python
from redis.cluster import RedisCluster
rc = RedisCluster(host='localhost', port=7000, password='A12345!')
rc.set('key', 'value')

# 方式3: 传统方式 (不推荐，会收到MOVED错误)
redis-cli -h localhost -p 7000 -a "A12345!" set key value
# 返回: MOVED 12345 127.0.0.1:7002
```

### 集群管理
```bash
cluster info      # 集群状态
cluster nodes     # 节点列表
info memory       # 内存使用
```

```bash
docker-compose exec redis-7000 redis-cli -p 7000 -a 'A12345!' cluster info
```

## 🔍 故障排除

```bash
# 启动失败
docker-compose logs
docker-compose down -v && ./start-cluster.sh

# 连接问题
redis-cli -c -h localhost -p 7000 -a "A12345!"

# 状态检查
./monitor-cluster.sh
cluster info
```

## 🧹 清理环境
```bash
# 停止集群
docker-compose down

# 清理数据
docker-compose down -v
```

---


## Linux 系统调优
# 增大文件描述符限制
ulimit -n 65535

# TCP 参数优化
sysctl -w net.core.somaxconn=65535
sysctl -w net.ipv4.tcp_max_syn_backlog=65535
sysctl -w net.ipv4.tcp_fin_timeout=15
sysctl -w net.ipv4.tcp_tw_reuse=1


