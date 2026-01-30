package data

import (
	"context"
	"fmt"
	"time"

	"stress/internal/biz/chart"
	"stress/internal/biz/task"

	"github.com/go-kratos/kratos/v2/log"
	"xorm.io/xorm"
)

const (
	orderUnit  = 1e4
	timeLayout = "2006-01-02 15:04:05"
)

func (r *dataRepo) QueryGameOrderPoints(ctx context.Context, scope task.OrderScope) ([]chart.Point, error) {
	orderDB, err := r.orderEngine()
	if err != nil {
		return nil, err
	}
	if scope.GameID == 0 || scope.Merchant == "" {
		return nil, fmt.Errorf("game_id and merchant are required")
	}

	start := time.Now()
	excludeAmount := scope.ExcludeAmt
	if excludeAmount <= 0 {
		excludeAmount = 0.01
	}

	const sampleMax = 5000

	type record struct {
		Amount      float64 `xorm:"amount"`
		BonusAmount float64 `xorm:"bonus_amount"`
		CreatedAt   int64   `xorm:"created_at"`
		ID          int64   `xorm:"id"`
	}

	// 获取总订单数
	totalOrders, err := r.getTotalOrders(orderDB, scope, excludeAmount)
	if err != nil {
		return nil, fmt.Errorf("get total orders failed: %w", err)
	}
	if totalOrders == 0 {
		return []chart.Point{}, nil
	}

	// 根据总订单数动态调整批次大小
	batchSize := 500000
	if totalOrders >= 10000000 {
		batchSize = 2000000
	} else if totalOrders >= 5000000 {
		batchSize = 1000000
	}

	step := totalOrders / sampleMax
	if step < 1 {
		step = 1
	}

	var (
		cumBet, cumWin   float64
		bet, win         float64
		orders           int64
		lastTime, lastID int64
		batchCnt         int
		hasPendingData   bool
		lastPointTime    int64
		// 明确保存第一个点的所有信息
		firstPointOrders int64
		firstPointTime   int64
		firstPointCumBet float64
		firstPointCumWin float64
		firstPointSaved  bool
	)

	sampledPts := make([]chart.Point, 0, sampleMax+2)

	// flush 函数：处理一个完整的数据点
	flush := func(pointOrders int64, pointTime int64, isLast bool) {
		if !hasPendingData {
			return
		}

		// 累积到cumBet/cumWin
		cumBet += bet
		cumWin += win

		rate := 0.0
		if cumBet > 0 {
			rate = (cumBet - cumWin) / cumBet
		}

		// 保存第一个点的完整信息（无论是否被采样）
		if !firstPointSaved && pointOrders > 0 {
			firstPointOrders = pointOrders
			firstPointTime = pointTime
			firstPointCumBet = cumBet
			firstPointCumWin = cumWin
			firstPointSaved = true
		}

		// 采样条件：第一个点、最后一个点、等距点
		if pointOrders == 1 || isLast || (pointOrders-1)%step == 0 {
			sampledPts = append(sampledPts, chart.Point{
				X:    float64(pointOrders) / orderUnit,
				Y:    rate,
				Time: time.Unix(pointTime, 0).In(time.Local).Format(timeLayout),
			})
		}

		// 重置当前累积
		bet = 0
		win = 0
		hasPendingData = false
	}

	for {
		batchCnt++
		query := `
			SELECT amount, bonus_amount, created_at, id
			FROM game_order
			WHERE game_id = ?
			  AND merchant = ?
			  AND amount != ?
			  AND (created_at > ? OR (created_at = ? AND id > ?))
			ORDER BY created_at, id
			LIMIT ?`

		args := []any{scope.GameID, scope.Merchant, excludeAmount, lastTime, lastTime, lastID, batchSize}

		var batch []record
		if err := orderDB.SQL(query, args...).Find(&batch); err != nil {
			return nil, fmt.Errorf("query failed: %w", err)
		}
		if len(batch) == 0 {
			break
		}

		for _, rec := range batch {
			orders++
			lastTime = rec.CreatedAt
			lastID = rec.ID

			if rec.Amount > 0 {
				// flush上一个完整的数据点
				if hasPendingData {
					flush(orders-1, lastPointTime, false)
				}

				// 开始新的数据点累积
				bet = rec.Amount
				win = rec.BonusAmount
				hasPendingData = true
				lastPointTime = rec.CreatedAt
			} else {
				// 累计到当前数据点
				win += rec.BonusAmount
				if !hasPendingData && win > 0 {
					hasPendingData = true
					lastPointTime = rec.CreatedAt
				}
			}
		}
	}

	// flush最后一个数据点
	if hasPendingData {
		flush(orders, lastPointTime, true)
	}

	// 确保首尾点存在且正确
	sampledPts = ensureEdgePoints(sampledPts, orders,
		firstPointOrders, firstPointTime, firstPointCumBet, firstPointCumWin,
		cumBet, cumWin, lastPointTime)

	// 超出采样点限制均匀截断
	if len(sampledPts) > sampleMax {
		sampledPts = uniformTruncate(sampledPts, sampleMax)
	}

	log.Infof("处理完成: 总订单数=%d, 批次数=%d, 采样点数=%d, 耗时=%v",
		orders, batchCnt, len(sampledPts), time.Since(start))

	return sampledPts, nil
}

// 确保首尾点正确
func ensureEdgePoints(points []chart.Point, totalOrders int64,
	firstOrders int64, firstTime int64, firstCumBet, firstCumWin float64,
	finalCumBet, finalCumWin float64, lastTime int64) []chart.Point {

	if len(points) == 0 || totalOrders <= 0 {
		return points
	}

	// 检查是否已包含第一个点
	hasFirst := false
	if len(points) > 0 && int64(points[0].X*orderUnit) == 1 {
		hasFirst = true
	}

	// 检查是否已包含最后一个点
	hasLast := false
	if len(points) > 0 && int64(points[len(points)-1].X*orderUnit) == totalOrders {
		hasLast = true
	}

	result := make([]chart.Point, 0, len(points)+2)

	// 添加第一个点（如果需要）
	if !hasFirst && firstOrders > 0 {
		firstRate := 0.0
		if firstCumBet > 0 {
			firstRate = (firstCumBet - firstCumWin) / firstCumBet
		}
		result = append(result, chart.Point{
			X:    float64(firstOrders) / orderUnit,
			Y:    firstRate,
			Time: time.Unix(firstTime, 0).In(time.Local).Format(timeLayout),
		})
	}

	// 添加所有原始点
	result = append(result, points...)

	// 添加最后一个点（如果需要）
	if !hasLast {
		lastRate := 0.0
		if finalCumBet > 0 {
			lastRate = (finalCumBet - finalCumWin) / finalCumBet
		}
		result = append(result, chart.Point{
			X:    float64(totalOrders) / orderUnit,
			Y:    lastRate,
			Time: time.Unix(lastTime, 0).In(time.Local).Format(timeLayout),
		})
	}

	return result
}

// uniformTruncate 均匀截断采样点
func uniformTruncate(points []chart.Point, maxPoints int) []chart.Point {
	if len(points) <= maxPoints {
		return points
	}
	n := len(points)
	result := make([]chart.Point, 0, maxPoints)
	result = append(result, points[0])
	step := (n - 1) / (maxPoints - 1)
	if step < 1 {
		step = 1
	}
	for i := step; i <= n-step && len(result) < maxPoints-1; i += step {
		result = append(result, points[i])
	}
	if len(result) < maxPoints {
		result = append(result, points[n-1])
	}
	return result
}

// getTotalOrders 获取总订单数
func (r *dataRepo) getTotalOrders(orderDB *xorm.Engine, scope task.OrderScope, excludeAmount float64) (int64, error) {
	query := `SELECT COUNT(*) FROM game_order WHERE game_id = ? AND merchant = ? AND amount != ?`
	var total int64
	_, err := orderDB.SQL(query, scope.GameID, scope.Merchant, excludeAmount).Get(&total)
	return total, err
}
