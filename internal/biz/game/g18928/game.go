package g18928

import (
	"fmt"
	"stress/internal/biz/game/base"
	"stress/internal/biz/game/g18928/pb"

	"google.golang.org/protobuf/proto"
)

const ID = 18928
const Name = "马里奥世界"

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (*Game) IsSpinOver(data map[string]any) bool {
	next, exists := data["next"]
	if !exists {
		return false
	}

	if over, ok := next.(bool); ok {
		return !over
	}
	return true
}

// GetProtobufConverter 实现protobuf转换器
func (g *Game) GetProtobufConverter() base.ProtobufConverter {
	return func(bytes []byte) (map[string]any, error) {
		out := new(pb.Mlasj_BetOrderResponse)
		if err := proto.Unmarshal(bytes, out); err != nil {
			return nil, fmt.Errorf("failed to unmarshal protobuf: %v", err)
		}
		return map[string]any{"next": *out.WinInfo.Next}, nil
	}
}
