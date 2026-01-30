package g18905

import (
	"stress/internal/biz/game/base"
)

const ID int64 = 18905
const Name = "加勒比海盗"

var Register = New()
var _ base.IGame = (*Game)(nil)

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (*Game) IsSpinOver(data map[string]any) bool {

	spinOver, exists := data["isSpinOver"]
	if !exists {
		return true
	}

	if over, ok := spinOver.(bool); ok {
		return over
	}
	return true
}
