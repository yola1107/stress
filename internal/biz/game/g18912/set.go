package g18912

import (
	"fmt"
	"stress/internal/biz/game/base"
)

type Game struct {
	base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(18912, "金钱虎")}
}

func (*Game) IsSpinOver(data map[string]any) bool {
	return fmt.Sprintf("%v", data["free"]) == "0"
}
