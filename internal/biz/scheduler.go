package biz

import (
	"context"
	"fmt"

	v1 "stress/api/stress/v1"
	"stress/internal/biz/game/base"
	"stress/internal/biz/task"
)

// scheduleLoop 调度器主循环，阻塞等待任务变更信号
func (uc *UseCase) scheduleLoop() {
	for {
		select {
		case <-uc.ctx.Done():
			return
		case <-uc.scheduleCh:
			uc.doSchedule()
		}
	}
}

// doSchedule 执行实际调度逻辑
func (uc *UseCase) doSchedule() {
	for {
		select {
		case <-uc.ctx.Done():
			return
		default:
		}

		// 单线程，控制宿主机cpu+内存; 控制一个任务在跑
		if uc.taskPool.IsRateLimited(1) {
			break
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
		if !t.CompareAndSetStatus(v1.TaskStatus_TASK_PENDING, v1.TaskStatus_TASK_RUNNING) {
			uc.memberPool.Release(taskID)
			continue
		}
		go uc.runTask(t, allocated)
	}
}

// WakeScheduler 唤醒调度器（非阻塞）
func (uc *UseCase) WakeScheduler() {
	select {
	case uc.scheduleCh <- struct{}{}:
	default: // channel 已满，已有待处理信号
	}
}

// runTask 执行任务，cleanup 后通过回调唤醒调度
func (uc *UseCase) runTask(t *task.Task, allocated []task.MemberInfo) {
	deps := &task.ExecDeps{
		GetOrderCount:     uc.repo.GetGameOrderCount,
		GetOrderAmounts:   uc.repo.GetDetailedOrderAmounts,
		QueryOrderPoints:  uc.repo.QueryGameOrderPoints,
		UploadBytes:       uc.repo.UploadBytes,
		CleanRedisBySites: uc.repo.CleanRedisBySites,
		CleanOrderTable:   uc.repo.CleanGameOrderTable,
		ReturnMembers:     uc.memberPool.Release,
		Conf:              uc.conf,
		Notify:            uc.notify,
		Chart:             uc.chart,
		OnComplete:        uc.WakeScheduler,
	}
	t.Execute(allocated, deps)
}

// CreateTask 创建并尝试运行
func (uc *UseCase) CreateTask(ctx context.Context, g base.IGame, config *v1.TaskConfig) (*task.Task, error) {
	if config.MemberCount > uc.conf.Member.MaxLoadTotal {
		return nil, fmt.Errorf("member count %d exceeds limit %d", config.MemberCount, uc.conf.Member.MaxLoadTotal)
	}

	taskID, err := uc.repo.NextTaskID(ctx, config.GameId)
	if err != nil {
		return nil, fmt.Errorf("generate task id failed: %w", err)
	}

	t, err := task.NewTask(uc.ctx, taskID, g, config, uc.log.Logger())
	if err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}

	uc.taskPool.Add(t)
	uc.WakeScheduler()
	return t, nil
}

// DeleteTask 删除任务（异步，不等待 Execute 退出）
func (uc *UseCase) DeleteTask(id string) error {
	t, ok := uc.taskPool.Remove(id)
	if !ok {
		return nil
	}

	// 停止任务上下文，触发 Execute 退出
	t.Stop()
	// 成员由 Execute.cleanup 释放，避免重复释放导致的数据混乱
	return nil
}

// CancelTask 取消任务（异步，不等待 Execute 退出）
func (uc *UseCase) CancelTask(id string) error {
	t, ok := uc.taskPool.Get(id)
	if !ok {
		return fmt.Errorf("task %s not found", id)
	}
	if err := t.Cancel(); err != nil {
		return err
	}
	uc.taskPool.DropPending(id) // 如果有
	// 成员由 Execute.cleanup 释放，避免重复释放导致的数据混乱
	return nil
}
