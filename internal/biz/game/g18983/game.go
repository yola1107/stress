package g18983

import (
	"stress/internal/biz/game/base"
	"stress/internal/biz/game/g18983/pb"
)

const ID = 18983
const Name = "财神降福"

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (*Game) IsSpinOver(data map[string]any) bool {
	next, exists := data["isSpinOver"]
	if !exists {
		return true
	}

	if over, ok := next.(bool); ok {
		return over
	}
	return true
}

// GetProtobufConverter 实现protobuf转换器
func (g *Game) GetProtobufConverter() base.ProtobufConverter {
	return base.ProtoToMapConverter(&pb.Csjf_BetOrderResponse{})
}
