package g19004

import (
	"google.golang.org/protobuf/proto"

	"stress/internal/biz/game/base"
	"stress/internal/biz/game/g19004/pb"
)

const ID = 19004
const Name = "埃及探秘"

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
		out := new(pb.Ajtm_BetOrderResponse)
		if err := proto.Unmarshal(bytes, out); err != nil {
			return nil, err
		}
		return map[string]any{"isSpinOver": *out.IsRoundOver && *out.FreeNum == 0}, nil
	}
}
