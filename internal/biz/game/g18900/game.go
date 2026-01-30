package g18900

import (
	"stress/internal/biz/game/base"
)

const ID int64 = 18900
const Name = "炸弹甜妞"

var Register = New()
var _ base.IGame = (*Game)(nil)

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (*Game) IsSpinOver(data map[string]any) bool {
	isRoundOver, ok := data["isRoundOver"]
	if !ok {
		return false
	}
	isSpinOver, ok := data["isSpinOver"]
	if !ok {
		return false
	}
	if isRoundOver.(bool) && isSpinOver.(bool) {
		return true
	}
	return false
}
