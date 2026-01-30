package data

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
)

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
