package data

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"
)

const (
	scanCount = 10000 // 每轮 SCAN 建议返回数量（Redis 可能多返回）
	pipeBatch = 1000  // 每批 Pipeline DEL 数量，平衡 RTT 与单次请求大小
)

// pipeliner 用于 SCAN + Pipeline DEL（同一节点上批量删，减少 RTT）
type pipeliner interface {
	Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd
	Pipeline() redis.Pipeliner
}

// scanAndDelete 在单个 client 上 SCAN，用 Pipeline 分批 DEL，返回本节点删除数量
func (r *dataRepo) scanAndDelete(ctx context.Context, pattern string, client pipeliner) (int, error) {
	cursor := uint64(0)
	totalDeleted := 0

	for {
		// 检查上下文取消
		select {
		case <-ctx.Done():
			return totalDeleted, ctx.Err()
		default:
		}

		keys, cursor, err := client.Scan(ctx, cursor, pattern, scanCount).Result()
		if err != nil {
			return totalDeleted, fmt.Errorf("scan failed: %w", err)
		}

		// 批量删除 keys
		for i := 0; i < len(keys); i += pipeBatch {
			select {
			case <-ctx.Done():
				return totalDeleted, ctx.Err()
			default:
			}

			batch := keys[i:min(i+pipeBatch, len(keys))]
			pipe := client.Pipeline()
			for _, key := range batch {
				pipe.Del(ctx, key)
			}

			cmds, err := pipe.Exec(ctx)
			if err != nil {
				return totalDeleted, fmt.Errorf("pipeline del: %w", err)
			}

			for _, cmd := range cmds {
				if n, ok := cmd.(*redis.IntCmd); ok {
					if v, err := n.Result(); err == nil {
						totalDeleted += int(v)
					}
				}
			}
		}

		if cursor == 0 {
			break
		}
	}

	return totalDeleted, nil
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// CleanRedisBySite 清理 Redis 中 site:* 的键
func (r *dataRepo) CleanRedisBySite(ctx context.Context, site, _ string) error {
	pattern := site + ":*"
	rdb := r.data.rdb
	totalDeleted := 0

	var err error
	switch client := rdb.(type) {
	case *redis.ClusterClient:
		// 集群模式：对每个 master 节点执行清理
		err = client.ForEachMaster(ctx, func(ctx context.Context, node *redis.Client) error {
			n, err := r.scanAndDelete(ctx, pattern, node)
			totalDeleted += n
			return err
		})
	case pipeliner:
		// 单机/哨兵模式
		totalDeleted, err = r.scanAndDelete(ctx, pattern, client)
	default:
		return fmt.Errorf("unsupported redis client type: %T", rdb)
	}

	if err != nil {
		return fmt.Errorf("cleanup failed for pattern %s: %w", pattern, err)
	}

	if totalDeleted > 0 {
		r.log.Infof("Cleaned %d keys for site: %s", totalDeleted, site)
	}
	return nil
}

// CleanRedisBySites 批量清理多个 sites 的 Redis 键
func (r *dataRepo) CleanRedisBySites(ctx context.Context, sites []string) error {
	if len(sites) == 0 {
		return nil
	}

	r.log.Infof("Cleaning Redis for %d sites: %v", len(sites), sites)

	// 使用 errgroup 进行并发控制
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(10) // 限制并发数量

	for _, site := range sites {
		site := site
		g.Go(func() error {
			return r.CleanRedisBySite(gctx, site, "")
		})
	}

	return g.Wait()
}

// CleanGameOrderTable 清空 egame_order.game_order
func (r *dataRepo) CleanGameOrderTable(ctx context.Context) error {
	orderDB := r.data.order
	if orderDB == nil {
		return fmt.Errorf("order database not configured")
	}

	// 获取清理前的记录数
	if count, err := r.GetGameOrderCount(ctx); err == nil && count > 0 {
		r.log.Infof("Truncating game_order table with %d records", count)
	}

	// 执行 TRUNCATE TABLE
	if _, err := orderDB.Context(ctx).Exec("TRUNCATE TABLE game_order"); err != nil {
		return fmt.Errorf("failed to truncate game_order table: %w", err)
	}

	// 验证清理结果
	if finalCount, err := r.GetGameOrderCount(ctx); err != nil {
		r.log.Warnf("Failed to verify cleanup: %v", err)
	} else if finalCount > 0 {
		r.log.Warnf("Table still has %d records after truncate", finalCount)
	} else {
		r.log.Info("Game order table truncated successfully")
	}

	return nil
}

// GetGameOrderCount 获取 game_order 表记录数
func (r *dataRepo) GetGameOrderCount(ctx context.Context) (int64, error) {
	orderDB := r.data.order
	if orderDB == nil {
		return 0, fmt.Errorf("order database not configured")
	}

	var count int64
	_, err := orderDB.Context(ctx).SQL("SELECT COUNT(*) FROM game_order").Get(&count)
	return count, err
}

// GetDetailedOrderAmounts 查询详细的订单统计信息（总下注/总奖金/下注订单数/奖励订单数）
func (r *dataRepo) GetDetailedOrderAmounts(ctx context.Context) (totalBet, totalWin, betOrderCount, bonusOrderCount int64, err error) {
	if r.data.order == nil {
		return 0, 0, 0, 0, fmt.Errorf("order database not configured")
	}
	var result struct {
		TotalBet        int64 `xorm:"total_bet"`
		TotalWin        int64 `xorm:"total_win"`
		BetOrderCount   int64 `xorm:"bet_order_count"`
		BonusOrderCount int64 `xorm:"bonus_order_count"`
	}
	// amount/bonus_amount 为 decimal(16,4)，*10000 转为整型
	// 通过 bonus_amount > 0 判断是否为奖励订单
	_, err = r.data.order.Context(ctx).SQL(`
		SELECT
			COALESCE(ROUND(SUM(amount)*10000), 0) as total_bet,
			COALESCE(ROUND(SUM(bonus_amount)*10000), 0) as total_win,
			COUNT(*) as bet_order_count,
			COALESCE(SUM(CASE WHEN bonus_amount > 0 THEN 1 ELSE 0 END), 0) as bonus_order_count
		FROM game_order
	`).Get(&result)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("query detailed order amounts: %w", err)
	}
	return result.TotalBet, result.TotalWin, result.BetOrderCount, result.BonusOrderCount, nil
}
