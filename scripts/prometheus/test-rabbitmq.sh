#!/bin/bash

echo "=== RabbitMQ 连接测试 ==="
echo ""

RABBITMQ_HOST="192.168.10.83"
RABBITMQ_PORT="15672"
RABBITMQ_USER="admin"
RABBITMQ_PASS="admin"

# 测试管理界面端口连通性
echo "1. 测试管理界面连通性..."
if timeout 10 bash -c "</dev/tcp/${RABBITMQ_HOST}/${RABBITMQ_PORT}" 2>/dev/null; then
    echo "✓ 管理界面连接成功 ($RABBITMQ_HOST:$RABBITMQ_PORT)"
else
    echo "✗ 管理界面连接失败"
    echo "  请检查："
    echo "  - RabbitMQ 集群是否运行"
    echo "  - 管理界面端口是否正确 (默认15672)"
    echo "  - 防火墙设置"
    exit 1
fi

# 测试 API 访问
echo ""
echo "2. 测试 API 访问..."
if command -v curl >/dev/null 2>&1; then
    # 测试基本 API 访问
    response=$(curl -s -o /dev/null -w "%{http_code}" -u "$RABBITMQ_USER:$RABBITMQ_PASS" \
        "http://$RABBITMQ_HOST:$RABBITMQ_PORT/api/overview")
    
    if [ "$response" = "200" ]; then
        echo "✓ API 访问成功"
    else
        echo "✗ API 访问失败 (HTTP $response)"
        echo "  请检查："
        echo "  - 用户名密码是否正确"
        echo "  - 用户是否有管理权限"
    fi
    
    # 测试集群状态
    nodes_response=$(curl -s -o /dev/null -w "%{http_code}" -u "$RABBITMQ_USER:$RABBITMQ_PASS" \
        "http://$RABBITMQ_HOST:$RABBITMQ_PORT/api/nodes")
    
    if [ "$nodes_response" = "200" ]; then
        echo "✓ 集群节点信息获取成功"
        
        # 获取集群节点数量
        nodes_count=$(curl -s -u "$RABBITMQ_USER:$RABBITMQ_PASS" \
            "http://$RABBITMQ_HOST:$RABBITMQ_PORT/api/nodes" | jq '. | length' 2>/dev/null || echo "N/A")
        echo "  集群节点数: $nodes_count"
    else
        echo "✗ 集群节点信息获取失败 (HTTP $nodes_response)"
    fi
else
    echo "⚠ curl 命令未安装，跳过 API 测试"
fi

echo ""
echo "=== 测试完成 ==="
echo ""
echo "如果测试通过，RabbitMQ Exporter 应该可以正常工作"
echo "RabbitMQ Exporter 配置："
echo "- 管理界面: http://$RABBITMQ_HOST:$RABBITMQ_PORT"
echo "- 用户: $RABBITMQ_USER"
echo "- Exporter 端口: 9419"
echo ""
echo "如遇问题，请运行："
echo "docker-compose logs rabbitmq-exporter"