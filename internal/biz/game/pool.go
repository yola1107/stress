package game

import (
	"context"
	"sort"
	"sync"

	"stress/internal/biz/game/base"
)

type Pool struct {
	mu       sync.RWMutex
	list     []base.IGame
	registry map[int64]base.IGame
}

type BetSizeFunc func(ctx context.Context, gameIDs []int64) (map[int64][]float64, error)

func NewPool(fn BetSizeFunc) *Pool {
	p := &Pool{
		registry: make(map[int64]base.IGame),
		list:     make([]base.IGame, 0, len(registry)),
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

func (p *Pool) List() []base.IGame {
	p.mu.RLock()
	defer p.mu.RUnlock()
	cpy := append([]base.IGame{}, p.list...)
	return cpy
}
