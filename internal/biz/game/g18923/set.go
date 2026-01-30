package g18923

import (
	"fmt"
	"stress/internal/biz/game/base"
)

type Game struct {
	base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(18923, "巨龙传说")}
}

func (*Game) IsSpinOver(data map[string]any) bool {
	if fmt.Sprintf("%v", data["freeNum"]) == "0" && fmt.Sprintf("%v", data["win"]) == "0" {
		return true
	}
	return false
}
