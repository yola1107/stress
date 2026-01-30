package g18897

import (
	"fmt"
	"stress/internal/biz/game/base"
)

const ID int64 = 18897
const Name = "僵尸冲冲冲"

var Register = New()
var _ base.IGame = (*Game)(nil)

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (*Game) IsSpinOver(data map[string]any) bool {

	respinFree := fmt.Sprintf("%v", data["respinFree"])
	freeNum := fmt.Sprintf("%v", data["remFCot"])
	if respinFree == "0" && freeNum == "0" {
		return true
	}

	return false

}
