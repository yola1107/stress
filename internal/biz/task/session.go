package task

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	v1 "stress/api/stress/v1"
)

const (
	defaultRetryDelay    = 100 * time.Millisecond
	defaultMaxRetries    = 5
	defaultSleepOnCancel = 100 * time.Millisecond
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
	MemberName string

	State SessionState
	Token string
	mu    sync.RWMutex

	Process  int32
	TryTimes int32

	LastError string
}

func NewSession(memberName string) *Session {
	return &Session{
		MemberName: memberName,
		State:      SessionStateIdle,
	}
}

func (s *Session) getState() SessionState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.State
}

func (s *Session) setState(state SessionState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.State = state
}

func (s *Session) getToken() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Token
}

func (s *Session) setToken(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Token = token
}

func (s *Session) Execute(client *APIClient) error {
	env := client.Env()
	if env == nil {
		return fmt.Errorf("session env is nil")
	}

	maxRetries := defaultMaxRetries
	for {
		if state := s.getState(); state == SessionStateCompleted || state == SessionStateFailed {
			break
		}

		select {
		case <-env.ctx.Done():
			s.setState(SessionStateFailed)
			return env.ctx.Err()
		default:
		}

		if env.task.GetStatus() == v1.TaskStatus_TASK_CANCELLED {
			s.setState(SessionStateFailed)
			s.LastError = "task cancelled"
			return nil
		}

		if err := s.executeStep(env, client); err != nil {
			if !s.handleError(err, maxRetries, env) {
				return err
			}
		}
	}
	return nil
}

func (s *Session) executeStep(env *SessionEnv, client *APIClient) error {
	atomic.AddInt32(&s.TryTimes, 1)

	switch s.getState() {
	case SessionStateIdle, SessionStateLaunching:
		token, err := client.Launch(env.ctx, env.cfg, s.MemberName)
		if err == nil {
			s.setToken(token)
			s.setState(SessionStateLoggingIn)
			atomic.StoreInt32(&s.TryTimes, 0)
		} else {
			var apiErr *APIError
			if errors.As(err, &apiErr) && apiErr.Op == "launch" {
				s.setState(SessionStateFailed)
			}
		}
		return err

	case SessionStateLoggingIn:
		token, freeData, err := client.Login(env.ctx, env.cfg, s.getToken())
		if err == nil {
			s.setToken(token)
			s.setState(SessionStateBetting)
			if env.game.NeedBetBonus(freeData) {
				s.setState(SessionStateBonusSelect)
			}
			atomic.StoreInt32(&s.TryTimes, 0)
		}
		return err

	case SessionStateBetting:
		start := time.Now()
		data, err := client.BetOrder(env.ctx, env.cfg, s.getToken())
		if err == nil {
			duration := time.Since(start)
			spinOver := env.game.IsSpinOver(data)
			if spinOver && atomic.AddInt32(&s.Process, 1) >= env.cfg.TimesPerMember {
				s.setState(SessionStateCompleted)
			} else if env.game.NeedBetBonus(data) {
				s.setState(SessionStateBonusSelect)
			}
			env.task.AddBetOrder(duration, spinOver)
			atomic.StoreInt32(&s.TryTimes, 0)
		} else {
			err = s.handleBetOrderError(err, env)
		}
		return err

	case SessionStateBonusSelect:
		start := time.Now()
		res, err := client.BetBonus(env.ctx, env.cfg, s.getToken(), env.game.PickBonusNum())
		if err == nil {
			duration := time.Since(start)
			if !res.NeedContinue {
				s.setState(SessionStateBetting)
			}
			env.task.AddBetBonus(duration)
			atomic.StoreInt32(&s.TryTimes, 0)
		}
		return err

	default:
		return fmt.Errorf("unknown state: %v", s.getState())
	}
}

func (s *Session) handleBetOrderError(err error, env *SessionEnv) error {
	var betErr *BetOrderError
	if !errors.As(err, &betErr) {
		return err
	}
	if betErr.SleepDuration > 0 && !s.sleepOrCancel(betErr.SleepDuration, env) {
		return fmt.Errorf("bet order cancelled")
	}
	if betErr.NeedRelaunch {
		s.setState(SessionStateLaunching)
		s.setToken("")
	} else if betErr.NeedRelogin {
		s.setState(SessionStateLoggingIn)
	}
	return betErr
}

func (s *Session) handleError(err error, maxRetries int, env *SessionEnv) bool {
	if s.LastError != err.Error() {
		env.task.AddError()
	}
	s.LastError = err.Error()

	if atomic.LoadInt32(&s.TryTimes) > int32(maxRetries) {
		s.setState(SessionStateFailed)
		return false
	}

	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Op == "launch" {
		if !s.sleepOrCancel(defaultSleepOnCancel, env) {
			return false
		}
		s.setState(SessionStateFailed)
		return false
	}

	var betErr *BetOrderError
	if errors.As(err, &betErr) && !betErr.NeedRelaunch && !betErr.NeedRelogin {
		return s.sleepOrCancel(betErr.SleepDuration, env)
	}

	return s.sleepOrCancel(defaultRetryDelay, env)
}

func (s *Session) sleepOrCancel(duration time.Duration, env *SessionEnv) bool {
	timer := time.NewTimer(duration)
	defer func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}()
	select {
	case <-timer.C:
		return true
	case <-env.ctx.Done():
		s.setState(SessionStateFailed)
		return false
	}
}

func (s *Session) IsFailed() bool {
	return s.getState() == SessionStateFailed
}
