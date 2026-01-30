package jqt

import (
	"fmt"

	"google.golang.org/protobuf/proto"
)

// ConvertProtobufToMap 将protobuf消息转换为map[string]any格式
func ConvertProtobufToMap(protoBytes []byte) (map[string]any, error) {
	jqtResponse := new(Jqt_BetOrderResponse)
	if err := proto.Unmarshal(protoBytes, jqtResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal protobuf: %v", err)
	}
	return map[string]any{"next": jqtResponse.WinInfo.Next}, nil
}
