package biz

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"stress/internal/biz/task"
)

// Prometheus 指标定义 - 只上报基础数据，计算由监控层处理
var (
	stressTaskProgressPct = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "stress_task_progress_pct",
		Help: "Task progress percentage (0-100)",
	}, []string{"task_id"})

	stressTaskOrderCount = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "stress_task_order_count",
		Help: "Current order count",
	}, []string{"task_id"})

	stressTaskTotalBet = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "stress_task_total_bet",
		Help: "Total bet amount (scaled by 1e4)",
	}, []string{"task_id"})

	stressTaskTotalWin = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "stress_task_total_win",
		Help: "Total win amount (scaled by 1e4)",
	}, []string{"task_id"})

	stressTaskQPS = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "stress_task_qps",
		Help: "Task QPS (queries per second)",
	}, []string{"task_id"})

	stressTaskActiveMembers = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "stress_task_active_members",
		Help: "Number of active members",
	}, []string{"task_id"})

	stressTaskFailedRequests = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "stress_task_failed_requests",
		Help: "Number of failed requests",
	}, []string{"task_id"})
)

// ReportTaskMetrics 启动 Prometheus 指标上报 goroutine，任务结束时 ctx 取消后退出
func ReportTaskMetrics(ctx context.Context, t *task.Task, repo DataRepo) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	taskID := t.GetID()
	labels := prometheus.Labels{"task_id": taskID}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snap := t.LoadProgressSnapshot()

			totalBet, totalWin, err := repo.GetOrderAmounts(ctx)
			orderCount, errOrder := repo.GetGameOrderCount(ctx)
			if err != nil {
				log.Warnf("[%s] failed to query order amounts for Prometheus: %v", taskID, err)
			}
			if errOrder != nil {
				log.Debugf("[%s] GetGameOrderCount for Prometheus: %v", taskID, errOrder)
			}

			progressPct := float64(0)
			if snap.Target > 0 {
				progressPct = float64(snap.Step) / float64(snap.Target) * 100
				if progressPct > 100 {
					progressPct = 100
				}
			}

			elapsed := time.Since(t.GetCreatedAt()).Seconds()
			qps := float64(0)
			if elapsed > 0 {
				qps = float64(snap.Process) / elapsed
			}

			stressTaskProgressPct.With(labels).Set(progressPct)
			stressTaskOrderCount.With(labels).Set(float64(orderCount))
			stressTaskTotalBet.With(labels).Set(float64(totalBet))
			stressTaskTotalWin.With(labels).Set(float64(totalWin))
			stressTaskQPS.With(labels).Set(qps)
			stressTaskActiveMembers.With(labels).Set(float64(t.GetActiveMembers()))
			stressTaskFailedRequests.With(labels).Set(float64(t.GetFailedRequests()))
		}
	}
}
