#!/bin/bash
set -e

# --------------------------
# RabbitMQ 集群启动
# --------------------------

# 默认用户名密码，可修改
RABBIT_USER=admin
RABBIT_PASS=admin

echo "=== 清理旧容器和网络 ==="
# 停止并删除所有相关容器
docker-compose down --remove-orphans 2>/dev/null || true

# 强制删除可能残留的容器（包括停止状态的）
for container in rabbitmq1 rabbitmq2 rabbitmq3 rabbitmq-haproxy; do
  if docker ps -a --format "{{.Names}}" | grep -q "^${container}$"; then
    echo "删除残留容器: $container"
    docker rm -f "$container" 2>/dev/null || true
  fi
done

# 清理未使用的网络
docker network prune -f 2>/dev/null || true

# 清理未使用的卷（可选）
docker volume prune -f 2>/dev/null || true

# 最终检查，确保没有残留容器
echo "检查清理结果..."
REMAINING_CONTAINERS=$(docker ps -a --format "{{.Names}}" | grep -E "^(rabbitmq[1-3]|rabbitmq-haproxy)$" | wc -l)
if [ "$REMAINING_CONTAINERS" -gt 0 ]; then
  echo "警告: 仍有 $REMAINING_CONTAINERS 个相关容器存在，尝试强制清理..."
  docker ps -a --format "{{.Names}}" | grep -E "^(rabbitmq[1-3]|rabbitmq-haproxy)$" | xargs docker rm -f 2>/dev/null || true
fi

echo "=== 创建数据目录并设置权限 ==="
for d in data/rabbitmq{1,2,3}; do
  mkdir -p "$d"
  chmod 755 "$d"
done

# 确保 Erlang cookie 文件权限正确
if [ -f "rabbitmq/.erlang.cookie" ]; then
  chmod 400 rabbitmq/.erlang.cookie
fi

echo "=== 启动 RabbitMQ 节点和 HAProxy ==="
docker-compose up -d

echo "=== 等待节点就绪 ==="
for node in rabbitmq1 rabbitmq2 rabbitmq3; do
  echo "等待 $node 启动..."
  for i in {1..30}; do
    if docker exec "$node" rabbitmq-diagnostics ping >/dev/null 2>&1; then
      echo "✓ $node 已就绪"
      break
    fi
    if [ $i -eq 30 ]; then
      echo "✗ $node 启动超时"
      exit 1
    fi
    sleep 2
  done
done

echo "=== 组建 RabbitMQ 集群 ==="
# 检查集群状态
CLUSTER_STATUS=$(docker exec rabbitmq1 rabbitmqctl cluster_status 2>/dev/null || echo "")

if echo "$CLUSTER_STATUS" | grep -q "rabbit@rabbitmq2\|rabbit@rabbitmq3"; then
  echo "集群已存在，跳过组建步骤"
else
  echo "集群未组建，开始组建..."
  
  # 逐个加入其他节点到集群
  for node in rabbitmq2 rabbitmq3; do
    echo "将 $node 加入集群..."
    # 检查节点是否已经在集群中
    NODE_STATUS=$(docker exec "$node" rabbitmqctl cluster_status 2>/dev/null || echo "")
    if echo "$NODE_STATUS" | grep -q "rabbit@rabbitmq1"; then
      echo "  $node 已在集群中，跳过"
      continue
    fi
    
    docker exec "$node" rabbitmqctl stop_app 2>/dev/null || true
    docker exec "$node" rabbitmqctl join_cluster rabbit@rabbitmq1
    docker exec "$node" rabbitmqctl start_app
    sleep 3
  done
  
  echo "等待集群同步..."
  sleep 10
fi

echo "=== 集群状态 ==="
docker exec rabbitmq1 rabbitmqctl cluster_status | grep -E "Cluster name|Running Nodes"

echo ""
echo "集群访问地址: localhost:5672 (通过 HAProxy 负载均衡)"
echo "默认账号密码: $RABBIT_USER / $RABBIT_PASS"
echo "管理界面访问:"
echo "  docker exec -it rabbitmq1 /bin/bash"
echo "  rabbitmq-plugins enable rabbitmq_management"
echo "  然后访问: http://<node-ip>:15672"
