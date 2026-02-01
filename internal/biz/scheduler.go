package biz

import (
	"context"
	"fmt"
	"stress/internal/biz/member"
	"stress/internal/biz/user"
	"sync"
	"time"

	v1 "stress/api/stress/v1"
	"stress/internal/biz/game/base"
	"stress/internal/biz/metrics"
	"stress/internal/biz/task"
	"stress/internal/notify"
	"stress/pkg/xgo"

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
	client := user.NewAPIClient(httpClient, user.NoopSecretProvider, g, uc.gamePool.RequireProtobuf)
	antsPool, _ := ants.NewPool(capacity)

	closeChan := make(chan struct{})
	go func() {
		for {
			select {
			case <-closeChan:

			default:
			}

		}
	}()

	var wg sync.WaitGroup
	wg.Add(len(members))
	for _, m := range members {
		m := m
		sess := user.NewSession(m.ID, m.Name, t)
		if err := antsPool.Submit(func() {
			defer wg.Done()
			defer t.MarkSessionDone(!sess.IsFailed())
			_ = sess.Execute(t.Context(), client, user.NoopSecretProvider)
		}); err != nil {
			wg.Done()
			t.MarkSessionDone(false)
		}
	}
	wg.Wait()

	// 资源释放
	t.Stop()
	httpClient.CloseIdleConnections()
	uc.memberPool.Release(taskID)
	antsPool.Release()

	// 释放成员、置完成态、等待订单落库→通知飞书→清理环境、调度下一批
	if t.GetStatus() == v1.TaskStatus_TASK_RUNNING {
		t.SetStatus(v1.TaskStatus_TASK_COMPLETED)

		// 阻塞等待DB写库完成
		ch := make(chan struct{})

		//scope := buildOrderScopeFromTask(t)
		//uc.finishTaskCleanup(taskID, scope, t.GetStep(), t)
	}

	uc.Schedule()

}

// startTaskReporting 启动任务指标上报
func (uc *UseCase) startTaskReporting(ctx context.Context, t *task.Task) {
	reportTicker := time.NewTicker(reportInterval)
	go func() {
		defer reportTicker.Stop()
		for {
			select {
			case <-ctx.Done():
				// 任务结束，进行最终完整上报
				uc.reportFinalTaskMetrics(t)
				return
			case <-reportTicker.C:
				// 定期上报任务统计指标
				report := t.Snapshot(time.Now())
				metrics.ReportTask(report)
			}
		}
	}()
}

// reportFinalTaskMetrics 任务完成时进行完整指标上报（包含订单数据）
func (uc *UseCase) reportFinalTaskMetrics(t *task.Task) {
	report := t.Snapshot(time.Now())

	// 填充完整的订单统计数据用于最终指标上报
	ctx := context.Background()
	if totalBet, totalWin, betOrderCount, _, err := uc.repo.GetDetailedOrderAmounts(ctx); err == nil {
		report.TotalBet = totalBet
		report.TotalWin = totalWin
		report.OrderCount = betOrderCount
		report.RtpPct = xgo.Pct(totalWin, totalBet)
	} else if orderCount, err := uc.repo.GetGameOrderCount(ctx); err == nil {
		report.OrderCount = orderCount
	}

	// 上报完整指标
	metrics.ReportTask(report)
}

// CreateTask 创建并尝试运行
func (uc *UseCase) CreateTask(ctx context.Context, g base.IGame, config *v1.TaskConfig) (*task.Task, error) {
	taskID, err := uc.repo.NextTaskID(ctx, config.GameId)
	if err != nil {
		return nil, fmt.Errorf("failed to generate task ID: %w", err)
	}

	t, err := task.NewTask(taskID, g, config)
	if err != nil {
		return nil, err
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
		return fmt.Errorf("task not found")
	}
	// 先 Cancel 再 Release，避免任务仍在跑时成员被提前复用
	if err := t.Cancel(); err != nil {
		return err
	}
	uc.memberPool.Release(id)
	uc.Schedule()
	return nil
}

// processTaskFinish 等待订单落库 → 飞书通知 → 清理环境（阻塞）
func (uc *UseCase) processTaskFinish(taskID string, t *task.Task) {
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()

	threshold := t.GetStep()
	scope := buildOrderScopeFromTask(t)

	// 等待订单落库
	uc.waitForOrdersToComplete(ctx, taskID, threshold, scope)

	// 发送任务完成通知
	uc.sendTaskCompletionNotification(ctx, taskID, t)

	// 执行环境清理
	uc.performEnvironmentCleanup(ctx, taskID, scope)
}

// waitForOrdersToComplete 等待订单数据完全落库
func (uc *UseCase) waitForOrdersToComplete(ctx context.Context, taskID string, threshold int64, scope OrderScope) {
	time.Sleep(cleanupRetryDelay)

	for ctx.Err() == nil {
		n, err := uc.repo.GetOrderCountByScope(ctx, scope)
		if err == nil && n >= threshold {
			break
		}
		if err != nil {
			uc.log.Errorf("[%s] order count: %v", taskID, err)
		}
		time.Sleep(cleanupRetryDelay)
	}
	if ctx.Err() != nil {
		uc.log.Warnf("[%s] wait orders timeout", taskID)
	}
}

// sendTaskCompletionNotification 发送任务完成通知
func (uc *UseCase) sendTaskCompletionNotification(ctx context.Context, taskID string, t *task.Task) {
	if uc.notify == nil {
		return
	}

	report := t.Snapshot(time.Now())
	// 填充订单统计数据
	if totalBet, totalWin, betOrderCount, _, err := uc.repo.GetDetailedOrderAmounts(ctx); err == nil {
		report.TotalBet = totalBet
		report.TotalWin = totalWin
		report.OrderCount = betOrderCount
		report.RtpPct = xgo.Pct(totalWin, totalBet)
	} else if orderCount, err := uc.repo.GetGameOrderCount(ctx); err == nil {
		report.OrderCount = orderCount
	}
	msg := notify.BuildTaskCompletionMessage(report)
	if err := uc.notify.Send(ctx, msg); err != nil {
		uc.log.Warnf("[%s] notify task completion: %v", t.GetID(), err)
	}
}

// performEnvironmentCleanup 执行环境清理工作
func (uc *UseCase) performEnvironmentCleanup(ctx context.Context, taskID string, scope OrderScope) {
	// 清理Redis缓存
	if err := uc.repo.CleanRedisBySites(ctx, uc.c.Sites); err != nil {
		uc.log.Errorf("[%s] Redis cleanup: %v", taskID, err)
	}

	// 删除测试订单数据
	if _, err := uc.repo.DeleteOrdersByScope(ctx, scope); err != nil {
		uc.log.Errorf("[%s] delete orders: %v", taskID, err)
	}

	uc.log.Infof("[%s] task finished", taskID)
}

// buildOrderScopeFromTask 从任务构建订单范围
func buildOrderScopeFromTask(t *task.Task) OrderScope {
	cfg := t.GetConfig()
	s := OrderScope{GameID: cfg.GameId, Merchant: cfg.Merchant, StartTime: t.GetCreatedAt(), EndTime: t.GetFinishedAt()}
	if s.EndTime.IsZero() {
		s.EndTime = time.Now()
	}
	if cfg.BetOrder != nil && cfg.BetOrder.BaseMoney > 0 {
		s.ExcludeAmt = cfg.BetOrder.BaseMoney
	}
	return s
}
