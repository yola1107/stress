package g8917

import (
	"google.golang.org/protobuf/proto"

	"stress/internal/biz/game/base"
	"stress/internal/biz/game/g8917/pb"
)

const ID = 8917
const Name = "桑巴与亚马逊"

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
		out := new(pb.Sbyymx_BetOrderResponse)
		if err := proto.Unmarshal(bytes, out); err != nil {
			return nil, err
		}
		return map[string]any{"isSpinOver": *out.IsRoundOver}, nil
	}
}
