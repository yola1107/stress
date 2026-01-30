package g18965

import (
	"fmt"
	"stress/internal/biz/game/base"
)

const ID int64 = 18965
const Name = "巴西狂欢"

var Register = New()
var _ base.IGame = (*Game)(nil)

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (*Game) IsSpinOver(data map[string]any) bool {

	// 1. 获取 winInfo 字段
	winInfoRaw, ok := data["winInfo"]
	if !ok {
		// winInfo 字段不存在
		return false
	}
	// 2. 将 winInfo 断言为 map[string]any
	winInfoMap, ok := winInfoRaw.(map[string]any)
	if !ok {
		// winInfo 不是预期的类型
		return false
	}
	// 3. 获取 FreeNum 字段
	freeNumRaw, ok := winInfoMap["freeNum"]
	if !ok {
		// FreeNum 字段不存在
		return false
	}
	freeNum := freeNumRaw.(float64)
	win := fmt.Sprintf("%v", data["currentWin"])
	if win == "0" && freeNum == 0 {
		return true
	}

	return false

}
