package biz

import (
	"context"
	"fmt"
	"time"

	"stress/internal/biz/task"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// 通用标签
const labelTaskID, labelGameID = "task_id", "game_id"

// 上报间隔（可通过常量调整，后续可接入 conf）
const metricsReportInterval = 10 * time.Second

// RTP 里程碑间隔：每 50 万次（snap.Process 已完成局数）标注一次
const rtpMilestoneStep = int64(500000)

// 任务进度与性能
var (
	gProgressPct      = newGauge("stress_task_progress_pct", "任务进度 (0-100)")
	gQPS              = newGauge("stress_task_qps", "QPS")
	gActiveMembers    = newGauge("stress_task_active_members", "活跃成员数")
	gCompletedMembers = newGauge("stress_task_completed_members", "已完成成员数")
	gFailedMembers    = newGauge("stress_task_failed_members", "失败成员数")
	gFailedRequests   = newGauge("stress_task_failed_requests", "失败请求数")
	gDurationSec      = newGauge("stress_task_duration_seconds", "运行时长(秒)")
	gAvgRespTimeMs    = newGauge("stress_task_avg_response_time_ms", "平均响应时间(毫秒)")
)

// RTP 与订单（从数据库）
var (
	gTotalBet      = newGauge("stress_task_total_bet", "总下注(×1e4)")
	gTotalWin      = newGauge("stress_task_total_win", "总赢(×1e4)")
	gRTPPct        = newGauge("stress_task_rtp_pct", "RTP %")
	gOrderCount    = newGauge("stress_task_order_count", "订单数")
	gBetOrderCnt   = newGauge("stress_task_bet_order_count", "下注订单数")
	gBonusOrderCnt = newGauge("stress_task_bonus_order_count", "奖励订单数")
)

// 任务配置（仅 task_id）
var (
	gCfgGameID    = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "stress_task_config_game_id", Help: "游戏ID"}, []string{labelTaskID})
	gCfgMemberCnt = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "stress_task_config_member_count", Help: "用户数"}, []string{labelTaskID})
	gCfgTimesPer  = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "stress_task_config_times_per_member", Help: "每用户次数"}, []string{labelTaskID})
	gCfgTotalTgt  = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "stress_task_config_total_target", Help: "总目标次数"}, []string{labelTaskID})
	gCfgBaseMoney = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "stress_task_config_bet_base_money", Help: "基础金额"}, []string{labelTaskID})
	gCfgMultiple  = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "stress_task_config_bet_multiple", Help: "倍数"}, []string{labelTaskID})
	gCfgPurchase  = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "stress_task_config_bet_purchase", Help: "购买"}, []string{labelTaskID})
	gCfgSignReq   = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "stress_task_config_sign_required", Help: "需签名(0/1)"}, []string{labelTaskID})
	gCfgBonusCnt  = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "stress_task_config_bonus_count", Help: "奖励配置数"}, []string{labelTaskID})
)

// RTP 里程碑：每 rtpMilestoneStep 次（Process）标注 (RTP, 总下注, 总回报)，供 Grafana 做分割线/标注
var gRTPMilestone = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "stress_task_rtp_milestone",
	Help: "RTP at 500k process milestones (for annotations)",
}, []string{labelTaskID, labelGameID, "milestone", "total_bet", "total_win"})

func newGauge(name, help string) *prometheus.GaugeVec {
	return promauto.NewGaugeVec(prometheus.GaugeOpts{Name: name, Help: help}, []string{labelTaskID, labelGameID})
}

func set(g *prometheus.GaugeVec, labels prometheus.Labels, v float64) { g.With(labels).Set(v) }

// ReportTaskMetrics 启动指标上报，ctx 取消后退出
func ReportTaskMetrics(ctx context.Context, t *task.Task, repo DataRepo) {
	ticker := time.NewTicker(metricsReportInterval)
	defer ticker.Stop()

	snap := t.GetStats().StatsSnapshot()
	taskID := snap.ID
	gameID := "unknown"
	if snap.Config != nil && snap.Config.GameId > 0 {
		gameID = fmt.Sprintf("%d", snap.Config.GameId)
	}
	labels := prometheus.Labels{labelTaskID: taskID, labelGameID: gameID}
	cfgLabels := prometheus.Labels{labelTaskID: taskID}

	var lastProcessMilestone int64
	reportConfig(snap, cfgLabels)
	reportRuntime(ctx, repo, labels, snap, &lastProcessMilestone)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snap = t.GetStats().StatsSnapshot()
			reportRuntime(ctx, repo, labels, snap, &lastProcessMilestone)
		}
	}
}

func reportConfig(snap task.StatsSnapshot, lbl prometheus.Labels) {
	cfg := snap.Config
	if cfg == nil {
		return
	}
	set(gCfgGameID, lbl, float64(cfg.GameId))
	set(gCfgMemberCnt, lbl, float64(cfg.MemberCount))
	set(gCfgTimesPer, lbl, float64(cfg.TimesPerMember))
	set(gCfgTotalTgt, lbl, float64(int64(cfg.MemberCount)*int64(cfg.TimesPerMember)))

	baseMoney, multiple, purchase := 0.0, 0.0, 0.0
	if cfg.BetOrder != nil {
		baseMoney, multiple, purchase = cfg.BetOrder.BaseMoney, float64(cfg.BetOrder.Multiple), float64(cfg.BetOrder.Purchase)
	}
	set(gCfgBaseMoney, lbl, baseMoney)
	set(gCfgMultiple, lbl, multiple)
	set(gCfgPurchase, lbl, purchase)
	signReq := 0.0
	if cfg.SignRequired {
		signReq = 1
	}
	set(gCfgSignReq, lbl, signReq)
	set(gCfgBonusCnt, lbl, float64(len(cfg.BetBonus)))
}

func reportRuntime(ctx context.Context, repo DataRepo, labels prometheus.Labels, snap task.StatsSnapshot, lastProcessMilestone *int64) {
	totalBet, totalWin, betCnt, bonusCnt, totalCnt, _ := loadOrderData(ctx, repo, snap.ID)

	rtp := 0.0
	if totalBet > 0 {
		rtp = float64(totalWin) / float64(totalBet) * 100
	}

	// 每 rtpMilestoneStep 次（snap.Process 已完成局数）标注一次 RTP 里程碑
	process := snap.Process
	for m := *lastProcessMilestone + rtpMilestoneStep; m <= process; m += rtpMilestoneStep {
		milestoneLabels := prometheus.Labels{
			labelTaskID: snap.ID,
			labelGameID: labels[labelGameID],
			"milestone": fmt.Sprintf("%d", m),
			"total_bet": fmt.Sprintf("%d", totalBet),
			"total_win": fmt.Sprintf("%d", totalWin),
		}
		gRTPMilestone.With(milestoneLabels).Set(rtp)
		*lastProcessMilestone = m
	}

	progressPct := 0.0
	if snap.Target > 0 {
		progressPct = float64(snap.Process) / float64(snap.Target) * 100
		if progressPct > 100 {
			progressPct = 100
		}
	}

	end := time.Now()
	if !snap.FinishedAt.IsZero() {
		end = snap.FinishedAt
	}
	elapsed := end.Sub(snap.CreatedAt).Seconds()
	qps := 0.0
	if elapsed > 0 {
		qps = float64(snap.Process) / elapsed
	}

	avgMs := 0.0
	if snap.Step > 0 && snap.TotalDuration > 0 {
		avgMs = float64(snap.TotalDuration) / float64(snap.Step) / 1e6
	}

	set(gProgressPct, labels, progressPct)
	set(gQPS, labels, qps)
	set(gActiveMembers, labels, float64(snap.ActiveMembers))
	set(gCompletedMembers, labels, float64(snap.CompletedMembers))
	set(gFailedMembers, labels, float64(snap.FailedMembers))
	set(gFailedRequests, labels, float64(snap.FailedRequests))
	set(gDurationSec, labels, elapsed)
	set(gAvgRespTimeMs, labels, avgMs)
	set(gTotalBet, labels, float64(totalBet))
	set(gTotalWin, labels, float64(totalWin))
	set(gRTPPct, labels, rtp)
	set(gOrderCount, labels, float64(totalCnt))
	set(gBetOrderCnt, labels, float64(betCnt))
	set(gBonusOrderCnt, labels, float64(bonusCnt))
}

func loadOrderData(ctx context.Context, repo DataRepo, taskID string) (totalBet, totalWin, betCnt, bonusCnt, totalCnt int64, err error) {
	totalBet, totalWin, betCnt, bonusCnt, err = repo.GetDetailedOrderAmounts(ctx)
	totalCnt = betCnt
	if err != nil {
		log.Warnf("[%s] GetDetailedOrderAmounts failed: %v", taskID, err)
		totalBet, totalWin, betCnt, bonusCnt = 0, 0, 0, 0
		// 降级：用 GetGameOrderCount 至少拿到订单总数
		totalCnt, _ = repo.GetGameOrderCount(ctx)
	}
	return totalBet, totalWin, betCnt, bonusCnt, totalCnt, err
}
