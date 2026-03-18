package g18921

import (
	"fmt"

	"stress/api/common/pb"
	"stress/internal/biz/game/base"

	"github.com/go-kratos/kratos/v2/log"
	jsoniter "github.com/json-iterator/go"
	"google.golang.org/protobuf/proto"
)

const ID = 18921
const Name = "三国志"

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (*Game) IsSpinOver(data map[string]any) bool {
	next, exists := data["isRoundOver"]
	if !exists {
		log.Error("isRoundOver field not found in response data")
		return false
	}

	if over, ok := next.(bool); ok {
		return over
	}

	log.Error("isRoundOver field is not a boolean", "value", next)
	return false
}

// GetProtobufConverter 实现protobuf转换器
func (g *Game) GetProtobufConverter() base.ProtobufConverter {
	return func(bytes []byte) (map[string]any, error) {
		out := new(pb.Sgz_BetOrderResponse)
		if err := proto.Unmarshal(bytes, out); err != nil {
			return nil, fmt.Errorf("failed to unmarshal protobuf: %v", err)
		}

		//return map[string]any{
		//	"isRoundOver": out.GetIsGameOver(),
		//}, nil

		if err := proto.Unmarshal(bytes, out); err != nil {
			return nil, fmt.Errorf("failed to unmarshal protobuf: %v", err)
		}
		b, _ := jsoniter.Marshal(out)
		mp := make(map[string]any)
		_ = jsoniter.Unmarshal(b, &mp)
		return mp, nil
	}
}
