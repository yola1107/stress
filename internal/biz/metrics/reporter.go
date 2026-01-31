package metrics

import (
	"context"
	"fmt"
	"time"

	"stress/internal/biz/task"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/prometheus/client_golang/prometheus"
)

// OrderReader 订单数据读取接口
type OrderReader interface {
	GetDetailedOrderAmounts(ctx context.Context) (totalBet, totalWin, betOrderCount, bonusOrderCount int64, err error)
	GetGameOrderCount(ctx context.Context) (int64, error)
}

// ReportTaskMetrics 启动指标上报，ctx 取消后退出
func ReportTaskMetrics(ctx context.Context, t *task.Task, repo OrderReader) {
	ticker := time.NewTicker(reportInterval)
	defer ticker.Stop()

	snap := t.StatsSnapshot()
	labels := baseLabels(snap)
	reportOnce(ctx, repo, labels, snap)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snap = t.StatsSnapshot()
			reportOnce(ctx, repo, labels, snap)
		}
	}
}

func baseLabels(snap task.StatsSnapshot) prometheus.Labels {
	gameID := "unknown"
	if snap.Config != nil && snap.Config.GameId > 0 {
		gameID = fmt.Sprintf("%d", snap.Config.GameId)
	}
	return prometheus.Labels{labelTaskID: snap.ID, labelGameID: gameID}
}

func reportOnce(ctx context.Context, repo OrderReader, labels prometheus.Labels, snap task.StatsSnapshot) {
	order := loadOrderData(ctx, repo, snap.ID)

	progressPctVal := 0.0
	if snap.Target > 0 {
		progressPctVal = float64(snap.Process) / float64(snap.Target) * 100
		if progressPctVal > 100 {
			progressPctVal = 100
		}
	}

	elapsed := time.Since(snap.CreatedAt).Seconds()
	if !snap.FinishedAt.IsZero() {
		elapsed = snap.FinishedAt.Sub(snap.CreatedAt).Seconds()
	}
	qpsVal := 0.0
	if elapsed > 0 {
		qpsVal = float64(snap.Process) / elapsed
	}

	set(progressPct, labels, progressPctVal)
	set(qps, labels, qpsVal)
	set(activeMembers, labels, float64(snap.ActiveMembers))
	set(failedReqs, labels, float64(snap.FailedRequests))
	set(totalBet, labels, float64(order.totalBet))
	set(totalWin, labels, float64(order.totalWin))
	set(rtpPct, labels, order.rtpPct)
	set(orderCount, labels, float64(order.totalCnt))
}

type orderData struct {
	totalBet, totalWin, totalCnt int64
	rtpPct                       float64
}

func loadOrderData(ctx context.Context, repo OrderReader, taskID string) orderData {
	totalBet, totalWin, betCnt, _, err := repo.GetDetailedOrderAmounts(ctx)
	totalCnt := betCnt

	if err != nil {
		log.Warnf("[%s] GetDetailedOrderAmounts failed: %v", taskID, err)
		totalBet, totalWin = 0, 0
		totalCnt, _ = repo.GetGameOrderCount(ctx)
	}

	rtpPctVal := 0.0
	if totalBet > 0 {
		rtpPctVal = float64(totalWin) / float64(totalBet) * 100
	}

	return orderData{
		totalBet: totalBet, totalWin: totalWin, totalCnt: totalCnt,
		rtpPct: rtpPctVal,
	}
}
