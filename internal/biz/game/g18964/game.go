package g18964

import (
	"fmt"

	"stress/api/common/pb"
	"stress/internal/biz/game/base"

	jsoniter "github.com/json-iterator/go"
	"google.golang.org/protobuf/proto"
)

const ID = 18964
const Name = "马行大运"

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (*Game) NeedBetBonus(freeData map[string]any) bool {
	return false
}

func (*Game) IsSpinOver(data map[string]any) bool {
	isSpinOver := fmt.Sprintf("%v", data["spinOver"])
	if isSpinOver == "true" {
		return true
	}
	return false
}

func (g *Game) BonusNextState(data map[string]any) bool {
	return false
}

// GetProtobufConverter 实现protobuf转换器
func (g *Game) GetProtobufConverter() base.ProtobufConverter {
	return func(bytes []byte) (map[string]any, error) {
		out := new(pb.Mxdy_SpinResponse)
		if err := proto.Unmarshal(bytes, out); err != nil {
			return nil, fmt.Errorf("failed to unmarshal protobuf: %v", err)
		}
		b, _ := jsoniter.Marshal(out)
		mp := make(map[string]any)
		_ = jsoniter.Unmarshal(b, &mp)
		return mp, nil
	}
}
