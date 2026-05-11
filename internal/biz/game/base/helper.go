package base

import (
	"stress/pkg/xgo"

	jsoniter "github.com/json-iterator/go"
	"google.golang.org/protobuf/proto"
)

// ProtoToMapConverter 生成通用的 protobuf → map 转换器，消除游戏间的模板代码
func ProtoToMapConverter(prototype proto.Message) ProtobufConverter {
	msgType := prototype.ProtoReflect().Type()
	return func(b []byte) (map[string]any, error) {
		out := msgType.New().Interface()
		if err := proto.Unmarshal(b, out); err != nil {
			return nil, err
		}
		raw, _ := jsoniter.Marshal(out)
		var m map[string]any
		_ = jsoniter.Unmarshal(raw, &m)
		return m, nil
	}
}

func CalcBonusNum(bonusNum []int64) int64 {
	switch len(bonusNum) {
	case 0:
		return -1
	case 1:
		return bonusNum[0]
	default:
		return bonusNum[xgo.RandInt(0, len(bonusNum))]
	}
}
