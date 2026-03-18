package g18912

import (
	"fmt"
	"stress/internal/biz/game/base"
)

const ID = 18912
const Name = "金钱虎"

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
