package g18947

import (
	"fmt"
	"stress/internal/biz/game/base"
)

const ID = 18947
const Name = "五福临门"

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
