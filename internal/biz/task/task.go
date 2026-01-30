package task

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	v1 "stress/api/stress/v1"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/panjf2000/ants/v2"
)

var maxWorkerPerTask = 1000

// Task 压测任务结构体
type Task struct {
	id          string
	description string
	status      v1.TaskStatus
	config      *v1.TaskConfig
	createdAt   time.Time
	finishedAt  time.Time
	userIDs     []int64

	mu     sync.RWMutex
	pool   *ants.Pool
	ctx    context.Context
	cancel context.CancelFunc

	// 统计信息（原子操作，对齐 go-rtp-tool 的全局统计）
	process       int64
	target        int64
	betOrderCount int64
	betBonusCount int64
	totalDuration int64 // 纳秒

	// TaskProgress 补齐：成员与失败请求统计
	activeMembers    int64
	completedMembers int64
	failedMembers    int64
	failedRequests   int64

	errorMu     sync.Mutex
	errorCounts map[string]int64
}

// NewTask 创建新任务
func NewTask(id, description string, config *v1.TaskConfig) (*Task, error) {
	capacity := maxWorkerPerTask
	if config != nil && config.MemberCount > 0 {
		capacity = int(config.MemberCount)
	}

	pool, err := ants.NewPool(capacity)
	if err != nil {
		return nil, fmt.Errorf("failed to create ants pool: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	var targetCount int64
	if config != nil {
		targetCount = int64(config.MemberCount) * int64(config.TimesPerMember)
	}

	return &Task{
		id:          id,
		description: description,
		status:      v1.TaskStatus_TASK_PENDING,
		config:      config,
		createdAt:   time.Now(),
		pool:        pool,
		ctx:         ctx,
		cancel:      cancel,
		target:      targetCount,
		errorCounts: make(map[string]int64),
	}, nil
}

// MetaSnapshot 一次性获取所有元数据字段，减少锁竞争
type MetaSnapshot struct {
	ID          string
	Description string
	Status      v1.TaskStatus
	Config      *v1.TaskConfig
	UserIDCount int
	Step        int64 // 总步数 = betOrderCount + betBonusCount
}

func (t *Task) MetaSnapshot() MetaSnapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return MetaSnapshot{
		ID:          t.id,
		Description: t.description,
		Status:      t.status,
		Config:      t.config,
		UserIDCount: len(t.userIDs),
		Step:        atomic.LoadInt64(&t.betOrderCount) + atomic.LoadInt64(&t.betBonusCount),
	}
}

func (t *Task) GetID() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.id
}

func (t *Task) GetDescription() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.description
}

func (t *Task) GetConfig() *v1.TaskConfig {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.config
}

func (t *Task) GetCreatedAt() time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.createdAt
}

func (t *Task) SetUserIDs(ids []int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(ids) == 0 {
		t.userIDs = nil
		return
	}
	t.userIDs = append([]int64(nil), ids...)
}

func (t *Task) GetUserIDs() []int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if len(t.userIDs) == 0 {
		return nil
	}
	return append([]int64(nil), t.userIDs...)
}

func (t *Task) GetStatus() v1.TaskStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.status
}

func (t *Task) Context() context.Context { return t.ctx }

func (t *Task) GetActiveMembers() int64    { return atomic.LoadInt64(&t.activeMembers) }
func (t *Task) GetCompletedMembers() int64 { return atomic.LoadInt64(&t.completedMembers) }
func (t *Task) GetFailedMembers() int64    { return atomic.LoadInt64(&t.failedMembers) }
func (t *Task) GetFailedRequests() int64   { return atomic.LoadInt64(&t.failedRequests) }

// isTerminalStatus 判断是否为终态（完成/失败/取消）
func isTerminalStatus(status v1.TaskStatus) bool {
	return status == v1.TaskStatus_TASK_COMPLETED ||
		status == v1.TaskStatus_TASK_FAILED ||
		status == v1.TaskStatus_TASK_CANCELLED
}

func (t *Task) SetStatus(status v1.TaskStatus) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.status == status {
		return
	}
	if isTerminalStatus(status) && t.finishedAt.IsZero() {
		t.finishedAt = time.Now()
	}
	t.status = status
}

func (t *Task) Cancel() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if isTerminalStatus(t.status) {
		return fmt.Errorf("task already finished")
	}
	t.status = v1.TaskStatus_TASK_CANCELLED
	if t.cancel != nil {
		t.cancel()
	}
	log.Infof("[task %s] cancelled", t.id)
	return nil
}

// Start 启动任务进入运行状态
func (t *Task) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.status != v1.TaskStatus_TASK_PENDING {
		return fmt.Errorf("task status %v cannot be started", t.status)
	}
	t.status = v1.TaskStatus_TASK_RUNNING
	log.Infof("[task %s] started at %s", t.id, t.createdAt.Format("15:04:05"))
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
	log.Infof("[%s] task stopped", t.id)
}

func (t *Task) Submit(fn func()) error {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.pool == nil {
		return fmt.Errorf("task pool already released")
	}
	return t.pool.Submit(fn)
}

func (t *Task) MarkMemberStart() { atomic.AddInt64(&t.activeMembers, 1) }

func (t *Task) MarkMemberDone(failed bool) {
	atomic.AddInt64(&t.activeMembers, -1)
	if failed {
		atomic.AddInt64(&t.failedMembers, 1)
	} else {
		atomic.AddInt64(&t.completedMembers, 1)
	}
}

func (t *Task) addStep(betOrder, betBonus int64, duration time.Duration, spinOver bool) {
	if betOrder > 0 {
		atomic.AddInt64(&t.betOrderCount, betOrder)
	}
	if betBonus > 0 {
		atomic.AddInt64(&t.betBonusCount, betBonus)
	}
	atomic.AddInt64(&t.totalDuration, duration.Nanoseconds())
	if spinOver {
		atomic.AddInt64(&t.process, 1)
	}
}

func (t *Task) AddBetOrder(duration time.Duration, spinOver bool) {
	t.addStep(1, 0, duration, spinOver)
}
func (t *Task) AddBetBonus(duration time.Duration) { t.addStep(0, 1, duration, false) }

func (t *Task) AddError(errMsg string) {
	t.errorMu.Lock()
	t.errorCounts[errMsg]++
	t.errorMu.Unlock()
	atomic.AddInt64(&t.failedRequests, 1)
}

// StatsSnapshot 供 API 填充 proto 用
type StatsSnapshot struct {
	Process, Target, BetOrders, BetBonuses         int64
	TotalDuration                                  time.Duration
	ActiveMembers, CompletedMembers, FailedMembers int64
	FailedRequests                                 int64
	CreatedAt, FinishedAt                          time.Time
	Config                                         *v1.TaskConfig
	ErrorCounts                                    map[string]int64
}

func (t *Task) StatsSnapshot() StatsSnapshot {
	t.mu.RLock()
	createdAt, finishedAt, cfg := t.createdAt, t.finishedAt, t.config
	t.mu.RUnlock()
	t.errorMu.Lock()
	ec := make(map[string]int64, len(t.errorCounts))
	for k, v := range t.errorCounts {
		ec[k] = v
	}
	t.errorMu.Unlock()
	return StatsSnapshot{
		Process:          atomic.LoadInt64(&t.process),
		Target:           atomic.LoadInt64(&t.target),
		BetOrders:        atomic.LoadInt64(&t.betOrderCount),
		BetBonuses:       atomic.LoadInt64(&t.betBonusCount),
		TotalDuration:    time.Duration(atomic.LoadInt64(&t.totalDuration)),
		ActiveMembers:    atomic.LoadInt64(&t.activeMembers),
		CompletedMembers: atomic.LoadInt64(&t.completedMembers),
		FailedMembers:    atomic.LoadInt64(&t.failedMembers),
		FailedRequests:   atomic.LoadInt64(&t.failedRequests),
		CreatedAt:        createdAt,
		FinishedAt:       finishedAt,
		Config:           cfg,
		ErrorCounts:      ec,
	}
}

func (s *StatsSnapshot) QPS() float64 {
	end := time.Now()
	if !s.FinishedAt.IsZero() {
		end = s.FinishedAt
	}
	d := end.Sub(s.CreatedAt).Seconds()
	if d <= 0 {
		return 0
	}
	return float64(s.Process) / d
}

func (s *StatsSnapshot) AvgLatencyMs() float64 {
	total := s.BetOrders + s.BetBonuses
	if total <= 0 {
		return 0
	}
	return float64(s.TotalDuration.Milliseconds()) / float64(total)
}

func (s *StatsSnapshot) SuccessRate() float64 {
	if s.Process <= 0 {
		return 0
	}
	return float64(s.Process-s.FailedRequests) / float64(s.Process)
}

// ProgressSnapshot 进度快照（Monitor / Prometheus 用）
type ProgressSnapshot struct {
	ID            string
	Process       int64
	Target        int64
	Step          int64
	TotalDuration time.Duration
}

func (t *Task) LoadProgressSnapshot() ProgressSnapshot {
	return ProgressSnapshot{
		ID:            t.id, // 不可变，无需锁
		Process:       atomic.LoadInt64(&t.process),
		Target:        atomic.LoadInt64(&t.target),
		Step:          atomic.LoadInt64(&t.betOrderCount) + atomic.LoadInt64(&t.betBonusCount),
		TotalDuration: time.Duration(atomic.LoadInt64(&t.totalDuration)),
	}
}

// Monitor 启动进度监控，每秒打印进度；ctx 取消时打一次收尾并退出（由 runTaskSessions 传入 runCtx 控制）
func (t *Task) Monitor(ctx context.Context) {
	start := time.Now()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			t.printFinalStats(start)
			return
		case <-ticker.C:
			t.printProgress(start)
		}
	}
}

func (t *Task) printProgress(start time.Time) {
	snap := t.LoadProgressSnapshot()
	elapsed := time.Since(start)

	pct := float64(0)
	if snap.Target > 0 {
		pct = float64(snap.Process) / float64(snap.Target) * 100
	}
	remaining := time.Duration(0)
	if pct > 0 {
		remaining = time.Duration(int64(float64(elapsed)/pct*100)) - elapsed
	}

	log.Infof("[%s]: 进度:%d/%d(%.2f%%), 用时:%s, 剩余:%s, QPS:%.2f, step:%.2f, 延迟:%s    ",
		snap.ID,
		snap.Process, snap.Target, pct,
		shortDuration(elapsed),
		shortDuration(remaining),
		float64(snap.Process)/elapsed.Seconds(),
		float64(snap.Step)/elapsed.Seconds(),
		avgDuration(snap.TotalDuration, snap.Step),
	)
}

func (t *Task) printFinalStats(start time.Time) {
	snap := t.LoadProgressSnapshot()
	elapsed := time.Since(start)
	log.Infof("[%s] 任务结束: 进度:%d/%d, 总步数:%d, 耗时:%v, QPS:%.2f, 平均延迟:%s",
		snap.ID, snap.Process, snap.Target, snap.Step, elapsed,
		float64(snap.Process)/elapsed.Seconds(),
		avgDuration(snap.TotalDuration, snap.Step),
	)
}

func avgDuration(d time.Duration, step int64) string {
	if step == 0 {
		return "0"
	}
	return shortDuration(time.Duration(int64(d) / step))
}

func shortDuration(d time.Duration) string {
	if d == 0 {
		return "0"
	}
	sec := d.Seconds()
	for _, u := range []struct {
		div float64
		sym string
	}{
		{60 * 60 * 24, "d"},
		{60 * 60, "h"},
		{60, "m"},
		{1, "s"},
		{1e-3, "ms"},
		{1e-6, "µs"},
		{1e-9, "ns"},
	} {
		if sec >= u.div {
			val := sec / u.div
			if val >= 100 {
				return fmt.Sprintf("%.0f%s", val, u.sym)
			} else if val >= 10 {
				return fmt.Sprintf("%.1f%s", val, u.sym)
			}
			return fmt.Sprintf("%.2f%s", val, u.sym)
		}
	}
	return "0"
}
