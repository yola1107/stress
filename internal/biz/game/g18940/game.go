package g18940

import (
	"fmt"
	"stress/internal/biz/game/base"
)

const ID int64 = 18940
const Name = "寻宝黄金城"

var Register = New()
var _ base.IGame = (*Game)(nil)

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (*Game) IsSpinOver(data map[string]any) bool {
	// 判断条件
	// 普通：win=0 （freeNum=0）
	// 购买：win=0且freeNum=0
	win := fmt.Sprintf("%v", data["win"])
	freeNum := fmt.Sprintf("%v", data["freeNum"])
	if win == "0" && freeNum == "0" {
		return true
	}

	return false
}
