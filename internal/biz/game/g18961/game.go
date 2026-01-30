package g18961

import (
	"fmt"
	"stress/internal/biz/game/base"
)

const ID int64 = 18961
const Name = "幸运熊猫"

var Register = New()
var _ base.IGame = (*Game)(nil)

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (*Game) IsSpinOver(data map[string]any) bool {

	freeNum := fmt.Sprintf("%v", data["remainNum"])
	if freeNum == "0" {
		return true
	}

	return false

}
