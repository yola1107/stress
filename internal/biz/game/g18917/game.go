package g18917

import (
	"stress/internal/biz/game/base"
	"stress/internal/biz/game/g18917/pb"
)

const ID = 18917
const Name = "齐鲁佳肴"

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
	return base.ProtoToMapConverter(&pb.Qljy_BetOrderResponse{})
}
