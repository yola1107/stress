package g18933

import (
	"fmt"
	"stress/internal/biz/game/base"
)

const ID = 18933
const Name = "金龙送宝2"

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (*Game) IsSpinOver(data map[string]any) bool {
	over, ok := data["over"]
	if !ok {
		return false
	}
	return fmt.Sprintf("%v", over) == "1"
}
