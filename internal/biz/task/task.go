package task

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	v1 "stress/api/stress/v1"
	"stress/internal/biz/game/base"
	"stress/pkg/xgo"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/panjf2000/ants/v2"
)

// Task 压测任务
type Task struct {
	mu        sync.RWMutex
	id        string
	game      base.IGame
	createdAt time.Time

	finishedAt time.Time

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

func (t *Task) GetCreatedAt() time.Time {
	t.mu.RLock()
	createdAt := t.createdAt
	t.mu.RUnlock()
	return createdAt
}

func (t *Task) GetFinishedAt() time.Time {
	t.mu.RLock()
	finishedAt := t.finishedAt
	t.mu.RUnlock()
	return finishedAt
}

func (t *Task) GetStep() int64 {
	return atomic.LoadInt64(&t.step)
}

func (t *Task) SetStatus(s v1.TaskStatus) {
	t.mu.Lock()
	t.status = s
	if s == v1.TaskStatus_TASK_COMPLETED ||
		s == v1.TaskStatus_TASK_FAILED {
		t.finishedAt = time.Now()
	}
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
	log.Infof("[%s] task stopped", t.id)
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

// CompletionReport 生成任务报告（供 metrics、notify、logging 复用）
func (t *Task) CompletionReport(now time.Time) *v1.TaskCompletionReport {
	t.mu.RLock()
	id, cfg := t.id, t.config
	createdAt := t.createdAt
	t.mu.RUnlock()

	process := atomic.LoadInt64(&t.process)
	step := atomic.LoadInt64(&t.step)
	totalDur := time.Duration(atomic.LoadInt64(&t.duration))
	active := atomic.LoadInt64(&t.active)
	completed := atomic.LoadInt64(&t.completed)
	failed := atomic.LoadInt64(&t.failed)
	errors := atomic.LoadInt64(&t.errors)

	elapsed := now.Sub(createdAt)
	if !t.finishedAt.IsZero() {
		elapsed = t.finishedAt.Sub(createdAt)
	}
	sec := elapsed.Seconds()
	qps := 0.0
	if sec > 0 {
		qps = float64(process) / sec
	}

	gameID := int64(0)
	if cfg != nil {
		gameID = cfg.GameId
	}

	avgLatency := xgo.AvgDuration(totalDur, step)

	return &v1.TaskCompletionReport{
		TaskId:        id,
		GameId:        gameID,
		Process:       process,
		Target:        t.target,
		Step:          step,
		Duration:      xgo.FormatDuration(elapsed),
		Qps:           qps,
		AvgLatency:    avgLatency,
		ActiveMembers: active,
		Completed:     completed,
		Failed:        failed,
		FailedReqs:    errors,
		ProgressPct:   xgo.PctCap100(process, t.target),
		// OrderCount, TotalBet, TotalWin, RtpPct 由 stats 包补充
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
	r := t.CompletionReport(time.Now())
	elapsed := time.Since(start)
	sec := elapsed.Seconds()
	if sec <= 0 {
		return
	}
	remaining := time.Duration(0)
	if r.ProgressPct > 0 && r.ProgressPct < 100 {
		remaining = time.Duration(float64(elapsed)/r.ProgressPct*100) - elapsed
	}
	qps := 0.0
	if sec > 0 {
		qps = float64(r.Process) / sec
	}
	log.Infof("[%s]: 进度:%d/%d(%.2f%%), 用时:%s, 剩余:%s, QPS:%.2f, step:%.2f, 延迟:%s    ",
		r.TaskId,
		r.Process,
		r.Target,
		r.ProgressPct,
		xgo.ShortDuration(elapsed),
		xgo.ShortDuration(remaining),
		qps,
		float64(r.Step)/sec,
		r.AvgLatency,
	)
}

func (t *Task) printFinalStats(start time.Time) {
	r := t.CompletionReport(time.Now())
	elapsed := time.Since(start)
	qps := 0.0
	if sec := elapsed.Seconds(); sec > 0 {
		qps = float64(r.Process) / sec
	}
	log.Infof("[%s] 任务结束: 进度:%d/%d, 总步数:%d, 耗时:%v, QPS:%.2f, 平均延迟:%s",
		r.TaskId,
		r.Process,
		r.Target,
		r.Step,
		elapsed,
		qps,
		r.AvgLatency,
	)
}
