package g18971

import (
	"stress/internal/biz/game/base"
)

const ID int64 = 18971
const Name = "哪吒之魔童闹海"

var Register = New()
var _ base.IGame = (*Game)(nil)

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (*Game) IsSpinOver(data map[string]any) bool {
	winInfo, ok := data["winInfo"]
	if !ok {
		return false
	}
	info, ok := winInfo.(map[string]any)
	if !ok {
		return false
	}

	if _, ok = info["over"]; ok {
		if over, ok := info["over"].(bool); ok {
			return over
		}
	}

	return false

}
