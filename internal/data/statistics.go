package data

import (
	"context"
	"fmt"
	"strings"
	"time"

	"stress/internal/biz"
	"stress/internal/biz/stats"

	"github.com/go-kratos/kratos/v2/log"
)

const (
	orderUnit       = 1e4
	timeLayout      = "2006-01-02 15:04:05"
	maxSamplePoints = 5000
)

var locSH, _ = time.LoadLocation("Asia/Shanghai")

// QueryGameOrderPoints 查询并返回图表点
func (r *dataRepo) QueryGameOrderPoints(ctx context.Context, filter biz.QueryFilter) ([]stats.Point, error) {
	if r.data.order == nil {
		return nil, fmt.Errorf("order database not configured")
	}

	ex := filter.ExcludeAmount
	if ex <= 0 {
		ex = excludeAmt
	}

	conds := []string{"game_id = ?", "amount != ?"}
	args := []any{filter.GameID, ex}

	if filter.Merchant != "" {
		conds = append(conds, "merchant = ?")
		args = append(args, filter.Merchant)
	}
	if filter.Member != "" {
		conds = append(conds, "member = ?")
		args = append(args, filter.Member)
	}
	if filter.StartTime != "" && filter.EndTime != "" {
		conds = append(conds, "created_at BETWEEN UNIX_TIMESTAMP(?) AND UNIX_TIMESTAMP(?)")
		args = append(args, filter.StartTime, filter.EndTime)
	}

	type orderRecord struct {
		Amount      float64
		BonusAmount float64
		CreatedAt   int64
	}
	var recs []orderRecord
	sql := "SELECT amount, bonus_amount, created_at FROM game_order WHERE " + strings.Join(conds, " AND ") + " ORDER BY id"
	if err := r.data.order.Context(ctx).SQL(sql, args...).Find(&recs); err != nil {
		return nil, err
	}

	var pts []stats.Point
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
			pts = append(pts, stats.Point{
				X:    float64(orders) / orderUnit,
				Y:    rate,
				Time: t,
			})
		}
	}

	for _, rec := range recs {
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

	return samplePoints(pts), nil
}

// samplePoints 等间距采样
func samplePoints(pts []stats.Point) []stats.Point {
	n := len(pts)
	if n <= maxSamplePoints {
		return pts
	}
	step := (n - 1) / (maxSamplePoints - 1)
	if step < 1 {
		step = 1
	}
	out := make([]stats.Point, 0, maxSamplePoints)
	for i := 0; i < n && len(out) < maxSamplePoints-1; i += step {
		out = append(out, pts[i])
	}
	out = append(out, pts[n-1])
	log.Info("采样", "原", n, "后", len(out))
	return out
}
