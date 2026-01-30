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

const (
	delayMills        = 100 * time.Millisecond // 减少重试延迟，提升响应速度
	defaultMaxRetries = 5                      // 减少最大重试次数，避免过多等待
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
	GameID     int64
	TaskID     string

	State SessionState
	Token string

	Process  int32
	Target   int32
	TryTimes int32

	BonusSequence []int64

	LastError string
	CreatedAt time.Time
	UpdatedAt time.Time

	Config          *v1.TaskConfig
	Game            base.IGame
	Client          *APIClient
	protobufChecker func(gameID int64) bool
	task            *task.Task

	bonusIndex       int64
	bonusConfigCache atomic.Value
}

func NewSession(userID int64, memberName string, gameID int64, taskID string, task *task.Task, protobufChecker func(gameID int64) bool) *Session {
	now := time.Now()
	return &Session{
		UserID:           userID,
		MemberName:       memberName,
		GameID:           gameID,
		TaskID:           taskID,
		State:            SessionStateIdle,
		Token:            "",
		Process:          0,
		Target:           0,
		TryTimes:         0,
		BonusSequence:    nil,
		LastError:        "",
		CreatedAt:        now,
		UpdatedAt:        now,
		Config:           nil,
		Game:             nil,
		Client:           nil,
		protobufChecker:  protobufChecker,
		task:             task,
		bonusIndex:       0,
		bonusConfigCache: atomic.Value{},
	}
}

func (s *Session) Execute(ctx context.Context, cfg *v1.TaskConfig, game base.IGame, secretProvider base.SecretProvider) error {
	if cfg == nil {
		return fmt.Errorf("task config is nil")
	}
	s.Config = cfg
	s.Game = game
	s.Client = NewAPIClient(DefaultHTTPClient(), secretProvider, game, s.protobufChecker)

	if atomic.LoadInt32(&s.Target) == 0 && cfg.TimesPerMember > 0 {
		atomic.StoreInt32(&s.Target, cfg.TimesPerMember)
	}

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

		if err := s.executeStep(ctx); err != nil {
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

func (s *Session) executeStep(ctx context.Context) error {
	atomic.AddInt32(&s.TryTimes, 1)
	defer func() { s.UpdatedAt = time.Now() }()

	var err error
	switch s.State {
	case SessionStateIdle, SessionStateLaunching:
		var token string
		token, err = s.Client.Launch(ctx, s.Config, s.MemberName)
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
		token, freeData, err = s.Client.Login(ctx, s.Config, s.Token)
		if err == nil {
			s.Token = token
			s.State = SessionStateBetting
			if s.checkNeedBonus(freeData) {
				s.State = SessionStateBonusSelect
			}
			atomic.StoreInt32(&s.TryTimes, 0)
		}
		return err
	case SessionStateBetting:
		start := time.Now()
		var data map[string]any
		data, err = s.Client.BetOrder(ctx, s.Config, s.Token)
		if err == nil {
			duration := time.Since(start)
			spinOver := s.checkSpinOver(data)
			if spinOver {
				if atomic.AddInt32(&s.Process, 1) >= atomic.LoadInt32(&s.Target) {
					s.State = SessionStateCompleted
				}
			}
			if s.checkNeedBonus(data) {
				s.State = SessionStateBonusSelect
			}
			if s.task != nil {
				s.task.GetStats().AddBetOrder(duration, spinOver)
			}
			atomic.StoreInt32(&s.TryTimes, 0)
		} else {
			err = s.handleBetOrderError(err)
		}
		return err
	case SessionStateBonusSelect:
		start := time.Now()
		var res *BetBonusResult
		res, err = s.Client.BetBonus(ctx, s.Config, s.Token, s.pickBonusNum())
		if err == nil {
			duration := time.Since(start)
			if !res.NeedContinue {
				s.State = SessionStateBetting
			}
			if s.task != nil {
				s.task.GetStats().AddBetBonus(duration)
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
	s.UpdatedAt = time.Now()

	if s.task != nil && s.LastError != errMsg {
		s.task.GetStats().AddError(errMsg)
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

func (s *Session) pickBonusNum() int64 {
	bonusCfg := s.getCachedBonusConfig()
	if bonusCfg == nil {
		return s.getNextBonus()
	}
	if bonusCfg.BonusNum > 0 {
		return bonusCfg.BonusNum
	}
	if len(bonusCfg.RandomNums) == 2 && bonusCfg.RandomNums[0] <= bonusCfg.RandomNums[1] {
		return bonusCfg.RandomNums[0] + rand.Int64N(bonusCfg.RandomNums[1]-bonusCfg.RandomNums[0]+1)
	}
	if len(bonusCfg.BonusSequence) > 0 {
		if len(s.BonusSequence) == 0 {
			s.BonusSequence = append([]int64{}, bonusCfg.BonusSequence...)
		}
		return s.getNextBonus()
	}
	return 1
}

func (s *Session) getNextBonus() int64 {
	if len(s.BonusSequence) == 0 {
		return 1
	}
	newIndex := atomic.AddInt64(&s.bonusIndex, 1)
	idx := int(newIndex) % len(s.BonusSequence)
	return s.BonusSequence[idx]
}

func (s *Session) getCachedBonusConfig() *v1.BetBonusConfig {
	if cached := s.bonusConfigCache.Load(); cached != nil {
		if config, ok := cached.(*v1.BetBonusConfig); ok {
			return config
		}
	}

	if s.Config == nil {
		return nil
	}

	for _, b := range s.Config.BetBonus {
		if b != nil && b.GameId == s.GameID {
			s.bonusConfigCache.Store(b)
			return b
		}
	}

	s.bonusConfigCache.Store((*v1.BetBonusConfig)(nil))
	return nil
}

func (s *Session) checkSpinOver(data map[string]any) bool {
	if s.Game != nil {
		return s.Game.IsSpinOver(data)
	}
	return isSpinOver(data)
}

func (s *Session) checkNeedBonus(data map[string]any) bool {
	if s.Game != nil && s.Game.NeedBetBonus(data) {
		return true
	}
	if s.getCachedBonusConfig() == nil {
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
