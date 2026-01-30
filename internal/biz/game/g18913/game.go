package g18913

import (
	"fmt"
	"stress/internal/biz/game/base"
)

const ID int64 = 18913
const Name = "vip欲望派对"

var Register = New()
var _ base.IGame = (*Game)(nil)

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (*Game) IsSpinOver(data map[string]any) bool {

	freeNum := fmt.Sprintf("%v", data["isFree"])
	if freeNum == "false" {
		return true
	}

	return false

}
