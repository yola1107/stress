#!/bin/bash
set -euo pipefail

NODES=(7000 7001 7002 7003 7004 7005)
PASSWORD="A12345!"
HOST_IP="192.168.10.83"  # 宿主机 IP，与你 redis.conf 中 cluster-announce-ip 一致

echo "=== Redis Cluster 启动 ==="

# 创建数据目录
for P in "${NODES[@]}"; do
  mkdir -p "data/$P"
done

# 启动容器
docker-compose up -d

# 等待节点就绪
echo "=== 等待节点就绪 ==="
for P in "${NODES[@]}"; do
  echo -n "节点 $P ... "
  for i in {1..30}; do
    # 这里用 redis-cli 连接到容器内 127.0.0.1
    if docker-compose exec -T redis-$P redis-cli -h 127.0.0.1 -p $P -a "$PASSWORD" ping >/dev/null 2>&1; then
      echo "OK"
      break
    fi
    sleep 1
    [ $i -eq 30 ] && { echo "FAILED"; exit 1; }
  done
done

# 生成 cluster create 参数
NODE_ARGS=""
for P in "${NODES[@]}"; do
  NODE_ARGS+="$HOST_IP:$P "
done

# 幂等创建集群
echo "=== 检查集群状态 ==="
CLUSTER_OK=$(docker exec redis-7000 redis-cli -h 127.0.0.1 -p 7000 -a "$PASSWORD" cluster info 2>/dev/null | grep -c "cluster_state:ok" || true)
if [ "$CLUSTER_OK" -eq 0 ]; then
  echo "创建 Redis Cluster..."
  docker exec -i redis-7000 redis-cli --cluster create $NODE_ARGS --cluster-replicas 1 -a "$PASSWORD" --cluster-yes
else
  echo "Cluster 已存在，跳过创建"
fi

# 输出集群状态
echo ""
echo "=== Redis Cluster 状态 ==="
docker exec redis-7000 redis-cli -h 127.0.0.1 -p 7000 -a "$PASSWORD" cluster nodes
docker exec redis-7000 redis-cli -h 127.0.0.1 -p 7000 -a "$PASSWORD" cluster info

echo ""
echo "=== Redis Cluster Ready ==="
echo "集群节点: 7000~7005 (3主3从)"
echo ""
echo "访问方式:"
echo "  直接访问: redis-cli -c -h $HOST_IP -p 7000 -a $PASSWORD"
echo "  Go代码: 连接任意节点，客户端自动发现集群"
echo ""
echo "管理命令:"
echo "  集群状态: docker exec redis-7000 redis-cli -p 7000 -a $PASSWORD cluster info"
echo "  节点列表: docker exec redis-7000 redis-cli -p 7000 -a $PASSWORD cluster nodes"
echo ""
echo "测试命令:"
echo "  docker exec redis-7000 redis-cli -c -p 7000 -a $PASSWORD SET test_key \"hello\""
echo "  docker exec redis-7000 redis-cli -c -p 7000 -a $PASSWORD GET test_key"
