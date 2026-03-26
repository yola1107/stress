package game

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"stress/internal/biz/game/base"
)

type Pool struct {
	mu          sync.RWMutex
	list        []base.IGame
	registry    map[int64]base.IGame
	betSizeFunc BetSizeFunc // 保存获取 betsize 的函数，用于动态获取
}

type BetSizeFunc func(ctx context.Context, gameIDs []int64) (map[int64][]float64, error)

func NewPool(fn BetSizeFunc) *Pool {
	p := &Pool{
		registry:    make(map[int64]base.IGame),
		list:        make([]base.IGame, 0, len(registry)),
		betSizeFunc: fn,
	}
	ids := make([]int64, 0, len(registry))
	for id, g := range registry {
		ids = append(ids, id)
		p.registry[id] = g
		p.list = append(p.list, g)
	}

	// 获取游戏的有效下注金额
	m, err := fn(context.Background(), ids)
	if err != nil {
		panic(err)
	}

	for id, g := range p.registry {
		g.SetBetSize(m[id])
	}

	sort.Slice(p.list, func(i, j int) bool {
		return p.list[i].GameID() < p.list[j].GameID()
	})
	return p
}

func (p *Pool) Get(gameID int64) (base.IGame, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	g, ok := p.registry[gameID]
	return g, ok
}

// EnsureGameBetSize 确保游戏有 betsize，如果没有则从数据库动态获取
func (p *Pool) EnsureGameBetSize(ctx context.Context, gameID int64) error {
	p.mu.RLock()
	g, ok := p.registry[gameID]
	hasBetSize := ok && len(g.BetSize()) > 0
	p.mu.RUnlock()

	if !ok {
		return fmt.Errorf("game %d not found in registry", gameID)
	}
	if hasBetSize {
		return nil
	}

	m, err := p.betSizeFunc(ctx, []int64{gameID})
	if err != nil {
		return fmt.Errorf("empty mysql betsize for game %d: %w", gameID, err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if len(g.BetSize()) == 0 {
		if betSize, exists := m[gameID]; exists && len(betSize) > 0 {
			g.SetBetSize(betSize)
		}
	}
	return nil
}

func (p *Pool) List() []base.IGame {
	p.mu.RLock()
	defer p.mu.RUnlock()
	cpy := append([]base.IGame{}, p.list...)
	return cpy
}
