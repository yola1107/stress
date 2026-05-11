#!/usr/bin/env bash
# 一键生成新款游戏 scaffold（目录 + stub.proto + game.go + registry）
# stub.pb.go 与 stub.proto 均由 templates 渲染/描述符生成（无需本机 protoc）
#
# 默认 GAME_ID / GAME_NAME 仅在下方两行维护（勿在 Makefile 再写一遍）。
#
# 用法:
#   ./scripts/gen-game/gen-game.sh
#   ./scripts/gen-game/gen-game.sh 19005 '埃及探秘'
#   GAME_NAME='带空格的名称' ./scripts/gen-game/gen-game.sh 999999
#   make game
#   make game id=19005 name='埃及探秘'
#
# 优先级: 位置参数 $1/$2（若给定）覆盖环境变量再覆盖默认值。
# 需在仓库根目录执行。

set -eu

GAME_ID="${GAME_ID:-999999}"
GAME_NAME="${GAME_NAME:-脚手架占位}"
if [ "${1:+x}" ]; then GAME_ID="$1"; fi
if [ "${2:+x}" ]; then GAME_NAME="$2"; fi

ROOT=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
cd "$ROOT"

case "$GAME_ID" in ''|*[!0-9]*)
	echo "GAME_ID 须为正整数" >&2
	exit 2
esac

exec env GAME_ID="$GAME_ID" GAME_NAME="$GAME_NAME" go run ./scripts/gen-game
