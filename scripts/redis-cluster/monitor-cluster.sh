#!/bin/bash
# 不使用严格错误处理，允许监控脚本继续运行
set +e

# ========================================
# Redis Cluster 监控脚本
# 显示: PING + 内存使用 + 集群状态
# ========================================

PASSWORD="A12345!"
NODES=(7000 7001 7002 7003 7004 7005)

# 颜色
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
NC='\033[0m'

echo "=== Redis Cluster 健康监控 ==="
echo "时间: $(date)"
echo

# 检查节点连通性
check_ping() {
  local PORT=$1
  if docker exec redis-$PORT redis-cli -p $PORT -a "$PASSWORD" ping &>/dev/null; then
    echo -e "节点 $PORT: ${GREEN}OK${NC}"
  else
    echo -e "节点 $PORT: ${RED}CONNECTION FAILED${NC}"
  fi
}

# 检查内存使用
check_memory() {
  local PORT=$1
  local MEM_INFO
  MEM_INFO=$(docker exec redis-$PORT redis-cli -p $PORT -a "$PASSWORD" info memory 2>/dev/null || echo "")
  if [ -z "$MEM_INFO" ]; then
    echo -e "节点 $PORT: ${RED}无法获取内存信息${NC}"
    return
  fi

  local USED MAX USED_H MAX_H USAGE COLOR
  USED=$(echo "$MEM_INFO" | grep "used_memory:" | cut -d: -f2)
  MAX=$(echo "$MEM_INFO" | grep "maxmemory:" | cut -d: -f2)
  USED_H=$(echo "$MEM_INFO" | grep "used_memory_human:" | cut -d: -f2)
  MAX_H=$(echo "$MEM_INFO" | grep "maxmemory_human:" | cut -d: -f2)

  if [ "$MAX" -eq 0 ]; then
    USAGE="N/A"
    COLOR=$RED
  else
    USAGE=$(( USED * 100 / MAX ))
    COLOR=$GREEN
    [ $USAGE -gt 80 ] && COLOR=$RED
    [ $USAGE -gt 60 ] && [ $USAGE -le 80 ] && COLOR=$YELLOW
  fi

  printf "节点 %s: ${COLOR}%s%%${NC} (%s / %s)\n" "$PORT" "$USAGE" "$USED_H" "$MAX_H"
}

# 检查集群状态
check_cluster() {
  echo
  echo "=== 集群状态 ==="
  local INFO
  INFO=$(docker exec redis-7000 redis-cli -p 7000 -a "$PASSWORD" cluster info 2>/dev/null || echo "")
  if [ -z "$INFO" ]; then
    echo -e "${RED}无法获取集群信息${NC}"
    return
  fi
  echo "$INFO" | grep -E "(cluster_state|cluster_size|cluster_known_nodes)"
}

# 执行检查
echo "节点连通性:"
for PORT in "${NODES[@]}"; do
  check_ping "$PORT"
done

echo
echo "节点内存使用情况:"
for PORT in "${NODES[@]}"; do
  check_memory "$PORT"
done

check_cluster

echo
echo "Legend:"
echo "- ${GREEN}Green${NC}: Memory usage < 60%"
echo "- ${YELLOW}Yellow${NC}: Memory usage 60-80%"
echo "- ${RED}Red${NC}: Memory usage > 80%"
echo "- OK / CONNECTION FAILED: 节点连通性"
