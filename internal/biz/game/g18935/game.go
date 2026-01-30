package g18935

import (
	"stress/internal/biz/game/base"
)

const ID int64 = 18935
const Name = "赏金船长"

var Register = New()
var _ base.IGame = (*Game)(nil)

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (*Game) IsSpinOver(data map[string]any) bool {

	spinOver, exists := data["SpinOver"]
	if !exists {
		return true
	}

	if over, ok := spinOver.(bool); ok {
		return over
	}
	return true
}
