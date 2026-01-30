package g18923

import (
	"fmt"
	"stress/internal/biz/game/base"
)

const ID int64 = 18923
const Name = "巨龙传说"

var Register = New()
var _ base.IGame = (*Game)(nil)

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (*Game) IsSpinOver(data map[string]any) bool {
	if fmt.Sprintf("%v", data["freeNum"]) == "0" && fmt.Sprintf("%v", data["win"]) == "0" {
		return true
	}
	return false
}
