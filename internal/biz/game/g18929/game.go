package g18929

import (
	"fmt"
	"stress/internal/biz/game/base"
	"stress/internal/biz/game/g18929/pb"

	"google.golang.org/protobuf/proto"
)

const ID = 18929
const Name = "生化危机"

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
	return func(bytes []byte) (map[string]any, error) {
		out := new(pb.Shwj_BetOrderResponse)
		if err := proto.Unmarshal(bytes, out); err != nil {
			return nil, fmt.Errorf("failed to unmarshal protobuf: %v", err)
		}
		return map[string]any{"isSpinOver": *out.IsSpinOver}, nil
	}
}
