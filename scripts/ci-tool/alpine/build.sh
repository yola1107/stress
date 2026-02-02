# 构建并推送部署客户端工具镜像到 Harbor

set -e

# 配置变量
REPO_HARBOR="192.168.10.67"
REPO_HARBOR_USER="admin"
REPO_HARBOR_PASS="P8jF3sH6vQ1yL5nT"
REPO_PATH="egame"
IMAGE_NAME="alpine"
IMAGE_TAG="${1:-latest}"  # 默认使用 latest，可以传入自定义 tag

FULL_IMAGE_NAME="${REPO_HARBOR}/${REPO_PATH}/${IMAGE_NAME}:${IMAGE_TAG}"

echo "=========================================="
echo "构建部署客户端工具镜像"
echo "=========================================="
echo "Harbor: ${REPO_HARBOR}"
echo "镜像: ${FULL_IMAGE_NAME}"
echo "Dockerfile: Dockerfile.alpine-client"
echo "=========================================="

# 登录 Harbor
echo "正在登录 Harbor..."
echo "${REPO_HARBOR_PASS}" | docker login ${REPO_HARBOR} \
    -u "${REPO_HARBOR_USER}" \
    --password-stdin

# 构建镜像
echo "正在构建镜像..."
docker build \
    -f Dockerfile.alpine-client \
    -t ${FULL_IMAGE_NAME} \
    --platform linux/amd64 \
    .

# 推送镜像
echo "正在推送镜像到 Harbor..."
docker push ${FULL_IMAGE_NAME}

echo "=========================================="
echo "✅ 镜像构建和推送完成！"
echo "镜像地址: ${FULL_IMAGE_NAME}"
echo "=========================================="

# 可选：验证镜像
echo "验证镜像信息..."
docker image inspect ${FULL_IMAGE_NAME} --format '{{.Size}} bytes'

echo "完成！"