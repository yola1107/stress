package game

import (
	"sort"
	"sync"

	"stress/internal/biz/game/base"
)

type Pool struct {
	mu   sync.RWMutex
	byID map[int64]base.IGame
	list []base.IGame
}

func NewPool() *Pool {
	p := &Pool{
		byID: make(map[int64]base.IGame),
		list: make([]base.IGame, 0, len(gameInstances)),
	}
	for _, g := range gameInstances {
		p.byID[g.GameID()] = g
		p.list = append(p.list, g)
	}
	sort.Slice(p.list, func(i, j int) bool {
		return p.list[i].GameID() < p.list[j].GameID()
	})
	return p
}

func (p *Pool) Get(gameID int64) (base.IGame, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	g, ok := p.byID[gameID]
	return g, ok
}

func (p *Pool) List() []base.IGame {
	p.mu.RLock()
	defer p.mu.RUnlock()
	cpy := append([]base.IGame{}, p.list...)
	return cpy
}

// RequireProtobuf 是否该游戏需要 protobuf 解析（由 IGame.GetProtobufConverter 决定）
func (p *Pool) RequireProtobuf(gameID int64) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	g, ok := p.byID[gameID]
	return ok && g != nil && g.GetProtobufConverter() != nil
}
