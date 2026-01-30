package g18945

import (
	"fmt"
	"stress/internal/biz/game/base"
)

const ID int64 = 18945
const Name = "加拿大28"

var Register = New()
var _ base.IGame = (*Game)(nil)

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (*Game) IsSpinOver(data map[string]any) bool {

	win := fmt.Sprintf("%v", data["win"])
	freeNum := fmt.Sprintf("%v", data["free"])
	if win == "0" && freeNum == "0" {
		return true
	}

	return false

}
