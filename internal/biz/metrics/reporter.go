package metrics

import (
	"fmt"
	v1 "stress/api/stress/v1"

	"github.com/prometheus/client_golang/prometheus"
)

// ReportTask 将任务报告上报到 Prometheus（供 task.Monitor 调用）
func ReportTask(r *v1.TaskCompletionReport) {
	if r == nil {
		return
	}

	labels := prometheus.Labels{
		labelTaskID: r.TaskId,
		labelGameID: fmt.Sprintf("%d", r.GameId),
	}

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
