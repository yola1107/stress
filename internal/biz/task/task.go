package task

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	v1 "stress/api/stress/v1"
	"stress/internal/biz/game/base"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/panjf2000/ants/v2"
)

// Task 压测任务
type Task struct {
	mu        sync.RWMutex
	id        string
	game      base.IGame
	createdAt time.Time

	status      v1.TaskStatus
	config      *v1.TaskConfig
	bonusConfig *v1.BetBonusConfig

	pool   *ants.Pool
	ctx    context.Context
	cancel context.CancelFunc

	target    int64
	process   int64
	step      int64
	duration  int64
	active    int64
	completed int64
	failed    int64
	errors    int64
}

// NewTask 创建任务，parent 取消时任务会收到信号（通常传 UseCase.ctx）
func NewTask(parent context.Context, id string, g base.IGame, cfg *v1.TaskConfig) (*Task, error) {
	capacity := 1000
	target := int64(0)
	if cfg != nil {
		if cfg.MemberCount > 0 {
			capacity = int(cfg.MemberCount)
			target = int64(cfg.MemberCount) * int64(cfg.TimesPerMember)
		}
	}

	pool, err := ants.NewPool(capacity)
	if err != nil {
		return nil, err
	}

	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	return &Task{
		id:        id,
		status:    v1.TaskStatus_TASK_PENDING,
		config:    cfg,
		createdAt: time.Now(),
		pool:      pool,
		game:      g,
		ctx:       ctx,
		cancel:    cancel,
		target:    target,
	}, nil
}

func (t *Task) GetID() string {
	return t.id
}

func (t *Task) Context() context.Context {
	return t.ctx
}

func (t *Task) GetConfig() *v1.TaskConfig {
	return t.config
}

func (t *Task) GetGame() base.IGame {
	t.mu.RLock()
	g := t.game
	t.mu.RUnlock()
	return g
}

func (t *Task) SetBonusConfig(cfg *v1.BetBonusConfig) {
	t.mu.Lock()
	t.bonusConfig = cfg
	t.mu.Unlock()
}

func (t *Task) GetBonusConfig() *v1.BetBonusConfig {
	t.mu.RLock()
	c := t.bonusConfig
	t.mu.RUnlock()
	return c
}

func (t *Task) GetStatus() v1.TaskStatus {
	t.mu.RLock()
	s := t.status
	t.mu.RUnlock()
	return s
}

func (t *Task) SetStatus(s v1.TaskStatus) {
	t.mu.Lock()
	t.status = s
	t.mu.Unlock()
}

func (t *Task) Start() error {
	t.mu.Lock()
	if t.status != v1.TaskStatus_TASK_PENDING {
		t.mu.Unlock()
		return fmt.Errorf("cannot start: status %v", t.status)
	}
	t.status = v1.TaskStatus_TASK_RUNNING
	t.mu.Unlock()
	log.Infof("[task %s] started", t.id)
	return nil
}

func (t *Task) Cancel() error {
	t.mu.Lock()
	if t.status == v1.TaskStatus_TASK_COMPLETED ||
		t.status == v1.TaskStatus_TASK_FAILED ||
		t.status == v1.TaskStatus_TASK_CANCELLED {
		t.mu.Unlock()
		return fmt.Errorf("task already finished/cancelled")
	}
	t.status = v1.TaskStatus_TASK_CANCELLED
	t.mu.Unlock()
	if t.cancel != nil {
		t.cancel()
	}
	log.Infof("[task %s] cancelled", t.id)
	return nil
}

func (t *Task) Submit(fn func()) error {
	t.mu.RLock()
	pool := t.pool
	t.mu.RUnlock()
	if pool == nil {
		return fmt.Errorf("pool released")
	}
	return pool.Submit(fn)
}

func (t *Task) Stop() {
	if t.cancel != nil {
		t.cancel()
	}
	t.mu.Lock()
	if t.pool != nil {
		t.pool.Release()
		t.pool = nil
	}
	t.mu.Unlock()
	log.Infof("[task %s] stopped", t.id)
}

// 统计方法
func (t *Task) MarkMemberStart() {
	atomic.AddInt64(&t.active, 1)
}

func (t *Task) MarkMemberDone(ok bool) {
	atomic.AddInt64(&t.active, -1)
	if ok {
		atomic.AddInt64(&t.completed, 1)
	} else {
		atomic.AddInt64(&t.failed, 1)
	}
}

func (t *Task) AddBetOrder(d time.Duration, spinOver bool) {
	atomic.AddInt64(&t.step, 1)
	atomic.AddInt64(&t.duration, d.Nanoseconds())
	if spinOver {
		atomic.AddInt64(&t.process, 1) // 已完成局数
	}
}

func (t *Task) AddBetBonus(d time.Duration) {
	atomic.AddInt64(&t.step, 1)
	atomic.AddInt64(&t.duration, d.Nanoseconds())
}

func (t *Task) AddError(msg string) {
	atomic.AddInt64(&t.errors, 1)
}

// StatsSnapshot 统计快照
type StatsSnapshot struct {
	ID               string
	Status           v1.TaskStatus
	Config           *v1.TaskConfig
	Process          int64
	Target           int64
	Step             int64
	TotalDuration    time.Duration
	ActiveMembers    int64
	CompletedMembers int64
	FailedMembers    int64
	FailedRequests   int64
	CreatedAt        time.Time
	FinishedAt       time.Time
}

func (t *Task) StatsSnapshot() StatsSnapshot {
	t.mu.RLock()
	id, status, cfg, createdAt := t.id, t.status, t.config, t.createdAt
	t.mu.RUnlock()

	return StatsSnapshot{
		ID:               id,
		Status:           status,
		Config:           cfg,
		Process:          atomic.LoadInt64(&t.process),
		Target:           t.target,
		Step:             atomic.LoadInt64(&t.step),
		TotalDuration:    time.Duration(atomic.LoadInt64(&t.duration)),
		ActiveMembers:    atomic.LoadInt64(&t.active),
		CompletedMembers: atomic.LoadInt64(&t.completed),
		FailedMembers:    atomic.LoadInt64(&t.failed),
		FailedRequests:   atomic.LoadInt64(&t.errors),
		CreatedAt:        createdAt,
	}
}

// 监控任务进度
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
	snap := t.StatsSnapshot()
	elapsed := time.Since(start)
	sec := elapsed.Seconds()
	if sec <= 0 {
		return
	}

	pct := 0.0
	if snap.Target > 0 {
		pct = float64(snap.Process) / float64(snap.Target) * 100
	}
	remaining := time.Duration(0)
	if pct > 0 && pct < 100 {
		remaining = time.Duration(float64(elapsed)/pct*100) - elapsed
	}

	log.Infof("[%s]: 进度:%d/%d(%.2f%%), 用时:%s, 剩余:%s, QPS:%.2f, step:%.2f, 延迟:%s    ",
		snap.ID, snap.Process, snap.Target, pct,
		shortDuration(elapsed), shortDuration(remaining),
		float64(snap.Process)/sec, float64(snap.Step)/sec,
		avgDuration(snap.TotalDuration, snap.Step),
	)
}

func (t *Task) printFinalStats(start time.Time) {
	snap := t.StatsSnapshot()
	elapsed := time.Since(start)
	sec := elapsed.Seconds()
	qps := 0.0
	if sec > 0 {
		qps = float64(snap.Process) / sec
	}
	log.Infof("[%s] 任务结束: 进度:%d/%d, 总步数:%d, 耗时:%v, QPS:%.2f, 平均延迟:%s",
		snap.ID, snap.Process, snap.Target, snap.Step, elapsed, qps,
		avgDuration(snap.TotalDuration, snap.Step),
	)
}

func avgDuration(d time.Duration, step int64) string {
	if step <= 0 {
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
