package g18890

import (
	"stress/internal/biz/game/base"
)

type Game struct {
	base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(18890, "战火西岐")}
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
	if _, ok := info["over"]; ok {
		if over, ok := info["over"].(bool); ok {
			return over
		}
	}
	return false
}
