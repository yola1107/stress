package g18922

import (
	jqt "stress/api/game/18922"
	"stress/internal/biz/game/base"
)

const ID int64 = 18922
const Name = "金钱兔"

var Register = New()
var _ base.IGame = (*Game)(nil)

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
	return jqt.ConvertProtobufToMap
}
