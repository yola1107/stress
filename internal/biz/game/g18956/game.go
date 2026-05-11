package g18956

import (
	"stress/internal/biz/game/base"
	"stress/internal/biz/game/g18956/pb"

	"google.golang.org/protobuf/proto"
)

const ID = 18956
const Name = "横财三千亿"

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
		out := new(pb.Hcsqy_BetOrderResponse)
		if err := proto.Unmarshal(bytes, out); err != nil {
			return nil, err
		}
		// 含购买/免费/重转）：
		// 1) 基础直出结束：isRoundOver=true && isFree=false
		// 2) 基础或购买触发免费后，免费链路最终结束：isRoundOver=true && isFree=true && freeNum=0
		// 说明：重转链路中的中间步 isRoundOver=false，不会被计为 spin over。
		isSpinOver := out.GetIsRoundOver() && (!out.GetIsFree() || out.GetFreeNum() == 0)
		return map[string]any{"isSpinOver": isSpinOver}, nil
	}
}
