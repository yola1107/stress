package g18926

import (
	"google.golang.org/protobuf/proto"

	"stress/internal/biz/game/base"
	"stress/internal/biz/game/g18926/pb"
)

const ID = 18926
const Name = "印钞机"

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
		out := new(pb.Ycj_BetOrderResponse)
		if err := proto.Unmarshal(bytes, out); err != nil {
			return nil, err
		}
		return map[string]any{"isSpinOver": !*out.IsFree && *out.Pend == 0}, nil
	}
}
