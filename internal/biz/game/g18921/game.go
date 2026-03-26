package g18921

import (
	"stress/api/common/pb"
	"stress/internal/biz/game/base"
)

const ID = 18921
const Name = "三国志"

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (*Game) IsSpinOver(data map[string]any) bool {
	v, ok := data["isRoundOver"]
	if !ok {
		return false
	}
	over, ok := v.(bool)
	return ok && over
}

func (g *Game) GetProtobufConverter() base.ProtobufConverter {
	return base.ProtoToMapConverter(&pb.Sgz_BetOrderResponse{})
}
