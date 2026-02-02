#!/bin/bash

echo "=== 监控系统启动 ==="
echo ""

# 检查网络连通性
echo "1. 检查网络连通性..."
if ! timeout 5 bash -c "</dev/tcp/192.168.10.83/3306" 2>/dev/null; then
    echo "⚠️  警告: 无法连接到 MySQL 服务器 (192.168.10.83:3306)"
    echo "   MySQL Exporter 可能无法正常工作"
fi

if ! timeout 5 bash -c "</dev/tcp/192.168.10.83/7000" 2>/dev/null; then
    echo "⚠️  警告: 无法连接到 Redis 集群 (192.168.10.83:7000)"
    echo "   Redis Exporter 可能无法正常工作"
fi

if ! timeout 5 bash -c "</dev/tcp/192.168.10.83/15672" 2>/dev/null; then
    echo "⚠️  警告: 无法连接到 RabbitMQ 管理界面 (192.168.10.83:15672)"
    echo "   RabbitMQ Exporter 可能无法正常工作"
fi

echo ""

# 停止现有服务
echo "2. 停止现有服务..."
docker-compose down

# 启动服务
echo "3. 启动监控服务..."
docker-compose up -d

# 等待服务启动
echo "4. 等待服务启动..."
sleep 10

# 检查服务状态
echo "5. 检查服务状态..."
docker-compose ps

echo ""
echo "6. 检查服务健康状态..."
for service in prometheus grafana node-exporter mysql-exporter redis-exporter rabbitmq-exporter; do
    status=$(docker-compose ps -q $service | xargs docker inspect --format='{{.State.Status}}' 2>/dev/null)
    if [ "$status" = "running" ]; then
        echo "✓ $service: 运行中"
    else
        echo "✗ $service: 未运行 (状态: $status)"
    fi
done

echo ""
echo "=== 启动完成 ==="
echo "Prometheus: http://localhost:9090"
echo "Grafana: http://localhost:3000 (admin/admin123)"
echo ""
echo "监控目标："
echo "- Redis 集群: 192.168.10.83:7000-7005"
echo "- MySQL 数据库: 192.168.10.83:3306"
echo "- RabbitMQ 集群: 192.168.10.83:15672"
echo ""
echo "如遇问题，请运行: docker-compose logs [服务名] 查看详细日志"