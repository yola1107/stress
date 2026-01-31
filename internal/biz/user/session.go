package user

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync/atomic"
	"time"

	v1 "stress/api/stress/v1"
	"stress/internal/biz/game/base"
	"stress/internal/biz/task"
)

// Session 相关常量
const (
	delayMills        = 100 * time.Millisecond // 重试延迟
	defaultMaxRetries = 5                      // 最大重试次数
)

type SessionState int32

const (
	SessionStateIdle        SessionState = 1
	SessionStateLaunching   SessionState = 2
	SessionStateLoggingIn   SessionState = 3
	SessionStateBetting     SessionState = 4
	SessionStateBonusSelect SessionState = 5
	SessionStateCompleted   SessionState = 6
	SessionStateFailed      SessionState = 7
)

type Session struct {
	UserID     int64
	MemberName string

	State SessionState
	Token string

	Process    int32
	TryTimes   int32
	bonusIndex int64

	LastError string

	Client *APIClient
	task   *task.Task
}

func NewSession(userID int64, memberName string, task *task.Task) *Session {
	return &Session{
		UserID:     userID,
		MemberName: memberName,
		State:      SessionStateIdle,
		Token:      "",
		Process:    0,
		TryTimes:   0,
		LastError:  "",
		Client:     nil,
		task:       task,
		bonusIndex: 0,
	}
}

func (s *Session) Execute(ctx context.Context, client *APIClient, _ base.SecretProvider) error {
	cfg := s.task.GetConfig()
	if cfg == nil {
		return fmt.Errorf("task config is nil")
	}
	s.Client = client
	target := cfg.TimesPerMember
	game := s.task.GetGame()
	bonusCfg := s.task.GetBonusConfig()

	maxRetries := defaultMaxRetries
	for {
		if s.State == SessionStateCompleted || s.State == SessionStateFailed {
			break
		}

		select {
		case <-ctx.Done():
			s.State = SessionStateFailed
			return ctx.Err()
		default:
		}

		if err := s.executeStep(ctx, cfg, target, game, bonusCfg); err != nil {
			if !s.handleError(err, maxRetries) {
				return err
			}
		}

		// 检查任务是否被取消
		if s.task != nil && s.task.GetStatus() == v1.TaskStatus_TASK_CANCELLED {
			s.State = SessionStateFailed
			s.LastError = "task cancelled"
			return nil
		}
	}
	return nil
}

func (s *Session) executeStep(ctx context.Context, cfg *v1.TaskConfig, target int32, game base.IGame, bonusCfg *v1.BetBonusConfig) error {
	atomic.AddInt32(&s.TryTimes, 1)

	var err error
	switch s.State {
	case SessionStateIdle, SessionStateLaunching:
		var token string
		token, err = s.Client.Launch(ctx, cfg, s.MemberName)
		if err == nil {
			s.Token = token
			s.State = SessionStateLoggingIn
			atomic.StoreInt32(&s.TryTimes, 0)
		} else if apiErr, ok := err.(*APIError); ok && apiErr.Op == "launch" {
			s.State = SessionStateFailed
		}
		return err
	case SessionStateLoggingIn:
		var token string
		var freeData map[string]any
		token, freeData, err = s.Client.Login(ctx, cfg, s.Token)
		if err == nil {
			s.Token = token
			s.State = SessionStateBetting
			if s.checkNeedBonus(freeData, game, bonusCfg) {
				s.State = SessionStateBonusSelect
			}
			atomic.StoreInt32(&s.TryTimes, 0)
		}
		return err
	case SessionStateBetting:
		start := time.Now()
		var data map[string]any
		data, err = s.Client.BetOrder(ctx, cfg, s.Token)
		if err == nil {
			duration := time.Since(start)
			spinOver := s.checkSpinOver(data, game)
			if spinOver && atomic.AddInt32(&s.Process, 1) >= target {
				s.State = SessionStateCompleted
			}
			if s.checkNeedBonus(data, game, bonusCfg) {
				s.State = SessionStateBonusSelect
			}
			if s.task != nil {
				s.task.AddBetOrder(duration, spinOver)
			}
			atomic.StoreInt32(&s.TryTimes, 0)
		} else {
			err = s.handleBetOrderError(err)
		}
		return err
	case SessionStateBonusSelect:
		start := time.Now()
		var res *BetBonusResult
		res, err = s.Client.BetBonus(ctx, cfg, s.Token, s.pickBonusNum(bonusCfg))
		if err == nil {
			duration := time.Since(start)
			if !res.NeedContinue {
				s.State = SessionStateBetting
			}
			if s.task != nil {
				s.task.AddBetBonus(duration)
			}
			atomic.StoreInt32(&s.TryTimes, 0)
		}
		return err
	case SessionStateCompleted, SessionStateFailed:
		return nil
	default:
		return fmt.Errorf("unknown state: %v", s.State)
	}
}

func (s *Session) handleBetOrderError(err error) error {
	betErr, ok := err.(*BetOrderError)
	if !ok {
		return err
	}
	if betErr.SleepDuration > 0 {
		if !s.sleepOrCancel(betErr.SleepDuration) {
			return fmt.Errorf("bet order cancelled")
		}
	}
	if betErr.NeedRelaunch {
		s.State = SessionStateLaunching
		s.Token = ""
	} else if betErr.NeedRelogin {
		s.State = SessionStateLoggingIn
	}
	return betErr
}

func (s *Session) handleError(err error, maxRetries int) bool {
	errMsg := err.Error()

	if s.task != nil && s.LastError != errMsg {
		s.task.AddError(errMsg)
	}
	s.LastError = errMsg

	tryTimes := atomic.LoadInt32(&s.TryTimes)

	if tryTimes > int32(maxRetries) {
		s.State = SessionStateFailed
		return false
	}

	if apiErr, ok := err.(*APIError); ok && apiErr.Op == "launch" {
		if !s.sleepOrCancel(delayMills) {
			return false
		}
		s.State = SessionStateFailed
		return false
	}

	if betErr, ok := err.(*BetOrderError); ok {
		if !s.sleepOrCancel(betErr.SleepDuration) {
			return false
		}
		return true
	}

	if !s.sleepOrCancel(delayMills) {
		return false
	}

	return true
}

func (s *Session) sleepOrCancel(duration time.Duration) bool {
	select {
	case <-time.After(duration):
		return true
	case <-s.task.Context().Done():
		s.State = SessionStateFailed
		return false
	}
}

func (s *Session) pickBonusNum(bonusCfg *v1.BetBonusConfig) int64 {
	if bonusCfg == nil {
		return s.getNextBonus(nil)
	}
	if bonusCfg.BonusNum > 0 {
		return bonusCfg.BonusNum
	}
	if len(bonusCfg.RandomNums) == 2 && bonusCfg.RandomNums[0] <= bonusCfg.RandomNums[1] {
		return bonusCfg.RandomNums[0] + rand.Int64N(bonusCfg.RandomNums[1]-bonusCfg.RandomNums[0]+1)
	}
	if len(bonusCfg.BonusSequence) > 0 {
		return s.getNextBonus(bonusCfg)
	}
	return 1
}

func (s *Session) getNextBonus(bonusCfg *v1.BetBonusConfig) int64 {
	if bonusCfg == nil || len(bonusCfg.BonusSequence) == 0 {
		return 1
	}
	seq := bonusCfg.BonusSequence
	newIndex := atomic.AddInt64(&s.bonusIndex, 1)
	idx := int(newIndex) % len(seq)
	return seq[idx]
}

func (s *Session) checkSpinOver(data map[string]any, game base.IGame) bool {
	if game != nil {
		return game.IsSpinOver(data)
	}
	return isSpinOver(data)
}

func (s *Session) checkNeedBonus(data map[string]any, game base.IGame, bonusCfg *v1.BetBonusConfig) bool {
	if game != nil && game.NeedBetBonus(data) {
		return true
	}
	if bonusCfg == nil {
		return false
	}
	return needBetBonus(data)
}

func isSpinOver(data map[string]any) bool {
	if data == nil {
		return false
	}
	if over, ok := data["isOver"].(bool); ok {
		return over
	}
	if rtp, ok := data["rtp"].(map[string]any); ok {
		if over, ok := rtp["isOver"].(bool); ok {
			return over
		}
	}
	win := fmt.Sprintf("%v", data["win"])
	freeNum := fmt.Sprintf("%v", data["freeNum"])
	return win == "0" && freeNum == "0"
}

func needBetBonus(data map[string]any) bool {
	if data == nil {
		return false
	}
	if v, ok := data["bonusState"]; ok {
		return fmt.Sprintf("%v", v) == "1"
	}
	if rtp, ok := data["rtp"].(map[string]any); ok {
		if v, ok := rtp["bonusState"]; ok {
			return fmt.Sprintf("%v", v) == "1"
		}
	}
	return false
}

func (s *Session) IsFailed() bool {
	return s.State == SessionStateFailed
}
