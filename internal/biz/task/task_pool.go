package task

import (
	"sort"
	"sync"
)

// Pool TaskPool 任务池，封装任务的存储与队列
type Pool struct {
	mu           sync.RWMutex
	allTasks     map[string]*Task
	pendingQueue []string
}

// NewTaskPool 创建任务池
func NewTaskPool() *Pool {
	return &Pool{
		allTasks:     make(map[string]*Task),
		pendingQueue: make([]string, 0),
	}
}

// Add 注册任务并入队
func (p *Pool) Add(t *Task) {
	p.mu.Lock()
	defer p.mu.Unlock()
	taskID := t.GetID()
	p.allTasks[taskID] = t
	p.pendingQueue = append(p.pendingQueue, taskID)
}

// PeekPending 取队首待调度任务，不出队
func (p *Pool) PeekPending() (taskID string, t *Task, ok bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if len(p.pendingQueue) == 0 {
		return "", nil, false
	}
	taskID = p.pendingQueue[0]
	t, ok = p.allTasks[taskID]
	return taskID, t, ok
}

// DequeuePending 队首出队，仅当 taskID 与队首一致时执行
func (p *Pool) DequeuePending(taskID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.pendingQueue) == 0 || p.pendingQueue[0] != taskID {
		return false
	}
	p.pendingQueue = p.pendingQueue[1:]
	return true
}

// RequeueAtHead 将 taskID 重新放回待调度队列队首（分配失败时回退用）
func (p *Pool) RequeueAtHead(taskID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pendingQueue = append([]string{taskID}, p.pendingQueue...)
}

// DropPendingHead 仅丢弃队首（用于跳过无效任务）
func (p *Pool) DropPendingHead() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.pendingQueue) > 0 {
		p.pendingQueue = p.pendingQueue[1:]
	}
}

// Get 按 ID 获取任务
func (p *Pool) Get(id string) (*Task, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	t, ok := p.allTasks[id]
	return t, ok
}

// List 返回所有任务，按创建时间倒序
func (p *Pool) List() []*Task {
	p.mu.RLock()
	if len(p.allTasks) == 0 {
		p.mu.RUnlock()
		return nil
	}
	out := make([]*Task, 0, len(p.allTasks))
	for _, v := range p.allTasks {
		out = append(out, v)
	}
	p.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool {
		return out[i].GetCreatedAt().After(out[j].GetCreatedAt())
	})
	return out
}

// Remove 移除任务并返回；队列中残留的该 ID 由调度逻辑 Drop 处理
func (p *Pool) Remove(id string) (*Task, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	t, ok := p.allTasks[id]
	if !ok {
		return nil, false
	}
	delete(p.allTasks, id)
	return t, true
}
