package g18892

import (
	"fmt"
	"stress/internal/biz/game/base"
)

const ID int64 = 18892
const Name = "血色浪漫"

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
	freeNum := fmt.Sprintf("%v", data["remainingFreeRoundCount"])
	return freeNum == "0"
}
