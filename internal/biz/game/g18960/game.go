package g18960

import (
	"fmt"
	"strconv"
	"stress/pkg/xgo"

	"stress/internal/biz/game/base"
	"stress/internal/biz/game/g18960/pb"

	jsoniter "github.com/json-iterator/go"
	"google.golang.org/protobuf/proto"
)

const ID = 18960
const Name = "大航海家"

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (*Game) IsSpinOver(data map[string]any) bool {
	isSpinOver := fmt.Sprintf("%v", data["isSpinOver"])
	if isSpinOver == "true" {
		return true
	}
	return false
}

// GetProtobufConverter 实现protobuf转换器
func (g *Game) GetProtobufConverter() base.ProtobufConverter {
	return func(bytes []byte) (map[string]any, error) {
		out := new(pb.Dhhj_BetOrderResponse)
		if err := proto.Unmarshal(bytes, out); err != nil {
			return nil, fmt.Errorf("failed to unmarshal protobuf: %v", err)
		}
		b, _ := jsoniter.Marshal(out)
		mp := make(map[string]any)
		_ = jsoniter.Unmarshal(b, &mp)
		return mp, nil
	}
}

func (*Game) NeedBetBonus(data map[string]any) bool {
	treasureNum, ok := data["scatterCount"]
	if !ok {
		return false
	}
	bonusState, ok := data["bonusState"]
	if !ok {
		return false
	}

	treasureNumInt, err := strconv.ParseInt(fmt.Sprintf("%v", treasureNum), 10, 64)
	if err != nil {
		return false
	}

	if treasureNumInt >= 3 && fmt.Sprintf("%v", bonusState) == "1" {
		return true
	}
	return false
}

func (*Game) PickBonusNum(bonusNum []int64) int64 {
	cnt := len(bonusNum)
	if cnt == 0 {
		return -1
	}
	if cnt == 1 {
		return bonusNum[0]
	}
	return bonusNum[xgo.RandInt(0, cnt)]
}
