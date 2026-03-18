package g18922

import (
	"fmt"
	"stress/api/common/pb"
	"stress/internal/biz/game/base"

	"google.golang.org/protobuf/proto"
)

const ID = 18922
const Name = "金钱兔"

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
	return func(protoBytes []byte) (map[string]any, error) {
		jqtResponse := new(pb.Jqt_BetOrderResponse)
		if err := proto.Unmarshal(protoBytes, jqtResponse); err != nil {
			return nil, fmt.Errorf("failed to unmarshal protobuf: %v", err)
		}
		return map[string]any{"next": jqtResponse.WinInfo.Next}, nil
	}
}
