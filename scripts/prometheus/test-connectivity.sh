#!/bin/bash

echo "================================"
echo "测试网络连通性"
echo "================================"

# 测试 Redis 集群连通性
echo "1. 测试 Redis 集群连通性 (192.168.10.83:7000-7005)"
for port in 7000 7001 7002 7003 7004 7005; do
    echo -n "测试 192.168.10.83:$port ... "
    if timeout 3 bash -c "</dev/tcp/192.168.10.83/$port"; then
        echo "✓ 连接成功"
    else
        echo "✗ 连接失败"
    fi
done

echo ""

# 测试 MySQL 连通性
echo "2. 测试 MySQL 连通性 (192.168.10.83:3306)"
echo -n "测试 192.168.10.83:3306 ... "
if timeout 3 bash -c "</dev/tcp/192.168.10.83/3306"; then
    echo "✓ 连接成功"
else
    echo "✗ 连接失败"
fi

echo ""

# 测试 MySQL 用户连接
echo "3. 测试 MySQL 监控用户连接"
echo "测试 exporter 用户连接..."
if command -v mysql >/dev/null 2>&1; then
    if mysql -h 192.168.10.83 -P 3306 -u exporter -pexporter123 -e "SELECT 1;" >/dev/null 2>&1; then
        echo "✓ MySQL 用户连接成功"
    else
        echo "✗ MySQL 用户连接失败，请检查用户权限或密码"
    fi
else
    echo "⚠ MySQL 客户端未安装，跳过用户连接测试"
fi

echo ""
echo "================================"
echo "连通性测试完成"
echo "================================"