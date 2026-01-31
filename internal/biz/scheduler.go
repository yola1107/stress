package biz

import (
	"context"
	"fmt"
	"sync"
	"time"

	v1 "stress/api/stress/v1"
	"stress/internal/biz/game/base"
	"stress/internal/biz/metrics"
	"stress/internal/biz/task"
	"stress/internal/biz/user"
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
		if err := t.Start(); err != nil {
			mp.Release(taskID)
			continue
		}
		go uc.runTaskSessions(t)
	}
}

// runTaskSessions 执行单任务完整生命周期，顺序严格：
// 1) 启动附属 goroutine（Monitor、ReportTaskMetrics），均绑定 runCtx
// 2) 跑完所有 Session（wg.Wait）
// 3) 结束附属：stopRun() 取消 runCtx → Monitor 与 ReportTaskMetrics 立即退出
// 4) 结束任务：t.Stop() 取消 task context、释放协程池
// 5) 释放成员、置状态、清理环境、触发下一轮调度
func (uc *UseCase) runTaskSessions(t *task.Task) {
	snap := t.StatsSnapshot()
	members := uc.memberPool.GetAllocated(snap.ID)
	if len(members) == 0 {
		t.Stop()
		uc.memberPool.Release(snap.ID)
		uc.Schedule()
		return
	}

	g, _ := uc.GetGame(snap.Config.GameId)
	checker := uc.gamePool.RequireProtobuf

	maxConns := 100
	if snap.Config != nil && snap.Config.MemberCount > 0 {
		maxConns = int(snap.Config.MemberCount)
	}
	httpClient := user.NewHTTPClient(maxConns)
	client := user.NewAPIClient(httpClient, user.NoopSecretProvider, g, checker)
	t.SetBonusConfig(getBonusConfigForGame(snap.Config, snap.Config.GameId))

	runCtx, stopRun := context.WithCancel(t.Context())
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

	// 3) 结束附属：不再打进度、不再上报 Prometheus
	stopRun()
	// 4) 结束任务：取消 task ctx、释放 ants 池
	t.Stop()
	// 立即释放 HTTP 空闲连接，不等待 GC
	httpClient.CloseIdleConnections()

	// 5) 释放成员、置完成态、清理环境、调度下一批
	uc.memberPool.Release(snap.ID)
	if t.GetStatus() == v1.TaskStatus_TASK_RUNNING {
		t.SetStatus(v1.TaskStatus_TASK_COMPLETED)
		<-uc.CleanTestEnvironment(snap.ID, snap.Step)
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

// CleanTestEnvironment 清理 Redis site:* 并等订单表达到阈值后 truncate，返回可读 channel
func (uc *UseCase) CleanTestEnvironment(taskID string, step int64) <-chan struct{} {
	time.Sleep(cleanupStartDelay)
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := uc.cleanupRedis(ctx); err != nil {
			uc.log.Errorf("[%s] Redis cleanup: %v", taskID, err)
		}
	}()
	go func() {
		defer wg.Done()
		uc.waitOrderThenClean(ctx, taskID, step)
	}()
	go func() {
		wg.Wait()
		cancel()
		uc.log.Infof("[%s] cleanup done", taskID)
		close(done)
	}()
	return done
}

// cleanupRedis 统一的 Redis 清理逻辑
func (uc *UseCase) cleanupRedis(ctx context.Context) error {
	if uc.c == nil || len(uc.c.Sites) == 0 {
		return nil
	}
	if err := uc.repo.CleanRedisBySites(ctx, uc.c.Sites); err != nil {
		return err
	}
	uc.log.Info("Redis cleanup done")
	return nil
}

func getBonusConfigForGame(cfg *v1.TaskConfig, gameID int64) *v1.BetBonusConfig {
	if cfg == nil {
		return nil
	}
	for _, b := range cfg.BetBonus {
		if b != nil && b.GameId == gameID {
			return b
		}
	}
	return nil
}

// waitOrderThenClean 每隔一段时间查订单数，>= threshold 时 truncate 并返回
func (uc *UseCase) waitOrderThenClean(ctx context.Context, taskID string, threshold int64) {
	ticker := time.NewTicker(cleanupRetryDelay)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			count, err := uc.repo.GetGameOrderCount(ctx)
			if err != nil {
				uc.log.Errorf("[%s] get order count: %v", taskID, err)
				continue
			}
			if count < threshold {
				continue
			}
			uc.log.Infof("[%s] order table %d>=%d, truncating", taskID, count, threshold)
			if err := uc.repo.CleanGameOrderTable(ctx); err != nil {
				uc.log.Errorf("[%s] truncate: %v", taskID, err)
			}
			return
		}
	}
}
