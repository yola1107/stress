package g18904

import (
	flgl "stress/api/game/18904"
	"stress/internal/biz/game/base"
)

const ID int64 = 18904
const Name = "法老归来"

var Register = New()
var _ base.IGame = (*Game)(nil)

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (*Game) IsSpinOver(data map[string]any) bool {

	spinOver, exists := data["isSpinOver"]
	if !exists {
		return false
	}

	if over, ok := spinOver.(bool); ok {
		return over
	}
	return true
}

// GetProtobufConverter 实现protobuf转换器
func (g *Game) GetProtobufConverter() base.ProtobufConverter {
	return flgl.ConvertProtobufToMap
}
