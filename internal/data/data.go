package data

import (
	"context"
	"fmt"
	"time"

	"stress/internal/biz"
	"stress/internal/conf"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
	"xorm.io/xorm"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(NewData, NewRedis, NewMysql, NewDataRepo, NewS3Bucket)

type dataRepo struct {
	data *Data
	log  *log.Helper
}

func NewDataRepo(data *Data, logger log.Logger) biz.DataRepo {
	return &dataRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

// Data .
type Data struct {
	db       *xorm.Engine
	order    *xorm.Engine
	rdb      redis.UniversalClient
	s3Bucket *S3Bucket
}

// NewData .
func NewData(c *conf.Data, logger log.Logger, db *xorm.Engine, rdb redis.UniversalClient, s3 *S3Bucket) (*Data, func(), error) {
	l := log.NewHelper(logger)
	order, orderCleanup, err := newMysqlFromConf(c.OrderDatabase, logger, "order")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		l.Info("closing the data resources")
		orderCleanup()
	}
	return &Data{db: db, order: order, rdb: rdb, s3Bucket: s3}, cleanup, nil
}

// NewRedis 创建并配置 Redis 客户端
func NewRedis(c *conf.Data, logger log.Logger) (redis.UniversalClient, func(), error) {
	l := log.NewHelper(logger)

	// 验证配置
	if len(c.Redis.Addr) == 0 {
		return nil, nil, errors.Newf(500, "REDIS_ADDR_REQUIRED", "redis address is required")
	}

	rdb := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:        c.Redis.Addr,
		Password:     c.Redis.Password,
		DB:           int(c.Redis.Db),
		ReadTimeout:  c.Redis.ReadTimeout.AsDuration(),
		WriteTimeout: c.Redis.WriteTimeout.AsDuration(),
		// 连接池配置 - 防止 "connection pool timeout"
		PoolSize:        50,               // 连接池大小（默认 10 * CPU 核心数）
		MinIdleConns:    10,               // 最小空闲连接数
		PoolTimeout:     5 * time.Second,  // 从连接池获取连接的超时时间
		ConnMaxLifetime: 10 * time.Minute, // 连接最大存活时间
		ConnMaxIdleTime: 5 * time.Minute,  // 空闲连接超时时间
		// 集群容错配置 - 防止 "CLUSTERDOWN" 错误
		MaxRetries:      3, // 命令失败最大重试次数
		MinRetryBackoff: 100 * time.Millisecond,
		MaxRetryBackoff: 500 * time.Millisecond,
	})

	// 测试 Redis 连接
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		l.Errorf("failed pinging redis: %v", err)
		return nil, nil, errors.Newf(500, "REDIS_PING_FAILED", "failed pinging redis: %v", err)
	}

	cleanup := func() {
		l.Infof("closing redis connection")
		if err := rdb.Close(); err != nil {
			l.Error(err)
		}
	}

	l.Info("Redis connection established successfully")
	return rdb, cleanup, nil
}

// NewMysql 创建默认库 MySQL 连接
func NewMysql(c *conf.Data, logger log.Logger) (*xorm.Engine, func(), error) {
	if c == nil || c.Database == nil {
		return nil, nil, errors.Newf(500, "DATA_CONFIG_REQUIRED", "data config is required")
	}
	return newMysqlFromConf(c.Database, logger, "default")
}

// newMysqlFromConf 创建并配置 MySQL 数据库连接
func newMysqlFromConf(c *conf.Data_Database, logger log.Logger, label string) (*xorm.Engine, func(), error) {
	l := log.NewHelper(logger)
	if c == nil {
		return nil, func() {}, nil
	}
	db, err := xorm.NewEngine(c.Driver, c.Source)
	if err != nil {
		l.Errorf("failed opening %s db: %v", label, err)
		return nil, nil, errors.Newf(500, "DB_OPEN_FAILED", "failed opening %s db: %v", label, err)
	}

	// 设置连接池参数
	db.SetMaxIdleConns(defaultInt(c.MaxIdleConns, 5))
	db.SetMaxOpenConns(defaultInt(c.MaxOpenConns, 30))
	if db.DB() != nil {
		db.DB().SetConnMaxLifetime(3 * time.Minute)
		db.DB().SetConnMaxIdleTime(1 * time.Minute)
	}

	if err := db.Ping(); err != nil {
		l.Errorf("failed pinging, db=%q, err=%v", label, err)
		_ = db.Close()
		return nil, nil, errors.Newf(500, "DB_PING_FAILED", "failed pinging db=%q: %v", label, err)
	}
	cleanup := func() {
		l.Infof("closing mysql connection. db=%q", label)
		if err := db.Close(); err != nil {
			l.Error(err)
		}
	}
	l.Infof("MySQL connection established successfully. db=%q", label)
	return db, cleanup, nil
}

// defaultInt 返回配置值或默认值
func defaultInt(value int32, defaultValue int) int {
	if v := int(value); v > 0 {
		return v
	}
	return defaultValue
}

func (r *dataRepo) orderEngine() (*xorm.Engine, error) {
	if r.data.order == nil {
		return nil, fmt.Errorf("order database not configured")
	}
	return r.data.order, nil
}
