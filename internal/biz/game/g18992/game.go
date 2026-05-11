package g18992

import (
	"stress/internal/biz/game/base"
	"stress/internal/biz/game/g18992/pb"
)

const ID = 18992
const Name = "谍战风云"

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (*Game) IsSpinOver(data map[string]any) bool {
	return data["spinOver"].(bool)
}

func (*Game) GetProtobufConverter() base.ProtobufConverter {
	return base.ProtoToMapConverter(&pb.Dzfy_BetOrderResponse{})
}
