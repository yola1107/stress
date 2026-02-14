package task

import (
	"context"
	"runtime"
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

// 依赖函数定义
type (
	GetOrderCountFunc    func(ctx context.Context) (int64, error)
	GetOrderAmountsFunc  func(ctx context.Context, scope OrderScope) (totalBet, totalWin, betOrderCount, bonusOrderCount int64, err error)
	QueryOrderPointsFunc func(ctx context.Context, scope OrderScope) ([]chart.Point, error)
	UploadBytesFunc      func(ctx context.Context, bucket, key, contentType string, data []byte) (string, error)
	CleanRedisFunc       func(ctx context.Context, sites []string) error
	CleanTableFunc       func(ctx context.Context) error
	ReturnMembersFunc    func(taskID string)
)

// ExecDeps 任务执行依赖
type ExecDeps struct {
	GetOrderCount     GetOrderCountFunc
	GetOrderAmounts   GetOrderAmountsFunc
	QueryOrderPoints  QueryOrderPointsFunc
	UploadBytes       UploadBytesFunc
	CleanRedisBySites CleanRedisFunc
	CleanOrderTable   CleanTableFunc
	ReturnMembers     ReturnMembersFunc
	Conf              *conf.Stress
	Notify            notify.Notifier
	Chart             chart.IGenerator
	OnComplete        func()
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
	capacity := len(members)

	// 初始化资源
	apiClient := NewAPIClient(capacity, NoopSecretProvider, deps.Conf.Launch)
	if err := apiClient.BindSessionEnv(t.Context(), t); err != nil {
		t.log.Errorf("[%s] bind session env failed: %v", t.GetID(), err)
		t.SetStatus(v1.TaskStatus_TASK_FAILED)
		t.cleanup(deps, apiClient, nil)
		return
	}
	antsPool, _ := ants.NewPool(capacity)

	// 启动日志
	t.Monitor()

	// 启动周期 reporter
	stopReporter, wg := t.startReporter(deps)

	// 执行 session
	t.runSessions(members, apiClient, antsPool)

	// 停止 session 阶段
	t.Stop()

	// 等待 DB 写完（阻塞）
	t.waitOrderWrite(deps)

	// 停止 reporter
	stopReporter()

	// 等 final report 完成
	wg.Wait()

	// cleanup
	t.cleanup(deps, apiClient, antsPool)
}

// runSessions 运行所有会话
func (t *Task) runSessions(members []MemberInfo, apiClient *APIClient, antsPool *ants.Pool) {
	var wg sync.WaitGroup
	submitErrCount := 0

	for _, m := range members {
		m := m
		sess := NewSession(m.Name)
		wg.Add(1)
		t.AddActive(1)
		if err := antsPool.Submit(func() {
			defer wg.Done()
			defer t.MarkSessionDone(!sess.IsFailed())
			_ = sess.Execute(apiClient)
		}); err != nil {
			wg.Done()
			t.MarkSessionDone(false)
			submitErrCount++
		}
	}

	if submitErrCount > 0 {
		t.log.Infof("[%s] failed to submit %d sessions to ants pool", t.GetID(), submitErrCount)
	}

	wg.Wait()
}

// Monitor 运行监控：1s 日志输出，task context 取消后退出
func (t *Task) Monitor() {
	go func() {
		tick := time.NewTicker(time.Second)
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

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				t.SetFinishAt()
				t.report(deps, true)
				return
			case <-ticker.C:
				t.report(deps, false)
			}
		}
	}()

	return cancel, &wg
}

// waitOrderWrite 监控订单写入完成
func (t *Task) waitOrderWrite(deps *ExecDeps) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeout := time.After(5 * time.Minute)

	for {
		select {
		case <-timeout:
			t.log.Warnf("[%s] waitOrderWrite timeout, forcing completion", t.GetID())
			return
		case <-ticker.C:
			if orderCount, err := deps.GetOrderCount(context.Background()); err == nil && orderCount >= t.GetStep() {
				return
			}
		}
	}
}

func (t *Task) fillOrderStats(ctx context.Context, deps *ExecDeps, rpt *v1.TaskCompletionReport, scope OrderScope) {
	if totalBet, totalWin, betOrderCount, _, err := deps.GetOrderAmounts(ctx, scope); err == nil {
		rpt.TotalBet, rpt.TotalWin, rpt.OrderCount = totalBet, totalWin, betOrderCount
		if totalBet > 0 {
			rpt.RtpPct = float64(totalWin*100) / float64(totalBet)
		}
		return
	}
	if orderCount, err := deps.GetOrderCount(ctx); err == nil {
		rpt.OrderCount = orderCount
	}
}

// report 上报任务指标
func (t *Task) report(deps *ExecDeps, completed bool) {
	ctx := context.Background()
	rpt := t.Snapshot(time.Now())

	scope := t.buildOrderScope(deps)
	t.fillOrderStats(ctx, deps, rpt, scope)

	metrics.ReportTask(rpt)

	if completed {
		t.SetStatus(v1.TaskStatus_TASK_PROCESSING)
		t.uploadChart(deps, ctx, rpt, scope)
		t.sendNotification(deps, ctx, rpt)
		t.cleanupEnvironment(deps, ctx, scope)
		t.SetStatus(v1.TaskStatus_TASK_COMPLETED)

		t.log.Infof("[%s] task completed, use=%v", t.GetID(), time.Since(t.GetStartAt()))
	}
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
	if deps.Chart == nil || (!deps.Conf.Chart.GenerateLocal && !deps.Conf.Chart.UploadToS3) {
		return
	}

	pts, err := deps.QueryOrderPoints(ctx, scope)
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
	htmlUrl, err := deps.UploadBytes(ctx, "", htmlKey, "text/html; charset=utf-8", []byte(result.HTMLContent))
	if err != nil {
		t.log.Errorf("failed to upload HTML to S3: %v", err)
		return
	}
	t.SetRecordUrl(htmlUrl)
	report.Url = htmlUrl

	// 立即清除大HTML字符串，释放内存
	result.HTMLContent = ""
	runtime.GC()
}

// sendNotification 发送通知
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

// cleanupEnvironment 清理环境
func (t *Task) cleanupEnvironment(deps *ExecDeps, ctx context.Context, scope OrderScope) {
	cleanupCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if err := deps.CleanRedisBySites(cleanupCtx, deps.Conf.Launch.Sites); err != nil {
			t.log.Errorf("[%s] Redis cleanup: %v", t.GetID(), err)
		}
	}()
	go func() {
		defer wg.Done()
		if err := deps.CleanOrderTable(cleanupCtx); err != nil {
			t.log.Errorf("[%s] Mysql delete orders: %v", t.GetID(), err)
		}
	}()
	wg.Wait()
}

// ==================== 资源清理 ====================

// cleanup 清理任务资源
func (t *Task) cleanup(deps *ExecDeps, apiClient *APIClient, antsPool *ants.Pool) {
	t.cancel()

	// 关闭API客户端
	if apiClient != nil {
		apiClient.Close()
	}

	// 释放协程池
	if antsPool != nil {
		antsPool.Release()
	}

	// 归还到成员池
	if deps.ReturnMembers != nil {
		deps.ReturnMembers(t.GetID())
	}

	// 清除game引用
	t.mu.Lock()
	t.game = nil
	t.mu.Unlock()

	// 触发完成回调，通知 UseCase 唤醒调度
	if deps.OnComplete != nil {
		deps.OnComplete()
	}
}
