package member

import "sync"

// DefaultMemberNamePrefix 默认成员名前缀（填充空闲池时生成名称用）
const DefaultMemberNamePrefix = "gopgct"

// Info MemberInfo 玩家信息
type Info struct {
	ID      int64
	Name    string
	Balance float64
}

// Pool MemberPool 玩家资源池，封装空闲/已分配成员的存储与操作
type Pool struct {
	mu         sync.RWMutex
	idle       []Info
	allocated  map[string][]Info // taskID -> 分配给该任务的玩家
	totalCount int
}

// NewMemberPool 创建玩家资源池
func NewMemberPool() *Pool {
	return &Pool{
		idle:      make([]Info, 0),
		allocated: make(map[string][]Info),
	}
}

// AddIdle 将玩家加入空闲池
func (p *Pool) AddIdle(members []Info) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.idle = append(p.idle, members...)
	p.totalCount += len(members)
}

// CanAllocate 是否有足够空闲玩家可分配
func (p *Pool) CanAllocate(count int) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.idle) >= count
}

// Allocate 为任务分配玩家，返回分配的成员；若不足则返回 nil
func (p *Pool) Allocate(taskID string, count int) []Info {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.idle) < count {
		return nil
	}
	allocated := append([]Info{}, p.idle[:count]...)
	p.idle = p.idle[count:]
	p.allocated[taskID] = allocated
	return allocated
}

// GetAllocated 返回某任务当前占用的成员（只读副本，用于 runTaskSessions 等）
func (p *Pool) GetAllocated(taskID string) []Info {
	p.mu.RLock()
	defer p.mu.RUnlock()
	m, ok := p.allocated[taskID]
	if !ok || len(m) == 0 {
		return nil
	}
	return append([]Info{}, m...)
}

// Release 释放任务占用的玩家回空闲池
func (p *Pool) Release(taskID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if m, ok := p.allocated[taskID]; ok {
		p.idle = append(p.idle, m...)
		delete(p.allocated, taskID)
	}
}

// Stats 返回 idle 数、已分配总数、总玩家数
func (p *Pool) Stats() (idle, allocated, total int) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	allocatedCount := 0
	for _, m := range p.allocated {
		allocatedCount += len(m)
	}
	return len(p.idle), allocatedCount, p.totalCount
}
