package g18894

import (
	"fmt"
	"stress/internal/biz/game/base"
)

const ID int64 = 18894
const Name = "水果盛宴"

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
