package data

import (
	"context"
	"fmt"
	"strconv"
	"stress/internal/biz"
	"time"

	"stress/internal/conf"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
	"xorm.io/xorm"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(NewData, NewRedis, NewMysql, NewDataRepo)

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
	db    *xorm.Engine
	order *xorm.Engine
	rdb   redis.UniversalClient
}

// NewData .
func NewData(c *conf.Data, logger log.Logger, db *xorm.Engine, rdb redis.UniversalClient) (*Data, func(), error) {
	l := log.NewHelper(logger)
	order, orderCleanup, err := newMysqlFromConf(c.OrderDatabase, logger, "order")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		l.Info("closing the data resources")
		orderCleanup()
	}
	return &Data{db: db, order: order, rdb: rdb}, cleanup, nil
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
	maxIdleConns := int(c.MaxIdleConns)
	if maxIdleConns <= 0 {
		maxIdleConns = 10
	}
	db.SetMaxIdleConns(maxIdleConns)
	maxOpenConns := int(c.MaxOpenConns)
	if maxOpenConns <= 0 {
		maxOpenConns = 100
	}
	db.SetMaxOpenConns(maxOpenConns)
	if db.DB() != nil {
		db.DB().SetConnMaxLifetime(5 * time.Minute)
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

// NextTaskID 实现 DataRepo：Redis Hash stress-pool:count:YYYY-MM-DD，field=gameID，过期为次日 0 点
func (r *dataRepo) NextTaskID(ctx context.Context, gameID int64) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	now := time.Now()
	date := now.Format("20060102")
	key := fmt.Sprintf("stress-pool:count:%s", date)
	field := strconv.FormatInt(gameID, 10)

	count, err := r.data.rdb.HIncrBy(ctx, key, field, 1).Result()
	if err != nil {
		return "", errors.Newf(500, "REDIS_COUNTER_FAILED", "redis counter: %v", err)
	}

	if count == 1 {
		tomorrow := now.AddDate(0, 0, 1)
		midnight := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, now.Location())
		_ = r.data.rdb.ExpireAt(ctx, key, midnight).Err()
	}

	return fmt.Sprintf("%s-%d-%d", date, gameID, count), nil
}
