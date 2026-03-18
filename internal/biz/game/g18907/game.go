package g18907

import (
	"stress/internal/biz/game/base"
)

const ID = 18907
const Name = "英雄联盟"

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (*Game) IsSpinOver(data map[string]any) bool {
	if winInfo, ok := data["spinInfo"].(map[string]interface{}); ok {
		if next, ok := winInfo["next"].(bool); ok && next {
			return false
		}
	}
	return true
}
