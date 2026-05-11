package g18989

import (
	"fmt"
	"stress/internal/biz/game/base"
	"stress/internal/biz/game/g18989/pb"
)

const ID = 18989
const Name = "鬼吹灯"

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (*Game) IsSpinOver(data map[string]any) bool {
	return data["isSpinOver"].(bool)
}

func (g *Game) GetProtobufConverter() base.ProtobufConverter {
	return base.ProtoToMapConverter(&pb.Gcd_BetOrderResponse{})
}

func (*Game) NeedBetBonus(data map[string]any) bool {
	bonusState, ok := data["bonusState"]
	return ok && fmt.Sprintf("%v", bonusState) == "1"
}

func (*Game) PickBonusNum(bonusNum []int64) int64 {
	return base.CalcBonusNum(bonusNum)
}
