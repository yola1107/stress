package g18901

import (
	"fmt"
	"stress/internal/biz/game/base"
)

const ID int64 = 18901
const Name = "清爽夏日"

var Register = New()
var _ base.IGame = (*Game)(nil)

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (*Game) IsSpinOver(data map[string]any) bool {

	isSpinOver := fmt.Sprintf("%v", data["isSpinOver"])
	if isSpinOver == "true" {
		return true
	}

	return false
}
