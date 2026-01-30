package task

import (
	"context"
	"fmt"
	"sync"
	"time"

	v1 "stress/api/stress/v1"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/panjf2000/ants/v2"
)

var maxWorkerPerTask = 1000

// taskMeta 任务元数据，Task 与 TaskStats 共享
type taskMeta struct {
	mu          sync.RWMutex
	id          string
	description string
	status      v1.TaskStatus
	config      *v1.TaskConfig
	createdAt   time.Time
	finishedAt  time.Time
	userIDs     []int64
}

// Task 压测任务结构体
type Task struct {
	meta *taskMeta

	mu     sync.RWMutex
	pool   *ants.Pool
	ctx    context.Context
	cancel context.CancelFunc

	stats *TaskStats
}

// NewTask 创建新任务
func NewTask(id, description string, config *v1.TaskConfig) (*Task, error) {
	capacity := maxWorkerPerTask
	targetCount := int64(0)
	if config != nil {
		if config.MemberCount > 0 {
			capacity = int(config.MemberCount)
		}
		targetCount = int64(config.MemberCount) * int64(config.TimesPerMember)
	}

	pool, err := ants.NewPool(capacity)
	if err != nil {
		return nil, fmt.Errorf("failed to create ants pool: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())

	createdAt := time.Now()
	meta := &taskMeta{
		id: id, description: description, status: v1.TaskStatus_TASK_PENDING,
		config: config, createdAt: createdAt,
	}
	return &Task{
		meta:   meta,
		pool:   pool,
		ctx:    ctx,
		cancel: cancel,
		stats:  NewTaskStats(targetCount, meta),
	}, nil
}

func (t *Task) GetID() string {
	t.meta.mu.RLock()
	defer t.meta.mu.RUnlock()
	return t.meta.id
}

func (t *Task) GetDescription() string {
	t.meta.mu.RLock()
	defer t.meta.mu.RUnlock()
	return t.meta.description
}

func (t *Task) GetConfig() *v1.TaskConfig {
	t.meta.mu.RLock()
	defer t.meta.mu.RUnlock()
	return t.meta.config
}

func (t *Task) GetCreatedAt() time.Time {
	t.meta.mu.RLock()
	defer t.meta.mu.RUnlock()
	return t.meta.createdAt
}

func (t *Task) SetUserIDs(ids []int64) {
	t.meta.mu.Lock()
	defer t.meta.mu.Unlock()
	if len(ids) == 0 {
		t.meta.userIDs = nil
	} else {
		t.meta.userIDs = append([]int64(nil), ids...)
	}
}

func (t *Task) GetUserIDs() []int64 {
	t.meta.mu.RLock()
	defer t.meta.mu.RUnlock()
	if len(t.meta.userIDs) == 0 {
		return nil
	}
	return append([]int64(nil), t.meta.userIDs...)
}

func (t *Task) GetStatus() v1.TaskStatus {
	t.meta.mu.RLock()
	defer t.meta.mu.RUnlock()
	return t.meta.status
}

func (t *Task) Context() context.Context {
	return t.ctx
}

// isTerminalStatus 判断是否为终态（完成/失败/取消）
func isTerminalStatus(status v1.TaskStatus) bool {
	switch status {
	case v1.TaskStatus_TASK_COMPLETED, v1.TaskStatus_TASK_FAILED, v1.TaskStatus_TASK_CANCELLED:
		return true
	default:
		return false
	}
}

func (t *Task) SetStatus(status v1.TaskStatus) {
	t.meta.mu.Lock()
	defer t.meta.mu.Unlock()
	if t.meta.status == status {
		return
	}
	if isTerminalStatus(status) && t.meta.finishedAt.IsZero() {
		t.meta.finishedAt = time.Now()
	}
	t.meta.status = status
}

func (t *Task) Cancel() error {
	t.meta.mu.Lock()
	if isTerminalStatus(t.meta.status) {
		t.meta.mu.Unlock()
		return fmt.Errorf("task already finished")
	}
	t.meta.status = v1.TaskStatus_TASK_CANCELLED
	if t.meta.finishedAt.IsZero() {
		t.meta.finishedAt = time.Now()
	}
	id := t.meta.id
	t.meta.mu.Unlock()
	if t.cancel != nil {
		t.cancel()
	}
	log.Infof("[task %s] cancelled", id)
	return nil
}

// Start 启动任务进入运行状态
func (t *Task) Start() error {
	t.meta.mu.Lock()
	defer t.meta.mu.Unlock()
	if t.meta.status != v1.TaskStatus_TASK_PENDING {
		return fmt.Errorf("task status %v cannot be started", t.meta.status)
	}
	t.meta.status = v1.TaskStatus_TASK_RUNNING
	log.Infof("[task %s] started at %s", t.meta.id, t.meta.createdAt.Format("15:04:05"))
	return nil
}

func (t *Task) Stop() {
	t.mu.Lock()
	if t.cancel != nil {
		t.cancel()
	}
	p := t.pool
	t.pool = nil
	t.mu.Unlock()
	if p != nil {
		p.Release()
	}
	t.meta.mu.RLock()
	id := t.meta.id
	t.meta.mu.RUnlock()
	log.Infof("[%s] task stopped", id)
}

func (t *Task) Submit(fn func()) error {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.pool == nil {
		return fmt.Errorf("task pool already released")
	}
	return t.pool.Submit(fn)
}

// GetStats 返回任务统计信息
func (t *Task) GetStats() *TaskStats {
	return t.stats
}
