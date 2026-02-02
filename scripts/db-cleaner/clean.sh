#!/usr/bin/env bash
set -euo pipefail

#
# RTP压测清理工具：清理mysql订单表，redis缓存，校验rabbitmq的exchange+queue
#

################################
# 基础配置
################################
SITE="${SITE:-egame50001}"

REDIS_HOST="192.168.10.83"
REDIS_PORTS=("7000" "7001" "7002" "7003" "7004" "7005")
REDIS_PASS="A12345!"

MYSQL_HOST="192.168.10.83"
MYSQL_PORT="3306"
MYSQL_USER="root"
MYSQL_PASS="Aa12345!@#"
MYSQL_DB="egame_order"
MYSQL_TABLE="game_order"

RABBITMQ_HOST="192.168.10.83"
RABBITMQ_PORT="15672"
RABBITMQ_USER="admin"
RABBITMQ_PASS="admin"

EXCHANGE="${SITE}.gameorder.exchange"
BASE_QUEUE="${SITE}.gameorder.queue"
ROUTING_KEY="game_order"

################################
# 日志 & 执行器
################################
log_ok()   { echo -e "[\033[32mOK\033[0m] $*"; }
log_warn() { echo -e "[\033[33mWARN\033[0m] $*"; }
log_err()  { echo -e "[\033[31mFAIL\033[0m] $*"; }

run() {
  local desc="$1"
  shift
  if "$@"; then
    log_ok "$desc"
  else
    log_err "$desc"
    exit 1
  fi
}

################################
# 工具管理（统一）
################################
TOOLS=(
  "curl|curl|curl"
  "jq|jq|jq"
  "redis-cli|redis|redis-tools"
  "mysql|mysql-client|mysql-client"
)

ensure_tools() {
  for t in "${TOOLS[@]}"; do
    IFS="|" read -r bin mac_pkg linux_pkg <<< "$t"
    if command -v "$bin" >/dev/null 2>&1; then
      log_ok "$bin exists"
    else
      log_warn "$bin missing, installing..."
      if [[ "$OSTYPE" == "darwin"* ]]; then
        brew install "$mac_pkg"
      else
        sudo apt update -y && sudo apt install -y "$linux_pkg"
      fi
      command -v "$bin" >/dev/null 2>&1 || {
        log_err "$bin install failed"
        exit 1
      }
    fi
  done
}

################################
# Redis Cluster 清理
################################
clean_redis() {
  for REDIS_PORT in "${REDIS_PORTS[@]}"; do
    log_ok "Cleaning Redis Cluster at $REDIS_HOST:$REDIS_PORT"

    # 使用 --scan 命令扫描所有键，确保使用集群模式进行删除操作
    redis-cli -c -h "$REDIS_HOST" -p "$REDIS_PORT" \
      -a "$REDIS_PASS" --no-auth-warning \
      --scan --pattern "$SITE:*" \
    | xargs -r -P 100 -I {} redis-cli -c -h "$REDIS_HOST" -p "$REDIS_PORT" \
        -a "$REDIS_PASS" --no-auth-warning DEL {} >/dev/null 2>&1
  done
}


################################
# MySQL
################################
mysql_count() {
  mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" \
    -u "$MYSQL_USER" -p"$MYSQL_PASS" \
    -e "SELECT COUNT(*) AS total FROM ${MYSQL_DB}.${MYSQL_TABLE};"
}

mysql_truncate() {
  mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" \
    -u "$MYSQL_USER" -p"$MYSQL_PASS" \
    -e "TRUNCATE TABLE ${MYSQL_DB}.${MYSQL_TABLE};"
}

################################
# RabbitMQ
################################
rabbit_api() {
  curl -s -u "$RABBITMQ_USER:$RABBITMQ_PASS" "$@"
}

setup_rabbitmq() {
  # Exchange
  rabbit_api -X PUT -H "Content-Type: application/json" \
    -d '{"type":"direct","durable":true}' \
    "http://${RABBITMQ_HOST}:${RABBITMQ_PORT}/api/exchanges/%2F/$EXCHANGE"

  # Base queue + shard queues
  for q in "$BASE_QUEUE" $(seq 0 9 | sed "s|^|$BASE_QUEUE:|"); do
    rabbit_api -X PUT -H "Content-Type: application/json" \
      -d '{"durable":true}' \
      "http://${RABBITMQ_HOST}:${RABBITMQ_PORT}/api/queues/%2F/$q"

    rabbit_api -X POST -H "Content-Type: application/json" \
      -d "{\"routing_key\":\"$ROUTING_KEY\"}" \
      "http://${RABBITMQ_HOST}:${RABBITMQ_PORT}/api/bindings/%2F/e/$EXCHANGE/q/$q"
  done
}

################################
# 连通性测试
################################
test_tcp() {
  local name="$1" host="$2" port="$3"
  timeout 3 bash -c "</dev/tcp/$host/$port" \
    && log_ok "$name reachable" \
    || { log_err "$name unreachable"; exit 1; }
}

################################
# 主入口（唯一）
################################
main() {
  echo "===== RTP CLEAN START (SITE=$SITE) ====="

  ensure_tools

  test_tcp "Redis" "$REDIS_HOST" "${REDIS_PORTS[0]}"  # 测试第一个 Redis 节点
  test_tcp "MySQL" "$MYSQL_HOST" "$MYSQL_PORT"
  test_tcp "RabbitMQ" "$RABBITMQ_HOST" "$RABBITMQ_PORT"

  run "Redis cluster clean ($SITE:*)" clean_redis
  run "MySQL count" mysql_count
  run "MySQL truncate" mysql_truncate
  run "RabbitMQ setup" setup_rabbitmq

  log_ok "ALL DONE"
}

main "$@"