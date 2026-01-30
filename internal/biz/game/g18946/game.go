package g18946

import (
	"fmt"
	"strconv"
	"stress/internal/biz/game/base"
)

const ID int64 = 18946
const Name = "庆余年"

var Register = New()
var _ base.IGame = (*Game)(nil)

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (*Game) NeedBetBonus(data map[string]any) bool {
	treasureNum, ok := data["treasureNum"]
	if !ok {
		return false
	}
	bonusState, ok := data["bonusState"]
	if !ok {
		return false
	}

	treasureNumInt, err := strconv.ParseInt(fmt.Sprintf("%v", treasureNum), 10, 64)
	if err != nil {
		return false
	}

	if treasureNumInt >= 3 && fmt.Sprintf("%v", bonusState) == "1" {
		return true
	}
	return false
}

func (*Game) IsSpinOver(data map[string]any) bool {
	bonusState, ok := data["bonusState"]
	if !ok {
		return false
	}
	freeNum, ok := data["freeNum"]
	if !ok {
		return false
	}
	win, ok := data["win"]
	if !ok {
		return false
	}

	if fmt.Sprintf("%v", bonusState) != "1" &&
		fmt.Sprintf("%v", freeNum) == "0" &&
		fmt.Sprintf("%v", win) == "0" {
		return true
	}

	return false
}
