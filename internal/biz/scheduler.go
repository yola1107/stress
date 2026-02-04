package biz

import (
	"context"
	"fmt"
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
	mp, tp := uc.memberPool, uc.taskPool
	for {
		select {
		case <-uc.ctx.Done():
			return
		default:
		}

		taskID, t, ok := tp.PeekPending()
		if !ok {
			break
		}
		if t == nil || t.GetStatus() != v1.TaskStatus_TASK_PENDING || t.GetConfig() == nil {
			tp.DropPendingHead()
			continue
		}
		config := t.GetConfig()
		if !mp.CanAllocate(int(config.MemberCount)) {
			break
		}
		if !tp.DequeuePending(taskID) {
			continue
		}
		allocated := mp.Allocate(taskID, int(config.MemberCount))
		if allocated == nil {
			tp.RequeueAtHead(taskID)
			break
		}
		go uc.ExecuteTask(t, config, allocated)
	}
}

func (uc *UseCase) ExecuteTask(t *task.Task, c *v1.TaskConfig, members []member.Info) {
	if t.GetStatus() != v1.TaskStatus_TASK_PENDING {
		return
	}

	taskID := t.GetID()
	capacity := len(members)
	t.SetStart(int64(capacity), c.BetBonus)

	g, _ := uc.GetGame(c.GameId)
	httpClient := user.NewHTTPClient(capacity)
	apiClient := user.NewAPIClient(httpClient, user.NoopSecretProvider, g, uc.gamePool.RequireProtobuf, uc.conf.Launch)
	antsPool, _ := ants.NewPool(capacity)

	// 创建DB写入完成channel
	dbWriteDone := make(chan struct{})

	// 启动任务指标上报（监听DB写入完成）
	go uc.startTaskReporting(taskID, t, dbWriteDone)

	// 启动DB写入状态检查
	go uc.monitorOrderWriteCompletion(t, dbWriteDone)

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

	// 资源释放
	t.Stop()
	t.SetStatus(v1.TaskStatus_TASK_COMPLETED)
	apiClient.Close() // 关闭APIClient，释放其内部资源
	httpClient.CloseIdleConnections()
	antsPool.Release()
	uc.memberPool.Release(taskID)

}

// startTaskReporting 启动任务指标上报
func (uc *UseCase) startTaskReporting(taskID string, t *task.Task, doneChan <-chan struct{}) {
	reportTicker := time.NewTicker(reportInterval)
	uc.ReportTask(t, false) // 先上报一次

	go func() {
		defer reportTicker.Stop()
		for {
			select {
			case <-doneChan:
				t.SetFinishAt()
				uc.ReportTask(t, true)
				uc.Schedule() // 调度下一任务 唤醒
				return
			case <-reportTicker.C:
				// 定期上报任务统计指标
				uc.ReportTask(t, false)
			}
		}
	}()
}

// monitorOrderWriteCompletion 监控订单写入DB完成状态
func (uc *UseCase) monitorOrderWriteCompletion(t *task.Task, done chan<- struct{}) {
	//<-t.Context().Done() // 等待任务完成
	//
	//ticker := time.NewTicker(monitorInterval)
	//defer ticker.Stop()
	//
	//timeout := time.After(monitorTimeout)
	//errCount := 0
	//
	//for {
	//	select {
	//	case <-timeout:
	//		uc.log.Warnf("monitor order completion timeout after %v", monitorTimeout)
	//		close(done)
	//		return
	//	case <-ticker.C:
	//		count, err := uc.repo.GetGameOrderCount(context.Background())
	//		if err != nil {
	//			errCount++
	//			if errCount >= 10 {
	//				uc.log.Warnf("monitor order failed %d times, give up", errCount)
	//				close(done)
	//				return
	//			}
	//			continue
	//		}
	//		errCount = 0 // 重置错误计数
	//		if count >= t.GetStep() {
	//			close(done)
	//			return
	//		}
	//	}
	//}

	// 等待任务完成或取消
	<-t.Context().Done()

	// 任务完成后，开始每5秒检查DB是否写完
	for {
		if orderCount, err := uc.repo.GetGameOrderCount(context.Background()); err == nil && orderCount >= t.GetStep() {
			close(done)
			return
		}
		time.Sleep(5 * time.Second)
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
	t.Stop() // 先停止，触发 runTaskSessions 退出
	// 仅当任务未在运行时释放成员；若 RUNNING，runTaskSessions 退出时会自行 Release
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
	// 先 Cancel 再 Release，避免任务仍在跑时成员被提前复用
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

	scope := OrderScope{
		GameID:     report.GameId,
		Merchant:   uc.conf.Launch.Merchant,
		StartTime:  t.GetCreatedAt(),
		EndTime:    t.GetFinishedAt(),
		ExcludeAmt: t.GetConfig().BetOrder.BaseMoney,
	}

	// 上报完整指标
	metrics.ReportTask(report)

	// 任务未结束
	if !completed {
		return
	}

	// 汇总订单数据 Statistics 数据上报给s3
	uc.sendS3Bucket(ctx, report.TaskId, report.GameName, scope)

	// 飞书通知
	uc.sendNotification(ctx, report)

	// 环境清理
	uc.performEnvironmentCleanup(t)

	uc.log.Infof("[%s] task completed, use=%v", t.GetID(), time.Since(t.GetCreatedAt()))
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

func (uc *UseCase) sendS3Bucket(ctx context.Context, taskId, gameName string, scope OrderScope) {
	// 检查图表配置是否启用
	if uc.conf.Chart == nil || !uc.conf.Chart.Enabled {
		return
	}

	pts, err := uc.repo.QueryGameOrderPoints(ctx, scope)
	if err != nil {
		uc.log.Errorf("failed to query game order points: %v", err)
		return
	}

	// 生成图表（一次生成，同时获取 HTML 内容和文件路径）
	result, err := uc.chartGen.Generate(pts, taskId, gameName, scope.Merchant, uc.conf.Chart.GenerateLocal)
	if err != nil {
		uc.log.Errorf("failed to generate chart: %v", err)
		return
	}

	// 记录本地保存结果
	if uc.conf.Chart.GenerateLocal && result.FilePath != "" {
		uc.log.Infof("Chart saved locally at: %s", result.FilePath)
	}

	// 上传到 S3
	if uc.conf.Chart.UploadToS3 && result.HTMLContent != "" {
		htmlKey := fmt.Sprintf("charts/%s/%s.html", scope.Merchant, taskId)
		htmlUrl, err := uc.repo.UploadBytes(ctx, "", htmlKey, "text/html; charset=utf-8", []byte(result.HTMLContent))
		if err != nil {
			uc.log.Errorf("failed to upload HTML to S3: %v", err)
		} else {
			uc.log.Infof("Chart uploaded to S3: %s", htmlUrl)
		}
	}
}

// sendNotification 发送飞书通知
func (uc *UseCase) sendNotification(ctx context.Context, report *v1.TaskCompletionReport) {
	// 检查通知配置
	if uc.conf.Notify == nil || !uc.conf.Notify.Enabled {
		uc.log.Debug("Notifications disabled, skipping notification")
		return
	}

	if uc.notify == nil {
		return
	}
	msg := notify.BuildTaskCompletionMessage(report)
	if err := uc.notify.Send(ctx, msg); err != nil {
		uc.log.Warnf("[%s] notify task completion: %v", report.TaskId, err)
	}
}

// performEnvironmentCleanup 执行环境清理工作
func (uc *UseCase) performEnvironmentCleanup(t *task.Task) {
	taskID := t.GetID()
	scope := uc.buildOrderScopeFromTask(t)

	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()
	// 清理Redis缓存
	if err := uc.repo.CleanRedisBySites(ctx, uc.conf.Launch.Sites); err != nil {
		uc.log.Errorf("[%s] Redis cleanup: %v", taskID, err)
	}
	// 删除测试订单数据
	if _, err := uc.repo.DeleteOrdersByScope(ctx, scope); err != nil {
		uc.log.Errorf("[%s] Mysql delete orders: %v", taskID, err)
	}
}

func (uc *UseCase) buildOrderScopeFromTask(t *task.Task) OrderScope {
	cfg := t.GetConfig()
	s := OrderScope{
		GameID:    cfg.GameId,
		Merchant:  uc.conf.Launch.Merchant,
		StartTime: t.GetCreatedAt(),
		EndTime:   t.GetFinishedAt(),
	}
	if s.EndTime.IsZero() {
		s.EndTime = time.Now()
	}
	if cfg.BetOrder != nil && cfg.BetOrder.BaseMoney > 0 {
		s.ExcludeAmt = cfg.BetOrder.BaseMoney
	}
	return s
}
