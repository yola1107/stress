package g18913

import (
	"fmt"
	"stress/internal/biz/game/base"
)

const ID = 18913
const Name = "vip欲望派对"

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
