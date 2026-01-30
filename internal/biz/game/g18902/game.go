package g18902

import (
	"fmt"
	"stress/internal/biz/game/base"
)

const ID int64 = 18902
const Name = "波塞冬之力"

var Register = New()
var _ base.IGame = (*Game)(nil)

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (*Game) NeedBetBonus(freeData map[string]any) bool {
	nextState, ok := freeData["nextState"]
	if ok {
		if fmt.Sprintf("%v", nextState) == "11" {
			return true
		}
	}

	if state, ok := freeData["state"]; ok {
		if fmt.Sprintf("%v", state) == "11" && fmt.Sprintf("%v", nextState) == "0" {
			return true
		}
	}

	return false
}

func (*Game) IsSpinOver(data map[string]any) bool {
	isSpinOver := fmt.Sprintf("%v", data["isSpinOver"])
	if isSpinOver == "true" {
		return true
	}

	return false
}

func (g *Game) BonusNextState(data map[string]any) bool {
	st, ok := data["state"]
	if !ok {
		return false
	}
	state := fmt.Sprintf("%v", st)
	if state != "11" {
		return false
	}
	nst, ok1 := data["nextState"]
	if ok1 {
		if fmt.Sprintf("%v", nst) == "0" {
			return true
		}
	}
	return false
}

// AsBonusInterface 实现 GameBonusInterface 接口
func (g *Game) AsBonusInterface() base.GameBonusInterface {
	return g
}
