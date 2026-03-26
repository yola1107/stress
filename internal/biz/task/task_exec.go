package task

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	v1 "stress/api/stress/v1"
	"stress/internal/biz/chart"
	"stress/internal/biz/member"
	"stress/internal/biz/metrics"
	"stress/internal/biz/notify"
	"stress/internal/conf"

	"github.com/panjf2000/ants/v2"
)

const (
	reportInterval     = 15 * time.Second // 指标上报间隔
	orderWaitInterval  = 10 * time.Second // 订单等待检查间隔
	orderWaitTimeout   = 15 * time.Minute // 订单等待超时
	cleanupTimeout     = 10 * time.Minute // 清理操作超时
	monitorLogInterval = 1 * time.Second  // 监控日志间隔
)

// Repo 任务执行期所需的数据操作（biz.DataRepo 的子集，便于直接传递）
type Repo interface {
	// GetGameOrderCount 全表订单数（用于等待异步写入完成）
	GetGameOrderCount(ctx context.Context) (int64, error)
	// GetDetailedOrderAmounts 按范围统计下注/赢额/订单数
	GetDetailedOrderAmounts(ctx context.Context, scope OrderScope) (totalBet, totalWin, betOrderCount, bonusOrderCount int64, err error)
	// QueryGameOrderPoints 按范围采样订单描点（用于绘图）
	QueryGameOrderPoints(ctx context.Context, scope OrderScope) ([]chart.Point, error)
	// UploadBytes 上传字节到 S3，返回访问 URL
	UploadBytes(ctx context.Context, bucket, key, contentType string, data []byte) (string, error)
	// CleanRedisBySites 批量清理指定 sites 的 Redis 缓存
	CleanRedisBySites(ctx context.Context, sites []string) error
	// CleanGameOrderTable 清空订单表
	CleanGameOrderTable(ctx context.Context) error
}

// ExecDeps 任务执行依赖
type ExecDeps struct {
	Repo          Repo
	Conf          *conf.Stress
	Notify        notify.Notifier
	Chart         chart.IGenerator
	ReturnMembers func(taskID string)
}

// MemberInfo 成员信息（避免循环依赖）
type MemberInfo = member.Info

// OrderScope 订单查询范围
type OrderScope struct {
	GameID     int64
	Merchant   string
	StartTime  time.Time
	EndTime    time.Time
	ExcludeAmt float64
}

func (t *Task) Execute(members []MemberInfo, deps *ExecDeps) {
	if t.GetStatus() != v1.TaskStatus_TASK_RUNNING {
		if !t.CompareAndSetStatus(v1.TaskStatus_TASK_PENDING, v1.TaskStatus_TASK_RUNNING) {
			t.log.Warnf("[%s] task status changed, skip execution", t.GetID())
			return
		}
	}

	t.SetStartAt()

	apiClient := NewAPIClient(len(members), NoopSecretProvider, deps.Conf.Launch)
	if err := apiClient.BindSessionEnv(t); err != nil {
		t.log.Errorf("[%s] bind session env failed: %v", t.GetID(), err)
		t.SetStatus(v1.TaskStatus_TASK_FAILED)
		t.SetFinishAt()
		t.waitOrderWrite(deps)
		t.finalize(deps)
		t.cleanup(deps, apiClient)
		return
	}

	t.Monitor()

	stopReporter, wg := t.startReporter(deps)

	t.runSessions(members, apiClient)

	t.Stop()

	t.SetFinishAt()

	t.waitOrderWrite(deps)

	stopReporter()

	wg.Wait()

	t.cleanup(deps, apiClient)
}

func (t *Task) runSessions(members []MemberInfo, apiClient *APIClient) {
	poolSize := len(members)
	if poolSize == 0 {
		t.log.Warnf("[%s] no members to run sessions", t.GetID())
		return
	}

	pool, err := ants.NewPool(poolSize, ants.WithPreAlloc(true))
	if err != nil {
		t.log.Errorf("[%s] create session pool failed: %v", t.GetID(), err)
		t.SetStatus(v1.TaskStatus_TASK_FAILED)
		return
	}
	defer pool.Release()

	var wg sync.WaitGroup

	for _, m := range members {
		m := m
		sess := NewSession(m.Name)
		wg.Add(1)
		t.AddActive(1)
		if err := pool.Submit(func() {
			defer wg.Done()
			defer t.MarkSessionDone(!sess.IsFailed())
			if execErr := sess.Execute(apiClient); execErr != nil && !errors.Is(execErr, context.Canceled) {
				t.log.Errorf("[%s] session execution failed: %v", t.GetID(), execErr)
			}
		}); err != nil {
			wg.Done()
			t.MarkSessionDone(false)
			t.log.Errorf("[%s] submit session to pool failed: %v", t.GetID(), err)
		}
	}

	wg.Wait()
}

// Monitor 运行监控：1s 日志输出，task context 取消后退出
func (t *Task) Monitor() {
	go func() {
		tick := time.NewTicker(monitorLogInterval)
		defer tick.Stop()

		t.LogProgress(false)

		for {
			select {
			case <-t.ctx.Done():
				t.LogProgress(true)
				return
			case <-tick.C:
				t.LogProgress(false)
			}
		}
	}()
}

func (t *Task) startReporter(deps *ExecDeps) (context.CancelFunc, *sync.WaitGroup) {
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()

		ticker := time.NewTicker(reportInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				t.finalize(deps)
				return
			case <-ticker.C:
				t.reportMetrics(deps)
			}
		}
	}()

	return cancel, &wg
}

// waitOrderWrite 监控订单写入完成（Step 只计 bet order，bonus 不写订单）
func (t *Task) waitOrderWrite(deps *ExecDeps) {
	ticker := time.NewTicker(orderWaitInterval)
	defer ticker.Stop()

	timeout := time.After(orderWaitTimeout)
	step := t.GetStep()

	for {
		select {
		case <-timeout:
			var dbCount int64
			if n, err := deps.Repo.GetGameOrderCount(context.Background()); err == nil {
				dbCount = n
			}
			warn := fmt.Sprintf("订单等待超时(%v): 任务steps=%d, DB订单数=%d, 差值=%d",
				orderWaitTimeout, step, dbCount, step-dbCount)
			t.setOrderWarning(warn)
			t.log.Errorf("[%s] %s", t.GetID(), warn)
			return
		case <-ticker.C:
			if orderCount, err := deps.Repo.GetGameOrderCount(context.Background()); err == nil && orderCount >= step {
				t.log.Infof("[%s] mysql write completed, order count: %d", t.GetID(), orderCount)
				return
			}
		}
	}
}

func (t *Task) fillOrderStats(ctx context.Context, deps *ExecDeps, rpt *v1.TaskCompletionReport, scope OrderScope) {
	if totalBet, totalWin, betOrderCount, _, err := deps.Repo.GetDetailedOrderAmounts(ctx, scope); err == nil {
		rpt.TotalBet, rpt.TotalWin, rpt.OrderCount = totalBet, totalWin, betOrderCount
		if totalBet > 0 {
			rpt.RtpPct = float64(totalWin*100) / float64(totalBet)
		}
		return
	}
	if orderCount, err := deps.Repo.GetGameOrderCount(ctx); err == nil {
		rpt.OrderCount = orderCount
	}
}

// reportMetrics 周期性 Prometheus 指标上报
func (t *Task) reportMetrics(deps *ExecDeps) {
	if deps.Conf.Metrics == nil || !deps.Conf.Metrics.Enabled {
		return
	}
	ctx := context.Background()
	rpt := t.Snapshot(time.Now())
	scope := t.buildOrderScope(deps)
	t.fillOrderStats(ctx, deps, rpt, scope)
	metrics.ReportTask(rpt)
}

// finalize 最终收尾：快照 + 图表上传 + 通知 + 环境清理 + 状态转换
func (t *Task) finalize(deps *ExecDeps) {
	ctx := context.Background()
	rpt := t.Snapshot(time.Now())
	scope := t.buildOrderScope(deps)
	t.fillOrderStats(ctx, deps, rpt, scope)
	rpt.OrderWarning = t.getOrderWarning()

	pre := t.GetStatus()

	t.SetStatus(v1.TaskStatus_TASK_PROCESSING)
	t.uploadChart(deps, ctx, rpt, scope)
	t.sendNotification(deps, ctx, rpt)
	t.cleanupEnvironment(deps, ctx)

	switch pre {
	case v1.TaskStatus_TASK_CANCELLED:
		t.SetStatus(v1.TaskStatus_TASK_CANCELLED)
		t.log.Warnf("[%s] task cancelled, use=%v", t.GetID(), time.Since(t.GetStartAt()))
	case v1.TaskStatus_TASK_FAILED:
		t.SetStatus(v1.TaskStatus_TASK_FAILED)
		t.log.Warnf("[%s] task failed (final report done), use=%v", t.GetID(), time.Since(t.GetStartAt()))
	default:
		t.SetStatus(v1.TaskStatus_TASK_COMPLETED)
		t.log.Infof("[%s] task completed, use=%v", t.GetID(), time.Since(t.GetStartAt()))
	}

	if deps.Conf.Metrics != nil && deps.Conf.Metrics.Enabled {
		metrics.CleanupTaskMetrics(t.GetID(), t.GetGame().GameID())
	}
}

func (t *Task) buildOrderScope(deps *ExecDeps) OrderScope {
	cfg := t.GetConfig()
	excludeAmt := 0.0
	if cfg.BetOrder != nil {
		excludeAmt = cfg.BetOrder.BaseMoney
	}

	scope := OrderScope{
		GameID:     cfg.GameId,
		Merchant:   deps.Conf.Launch.Merchant,
		StartTime:  t.GetStartAt(),
		EndTime:    t.GetFinishedAt(),
		ExcludeAmt: excludeAmt,
	}
	if scope.EndTime.IsZero() {
		scope.EndTime = time.Now()
	}
	return scope
}

func (t *Task) uploadChart(deps *ExecDeps, ctx context.Context, report *v1.TaskCompletionReport, scope OrderScope) {
	if deps.Chart == nil || (!deps.Conf.Chart.GenerateLocal && !deps.Conf.Chart.UploadToS3) {
		return
	}

	pts, err := deps.Repo.QueryGameOrderPoints(ctx, scope)
	if err != nil {
		t.log.Errorf("failed to query game order points: %v", err)
		return
	}

	result, err := deps.Chart.Generate(pts, report.TaskId, report.GameName, scope.Merchant, deps.Conf.Chart.GenerateLocal)
	if err != nil {
		t.log.Errorf("failed to generate chart: %v", err)
		return
	}

	if !deps.Conf.Chart.UploadToS3 {
		return
	}

	htmlKey := "charts/" + report.TaskId + ".html"
	htmlUrl, err := deps.Repo.UploadBytes(ctx, "", htmlKey, "text/html; charset=utf-8", []byte(result.HTMLContent))
	if err != nil {
		t.log.Errorf("failed to upload HTML to S3: %v", err)
		return
	}
	t.SetRecordUrl(htmlUrl)
	report.Url = htmlUrl

	result.HTMLContent = ""
}

func (t *Task) sendNotification(deps *ExecDeps, ctx context.Context, report *v1.TaskCompletionReport) {
	if deps.Notify == nil || !deps.Conf.Notify.Enabled {
		return
	}
	msg := notify.BuildTaskCompletionMessage(report)
	go func() {
		if err := deps.Notify.Send(ctx, msg); err != nil {
			t.log.Warnf("[%s] notify task completion: %v", report.TaskId, err)
		}
	}()
}

func (t *Task) cleanupEnvironment(deps *ExecDeps, ctx context.Context) {
	cleanupCtx, cancel := context.WithTimeout(ctx, cleanupTimeout)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if err := deps.Repo.CleanRedisBySites(cleanupCtx, deps.Conf.Launch.Sites); err != nil {
			t.log.Errorf("[%s] Redis cleanup: %v", t.GetID(), err)
		}
	}()
	go func() {
		defer wg.Done()
		if err := deps.Repo.CleanGameOrderTable(cleanupCtx); err != nil {
			t.log.Errorf("[%s] Mysql delete orders: %v", t.GetID(), err)
		}
	}()
	wg.Wait()
}

func (t *Task) cleanup(deps *ExecDeps, apiClient *APIClient) {
	t.cancel()

	if apiClient != nil {
		apiClient.Close()
	}

	if deps.ReturnMembers != nil {
		deps.ReturnMembers(t.GetID())
	}

	t.mu.Lock()
	t.game = nil
	t.mu.Unlock()
}
