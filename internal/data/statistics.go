package data

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"stress/internal/biz"
	"stress/internal/biz/stats/statistics"

	"github.com/go-kratos/kratos/v2/log"
)

const (
	orderUnit  = 1e4
	timeLayout = "2006-01-02 15:04:05"
)

var locSH, _ = time.LoadLocation("Asia/Shanghai")

func (r *dataRepo) QueryGameOrderPoints(ctx context.Context, scope biz.OrderScope) ([]statistics.Point, error) {
	if r.data.order == nil {
		return nil, fmt.Errorf("order database not configured")
	}

	if scope.GameID == 0 || scope.Merchant == "" {
		return nil, fmt.Errorf("game_id and merchant are required")
	}

	log.Debugf("QueryGameOrderPoints: %v", scope)

	excludeAmount := scope.ExcludeAmt
	if excludeAmount <= 0 {
		excludeAmount = 0.01
	}

	const batchSize = 500000
	const sampleMax = 5000

	type record struct {
		Amount      float64 `xorm:"amount"`
		BonusAmount float64 `xorm:"bonus_amount"`
		CreatedAt   int64   `xorm:"created_at"`
		ID          int64   `xorm:"id"`
	}

	var (
		cumBet       float64
		cumWin       float64
		orders       int64
		lastTime     int64
		lastID       int64
		totalBatches int
	)

	// ====== 核心：使用 Reservoir Sampling ======
	reservoir := make([]statistics.Point, 0, sampleMax)

	// 随机种子
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

	start := time.Now()

	for {
		totalBatches++

		// 改为更稳妥的分页条件
		query := `
			SELECT amount, bonus_amount, created_at, id 
			FROM game_order 
			WHERE game_id = ? 
			  AND merchant = ? 
			  AND amount != ? 
			  AND (created_at > ? OR (created_at = ? AND id > ?))
			ORDER BY created_at, id 
			LIMIT ?`

		args := []any{
			scope.GameID,
			scope.Merchant,
			excludeAmount,
			lastTime,
			lastTime,
			lastID,
			batchSize,
		}

		var batch []record
		if err := r.data.order.SQL(query, args...).Find(&batch); err != nil {
			log.Errorf("查询第 %d 批订单失败: %v", totalBatches, err)
			return nil, fmt.Errorf("query failed: %w", err)
		}

		if len(batch) == 0 {
			break
		}

		for _, rec := range batch {
			orders++

			lastTime = rec.CreatedAt
			lastID = rec.ID

			cumBet += rec.Amount
			cumWin += rec.BonusAmount

			rate := 0.0
			if cumBet > 0 {
				rate = (cumBet - cumWin) / cumBet
			}

			// 这里先不做时间格式化，只保存时间戳
			point := statistics.Point{
				X:    float64(orders) / orderUnit,
				Y:    rate,
				Time: fmt.Sprintf("%d", rec.CreatedAt),
			}

			// ===== 蓄水池采样 =====
			if len(reservoir) < sampleMax {
				reservoir = append(reservoir, point)
			} else {
				r := rnd.Int63n(orders)
				if r < sampleMax {
					reservoir[r] = point
				}
			}
		}
	}

	log.Infof("流式处理完成: 总订单数=%d, 批次数=%d, 耗时=%v",
		orders, totalBatches, time.Since(start))

	// ===== 最后只对 5000 条做时间格式化 =====
	for i := range reservoir {
		ts, err := strconv.ParseInt(reservoir[i].Time, 10, 64)
		if err == nil {
			reservoir[i].Time = time.Unix(ts, 0).
				In(locSH).
				Format(timeLayout)
		}
	}

	return reservoir, nil
}
