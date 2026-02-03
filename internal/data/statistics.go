package data

import (
	"context"
	"fmt"
	"math"
	"strings"
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

	start := time.Now()
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

	// 获取总订单数
	totalOrders, err := r.getTotalOrders(scope, excludeAmount)
	if err != nil {
		return nil, fmt.Errorf("get total orders failed: %w", err)
	}
	if totalOrders == 0 {
		return []statistics.Point{}, nil
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

	sampledPts := make([]statistics.Point, 0, sampleMax+2)

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
			sampledPts = append(sampledPts, statistics.Point{
				X:    float64(pointOrders) / orderUnit,
				Y:    rate,
				Time: time.Unix(pointTime, 0).In(locSH).Format(timeLayout),
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
		if err := r.data.order.SQL(query, args...).Find(&batch); err != nil {
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

	// 最终验证
	if false {
		if len(sampledPts) > 0 {
			if err := validateSampling(sampledPts, orders, step); err != nil {
				log.Warnf("采样验证警告: %v", err)
			}
		}
	}

	log.Infof("处理完成: 总订单数=%d, 批次数=%d, 采样点数=%d, 耗时=%v",
		orders, batchCnt, len(sampledPts), time.Since(start))

	return sampledPts, nil
}

// 确保首尾点正确（简化版）
func ensureEdgePoints(points []statistics.Point, totalOrders int64,
	firstOrders int64, firstTime int64, firstCumBet, firstCumWin float64,
	finalCumBet, finalCumWin float64, lastTime int64) []statistics.Point {

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

	result := make([]statistics.Point, 0, len(points)+2)

	// 添加第一个点（如果需要）
	if !hasFirst && firstOrders > 0 {
		firstRate := 0.0
		if firstCumBet > 0 {
			firstRate = (firstCumBet - firstCumWin) / firstCumBet
		}
		result = append(result, statistics.Point{
			X:    float64(firstOrders) / orderUnit,
			Y:    firstRate,
			Time: time.Unix(firstTime, 0).In(locSH).Format(timeLayout),
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
		result = append(result, statistics.Point{
			X:    float64(totalOrders) / orderUnit,
			Y:    lastRate,
			Time: time.Unix(lastTime, 0).In(locSH).Format(timeLayout),
		})
	}

	return result
}

// 验证采样结果
func validateSampling(points []statistics.Point, totalOrders int64, step int64) error {
	if len(points) == 0 {
		return nil
	}

	var errors []string

	// 验证第一个点
	firstOrder := int64(points[0].X * orderUnit)
	if firstOrder != 1 {
		errors = append(errors, fmt.Sprintf("第一个点订单号不是1: %v", firstOrder))
	}

	// 验证最后一个点
	lastOrder := int64(points[len(points)-1].X * orderUnit)
	if lastOrder != totalOrders {
		errors = append(errors, fmt.Sprintf("最后一个点订单号不是%d: %v", totalOrders, lastOrder))
	}

	// 验证采样间隔（统计信息）
	if len(points) >= 3 {
		intervals := make([]int64, 0, len(points)-1)
		for i := 1; i < len(points); i++ {
			prevOrder := int64(points[i-1].X * orderUnit)
			currOrder := int64(points[i].X * orderUnit)
			intervals = append(intervals, currOrder-prevOrder)
		}

		// 计算平均间隔
		var sum int64
		for _, interval := range intervals {
			sum += interval
		}
		avgInterval := float64(sum) / float64(len(intervals))

		// 检查与期望间隔的偏差
		deviation := math.Abs(avgInterval-float64(step)) / float64(step)
		if deviation > 0.3 { // 允许30%的偏差
			errors = append(errors, fmt.Sprintf("采样间隔偏差过大: 期望%d, 平均%.1f, 偏差%.1f%%",
				step, avgInterval, deviation*100))
		}
	}

	// 验证时间顺序
	for i := 1; i < len(points); i++ {
		prevTime, err1 := time.Parse(timeLayout, points[i-1].Time)
		currTime, err2 := time.Parse(timeLayout, points[i].Time)
		if err1 == nil && err2 == nil && currTime.Before(prevTime) {
			errors = append(errors, fmt.Sprintf("时间顺序错误: 第%d个点时间早于前一个点", i))
			break
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf(strings.Join(errors, "; "))
	}

	return nil
}

// 均匀截断采样点
func uniformTruncate(points []statistics.Point, maxPoints int) []statistics.Point {
	if len(points) <= maxPoints {
		return points
	}
	n := len(points)
	result := make([]statistics.Point, 0, maxPoints)
	result = append(result, points[0])
	step := (n - 1) / (maxPoints - 1)
	if step < 1 {
		step = 1
	}
	for i := step; i < n-step && len(result) < maxPoints-1; i += step {
		result = append(result, points[i])
	}
	if len(result) < maxPoints {
		result = append(result, points[n-1])
	}
	return result
}

// 获取总订单数
func (r *dataRepo) getTotalOrders(scope biz.OrderScope, excludeAmount float64) (int64, error) {
	query := `SELECT COUNT(*) FROM game_order WHERE game_id = ? AND merchant = ? AND amount != ?`
	var total int64
	_, err := r.data.order.SQL(query, scope.GameID, scope.Merchant, excludeAmount).Get(&total)
	return total, err
}

//——————————————————————————————————————————————————————————————————————————————————————————————————————————————————————————————————————

/*// QueryGameOrderPoints 查询并返回图表点
func (r *dataRepo) QueryGameOrderPoints2(ctx context.Context, filter biz.OrderScope) ([]statistics.Point, error) {
	if r.data.order == nil {
		return nil, fmt.Errorf("order database not configured")
	}

	ex := filter.ExcludeAmt
	if ex <= 0 {
		ex = excludeAmt
	}

	// 构建查询条件
	where := "game_id = ? AND amount != ?"
	args := []any{filter.GameID, 0.01}

	if filter.Merchant != "" {
		where += " AND merchant = ?"
		args = append(args, filter.Merchant)
	}
	//if filter.Member != "" {
	//	where += " AND member = ?"
	//	args = append(args, filter.Member)
	//}
	if !filter.StartTime.IsZero() && !filter.EndTime.IsZero() {
		where += " AND created_at BETWEEN UNIX_TIMESTAMP(?) AND UNIX_TIMESTAMP(?)"
		args = append(args, filter.StartTime, filter.EndTime)
	}

	// 查询数据
	type record struct {
		Amount      float64 `xorm:"amount"`
		BonusAmount float64 `xorm:"bonus_amount"`
		CreatedAt   int64   `xorm:"created_at"`
	}
	var records []record
	if err := r.data.order.SQL("SELECT amount, bonus_amount, created_at FROM game_order WHERE "+where+" ORDER BY id", args...).Find(&records); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if len(records) == 0 {
		return nil, nil
	}

	var pts []statistics.Point
	var bet, win, cumBet, cumWin float64
	var t string
	var orders int

	flush := func() {
		if bet > 0 || win > 0 {
			cumBet += bet
			cumWin += win
			rate := 0.0
			if cumBet > 0 {
				rate = (cumBet - cumWin) / cumBet
			}
			pts = append(pts, statistics.Point{
				X:    float64(orders) / orderUnit,
				Y:    rate,
				Time: t,
			})
		}
	}

	for _, rec := range records {
		orders++
		if rec.Amount > 0 {
			flush()
			bet, win = rec.Amount, rec.BonusAmount
			t = time.Unix(rec.CreatedAt, 0).In(locSH).Format(timeLayout)
		} else {
			win += rec.BonusAmount
		}
	}
	flush()

	return sample(pts), nil
}


const sampleMax = 5000

// sample 等间距采样
func sample(pts []statistics.Point) []statistics.Point {
	n := len(pts)
	if n <= sampleMax {
		return pts
	}
	step := (n - 1) / (sampleMax - 1)
	if step < 1 {
		step = 1
	}
	out := make([]statistics.Point, 0, sampleMax)
	for i := 0; i < n && len(out) < sampleMax-1; i += step {
		out = append(out, pts[i])
	}
	out = append(out, pts[n-1])
	fmt.Printf("采样: 原%d后%d", n, len(out))
	return out
}
*/
