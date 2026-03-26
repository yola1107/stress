package base

// IGame 游戏接口，定义所有游戏必须实现的方法
type IGame interface {
	GameID() int64
	Name() string
	BetSize() []float64
	SetBetSize(betSize []float64)
	ValidBetMoney(money float64) bool
	IsSpinOver(data map[string]any) bool
	NeedBetBonus(freeData map[string]any) bool
	BonusNextState(data map[string]any) bool // 是否还需继续选奖励（多轮 bonus 时用）
	PickBonusNum() int64                     // 选取 bonus 编号（压测用）
	GetProtobufConverter() ProtobufConverter // 返回 nil 表示不支持 protobuf，使用 JSON
}

// ProtobufConverter 定义 protobuf 到 map 的转换函数类型
type ProtobufConverter func([]byte) (map[string]any, error)

// SecretProvider 用于提供 merchant 对应的 secret（用于 launch 签名）
type SecretProvider func(merchant string) (secret string, ok bool)

// Default 基础游戏实现，提供默认行为
type Default struct {
	gameID  int64
	name    string
	betSize []float64
}

// NewBaseGame 创建基础游戏实例
func NewBaseGame(gameID int64, name string) *Default {
	return &Default{
		gameID: gameID,
		name:   name,
	}
}

func (g *Default) GameID() int64 {
	return g.gameID
}

func (g *Default) Name() string {
	return g.name
}

func (g *Default) BetSize() []float64 {
	return g.betSize
}

func (g *Default) SetBetSize(betSize []float64) {
	g.betSize = betSize
}

func (g *Default) ValidBetMoney(money float64) bool {
	for _, bet := range g.betSize {
		if money == bet {
			return true
		}
	}
	return false
}

func (g *Default) IsSpinOver(data map[string]any) bool {
	return true
}

func (g *Default) NeedBetBonus(freeData map[string]any) bool {
	return false
}

func (g *Default) BonusNextState(data map[string]any) bool {
	return false
}

// PickBonusNum 目前需要重写PickBonusNum的游戏有 18902 18920 18931 18946
func (g *Default) PickBonusNum() int64 {
	return -1
}

func (g *Default) GetProtobufConverter() ProtobufConverter {
	return nil
}
