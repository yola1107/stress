#!/bin/bash
set -e

# === GitLab Runner 批量注册脚本 ===
# 用法:
# ./register-runner.sh <gitlab-url> <runner-name> <runner-token> <config-dir> [tags]

if [ $# -lt 4 ]; then
    echo "用法: $0 <gitlab-url> <runner-name> <runner-token> <config-dir> [tags]"
    exit 1
fi

GITLAB_URL=$1
RUNNER_NAME=$2
RUNNER_TOKEN=$3
CONFIG_DIR=$4
TAGS=${5:-"docker"}

# 创建独立配置目录
mkdir -p "$CONFIG_DIR"

# 幂等检查：如果 config.toml 已经存在则跳过
if [ -f "$CONFIG_DIR/config.toml" ]; then
    echo "Runner '$RUNNER_NAME' 已经注册，跳过。"
    exit 0
fi

echo "注册 Runner '$RUNNER_NAME' 到 GitLab..."

docker run --rm \
    -v "$CONFIG_DIR:/etc/gitlab-runner" \
    gitlab/gitlab-runner:latest register \
    --non-interactive \
    --url "$GITLAB_URL" \
    --registration-token "$RUNNER_TOKEN" \
    --name "$RUNNER_NAME" \
    --executor docker \
    --docker-image docker:latest \
    --docker-privileged true \
    --docker-volumes /var/run/docker.sock:/var/run/docker.sock \
    --tag-list "$TAGS" \
    --run-untagged=false \
    --locked=false

echo "Runner '$RUNNER_NAME' 注册完成，配置目录：$CONFIG_DIR"