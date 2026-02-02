-- MySQL 监控用户创建脚本
-- 在 MySQL 服务器 (192.168.10.83) 上执行此脚本

-- 创建监控用户
CREATE USER IF NOT EXISTS 'exporter'@'%' IDENTIFIED BY 'exporter123';

-- 授予必要权限
GRANT PROCESS, REPLICATION CLIENT, SELECT ON *.* TO 'exporter'@'%';

-- 刷新权限
FLUSH PRIVILEGES;

-- 验证用户是否创建成功
SELECT User, Host FROM mysql.user WHERE User = 'exporter';

-- 显示用户的权限
SHOW GRANTS FOR 'exporter'@'%';