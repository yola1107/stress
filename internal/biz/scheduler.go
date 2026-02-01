package biz

import (
	"context"
	"fmt"
	"sync"
	"time"

	v1 "stress/api/stress/v1"
	"stress/internal/biz/game/base"
	"stress/internal/biz/metrics"
	"stress/internal/biz/stats"
	"stress/internal/biz/task"
	"stress/internal/biz/user"
	"stress/internal/notify"
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
			break // 无任务，退出循环
		}
		if t == nil || t.GetStatus() != v1.TaskStatus_TASK_PENDING || t.GetConfig() == nil {
			tp.DropPendingHead() // 无效任务，丢弃
			continue
		}
		config := t.GetConfig()
		if !mp.CanAllocate(int(config.MemberCount)) {
			break // 无足够成员，等待
		}
		if !tp.DequeuePending(taskID) {
			continue // 获取失败，可能被其他goroutine处理
		}
		allocated := mp.Allocate(taskID, int(config.MemberCount))
		if allocated == nil {
			tp.RequeueAtHead(taskID) // 分配失败，重新入队
			break
		}
		if err := t.Start(); err != nil {
			mp.Release(taskID) // 启动失败，释放成员
			continue
		}
		go uc.runTaskSessions(t)
	}
}

// runTaskSessions 执行单任务完整生命周期
func (uc *UseCase) runTaskSessions(t *task.Task) {
	taskID := t.GetID()
	cfg := t.GetConfig()
	members := uc.memberPool.GetAllocated(taskID)
	if len(members) == 0 {
		t.Stop()
		uc.memberPool.Release(taskID)
		uc.Schedule()
		return
	}

	g, _ := uc.GetGame(cfg.GameId)
	checker := uc.gamePool.RequireProtobuf

	httpClient := user.NewHTTPClient(int(cfg.MemberCount))
	defer httpClient.CloseIdleConnections() // 确保HTTP连接被释放

	client := user.NewAPIClient(httpClient, user.NoopSecretProvider, g, checker)
	t.SetBonusConfig(getBonusConfigForGame(cfg))

	runCtx, stopRun := context.WithCancel(t.Context())
	defer stopRun()

	go t.Monitor(runCtx)
	go metrics.ReportTaskMetrics(runCtx, t, uc.repo)

	var wg sync.WaitGroup
	wg.Add(len(members))
	for _, m := range members {
		m := m
		sess := user.NewSession(m.ID, m.Name, t)
		t.MarkMemberStart()
		if err := t.Submit(func() {
			defer wg.Done()
			defer t.MarkMemberDone(!sess.IsFailed())
			_ = sess.Execute(t.Context(), client, user.NoopSecretProvider)
		}); err != nil {
			wg.Done()
			t.MarkMemberDone(false)
		}
	}
	wg.Wait()

	t.Stop()

	uc.memberPool.Release(taskID)
	if t.GetStatus() == v1.TaskStatus_TASK_RUNNING {
		t.SetStatus(v1.TaskStatus_TASK_COMPLETED)
		uc.processTaskFinish(taskID, t)
	}
	uc.Schedule()
}

// CreateTask 创建并尝试运行
func (uc *UseCase) CreateTask(ctx context.Context, g base.IGame, config *v1.TaskConfig) (*task.Task, error) {
	taskID, err := uc.repo.NextTaskID(ctx, config.GameId)
	if err != nil {
		return nil, fmt.Errorf("failed to generate task ID: %w", err)
	}

	t, err := task.NewTask(uc.ctx, taskID, g, config)
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

	// === 阶段1: 等待订单落库 ===
	uc.waitForOrdersToComplete(ctx, taskID, threshold, scope)

	// === 阶段2: 发送任务完成通知 ===
	uc.sendTaskCompletionNotification(ctx, taskID, t)

	// === 阶段3: 执行环境清理 ===
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

	report := stats.BuildReport(ctx, uc.repo, t, time.Now())
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

func getBonusConfigForGame(cfg *v1.TaskConfig) *v1.BetBonusConfig {
	if cfg == nil {
		return nil
	}
	for _, b := range cfg.BetBonus {
		if b != nil && b.GameId == cfg.GameId {
			return b
		}
	}
	return nil
}
