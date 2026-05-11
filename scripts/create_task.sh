#!/usr/bin/env sh
# 调用 CreateTask 创建压测任务
# 用法: ./scripts/create_task.sh
# 可选: STRESS_URL=http://127.0.0.1:8001 ./scripts/create_task.sh

set -eu

BASE="${STRESS_URL:-http://127.0.0.1:8001}"
URL="${BASE%/}/stress/CreateTask"

curl -sS -X POST "$URL" \
  -H 'Content-Type: application/json' \
  -d '{
    "config": {
        "game_id": 18915,
        "member_count": 1000,
        "times_per_member": 5000,
        "bet_order": {
            "base_money": 0.1,
            "multiple": 1,
            "purchase": 0,
            "bonus_num": []
        }
    }
}'
printf '\n'
