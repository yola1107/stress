package game

import (
	"stress/internal/biz/game/base"
	"stress/internal/biz/game/g18890"
	"stress/internal/biz/game/g18912"
	"stress/internal/biz/game/g18923"
)

var gameInstances = []base.IGame{
	g18890.New(),
	g18923.New(),
	g18912.New(),
}
