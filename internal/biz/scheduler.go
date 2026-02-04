package biz

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	v1 "stress/api/stress/v1"
	"stress/internal/biz/game/base"
	"stress/internal/biz/member"
	"stress/internal/biz/metrics"
	"stress/internal/biz/task"
	"stress/internal/biz/user"
	"stress/internal/notify"
	"stress/pkg/xgo"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/panjf2000/ants/v2"
)

const (
	reportInterval = 5 * time.Second // Prometheus 上报间隔
)

// Schedule 从待调度队列取任务、分配成员、启动压测
func (uc *UseCase) Schedule() {
	for {
		select {
		case <-uc.ctx.Done():
			return
		default:
		}

		taskID, t, ok := uc.taskPool.PeekPending()
		if !ok {
			break
		}
		if t == nil || t.GetStatus() != v1.TaskStatus_TASK_PENDING || t.GetConfig() == nil {
			uc.taskPool.DropPendingHead()
			continue
		}
		config := t.GetConfig()
		if !uc.memberPool.CanAllocate(int(config.MemberCount)) {
			break
		}
		if !uc.taskPool.DequeuePending(taskID) {
			continue
		}
		allocated := uc.memberPool.Allocate(taskID, int(config.MemberCount))
		if allocated == nil {
			uc.taskPool.RequeueAtHead(taskID)
			break
		}
		go uc.ExecuteTask(t, allocated)
	}
}

// ExecuteTask 执行压测任务
func (uc *UseCase) ExecuteTask(t *task.Task, members []member.Info) {
	if !t.CompareAndSetStatus(v1.TaskStatus_TASK_PENDING, v1.TaskStatus_TASK_RUNNING) {
		uc.log.Warnf("[%s] task status changed, skip execution", t.GetID())
		return
	}

	config := t.GetConfig()
	capacity := len(members)
	if config.BetBonus != nil && config.BetBonus.Enable {
		t.SetBonusConfig(config.BetBonus)
	}
	t.AddActive(int64(capacity))
	go t.Monitor()

	// 初始化客户端
	g, _ := uc.GetGame(config.GameId)
	httpClient := user.NewHTTPClient(capacity)
	apiClient := user.NewAPIClient(httpClient, user.NoopSecretProvider, g, uc.gamePool.RequireProtobuf, uc.conf.Launch)
	antsPool, _ := ants.NewPool(capacity)

	// 启动监控和上报
	dbWriteDone := make(chan struct{})
	go uc.startTaskReporting(t, dbWriteDone)
	go uc.monitorOrderWriteCompletion(t, dbWriteDone)

	// 执行会话
	var wg sync.WaitGroup
	wg.Add(len(members))
	for _, m := range members {
		m := m
		sess := user.NewSession(m.ID, m.Name)
		if err := antsPool.Submit(func() {
			defer wg.Done()
			defer t.MarkSessionDone(!sess.IsFailed())
			_ = sess.Execute(t.Context(), apiClient, t, user.NoopSecretProvider)
		}); err != nil {
			wg.Done()
			t.MarkSessionDone(false)
		}
	}
	wg.Wait()

	// 清理资源
	uc.cleanupTaskResources(t, apiClient, httpClient, antsPool)
}

// cleanupTaskResources 清理任务相关资源
func (uc *UseCase) cleanupTaskResources(t *task.Task, apiClient *user.APIClient, httpClient *http.Client, antsPool *ants.Pool) {
	t.Stop()
	t.SetStatus(v1.TaskStatus_TASK_COMPLETED)
	apiClient.Close()
	httpClient.CloseIdleConnections()
	antsPool.Release()
	uc.memberPool.Release(t.GetID())
}

// startTaskReporting 启动任务指标上报
func (uc *UseCase) startTaskReporting(t *task.Task, doneChan <-chan struct{}) {
	reportTicker := time.NewTicker(reportInterval)
	defer reportTicker.Stop()

	uc.ReportTask(t, false) // 先上报一次

	for {
		select {
		case <-doneChan:
			t.SetFinishAt()
			uc.ReportTask(t, true)
			uc.Schedule() // 调度下一任务
			return
		case <-reportTicker.C:
			uc.ReportTask(t, false)
		}
	}
}

// monitorOrderWriteCompletion 监控订单写入DB完成状态
func (uc *UseCase) monitorOrderWriteCompletion(t *task.Task, done chan<- struct{}) {
	<-t.Context().Done() // 等待任务完成或取消

	// 任务完成后，每5秒检查DB是否写完
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		if orderCount, err := uc.repo.GetGameOrderCount(context.Background()); err == nil && orderCount >= t.GetStep() {
			close(done)
			return
		}
		<-ticker.C
	}
}

// CreateTask 创建并尝试运行
func (uc *UseCase) CreateTask(ctx context.Context, g base.IGame, config *v1.TaskConfig) (*task.Task, error) {
	taskID, err := uc.repo.NextTaskID(ctx, config.GameId)
	if err != nil {
		return nil, errors.Newf(500, "TASK_ID_GENERATE_FAILED", "failed to generate task ID: %v", err)
	}

	t, err := task.NewTask(uc.ctx, taskID, g, config)
	if err != nil {
		return nil, errors.Newf(500, "TASK_CREATE_FAILED", "failed to create task: %v", err)
	}

	uc.taskPool.Add(t)
	uc.Schedule()
	return t, nil
}

// DeleteTask 删除任务并释放成员
func (uc *UseCase) DeleteTask(id string) error {
	t, ok := uc.taskPool.Remove(id)
	if !ok {
		return nil
	}

	t.Stop()
	// 仅当任务未在运行时释放成员
	if t.GetStatus() != v1.TaskStatus_TASK_RUNNING {
		uc.memberPool.Release(id)
	}
	uc.Schedule()
	return nil
}

// CancelTask 取消任务并释放成员
func (uc *UseCase) CancelTask(id string) error {
	t, ok := uc.taskPool.Get(id)
	if !ok {
		return errors.NotFound("TASK_NOT_FOUND", "task not found")
	}

	if err := t.Cancel(); err != nil {
		return errors.Newf(500, "TASK_CANCEL_FAILED", "cancel task failed: %v", err)
	}
	uc.memberPool.Release(id)
	uc.Schedule()
	return nil
}

// ReportTask 任务完成时进行完整指标上报（包含订单数据）
func (uc *UseCase) ReportTask(t *task.Task, completed bool) {
	ctx := context.Background()
	report := t.Snapshot(time.Now())
	uc.fillOrderStats(ctx, report)

	// 上报 Prometheus 指标
	metrics.ReportTask(report)

	// 任务未完成，仅上报指标
	if !completed {
		return
	}

	// 构建订单查询范围
	cfg := t.GetConfig()
	excludeAmt := 0.0
	if cfg.BetOrder != nil {
		excludeAmt = cfg.BetOrder.BaseMoney
	}

	scope := OrderScope{
		GameID:     cfg.GameId,
		Merchant:   uc.conf.Launch.Merchant,
		StartTime:  t.GetCreatedAt(),
		EndTime:    t.GetFinishedAt(),
		ExcludeAmt: excludeAmt,
	}
	if scope.EndTime.IsZero() {
		scope.EndTime = time.Now()
	}

	// 任务完成后的处理
	uc.handleTaskCompletion(ctx, t, report, scope)
	uc.log.Infof("[%s] task completed, use=%v", t.GetID(), time.Since(t.GetCreatedAt()))
}

// handleTaskCompletion 处理任务完成后的操作：S3上传、通知、清理
func (uc *UseCase) handleTaskCompletion(ctx context.Context, t *task.Task, report *v1.TaskCompletionReport, scope OrderScope) {
	// 1. 上传图表到 S3
	uc.sendS3Bucket(ctx, t, report, scope)

	// 2. 发送飞书通知
	uc.sendNotification(ctx, report)

	// 3. 环境清理
	uc.performEnvironmentCleanup(ctx, t, scope)
}

// fillOrderStats 填充订单统计数据
func (uc *UseCase) fillOrderStats(ctx context.Context, report *v1.TaskCompletionReport) {
	if totalBet, totalWin, betOrderCount, _, err := uc.repo.GetDetailedOrderAmounts(ctx); err == nil {
		report.TotalBet = totalBet
		report.TotalWin = totalWin
		report.OrderCount = betOrderCount
		report.RtpPct = xgo.Pct(totalWin, totalBet)
	} else if orderCount, err := uc.repo.GetGameOrderCount(ctx); err == nil {
		report.OrderCount = orderCount
	}
}

// sendS3Bucket 上传图表到 S3
func (uc *UseCase) sendS3Bucket(ctx context.Context, t *task.Task, report *v1.TaskCompletionReport, scope OrderScope) {
	if uc.conf.Chart == nil || !uc.conf.Chart.Enabled {
		return
	}

	pts, err := uc.repo.QueryGameOrderPoints(ctx, scope)
	if err != nil {
		uc.log.Errorf("failed to query game order points: %v", err)
		return
	}

	result, err := uc.chartGen.Generate(pts, report.TaskId, report.GameName, scope.Merchant, uc.conf.Chart.GenerateLocal)
	if err != nil {
		uc.log.Errorf("failed to generate chart: %v", err)
		return
	}

	// 上传到 S3
	if uc.conf.Chart.UploadToS3 && result.HTMLContent != "" {
		htmlKey := fmt.Sprintf("charts/%s/%s.html", scope.Merchant, report.TaskId)
		htmlUrl, err := uc.repo.UploadBytes(ctx, "", htmlKey, "text/html; charset=utf-8", []byte(result.HTMLContent))
		if err != nil {
			uc.log.Errorf("failed to upload HTML to S3: %v", err)
			return
		}
		t.SetRecordUrl(htmlUrl)
		report.Url = htmlUrl
		uc.log.Infof("[%s] Chart uploaded to S3: %s", report.TaskId, htmlUrl)
	}
}

// sendNotification 发送飞书通知
func (uc *UseCase) sendNotification(ctx context.Context, report *v1.TaskCompletionReport) {
	if uc.conf.Notify == nil || !uc.conf.Notify.Enabled || uc.notify == nil {
		return
	}

	msg := notify.BuildTaskCompletionMessage(report)
	if err := uc.notify.Send(ctx, msg); err != nil {
		uc.log.Warnf("[%s] notify task completion: %v", report.TaskId, err)
	}
}

// performEnvironmentCleanup 执行环境清理工作
func (uc *UseCase) performEnvironmentCleanup(ctx context.Context, t *task.Task, scope OrderScope) {
	cleanupCtx, cancel := context.WithTimeout(ctx, cleanupTimeout)
	defer cancel()

	// 清理 Redis 缓存
	if err := uc.repo.CleanRedisBySites(cleanupCtx, uc.conf.Launch.Sites); err != nil {
		uc.log.Errorf("[%s] Redis cleanup: %v", t.GetID(), err)
	}

	// 删除测试订单数据
	if _, err := uc.repo.DeleteOrdersByScope(cleanupCtx, scope); err != nil {
		uc.log.Errorf("[%s] Mysql delete orders: %v", t.GetID(), err)
	}
}
