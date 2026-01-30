#!/bin/bash

echo "=== 监控系统故障排除 ==="
echo ""

# 检查 Docker 状态
echo "1. 检查 Docker 状态..."
if ! docker info >/dev/null 2>&1; then
    echo "✗ Docker 未运行"
    echo "  请启动 Docker Desktop 或 Docker 服务"
    exit 1
else
    echo "✓ Docker 运行正常"
fi

# 检查服务状态
echo ""
echo "2. 检查服务状态..."
docker-compose ps

echo ""
echo "3. 检查服务日志..."

# 检查每个服务的日志
for service in prometheus grafana node-exporter mysql-exporter redis-exporter rabbitmq-exporter; do
    echo ""
    echo "--- $service 最近10行日志 ---"
    docker-compose logs --tail=10 $service 2>/dev/null || echo "无日志或服务不存在"
done

echo ""
echo "4. 网络连通性测试..."
echo "测试 MySQL (192.168.10.83:3306):"
if timeout 5 bash -c "</dev/tcp/192.168.10.83/3306" 2>/dev/null; then
    echo "✓ 连接成功"
else
    echo "✗ 连接失败"
fi

echo "测试 Redis (192.168.10.83:7000):"
if timeout 5 bash -c "</dev/tcp/192.168.10.83/7000" 2>/dev/null; then
    echo "✓ 连接成功"
else
    echo "✗ 连接失败"
fi

echo "测试 RabbitMQ (192.168.10.83:15672):"
if timeout 5 bash -c "</dev/tcp/192.168.10.83/15672" 2>/dev/null; then
    echo "✓ 连接成功"
else
    echo "✗ 连接失败"
fi

echo ""
echo "=== 排除完成 ==="
echo ""
echo "常见解决方案："
echo "1. MySQL Exporter 重启："
echo "   - 检查 MySQL 用户权限: ./test-mysql.sh"
echo "   - 检查防火墙和网络连通性"
echo ""
echo "2. Redis Exporter 重启："
echo "   - 确保 Redis 集群正常运行"
echo "   - 检查 Redis metrics 功能是否开启"
echo ""
echo "3. RabbitMQ Exporter 重启："
echo "   - 检查 RabbitMQ 管理界面: ./test-rabbitmq.sh"
echo "   - 验证 admin 用户权限"
echo ""
echo "4. 重启所有服务："
echo "   docker-compose down"
echo "   docker-compose up -d"