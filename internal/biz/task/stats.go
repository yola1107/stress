package task

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	v1 "stress/api/stress/v1"

	"github.com/go-kratos/kratos/v2/log"
)

// StatsSnapshot 完整快照（元数据+统计），一次性返回
type StatsSnapshot struct {
	ID                                             string
	Description                                    string
	Status                                         v1.TaskStatus
	Config                                         *v1.TaskConfig
	UserIDCount                                    int
	Step                                           int64
	Process, Target                                int64
	BetOrders, BetBonuses                          int64
	TotalDuration                                  time.Duration
	ActiveMembers, CompletedMembers, FailedMembers int64
	FailedRequests                                 int64
	CreatedAt, FinishedAt                          time.Time
	ErrorCounts                                    map[string]int64
}

// TaskStats 任务统计
type TaskStats struct {
	meta *taskMeta // 与 Task 共享

	// 统计信息（原子操作）
	process       atomic.Int64
	target        atomic.Int64
	betOrderCount atomic.Int64
	betBonusCount atomic.Int64
	totalDuration atomic.Int64

	// TaskProgress 补齐：成员与失败请求统计
	activeMembers    atomic.Int64
	completedMembers atomic.Int64
	failedMembers    atomic.Int64
	failedRequests   atomic.Int64

	errorMu     sync.Mutex
	errorCounts map[string]int64
}

func NewTaskStats(target int64, meta *taskMeta) *TaskStats {
	s := &TaskStats{
		meta:        meta,
		errorCounts: make(map[string]int64),
	}
	s.target.Store(target)
	return s
}

func (s *TaskStats) Step() int64 {
	return s.betOrderCount.Load() + s.betBonusCount.Load()
}

func (s *TaskStats) MarkMemberStart() {
	s.activeMembers.Add(1)
}

func (s *TaskStats) MarkMemberDone(failed bool) {
	s.activeMembers.Add(-1)
	if failed {
		s.failedMembers.Add(1)
	} else {
		s.completedMembers.Add(1)
	}
}

func (s *TaskStats) addStep(betOrder, betBonus int64, duration time.Duration, spinOver bool) {
	if betOrder > 0 {
		s.betOrderCount.Add(betOrder)
	}
	if betBonus > 0 {
		s.betBonusCount.Add(betBonus)
	}
	s.totalDuration.Add(duration.Nanoseconds())
	if spinOver {
		s.process.Add(1)
	}
}

func (s *TaskStats) AddBetOrder(duration time.Duration, spinOver bool) {
	s.addStep(1, 0, duration, spinOver)
}

func (s *TaskStats) AddBetBonus(duration time.Duration) {
	s.addStep(0, 1, duration, false)
}

func (s *TaskStats) AddError(errMsg string) {
	s.errorMu.Lock()
	s.errorCounts[errMsg]++
	s.errorMu.Unlock()
	s.failedRequests.Add(1)
}

// StatsSnapshot 一次性返回完整快照（元数据+统计）
func (s *TaskStats) StatsSnapshot() StatsSnapshot {
	s.meta.mu.RLock()
	id, desc, status, cfg := s.meta.id, s.meta.description, s.meta.status, s.meta.config
	createdAt, finishedAt := s.meta.createdAt, s.meta.finishedAt
	userIDCount := len(s.meta.userIDs)
	s.meta.mu.RUnlock()

	s.errorMu.Lock()
	ec := make(map[string]int64, len(s.errorCounts))
	for k, v := range s.errorCounts {
		ec[k] = v
	}
	s.errorMu.Unlock()

	return StatsSnapshot{
		ID:               id,
		Description:      desc,
		Status:           status,
		Config:           cfg,
		UserIDCount:      userIDCount,
		Step:             s.Step(),
		Process:          s.process.Load(),
		Target:           s.target.Load(),
		BetOrders:        s.betOrderCount.Load(),
		BetBonuses:       s.betBonusCount.Load(),
		TotalDuration:    time.Duration(s.totalDuration.Load()),
		ActiveMembers:    s.activeMembers.Load(),
		CompletedMembers: s.completedMembers.Load(),
		FailedMembers:    s.failedMembers.Load(),
		FailedRequests:   s.failedRequests.Load(),
		CreatedAt:        createdAt,
		FinishedAt:       finishedAt,
		ErrorCounts:      ec,
	}
}

// QPS 计算每秒请求数
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

// AvgLatencyMs 计算平均延迟
func (s *StatsSnapshot) AvgLatencyMs() float64 {
	total := s.BetOrders + s.BetBonuses
	if total <= 0 {
		return 0
	}
	return float64(s.TotalDuration.Milliseconds()) / float64(total)
}

// SuccessRate 计算成功率
func (s *StatsSnapshot) SuccessRate() float64 {
	if s.Process <= 0 {
		return 0
	}
	return float64(s.Process-s.FailedRequests) / float64(s.Process)
}

// Monitor 启动进度监控
func (s *TaskStats) Monitor(ctx context.Context) {
	start := time.Now()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.printFinalStats(start)
			return
		case <-ticker.C:
			s.printProgress(start)
		}
	}
}

func (s *TaskStats) printProgress(start time.Time) {
	s.meta.mu.RLock()
	id := s.meta.id
	s.meta.mu.RUnlock()
	process, target := s.process.Load(), s.target.Load()
	step := s.Step()
	totalDuration := time.Duration(s.totalDuration.Load())
	elapsed := time.Since(start)
	pct := 0.0
	if target > 0 {
		pct = float64(process) / float64(target) * 100
	}
	remaining := time.Duration(0)
	if pct > 0 {
		remaining = time.Duration(int64(float64(elapsed)/pct*100)) - elapsed
	}
	log.Infof("[%s]: 进度:%d/%d(%.2f%%), 用时:%s, 剩余:%s, QPS:%.2f, step:%.2f, 延迟:%s    ",
		id, process, target, pct,
		shortDuration(elapsed), shortDuration(remaining),
		float64(process)/elapsed.Seconds(), float64(step)/elapsed.Seconds(),
		avgDuration(totalDuration, step),
	)
}

func (s *TaskStats) printFinalStats(start time.Time) {
	s.meta.mu.RLock()
	id := s.meta.id
	s.meta.mu.RUnlock()
	process, target := s.process.Load(), s.target.Load()
	step := s.Step()
	totalDuration := time.Duration(s.totalDuration.Load())
	elapsed := time.Since(start)
	log.Infof("[%s] 任务结束: 进度:%d/%d, 总步数:%d, 耗时:%v, QPS:%.2f, 平均延迟:%s",
		id, process, target, step, elapsed,
		float64(process)/elapsed.Seconds(),
		avgDuration(totalDuration, step),
	)
}

func avgDuration(d time.Duration, step int64) string {
	if step == 0 {
		return "0"
	}
	return shortDuration(time.Duration(int64(d) / step))
}

var durationUnits = []struct {
	div float64
	sym string
}{
	{60 * 60 * 24, "d"}, {60 * 60, "h"}, {60, "m"}, {1, "s"},
	{1e-3, "ms"}, {1e-6, "µs"}, {1e-9, "ns"},
}

func shortDuration(d time.Duration) string {
	if d == 0 {
		return "0"
	}
	sec := d.Seconds()
	for _, u := range durationUnits {
		if sec >= u.div {
			val := sec / u.div
			format := "%.2f%s"
			if val >= 100 {
				format = "%.0f%s"
			} else if val >= 10 {
				format = "%.1f%s"
			}
			return fmt.Sprintf(format, val, u.sym)
		}
	}
	return "0"
}
