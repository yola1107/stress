package task

import (
	"sort"
	"sync"
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
		return out[i].GetCreatedAt().After(out[j].GetCreatedAt())
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

// PeekPending 取队首待调度任务（不出队）
func (p *Pool) PeekPending() (taskID string, t *Task, ok bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if len(p.pending) == 0 {
		return "", nil, false
	}
	taskID = p.pending[0]
	t, ok = p.tasks[taskID]
	return taskID, t, ok
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
