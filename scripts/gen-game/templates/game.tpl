package {{ .Pkg }}

import (
	"stress/internal/biz/game/base"
	"stress/internal/biz/game/{{ .Pkg }}/pb"
)

const ID = {{ .ID }}
const Name = {{ .NameGoQuoted }}

type Game struct {
	*base.Default
}

func New() base.IGame {
	return &Game{Default: base.NewBaseGame(ID, Name)}
}

func (*Game) IsSpinOver(data map[string]any) bool {
	return data["isSpinOver"].(bool)
}

func (*Game) GetProtobufConverter() base.ProtobufConverter {
	return base.ProtoToMapConverter(&pb.Stub_BetOrderResponse{})
}
