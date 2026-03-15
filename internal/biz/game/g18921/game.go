package g18921

import (
	"github.com/go-kratos/kratos/v2/log"
	sgz "stress/api/game/18921"
	"stress/internal/biz/game/base"
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
	return sgz.ConvertProtobufToMap
}
