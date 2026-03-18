package g18925

import (
	"stress/internal/biz/game/base"
)

const ID = 18925
const Name = "牌九"

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (*Game) IsSpinOver(data map[string]any) bool {
	return true
}
