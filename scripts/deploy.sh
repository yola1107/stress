#!/usr/bin/env bash
set -e

HOST="${DEPLOY_HOST:-192.168.10.72}"
USER="${DEPLOY_USER:-aa}"
PASS="${DEPLOY_PASS:-ABC123}"
DIR="${DEPLOY_DIR:-/data/demo}"
BINARY_PATH="${1:-./stress}"
CONFIG_DIR="${CONFIG_DIR:-./configs}"
SSH=(sshpass -p "${PASS}" ssh -o StrictHostKeyChecking=no "${USER}@${HOST}")
SCP=(sshpass -p "${PASS}" scp -o StrictHostKeyChecking=no)

[ -f "${BINARY_PATH}" ] || { echo "错误: 找不到二进制文件 ${BINARY_PATH}"; echo "请先执行: make build"; exit 1; }

echo "上传 stress 到 ${USER}@${HOST}:${DIR}"
"${SSH[@]}" mkdir -p "${DIR}/bin" "${DIR}/configs"

if "${SSH[@]}" pgrep -f stress >/dev/null 2>&1; then
    echo "停止运行中的 stress 进程..."
    "${SSH[@]}" sudo pkill -ef stress || true
    sleep 2
fi

echo "上传二进制文件..."
"${SCP[@]}" "${BINARY_PATH}" "${USER}@${HOST}:${DIR}/bin/"

if [ -d "${CONFIG_DIR}" ] && [ -n "$(ls -A "${CONFIG_DIR}" 2>/dev/null)" ]; then
    echo "上传配置文件..."
    "${SCP[@]}" -r "${CONFIG_DIR}/" "${USER}@${HOST}:${DIR}/configs/"
fi

"${SSH[@]}" chmod +x "${DIR}/bin/stress" || true
echo "上传完成！路径: ${DIR}/bin/stress"

