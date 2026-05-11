package g18987

import (
	"google.golang.org/protobuf/proto"

	"stress/internal/biz/game/base"
	"stress/internal/biz/game/g18987/pb"
)

const ID = 18987
const Name = "聚宝盆"

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
	return func(bytes []byte) (map[string]any, error) {
		out := new(pb.Jbp_BetOrderResponse)
		if err := proto.Unmarshal(bytes, out); err != nil {
			return nil, err
		}
		return map[string]any{"isSpinOver": *out.IsSpinOver}, nil
	}
}
