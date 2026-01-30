package g18896

import (
	"fmt"
	"stress/internal/biz/game/base"
)

const ID int64 = 18896
const Name = "哪吒无极限"

var Register = New()
var _ base.IGame = (*Game)(nil)

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (*Game) IsSpinOver(data map[string]any) bool {

	win := fmt.Sprintf("%v", data["bonusAmount"])
	freeNum := fmt.Sprintf("%v", data["isFree"])
	if win == "0" && freeNum == "false" {
		return true
	}

	return false

}
