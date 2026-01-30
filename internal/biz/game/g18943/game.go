package g18943

import (
	"fmt"
	"stress/internal/biz/game/base"
)

const ID int64 = 18943
const Name = "麻将胡了"

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
	freeNum := fmt.Sprintf("%v", data["freeNum"])
	if win == "0" && freeNum == "0" {
		return true
	}

	return false

}
