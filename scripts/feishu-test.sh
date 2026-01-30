#!/usr/bin/env bash
# 飞书 Webhook 测试脚本（与 internal/notify/feishu.go 加签算法一致）
# 用法: ./scripts/feishu-test.sh [消息内容]
#       ./scripts/feishu-test.sh --no-sign [消息内容]  # 无签名（需在飞书里关闭签名校验）
# 配置: 可从 configs/config.yaml 的 notify.webhook_url、notify.signing_secret 同步

WEBHOOK="${FEISHU_WEBHOOK:-https://open.feishu.cn/open-apis/bot/v2/hook/6223fc9e-58c6-4526-b463-140820b5e7c9}"
SECRET="${FEISHU_SECRET:-HOyyTFJVwq05KjGwFR5isc}"

if [[ "$1" == "--no-sign" ]]; then
  shift
  MSG="${1:-【测试消息】这是一条来自 stress 压测系统的测试通知}"
  MSG_ESC=$(echo "$MSG" | sed 's/"/\\"/g')
  echo "发送中（无签名，需在飞书机器人设置中关闭签名校验）..."
  curl -s -X POST "$WEBHOOK" \
    -H "Content-Type: application/json" \
    -d "{\"msg_type\":\"text\",\"content\":{\"text\":\"$MSG_ESC\"}}"
  echo ""
  exit 0
fi

MSG="${1:-【测试消息】这是一条来自 stress 压测系统的测试通知}"

# 1. 生成签名 (飞书算法: HMAC-SHA256(key=timestamp+\n+secret, message=""))
TIMESTAMP=$(date +%s)
KEY=$(printf '%s\n%s' "$TIMESTAMP" "$SECRET")
SIGN=$(printf '' | openssl dgst -sha256 -hmac "$KEY" -binary | base64)

# 2. 发送请求
MSG_ESC=$(echo "$MSG" | sed 's/"/\\"/g')
BODY="{\"timestamp\":\"$TIMESTAMP\",\"sign\":\"$SIGN\",\"msg_type\":\"text\",\"content\":{\"text\":\"$MSG_ESC\"}}"

echo "发送中 (timestamp=$TIMESTAMP)..."
curl -s -X POST "$WEBHOOK" \
  -H "Content-Type: application/json" \
  -d "$BODY"
echo ""
