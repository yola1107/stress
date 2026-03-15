package sgz

import (
	"fmt"
	"google.golang.org/protobuf/proto"
)

// 将protobuf消息转换为map[string]any格式
func ConvertProtobufToMap(protoBytes []byte) (map[string]any, error) {
	msg := new(Sgz_BetOrderResponse)
	if err := proto.Unmarshal(protoBytes, msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal protobuf: %v", err)
	}

	result := make(map[string]any)
	if msg.WinInfo != nil && msg.WinInfo.IsRoundOver != nil {
		result["isRoundOver"] = *msg.WinInfo.IsRoundOver
	}
	return result, nil
}
