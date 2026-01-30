package g18891

import (
	"fmt"
	"stress/internal/biz/game/base"
)

const ID int64 = 18891
const Name = "吸血鬼"

var Register = New()
var _ base.IGame = (*Game)(nil)

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (*Game) IsSpinOver(data map[string]any) bool {

	// 判断条件 没有免费次数 == 一局完成
	freeNum := fmt.Sprintf("%v", data["remainingFreeCount"])
	return freeNum == "0"
}
