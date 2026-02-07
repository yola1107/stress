package task

import (
	"context"
	"net/http"
	"sync"
	"time"

	v1 "stress/api/stress/v1"
	"stress/internal/biz/chart"
	"stress/internal/biz/member"
	"stress/internal/biz/metrics"
	"stress/internal/conf"
	"stress/internal/notify"

	"github.com/panjf2000/ants/v2"
)

// ==================== 依赖定义 ====================

// ExecDeps 任务执行依赖
type ExecDeps struct {
	Repo       OrderRepo
	MemberPool MemberReleaser
	Conf       *conf.Stress
	Notify     notify.Notifier
	Chart      chart.IGenerator
	OnComplete func() // cleanup 完成后回调，用于唤醒调度器
}

// OrderRepo 订单数据接口
type OrderRepo interface {
	GetGameOrderCount(ctx context.Context) (int64, error)
	GetDetailedOrderAmounts(ctx context.Context, scope OrderScope) (totalBet, totalWin, betOrderCount, bonusOrderCount int64, err error)
	QueryGameOrderPoints(ctx context.Context, scope OrderScope) ([]chart.Point, error)
	UploadBytes(ctx context.Context, bucket, key, contentType string, data []byte) (string, error)
	CleanRedisBySites(ctx context.Context, sites []string) error
	DeleteOrdersByScope(ctx context.Context, scope OrderScope) (int64, error)
}

// MemberReleaser 成员释放接口
type MemberReleaser interface {
	Release(taskID string)
}

// MemberInfo 成员信息（避免循环依赖）
type MemberInfo = member.Info

// ProtobufChecker 检查游戏是否需要 protobuf
type ProtobufChecker interface {
	RequireProtobuf(gameID int64) bool
}

// OrderScope 订单查询范围
type OrderScope struct {
	GameID     int64
	Merchant   string
	StartTime  time.Time
	EndTime    time.Time
	ExcludeAmt float64
}

// ==================== 核心执行 ====================

// Execute 执行任务
func (t *Task) Execute(members []MemberInfo, checker ProtobufChecker, deps *ExecDeps) {
	if !t.CompareAndSetStatus(v1.TaskStatus_TASK_PENDING, v1.TaskStatus_TASK_RUNNING) {
		t.log.Warnf("[%s] task status changed, skip execution", t.GetID())
		return
	}

	t.SetStartAt()

	config := t.GetConfig()
	capacity := len(members)
	if config.BetBonus != nil && config.BetBonus.Enable {
		t.SetBonusConfig(config.BetBonus)
	}
	t.AddActive(int64(capacity))

	// 初始化资源
	httpClient := NewHTTPClient(capacity)
	apiClient := NewAPIClient(httpClient, NoopSecretProvider, t.game, checker.RequireProtobuf, deps.Conf.Launch)
	antsPool, _ := ants.NewPool(capacity)

	// 启动监控
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		t.Monitor()
	}()

	// 启动上报和订单监控
	dbWriteDone := make(chan struct{})
	t.wg.Add(2)
	go func() {
		defer t.wg.Done()
		t.startReporting(deps, dbWriteDone)
	}()
	go func() {
		defer t.wg.Done()
		t.monitorOrderWrite(deps, dbWriteDone)
	}()

	// 执行会话
	t.runSessions(members, apiClient, antsPool)

	// 停止任务上下文，触发所有 goroutine 退出
	t.Stop()

	// 等待所有 goroutine 完成
	t.wg.Wait()

	// 清理资源
	t.cleanup(deps, apiClient, httpClient, antsPool)
}

// runSessions 运行所有会话
func (t *Task) runSessions(members []MemberInfo, apiClient *APIClient, antsPool *ants.Pool) {
	var wg sync.WaitGroup
	submitErrCount := 0

	for _, m := range members {
		m := m
		sess := NewSession(m.ID, m.Name)
		wg.Add(1)
		if err := antsPool.Submit(func() {
			defer wg.Done()
			defer t.MarkSessionDone(!sess.IsFailed())
			_ = sess.Execute(t.Context(), apiClient, t, NoopSecretProvider)
		}); err != nil {
			wg.Done()
			t.MarkSessionDone(false)
			submitErrCount++
		}
	}

	if submitErrCount > 0 {
		t.log.Infof("[%s] failed to submit %d sessions to ants pool", t.GetID(), submitErrCount)
	}

	// 等待会话完成，支持 context 取消
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-t.Context().Done():
		t.log.Infof("[%s] runSessions cancelled, waiting for sessions to complete", t.GetID())
		<-done
	}
}

// ==================== 上报与监控 ====================

const reportInterval = 5 * time.Second

// startReporting 启动任务指标上报
func (t *Task) startReporting(deps *ExecDeps, doneChan <-chan struct{}) {
	ticker := time.NewTicker(reportInterval)
	defer ticker.Stop()

	t.report(deps, false)

	for {
		select {
		case <-doneChan:
			t.SetFinishAt()
			t.report(deps, true)
			return
		case <-ticker.C:
			t.report(deps, false)
		}
	}
}

// monitorOrderWrite 监控订单写入完成
func (t *Task) monitorOrderWrite(deps *ExecDeps, done chan<- struct{}) {
	<-t.Context().Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for i := 0; i < 60; i++ {
		if orderCount, err := deps.Repo.GetGameOrderCount(context.Background()); err == nil && orderCount >= t.GetStep() {
			close(done)
			return
		}
		<-ticker.C
	}

	t.log.Warnf("[%s] monitorOrderWrite timeout, forcing completion", t.GetID())
	close(done)
}

// report 上报任务指标
func (t *Task) report(deps *ExecDeps, completed bool) {
	ctx := context.Background()
	rpt := t.Snapshot(time.Now())

	// 填充订单统计
	scope := t.buildOrderScope(deps)
	if totalBet, totalWin, betOrderCount, _, err := deps.Repo.GetDetailedOrderAmounts(ctx, scope); err == nil {
		rpt.TotalBet, rpt.TotalWin, rpt.OrderCount = totalBet, totalWin, betOrderCount
		if totalBet > 0 {
			rpt.RtpPct = float64(totalWin*100) / float64(totalBet)
		}
	} else if orderCount, err := deps.Repo.GetGameOrderCount(ctx); err == nil {
		rpt.OrderCount = orderCount
	}

	metrics.ReportTask(rpt)

	if !completed {
		return
	}

	// 任务完成，执行后续处理
	t.handleCompletion(deps, ctx, rpt)
}

// ==================== 任务完成处理 ====================

// handleCompletion 处理任务完成
func (t *Task) handleCompletion(deps *ExecDeps, ctx context.Context, report *v1.TaskCompletionReport) {
	t.SetStatus(v1.TaskStatus_TASK_PROCESSING)
	defer t.SetStatus(v1.TaskStatus_TASK_COMPLETED)

	scope := t.buildOrderScope(deps)
	t.uploadChart(deps, ctx, report, scope)
	t.sendNotification(deps, ctx, report)
	t.cleanupEnvironment(deps, ctx, scope)

	t.log.Infof("[%s] task completed, use=%v", t.GetID(), time.Since(t.GetStartAt()))
}

// buildOrderScope 构建订单查询范围
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

// uploadChart 上传图表到 S3
func (t *Task) uploadChart(deps *ExecDeps, ctx context.Context, report *v1.TaskCompletionReport, scope OrderScope) {
	if deps.Chart == nil || !deps.Conf.Chart.Enabled {
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

	if !deps.Conf.Chart.UploadToS3 || result.HTMLContent == "" {
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
}

// sendNotification 发送通知
func (t *Task) sendNotification(deps *ExecDeps, ctx context.Context, report *v1.TaskCompletionReport) {
	if deps.Notify == nil || !deps.Conf.Notify.Enabled {
		return
	}

	msg := notify.BuildTaskCompletionMessage(report)
	if err := deps.Notify.Send(ctx, msg); err != nil {
		t.log.Warnf("[%s] notify task completion: %v", report.TaskId, err)
	}
}

// cleanupEnvironment 清理环境
func (t *Task) cleanupEnvironment(deps *ExecDeps, ctx context.Context, scope OrderScope) {
	cleanupCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	if err := deps.Repo.CleanRedisBySites(cleanupCtx, deps.Conf.Launch.Sites); err != nil {
		t.log.Errorf("[%s] Redis cleanup: %v", t.GetID(), err)
	}

	if _, err := deps.Repo.DeleteOrdersByScope(cleanupCtx, scope); err != nil {
		t.log.Errorf("[%s] Mysql delete orders: %v", t.GetID(), err)
	}
}

// ==================== 资源清理 ====================

// cleanup 清理任务资源
func (t *Task) cleanup(deps *ExecDeps, apiClient *APIClient, httpClient *http.Client, antsPool *ants.Pool) {
	apiClient.Close()
	httpClient.CloseIdleConnections()
	antsPool.Release()
	deps.MemberPool.Release(t.GetID())

	// 触发完成回调，通知 UseCase 唤醒调度
	if deps.OnComplete != nil {
		deps.OnComplete()
	}
}
