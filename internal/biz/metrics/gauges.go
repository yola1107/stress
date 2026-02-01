package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	labelTaskID    = "task_id"
	labelGameID    = "game_id"
	reportInterval = 5 * time.Second
)

// 指标名规范：stress_task_<name>，标签 task_id、game_id

var (
	_metric_progress       = newGauge("stress_task_progress", "Steps")
	_metric_total_steps    = newGauge("stress_task_total_steps", "total steps")
	_metric_progress_pct   = newGauge("stress_task_progress_pct", "任务进度 (0-100)")
	_metric_qps            = newGauge("stress_task_qps", "每秒完成局数")
	_metric_active_members = newGauge("stress_task_active_members", "活跃成员数")
	_metric_failed_reqs    = newGauge("stress_task_failed_requests", "累计失败请求数")
	_metric_total_bet      = newGauge("stress_task_total_bet", "总下注(×1e4)")
	_metric_total_win      = newGauge("stress_task_total_win", "总赢(×1e4)")
	_metric_rtp_pct        = newGauge("stress_task_rtp_pct", "RTP %")
	_metric_order_count    = newGauge("stress_task_order_count", "订单数")
)

func newGauge(name, help string) *prometheus.GaugeVec {
	return promauto.NewGaugeVec(prometheus.GaugeOpts{Name: name, Help: help}, []string{labelTaskID, labelGameID})
}

func set(g *prometheus.GaugeVec, labels prometheus.Labels, v float64) {
	g.With(labels).Set(v)
}
