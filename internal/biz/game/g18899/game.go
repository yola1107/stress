package g18899

import (
	"fmt"
	"stress/internal/biz/game/base"
)

const ID = 18899
const Name = "功夫"

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (*Game) IsSpinOver(data map[string]any) bool {

	win := fmt.Sprintf("%v", data["currentWin"])
	freeNum := fmt.Sprintf("%v", data["free"])
	if win == "0" && freeNum == "0" {
		return true
	}

	return false
}
