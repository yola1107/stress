package task

import (
	"context"
	"sort"
	"sync"
	"time"

	v1 "stress/api/stress/v1"

	"github.com/go-kratos/kratos/v2/log"
)

// Pool 任务池
type Pool struct {
	mu      sync.RWMutex
	tasks   map[string]*Task
	pending []string
}

// NewTaskPool 创建任务池
func NewTaskPool() *Pool {
	return &Pool{
		tasks:   make(map[string]*Task),
		pending: nil,
	}
}

// Add 添加任务到池中
func (p *Pool) Add(t *Task) {
	p.mu.Lock()
	p.tasks[t.GetID()] = t
	p.pending = append(p.pending, t.GetID())
	p.mu.Unlock()
}

// Get 获取任务
func (p *Pool) Get(id string) (*Task, bool) {
	p.mu.RLock()
	t, ok := p.tasks[id]
	p.mu.RUnlock()
	return t, ok
}

// List 列出所有任务（按创建时间倒序）
func (p *Pool) List() []*Task {
	p.mu.RLock()
	out := make([]*Task, 0, len(p.tasks))
	for _, t := range p.tasks {
		out = append(out, t)
	}
	p.mu.RUnlock()

	sort.Slice(out, func(i, j int) bool {
		return out[i].createdAt.After(out[j].createdAt)
	})
	return out
}

// Remove 移除任务，同时从 pending 中移除
func (p *Pool) Remove(id string) (*Task, bool) {
	p.mu.Lock()
	t, ok := p.tasks[id]
	if ok {
		delete(p.tasks, id)
		for i, pid := range p.pending {
			if pid == id {
				p.pending = append(p.pending[:i], p.pending[i+1:]...)
				break
			}
		}
	}
	p.mu.Unlock()
	return t, ok
}

// HasRunningTasks 检查是否有正在运行的任务
func (p *Pool) HasRunningTasks() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, t := range p.tasks {
		if s := t.GetStatus(); s == v1.TaskStatus_TASK_RUNNING ||
			s == v1.TaskStatus_TASK_PROCESSING {
			return true
		}
	}
	return false
}

// PeekPending 取队首待调度任务（不出队）
func (p *Pool) PeekPending() (taskID string, t *Task, ok bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 循环清理无效的任务 ID
	for len(p.pending) > 0 {
		taskID = p.pending[0]
		t, ok = p.tasks[taskID]
		if !ok {
			// 任务已不存在，清理并继续
			p.pending = p.pending[1:]
			continue
		}
		return taskID, t, true
	}
	return "", nil, false
}

// DequeuePending 队首出队，仅当 taskID 与队首一致时执行
func (p *Pool) DequeuePending(taskID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.pending) == 0 || p.pending[0] != taskID {
		return false
	}
	p.pending = p.pending[1:]
	return true
}

// RequeueAtHead 将 taskID 重新放回队首
func (p *Pool) RequeueAtHead(taskID string) {
	p.mu.Lock()
	p.pending = append([]string{taskID}, p.pending...)
	p.mu.Unlock()
}

// DropPendingHead 丢弃队首（跳过无效任务）
func (p *Pool) DropPendingHead() {
	p.mu.Lock()
	if len(p.pending) > 0 {
		p.pending = p.pending[1:]
	}
	p.mu.Unlock()
}

func (p *Pool) StartAutoCleanup(ctx context.Context, logger log.Logger, retention time.Duration, interval time.Duration) {
	logHelper := log.NewHelper(logger)
	logHelper.Infof("Task cleaner started, retention=%v, interval=%v", retention, interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logHelper.Info("closing task cleaner")
			return
		case <-ticker.C:
			if deleted := p.CleanupExpiredTasks(retention); deleted > 0 {
				logHelper.Infof("Task cleanup: deleted %d expired tasks", deleted)
			}
		}
	}
}

// CleanupExpiredTasks 清理指定时间之前完成的任务
// retention: 保留时长，例如 24*time.Hour 表示保留 24 小时内完成的任务
// 返回清理的任务数量
func (p *Pool) CleanupExpiredTasks(retention time.Duration) int {
	now := time.Now()
	cutoff := now.Add(-retention)

	p.mu.Lock()
	defer p.mu.Unlock()

	var deleted int
	for id, t := range p.tasks {
		// 只清理已完成/失败/取消的任务
		status := t.GetStatus()
		if status != v1.TaskStatus_TASK_COMPLETED &&
			status != v1.TaskStatus_TASK_FAILED &&
			status != v1.TaskStatus_TASK_CANCELLED {
			continue
		}

		// 检查完成时间
		finishedAt := t.GetFinishedAt()
		if finishedAt.IsZero() {
			continue // 没有完成时间，跳过
		}

		// 如果完成时间早于截止时间，删除
		if finishedAt.Before(cutoff) {
			delete(p.tasks, id)
			deleted++
		}
	}

	return deleted
}
