package base

import (
	"fmt"

	jsoniter "github.com/json-iterator/go"
	"google.golang.org/protobuf/proto"
)

// ProtoToMapConverter 生成通用的 protobuf → map 转换器，消除游戏间的模板代码
func ProtoToMapConverter(prototype proto.Message) ProtobufConverter {
	msgType := prototype.ProtoReflect().Type()
	return func(b []byte) (map[string]any, error) {
		msg := msgType.New().Interface()
		if err := proto.Unmarshal(b, msg); err != nil {
			return nil, fmt.Errorf("unmarshal protobuf: %w", err)
		}
		raw, _ := jsoniter.Marshal(msg)
		var m map[string]any
		_ = jsoniter.Unmarshal(raw, &m)
		return m, nil
	}
}
