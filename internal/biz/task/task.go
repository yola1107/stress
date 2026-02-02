package task

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	v1 "stress/api/stress/v1"
	"stress/internal/biz/game/base"
	"stress/pkg/xgo"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

const (
	logInterval = 1 * time.Second // 进度日志间隔
)

// Task 压测任务实体（领域模型）
type Task struct {
	mu         sync.RWMutex
	id         string
	game       base.IGame
	createdAt  time.Time
	finishedAt time.Time

	status      v1.TaskStatus
	config      *v1.TaskConfig
	bonusConfig *v1.BetBonusConfig

	ctx    context.Context
	cancel context.CancelFunc

	// 统计原子计数
	target    int64
	process   int64
	step      int64
	duration  int64
	active    int64
	completed int64
	failed    int64
	errors    int64
}

// NewTask 创建任务，parent 取消时任务会收到信号
func NewTask(parent context.Context, id string, g base.IGame, cfg *v1.TaskConfig) (*Task, error) {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	return &Task{
		id:        id,
		status:    v1.TaskStatus_TASK_PENDING,
		config:    cfg,
		createdAt: time.Now(),
		game:      g,
		ctx:       ctx,
		cancel:    cancel,
		target:    int64(cfg.MemberCount) * int64(cfg.TimesPerMember),
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
	return t.game
}

func (t *Task) GetBonusConfig() *v1.BetBonusConfig {
	return t.bonusConfig
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

func (t *Task) GetCreatedAt() time.Time {
	t.mu.RLock()
	createdAt := t.createdAt
	t.mu.RUnlock()
	return createdAt
}

func (t *Task) SetFinishAt() {
	t.mu.Lock()
	t.finishedAt = time.Now()
	t.mu.Unlock()
}

func (t *Task) GetFinishedAt() time.Time {
	t.mu.RLock()
	finishedAt := t.finishedAt
	t.mu.RUnlock()
	return finishedAt
}

func (t *Task) Cancel() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.status == v1.TaskStatus_TASK_COMPLETED ||
		t.status == v1.TaskStatus_TASK_FAILED ||
		t.status == v1.TaskStatus_TASK_CANCELLED {
		return errors.BadRequest("TASK_ALREADY_FINISHED", "task already finished or cancelled")
	}

	t.status = v1.TaskStatus_TASK_CANCELLED
	t.Stop()
	log.Infof("[%s] task cancelled", t.id)
	return nil
}

func (t *Task) Stop() {
	if t.cancel != nil {
		t.cancel()
	}
}

// ============== 统计方法 =============================

func (t *Task) GetStep() int64 {
	return atomic.LoadInt64(&t.step)
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

// Snapshot 获取当前任务状态快照（供 metrics、notify、logging 复用）
func (t *Task) Snapshot(now time.Time) *v1.TaskCompletionReport {
	t.mu.RLock()
	finishedAt := t.finishedAt
	t.mu.RUnlock()

	process := atomic.LoadInt64(&t.process)
	step := atomic.LoadInt64(&t.step)

	// 计算耗时和 QPS
	elapsed := now.Sub(t.createdAt)
	if !finishedAt.IsZero() {
		elapsed = finishedAt.Sub(t.createdAt)
	}
	qps := 0.0
	if sec := elapsed.Seconds(); sec > 0 {
		qps = float64(process) / sec
	}

	// 计算平均延迟
	avgLatency := xgo.AvgDuration(time.Duration(atomic.LoadInt64(&t.duration)), step)

	return &v1.TaskCompletionReport{
		TaskId:        t.id,
		GameId:        t.config.GameId,
		Process:       process,
		Target:        t.target,
		Step:          step,
		Duration:      xgo.FormatDuration(elapsed),
		Qps:           qps,
		AvgLatency:    avgLatency,
		ActiveMembers: atomic.LoadInt64(&t.active),
		Completed:     atomic.LoadInt64(&t.completed),
		Failed:        atomic.LoadInt64(&t.failed),
		FailedReqs:    atomic.LoadInt64(&t.errors),
		ProgressPct:   xgo.PctCap100(process, t.target),
		// OrderCount, TotalBet, TotalWin, RtpPct 由 上游 包补充
	}
}

// SetStart 标记会话开始执行
func (t *Task) SetStart(cnt int64, bounds []*v1.BetBonusConfig) {
	t.mu.Lock()
	t.status = v1.TaskStatus_TASK_RUNNING
	for _, b := range bounds {
		if b != nil && b.GameId == t.config.GameId {
			t.bonusConfig = b
			break
		}
	}
	t.mu.Unlock()

	atomic.AddInt64(&t.active, cnt)
	go t.Monitor()
}

// MarkSessionDone 标记会话执行完成
func (t *Task) MarkSessionDone(ok bool) {
	atomic.AddInt64(&t.active, -1)
	if ok {
		atomic.AddInt64(&t.completed, 1)
	} else {
		atomic.AddInt64(&t.failed, 1)
	}
}

// Monitor 运行监控：1s 日志输出，task context 取消后退出
func (t *Task) Monitor() {
	start := time.Now()
	tick := time.NewTicker(logInterval)
	defer tick.Stop()

	t.printProgress(start)

	for {
		select {
		case <-t.ctx.Done():
			t.printFinalStats(start)
			return
		case <-tick.C:
			t.printProgress(start)
		}
	}
}

func (t *Task) printProgress(start time.Time) {
	elapsed := time.Since(start)
	sec := elapsed.Seconds()
	if sec <= 0 {
		return
	}

	// 从Task获取数据
	process := atomic.LoadInt64(&t.process)
	step := atomic.LoadInt64(&t.step)
	target := t.target

	progressPct := xgo.PctCap100(process, target)
	remaining := time.Duration(0)
	if progressPct > 0 && progressPct < 100 {
		remaining = time.Duration(float64(elapsed)/progressPct*100) - elapsed
	}

	qps := float64(process) / sec
	totalDur := time.Duration(atomic.LoadInt64(&t.duration))
	avgLatency := xgo.AvgDuration(totalDur, step)

	log.Infof("[%s]: 进度:%d/%d(%.2f%%), 用时:%s, 剩余:%s, QPS:%.2f, step:%.2f, 延迟:%s",
		t.id,
		process,
		target,
		progressPct,
		xgo.ShortDuration(elapsed),
		xgo.ShortDuration(remaining),
		qps,
		float64(step)/sec,
		avgLatency,
	)
}

func (t *Task) printFinalStats(start time.Time) {
	elapsed := time.Since(start)
	process := atomic.LoadInt64(&t.process)
	step := atomic.LoadInt64(&t.step)
	target := t.target

	qps := 0.0
	if sec := elapsed.Seconds(); sec > 0 {
		qps = float64(process) / sec
	}

	totalDur := time.Duration(atomic.LoadInt64(&t.duration))
	avgLatency := xgo.AvgDuration(totalDur, step)

	log.Infof("[%s] 任务结束: 进度:%d/%d, 总步数:%d, 耗时:%v, QPS:%.2f, 平均延迟:%s",
		t.id,
		process,
		target,
		step,
		elapsed,
		qps,
		avgLatency,
	)
}
