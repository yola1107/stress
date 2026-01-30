package g18936

import (
	"fmt"
	"stress/internal/biz/game/base"
)

const ID int64 = 18936
const Name = "赏金大对决"

var Register = New()
var _ base.IGame = (*Game)(nil)

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (*Game) IsSpinOver(data map[string]any) bool {

	isOver, ok := data["isOver"]
	if !ok {
		return false
	}
	if fmt.Sprintf("%v", isOver) == "true" {
		return true
	}
	return false
}
