package metrics

import (
	"context"
	"fmt"
	"time"

	v1 "stress/api/stress/v1"
	"stress/internal/biz/stats"
	"stress/internal/biz/task"

	"github.com/prometheus/client_golang/prometheus"
)

// ReportTaskMetrics 启动指标上报，ctx 取消后退出
func ReportTaskMetrics(ctx context.Context, t *task.Task, repo stats.OrderLoader) {
	ticker := time.NewTicker(reportInterval)
	defer ticker.Stop()

	report := stats.BuildReport(ctx, repo, t, time.Now())
	labels := baseLabels(report)
	reportOnce(labels, report)

	for {
		select {
		case <-ctx.Done():
			// ctx 已取消，用 background 保证最终一次 DB 查询能完成
			report = stats.BuildReport(context.Background(), repo, t, time.Now())
			reportOnce(labels, report)
			return
		case <-ticker.C:
			report = stats.BuildReport(ctx, repo, t, time.Now())
			reportOnce(labels, report)
		}
	}
}

func baseLabels(report *v1.TaskCompletionReport) prometheus.Labels {
	gameID := "unknown"
	if report != nil && report.GameId > 0 {
		gameID = fmt.Sprintf("%d", report.GameId)
	}
	taskID := ""
	if report != nil {
		taskID = report.TaskId
	}
	return prometheus.Labels{labelTaskID: taskID, labelGameID: gameID}
}

func reportOnce(labels prometheus.Labels, r *v1.TaskCompletionReport) {
	set(_metric_progress, labels, float64(r.Process))
	set(_metric_total_steps, labels, float64(r.Step))
	set(_metric_progress_pct, labels, r.ProgressPct)
	set(_metric_qps, labels, r.Qps)
	set(_metric_active_members, labels, float64(r.ActiveMembers))
	set(_metric_failed_reqs, labels, float64(r.FailedReqs))
	set(_metric_total_bet, labels, float64(r.TotalBet))
	set(_metric_total_win, labels, float64(r.TotalWin))
	set(_metric_rtp_pct, labels, r.RtpPct)
	set(_metric_order_count, labels, float64(r.OrderCount))
}
