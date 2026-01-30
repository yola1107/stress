package g18949

import (
	"fmt"
	"stress/internal/biz/game/base"
)

const ID int64 = 18949
const Name = "霍比特人"

var Register = New()
var _ base.IGame = (*Game)(nil)

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (g *Game) IsSpinOver(data map[string]any) bool {
	return fmt.Sprintf("%v", data["free"]) == "0"
}
