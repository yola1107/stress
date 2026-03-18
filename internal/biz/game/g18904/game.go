package g18904

import (
	"stress/internal/biz/game/base"
)

const ID = 18904
const Name = "法老归来"

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
	return ConvertProtobufToMap
}
