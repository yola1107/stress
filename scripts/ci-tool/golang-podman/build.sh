#!/bin/bash
set -e

# ========= 配置区 =========
HARBOR="192.168.10.67"
PROJECT="egame"
IMAGE_NAME="ci-tools"
TAG="go1.24.5-podman"

FULL_IMAGE="${HARBOR}/${PROJECT}/${IMAGE_NAME}:${TAG}"

HARBOR_USER="admin"
HARBOR_PASS="P8jF3sH6vQ1yL5nT"
# =========================

echo "📦 构建 CI 工具镜像: ${FULL_IMAGE}"

# 构建镜像
podman build -t ${FULL_IMAGE} .

echo "🔐 登录 Harbor ${HARBOR}"
podman login --tls-verify=false -u "${HARBOR_USER}" -p "${HARBOR_PASS}" "${HARBOR}"

echo "🚀 推送镜像到 Harbor"
podman push --tls-verify=false "${FULL_IMAGE}"

echo "✅ CI 工具镜像推送完成"
echo "👉 ${FULL_IMAGE}"

~