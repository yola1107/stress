package g18898

import (
	"fmt"
	"stress/internal/biz/game/base"
)

const ID int64 = 18898
const Name = "埃及女王"

var Register = New()
var _ base.IGame = (*Game)(nil)

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
