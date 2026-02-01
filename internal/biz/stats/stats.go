package stats

import (
	"context"
	"time"

	v1 "stress/api/stress/v1"
	"stress/internal/biz/task"
	"stress/pkg/xgo"

	"github.com/go-kratos/kratos/v2/log"
)

// OrderLoader 订单数据加载接口（DataRepo 实现）
type OrderLoader interface {
	GetDetailedOrderAmounts(ctx context.Context) (totalBet, totalWin, betOrderCount, bonusOrderCount int64, err error)
	GetGameOrderCount(ctx context.Context) (int64, error)
}

// BuildReport 构建完整报告（Task 统计 + 订单统计）
func BuildReport(ctx context.Context, loader OrderLoader, t *task.Task, now time.Time) *v1.TaskCompletionReport {
	r := t.CompletionReport(now)
	o := loadOrderStats(ctx, loader, r.TaskId)
	r.OrderCount, r.TotalBet, r.TotalWin, r.RtpPct = o.orderCount, o.totalBet, o.totalWin, o.rtpPct
	return r
}

type orderStats struct {
	totalBet   int64
	totalWin   int64
	orderCount int64
	rtpPct     float64
}

func loadOrderStats(ctx context.Context, loader OrderLoader, taskID string) orderStats {
	totalBet, totalWin, betCnt, _, err := loader.GetDetailedOrderAmounts(ctx)
	orderCount := betCnt
	if err != nil {
		log.Warnf("[%s] GetDetailedOrderAmounts failed: %v", taskID, err)
		totalBet, totalWin = 0, 0
		orderCount, _ = loader.GetGameOrderCount(ctx)
	}
	return orderStats{
		totalBet:   totalBet,
		totalWin:   totalWin,
		orderCount: orderCount,
		rtpPct:     xgo.Pct(totalWin, totalBet),
	}
}
