package g18949

import (
	"fmt"
	"stress/internal/biz/game/base"
)

const ID = 18949
const Name = "霍比特人"

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (g *Game) IsSpinOver(data map[string]any) bool {
	return fmt.Sprintf("%v", data["free"]) == "0"
}
