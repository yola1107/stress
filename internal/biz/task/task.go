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
)

// Task 压测任务实体（领域模型）
type Task struct {
	mu        sync.RWMutex
	id        string
	game      base.IGame
	config    *v1.TaskConfig
	status    v1.TaskStatus
	createdAt time.Time
	startAt   time.Time // 实际开始执行时间
	finishAt  time.Time
	record    string // S3 HTML 图表 URL
	ctx       context.Context
	cancel    context.CancelFunc
	log       *log.Helper
	stats     Stats // 统计信息（线程安全）
}

// Stats TaskStats 任务统计信息（线程安全）
type Stats struct {
	Target    int64 // 目标请求数
	Process   int64 // 已完成局数
	Step      int64 // 总请求数
	Duration  int64 // 总耗时（纳秒）
	Active    int64 // 活跃成员数
	Completed int64 // 成功完成的成员数
	Failed    int64 // 失败的成员数
	Errors    int64 // 错误次数
}

// NewTask 创建任务，parent 取消时任务会收到信号
func NewTask(parent context.Context, id string, g base.IGame, cfg *v1.TaskConfig, logger log.Logger) (*Task, error) {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	return &Task{
		id:        id,
		game:      g,
		config:    cfg,
		status:    v1.TaskStatus_TASK_PENDING,
		createdAt: time.Now(),
		stats:     Stats{Target: int64(cfg.MemberCount) * int64(cfg.TimesPerMember)},
		ctx:       ctx,
		cancel:    cancel,
		log:       log.NewHelper(logger),
	}, nil
}

func (t *Task) GetID() string             { return t.id }
func (t *Task) Context() context.Context  { return t.ctx }
func (t *Task) GetConfig() *v1.TaskConfig { return t.config }
func (t *Task) GetGame() base.IGame       { return t.game }
func (t *Task) GetCreatedAt() time.Time   { return t.createdAt }

func (t *Task) AddActive(delta int64) { atomic.AddInt64(&t.stats.Active, delta) }

func (t *Task) GetStatus() v1.TaskStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.status
}

func (t *Task) SetStatus(s v1.TaskStatus) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status = s
}

func (t *Task) CompareAndSetStatus(old, new v1.TaskStatus) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.status == old {
		t.status = new
		return true
	}
	return false
}

func (t *Task) SetStartAt() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.startAt.IsZero() {
		t.startAt = time.Now()
	}
}

func (t *Task) GetStartAt() time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.startAt
}

func (t *Task) SetFinishAt() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.finishAt = time.Now()
}

func (t *Task) GetFinishedAt() time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.finishAt
}

func (t *Task) SetRecordUrl(url string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.record = url
}

func (t *Task) GetRecordUrl() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.record
}

func (t *Task) Cancel() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.status != v1.TaskStatus_TASK_PENDING && t.status != v1.TaskStatus_TASK_RUNNING {
		return fmt.Errorf("TASK_ALREADY_FINISHED. task_id: %s", t.id)
	}

	t.status = v1.TaskStatus_TASK_CANCELLED
	if t.finishAt.IsZero() {
		t.finishAt = time.Now()
	}
	t.Stop()
	t.log.Infof("[%s] task cancelled", t.id)
	return nil
}

func (t *Task) Stop() {
	if t.cancel != nil {
		t.cancel()
	}
}

// ============== 统计方法 =============================

func (t *Task) GetStep() int64   { return atomic.LoadInt64(&t.stats.Step) }
func (t *Task) GetTarget() int64 { return atomic.LoadInt64(&t.stats.Target) }
func (t *Task) IsTargetReached() bool {
	target := atomic.LoadInt64(&t.stats.Target)
	if target <= 0 {
		return false
	}
	return atomic.LoadInt64(&t.stats.Process) >= target
}

func (t *Task) AddBetOrder(d time.Duration, spinOver bool) {
	atomic.AddInt64(&t.stats.Step, 1)
	atomic.AddInt64(&t.stats.Duration, d.Nanoseconds())
	if spinOver {
		atomic.AddInt64(&t.stats.Process, 1)
	}
}

func (t *Task) AddBetBonus(d time.Duration) {
	atomic.AddInt64(&t.stats.Step, 1)
	atomic.AddInt64(&t.stats.Duration, d.Nanoseconds())
}

func (t *Task) AddError() { atomic.AddInt64(&t.stats.Errors, 1) }

type metricsData struct {
	Process     int64
	Step        int64
	Target      int64
	Elapsed     time.Duration
	QPS         float64
	AvgLatency  string
	ProgressPct float64
	Remaining   time.Duration
}

// calculateMetrics 直接计算并返回任务指标
func (t *Task) calculateMetrics(now time.Time) metricsData {
	t.mu.RLock()
	finishedAt := t.finishAt
	startAt := t.startAt
	t.mu.RUnlock()

	effectiveStart := startAt
	if effectiveStart.IsZero() {
		effectiveStart = t.createdAt
	}

	m := metricsData{
		Process: atomic.LoadInt64(&t.stats.Process),
		Step:    atomic.LoadInt64(&t.stats.Step),
		Target:  atomic.LoadInt64(&t.stats.Target),
	}
	duration := atomic.LoadInt64(&t.stats.Duration)

	// 计算耗时
	m.Elapsed = now.Sub(effectiveStart)
	if !finishedAt.IsZero() {
		m.Elapsed = finishedAt.Sub(effectiveStart)
	}

	// 计算QPS
	if sec := m.Elapsed.Seconds(); sec > 0 {
		m.QPS = float64(m.Process) / sec
	}

	// 计算平均延迟
	totalDur := time.Duration(duration)
	if m.Step > 0 {
		m.AvgLatency = fmt.Sprintf("%.2fms", float64(totalDur.Nanoseconds())/float64(m.Step)/1e6)
	} else {
		m.AvgLatency = "0ms"
	}

	// 计算进度百分比
	if m.Target > 0 {
		m.ProgressPct = float64(m.Process*100) / float64(m.Target)
		if m.ProgressPct > 100 {
			m.ProgressPct = 100
		}
	}

	// 计算剩余时间
	if m.ProgressPct > 0 && m.ProgressPct < 100 {
		m.Remaining = time.Duration(float64(m.Elapsed)/m.ProgressPct*100) - m.Elapsed
	}

	return m
}

// Snapshot 获取当前任务状态快照（供 metrics、notify、logging 复用）
func (t *Task) Snapshot(now time.Time) *v1.TaskCompletionReport {
	m := t.calculateMetrics(now)

	active := atomic.LoadInt64(&t.stats.Active)
	completed := atomic.LoadInt64(&t.stats.Completed)
	failed := atomic.LoadInt64(&t.stats.Failed)
	errors := atomic.LoadInt64(&t.stats.Errors)

	return &v1.TaskCompletionReport{
		TaskId:        t.id,
		GameId:        t.game.GameID(),
		GameName:      t.game.Name(),
		Process:       m.Process,
		Target:        m.Target,
		Step:          m.Step,
		ProgressPct:   m.ProgressPct,
		Duration:      m.Elapsed.String(),
		Qps:           m.QPS,
		AvgLatency:    m.AvgLatency,
		ActiveMembers: active,
		Completed:     completed,
		Failed:        failed,
		FailedReqs:    errors,
		// OrderCount, TotalBet, TotalWin, RtpPct 由 上游 包补充
		// 时间信息存储在其他地方或通过其他方式获取
	}
}

// MarkSessionDone 标记会话执行完成
func (t *Task) MarkSessionDone(ok bool) {
	atomic.AddInt64(&t.stats.Active, -1)
	if ok {
		atomic.AddInt64(&t.stats.Completed, 1)
	} else {
		atomic.AddInt64(&t.stats.Failed, 1)
	}
}

// LogProgress 打印当前任务进度
func (t *Task) LogProgress(isFinal bool) {
	now := time.Now()
	m := t.calculateMetrics(now)

	if isFinal {
		t.log.Infof("[%s] 任务结束: 进度:%d/%d, 总步数:%d, 耗时:%v, QPS:%.2f, 平均延迟:%s",
			t.id, m.Process, m.Target, m.Step, m.Elapsed, m.QPS, m.AvgLatency)
	} else {
		t.log.Infof("[%s]: 进度:%d/%d(%.2f%%), 用时:%s, 剩余:%s, QPS:%.2f, step:%.2f, 延迟:%s",
			t.id, m.Process, m.Target, m.ProgressPct, xgo.ShortDuration(m.Elapsed),
			xgo.ShortDuration(m.Remaining), m.QPS, float64(m.Step)/m.Elapsed.Seconds(), m.AvgLatency)
	}
}

// ToProto 将业务层 Task 转换为 protobuf Task
func (t *Task) ToProto() *v1.Task {
	if t == nil {
		return nil
	}
	t.mu.RLock()
	defer t.mu.RUnlock()

	var description string
	if t.config != nil {
		description = t.config.GetDescription()
	}

	ret := &v1.Task{
		TaskId:      t.id,
		Description: description,
		Status:      int32(t.status),
		Process:     atomic.LoadInt64(&t.stats.Process),
		Config:      t.config,
		RecordUrl:   t.record,
		CreatedAt:   t.createdAt.Format(time.DateTime),
	}
	if !t.startAt.IsZero() {
		ret.StartAt = t.startAt.Format(time.DateTime)
	}
	if !t.finishAt.IsZero() {
		ret.FinishAt = t.finishAt.Format(time.DateTime)
	}
	return ret
}
