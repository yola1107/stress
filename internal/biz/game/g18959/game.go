package g18959

import (
	"google.golang.org/protobuf/proto"

	"stress/internal/biz/game/base"
	"stress/internal/biz/game/g18959/pb"
)

const ID = 18959
const Name = "樱花物语"

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
		out := new(pb.Yhwy_BetOrderResponse)
		if err := proto.Unmarshal(bytes, out); err != nil {
			return nil, err
		}
		return map[string]any{"isSpinOver": *out.IsRoundOver && *out.FreeNum == 0}, nil
	}
}
