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
	GetBonusNum() int
	GetProtobufConverter() ProtobufConverter // 返回 nil 表示不支持protobuf，使用JSON格式
	AsBonusInterface() GameBonusInterface    // AsBonusInterface 如果游戏实现了 IGameBonus，返回该接口；否则返回 nil
}

// ProtobufConverter 定义protobuf到map的转换函数类型
type ProtobufConverter func([]byte) (map[string]any, error)

// GameBonusInterface 奖励选择接口，用于判断是否需要继续选择奖励
type GameBonusInterface interface {
	BonusNextState(data map[string]any) bool
}

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

func (g *Default) GetBonusNum() int {
	return 0
}

func (g *Default) GetProtobufConverter() ProtobufConverter {
	return nil
}

func (g *Default) AsBonusInterface() GameBonusInterface {
	return nil
}
