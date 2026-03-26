package data

import (
	"context"
	"fmt"
	"time"

	"stress/internal/biz/chart"
	"stress/internal/biz/task"
)

const (
	sampleMax  = 5000
	orderUnit  = 1e4
	timeLayout = "2006-01-02 15:04:05"
)

// QueryGameOrderPoints 通过 SQL 分桶聚合一次扫描完成采样，避免逐行传输 2000 万行到 Go。
// 原理：将 ID 范围等分为 ≤sampleMax 个桶，MySQL 在一次全表扫描中按桶做
// SUM(amount)/SUM(bonus_amount)/COUNT(*)，只返回 ~5000 行；Go 侧做前缀累加得到
// 与逐行扫描数学等价的 cumBet/cumWin，再计算盈利率。
func (r *dataRepo) QueryGameOrderPoints(ctx context.Context, scope task.OrderScope) ([]chart.Point, error) {
	orderDB, err := r.orderEngine()
	if err != nil {
		return nil, err
	}
	if scope.GameID == 0 || scope.Merchant == "" {
		return nil, fmt.Errorf("game_id and merchant are required")
	}

	start := time.Now()
	baseWhere, baseArgs := buildOrderWhere(scope)

	sess := orderDB.NewSession().Context(ctx)
	defer sess.Close()

	// ── 第 1 步：一次查询拿到 ID 范围与总行数 ──
	type rangeInfo struct {
		MinID int64 `xorm:"min_id"`
		MaxID int64 `xorm:"max_id"`
		Total int64 `xorm:"total"`
	}
	var info rangeInfo
	if _, err = sess.SQL(
		`SELECT MIN(id) AS min_id, MAX(id) AS max_id, COUNT(*) AS total FROM game_order WHERE `+baseWhere,
		baseArgs...,
	).Get(&info); err != nil {
		return nil, fmt.Errorf("range query: %w", err)
	}
	if info.Total == 0 {
		return []chart.Point{}, nil
	}

	// ── 第 2 步：按 ID 范围分桶，MySQL 侧完成聚合，仅返回 ≤sampleMax 行 ──
	numBuckets := int64(sampleMax)
	if info.Total < numBuckets {
		numBuckets = info.Total
	}
	bucketWidth := (info.MaxID - info.MinID + 1) / numBuckets
	if bucketWidth < 1 {
		bucketWidth = 1
	}

	type bucket struct {
		Cnt int64   `xorm:"cnt"`
		Bet float64 `xorm:"bet"`
		Win float64 `xorm:"win"`
		Ts  int64   `xorm:"ts"`
	}

	allArgs := make([]any, 0, len(baseArgs)+4)
	allArgs = append(allArgs, baseArgs...)
	allArgs = append(allArgs, info.MinID, bucketWidth, info.MinID, bucketWidth)

	var buckets []bucket
	if err = sess.SQL(`
		SELECT
			COUNT(*)                                          AS cnt,
			SUM(CASE WHEN amount > 0 THEN amount ELSE 0 END) AS bet,
			SUM(bonus_amount)                                 AS win,
			MAX(created_at)                                   AS ts
		FROM game_order
		WHERE `+baseWhere+`
		GROUP BY FLOOR((id - ?) / ?)
		ORDER BY FLOOR((id - ?) / ?)`,
		allArgs...,
	).Find(&buckets); err != nil {
		return nil, fmt.Errorf("bucket query: %w", err)
	}

	// ── 第 3 步：Go 侧前缀累加，与逐行 flush 数学等价 ──
	var cumBet, cumWin float64
	var cumRows int64
	pts := make([]chart.Point, 0, len(buckets))

	for _, b := range buckets {
		cumBet += b.Bet
		cumWin += b.Win
		cumRows += b.Cnt

		rate := 0.0
		if cumBet > 0 {
			rate = (cumBet - cumWin) / cumBet
		}
		pts = append(pts, chart.Point{
			X:    float64(cumRows) / orderUnit,
			Y:    rate,
			Time: time.Unix(b.Ts, 0).In(time.Local).Format(timeLayout),
		})
	}

	if len(pts) > sampleMax {
		pts = uniformTruncate(pts, sampleMax)
	}

	r.log.Infof("处理完成: 总订单数=%d, 桶数=%d, 采样点数=%d, 耗时=%v",
		info.Total, len(buckets), len(pts), time.Since(start))

	return pts, nil
}

// uniformTruncate 均匀截断采样点，保留首尾
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
