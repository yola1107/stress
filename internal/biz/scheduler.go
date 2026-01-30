package biz

import (
	"context"
	"fmt"
	"sync"
	"time"

	v1 "stress/api/stress/v1"
	"stress/internal/biz/task"
	"stress/internal/biz/user"
)

// Schedule 从待调度队列取任务、分配成员、启动压测
func (uc *UseCase) Schedule() {
	mp, tp := uc.memberPool, uc.taskPool
	for {
		taskID, t, ok := tp.PeekPending()
		if !ok {
			break
		}
		if t == nil || t.GetStatus() != v1.TaskStatus_TASK_PENDING || t.GetConfig() == nil {
			tp.DropPendingHead()
			continue
		}
		config := t.GetConfig()
		if !tp.DequeuePending(taskID) {
			continue
		}
		allocated := mp.Allocate(taskID, int(config.MemberCount))
		if allocated == nil {
			tp.RequeueAtHead(taskID)
			break
		}
		ids := make([]int64, len(allocated))
		for i, m := range allocated {
			ids[i] = m.ID
		}
		t.SetUserIDs(ids)
		if err := t.Start(); err != nil {
			t.SetUserIDs(nil)
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
	meta := t.MetaSnapshot()
	members := uc.memberPool.GetAllocated(meta.ID)
	if len(members) == 0 {
		return
	}

	g, _ := uc.GetGame(meta.Config.GameId)
	sp := func(string) (string, bool) { return "", false }
	checker := uc.gamePool.RequireProtobuf

	// 执行阶段 context：仅在本任务“跑 session”期间有效，stopRun 后 Monitor/Prometheus 立即停
	runCtx, stopRun := context.WithCancel(t.Context())
	go t.Monitor(runCtx)
	go ReportTaskMetrics(runCtx, t, uc.repo)

	var wg sync.WaitGroup
	wg.Add(len(members))
	for _, m := range members {
		m := m
		sess := user.NewSession(m.ID, m.Name, meta.Config.GameId, meta.ID, t, checker)
		t.MarkMemberStart()
		if err := t.Submit(func() {
			defer wg.Done()
			defer t.MarkMemberDone(sess.IsFailed())
			_ = sess.Execute(t.Context(), meta.Config, g, sp)
		}); err != nil {
			wg.Done()
			t.MarkMemberDone(true)
		}
	}
	wg.Wait()

	// 3) 结束附属：不再打进度、不再上报 Prometheus
	stopRun()
	// 4) 结束任务：取消 task ctx、释放 ants 池
	t.Stop()

	// 5) 释放成员、置完成态、清理环境、调度下一批
	uc.memberPool.Release(meta.ID)
	if t.GetStatus() == v1.TaskStatus_TASK_RUNNING {
		t.SetStatus(v1.TaskStatus_TASK_COMPLETED)
		<-uc.CleanTestEnvironment(meta)
	}
	uc.Schedule()
}

// CreateTask 创建并尝试运行
func (uc *UseCase) CreateTask(ctx context.Context, description string, config *v1.TaskConfig) (*task.Task, error) {
	taskID, err := uc.repo.NextTaskID(ctx, config.GameId)
	if err != nil {
		return nil, fmt.Errorf("failed to generate task ID: %w", err)
	}

	t, err := task.NewTask(taskID, description, config)
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
	uc.memberPool.Release(id)
	uc.Schedule()
	t.Stop()
	return nil
}

// CancelTask 取消任务并释放成员
func (uc *UseCase) CancelTask(id string) error {
	t, ok := uc.taskPool.Get(id)
	if !ok {
		return fmt.Errorf("task not found")
	}
	uc.memberPool.Release(id)
	uc.Schedule()
	return t.Cancel()
}

// GetTask 按 ID 获取任务
func (uc *UseCase) GetTask(id string) (*task.Task, bool) {
	return uc.taskPool.Get(id)
}

// ListTasks 返回所有任务（已按创建时间倒序）
func (uc *UseCase) ListTasks() []*task.Task {
	return uc.taskPool.List()
}

// GetMemberStats 玩家池统计
func (uc *UseCase) GetMemberStats() (idle, allocated, total int) {
	return uc.memberPool.Stats()
}

var (
	closedChanOnce sync.Once
	closedCh       chan struct{}
)

func closedChan() <-chan struct{} {
	closedChanOnce.Do(func() { closedCh = make(chan struct{}); close(closedCh) })
	return closedCh
}

// CleanTestEnvironment 清理 Redis site:* 并等订单表达到阈值后 truncate，超时 5 分钟，返回可读 channel
func (uc *UseCase) CleanTestEnvironment(snap task.MetaSnapshot) <-chan struct{} {
	time.Sleep(time.Second)
	taskID := snap.ID
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
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
		uc.waitOrderThenClean(ctx, taskID, snap.Step)
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

// waitOrderThenClean 每 5s 查订单数，>= threshold 时 truncate 并返回
func (uc *UseCase) waitOrderThenClean(ctx context.Context, taskID string, threshold int64) {
	ticker := time.NewTicker(5 * time.Second)
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
