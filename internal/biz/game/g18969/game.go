package g18969

import (
	"stress/internal/biz/game/base"
	"stress/internal/biz/game/g18969/pb"

	"google.golang.org/protobuf/proto"
)

const ID = 18969
const Name = "世界小姐"

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
		out := new(pb.Sjxj_BetOrderResponse)
		if err := proto.Unmarshal(bytes, out); err != nil {
			return nil, err
		}
		return map[string]any{"isSpinOver": *out.RemainingFreeTimes == 0}, nil
	}
}
