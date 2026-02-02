package data

import (
	"context"
	"fmt"
	"strings"
	"time"

	"stress/internal/biz/stats/statistics"

	"github.com/go-kratos/kratos/v2/log"
)

const (
	orderUnit       = 1e4
	statsExcludeAmt = 0.01
	timeLayout      = "2006-01-02 15:04:05"
)
const (
	sampleMax = 5000 // 最大采样数
)

var locSH, _ = time.LoadLocation("Asia/Shanghai")

// QueryGameOrderPoints 查询并返回图表点（查询 + 聚合为点）
func (r *dataRepo) QueryGameOrderPoints(ctx context.Context, filter statistics.QueryFilter) ([]statistics.Point, error) {
	if r.data.order == nil {
		return nil, fmt.Errorf("order database not configured")
	}

	ex := filter.ExcludeAmount
	if ex <= 0 {
		ex = statsExcludeAmt
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
	if err := r.data.order.Context(ctx).SQL(
		"SELECT amount, bonus_amount, created_at FROM game_order WHERE "+strings.Join(conds, " AND ")+" ORDER BY id",
		args...,
	).Find(&recs); err != nil {
		return nil, err
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
			pts = append(pts, statistics.Point{X: float64(orders) / orderUnit, Y: rate, Time: t})
		}
	}

	for _, r := range recs {
		orders++
		if r.Amount > 0 {
			flush()
			bet, win = r.Amount, r.BonusAmount
			t = time.Unix(r.CreatedAt, 0).In(locSH).Format(timeLayout)
		} else {
			win += r.BonusAmount
		}
	}
	flush()

	spts := sample(pts)
	return spts, nil
}

// sample 等间距采样，保留首尾
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
	out = append(out, pts[n-1]) // 保留最后一条
	log.Info("采样", "原", n, "后", len(out))
	return out
}
