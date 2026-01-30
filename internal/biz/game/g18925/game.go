package g18925

import (
	"stress/internal/biz/game/base"
)

const ID int64 = 18925
const Name = "牌九"

var Register = New()
var _ base.IGame = (*Game)(nil)

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (*Game) IsSpinOver(data map[string]any) bool {
	return true
}
