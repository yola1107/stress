package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	labelTaskID    = "task_id"
	labelGameID    = "game_id"
	reportInterval = 10 * time.Second
)

// 指标名规范：stress_task_<name>，标签 task_id、game_id

var (
	progressPct   = newGauge("stress_task_progress_pct", "任务进度 (0-100)")
	qps           = newGauge("stress_task_qps", "每秒完成局数")
	activeMembers = newGauge("stress_task_active_members", "活跃成员数")
	failedReqs    = newGauge("stress_task_failed_requests", "累计失败请求数")

	totalBet   = newGauge("stress_task_total_bet", "总下注(×1e4)")
	totalWin   = newGauge("stress_task_total_win", "总赢(×1e4)")
	rtpPct     = newGauge("stress_task_rtp_pct", "RTP %")
	orderCount = newGauge("stress_task_order_count", "订单数")
)

func newGauge(name, help string) *prometheus.GaugeVec {
	return promauto.NewGaugeVec(prometheus.GaugeOpts{Name: name, Help: help}, []string{labelTaskID, labelGameID})
}

func set(g *prometheus.GaugeVec, labels prometheus.Labels, v float64) {
	g.With(labels).Set(v)
}
