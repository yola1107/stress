package task

import (
	"fmt"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"
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
	MemberName string

	State SessionState
	Token string
	mu    sync.RWMutex

	Process    int32
	TryTimes   int32
	bonusIndex int64

	LastError string
}

func NewSession(memberName string) *Session {
	return &Session{
		MemberName: memberName,
		State:      SessionStateIdle,
		Token:      "",
		Process:    0,
		TryTimes:   0,
		LastError:  "",
		bonusIndex: 0,
	}
}

// State 操作方法（线程安全）
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

// Token 操作方法（线程安全）
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

		if err := s.executeStep(env, client); err != nil {
			if !s.handleError(err, maxRetries, env) {
				return err
			}
		}

		// 检查任务是否被取消
		if env.isTaskCancelled != nil && env.isTaskCancelled() {
			s.setState(SessionStateFailed)
			s.LastError = "task cancelled"
			return nil
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
		} else if apiErr, ok := err.(*APIError); ok && apiErr.Op == "launch" {
			s.setState(SessionStateFailed)
		}
		return err
	case SessionStateLoggingIn:
		token, freeData, err := client.Login(env.ctx, env.cfg, s.getToken())
		if err == nil {
			s.setToken(token)
			s.setState(SessionStateBetting)
			if s.checkNeedBonus(env, freeData) {
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
			spinOver := s.checkSpinOver(env, data)
			if spinOver && atomic.AddInt32(&s.Process, 1) >= env.target {
				s.setState(SessionStateCompleted)
			}
			if s.checkNeedBonus(env, data) {
				s.setState(SessionStateBonusSelect)
			}
			if env.addBetOrder != nil {
				env.addBetOrder(duration, spinOver)
			}
			atomic.StoreInt32(&s.TryTimes, 0)
		} else {
			err = s.handleBetOrderError(err, env)
		}
		return err
	case SessionStateBonusSelect:
		start := time.Now()
		res, err := client.BetBonus(env.ctx, env.cfg, s.getToken(), s.pickBonusNum(env))
		if err == nil {
			duration := time.Since(start)
			if !res.NeedContinue {
				s.setState(SessionStateBetting)
			}
			if env.addBetBonus != nil {
				env.addBetBonus(duration)
			}
			atomic.StoreInt32(&s.TryTimes, 0)
		}
		return err
	default:
		return fmt.Errorf("unknown state: %v", s.getState())
	}
}

func (s *Session) handleBetOrderError(err error, env *SessionEnv) error {
	betErr, ok := err.(*BetOrderError)
	if !ok {
		return err
	}
	if betErr.SleepDuration > 0 {
		if !s.sleepOrCancel(betErr.SleepDuration, env) {
			return fmt.Errorf("bet order cancelled")
		}
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
	errMsg := err.Error()

	if env.addError != nil && s.LastError != errMsg {
		env.addError()
	}
	s.LastError = errMsg

	tryTimes := atomic.LoadInt32(&s.TryTimes)

	if tryTimes > int32(maxRetries) {
		s.setState(SessionStateFailed)
		return false
	}

	if apiErr, ok := err.(*APIError); ok && apiErr.Op == "launch" {
		if !s.sleepOrCancel(delayMills, env) {
			return false
		}
		s.setState(SessionStateFailed)
		return false
	}

	if betErr, ok := err.(*BetOrderError); ok {
		if !s.sleepOrCancel(betErr.SleepDuration, env) {
			return false
		}
		return true
	}

	if !s.sleepOrCancel(delayMills, env) {
		return false
	}

	return true
}

func (s *Session) sleepOrCancel(duration time.Duration, env *SessionEnv) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-env.ctx.Done():
		s.setState(SessionStateFailed)
		return false
	}
}

func (s *Session) pickBonusNum(env *SessionEnv) int64 {
	if env == nil {
		return s.getNextBonusFrom(nil)
	}
	if env.bonusNum > 0 {
		return env.bonusNum
	}
	if len(env.randomNums) == 2 && env.randomNums[0] <= env.randomNums[1] {
		return env.randomNums[0] + rand.Int64N(env.randomNums[1]-env.randomNums[0]+1)
	}
	if len(env.bonusSeq) > 0 {
		return s.getNextBonusFrom(env)
	}
	return 1
}

func (s *Session) getNextBonusFrom(env *SessionEnv) int64 {
	if env == nil || len(env.bonusSeq) == 0 {
		return 1
	}
	seq := env.bonusSeq
	newIndex := atomic.AddInt64(&s.bonusIndex, 1)
	idx := int(newIndex) % len(seq)
	return seq[idx]
}

func (s *Session) checkSpinOver(env *SessionEnv, data map[string]any) bool {
	if env != nil && env.isSpinOver != nil {
		return env.isSpinOver(data)
	}
	return isSpinOver(data)
}

func (s *Session) checkNeedBonus(env *SessionEnv, data map[string]any) bool {
	if env != nil && env.needBonus != nil && env.needBonus(data) {
		return true
	}
	if env == nil {
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
	return s.getState() == SessionStateFailed
}
