package g18912

import (
	"fmt"
	"stress/internal/biz/game/base"
)

const ID int64 = 18912
const Name = "金钱虎"

var Register = New()
var _ base.IGame = (*Game)(nil)

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (*Game) IsSpinOver(data map[string]any) bool {

	freeNum := fmt.Sprintf("%v", data["free"])
	if freeNum == "0" {
		return true
	}

	return false

}
