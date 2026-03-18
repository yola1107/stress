package g18904

import (
	"fmt"
	"stress/api/common/pb"

	"google.golang.org/protobuf/proto"
)

// ConvertProtobufToMap 将protobuf消息转换为map[string]any格式
func ConvertProtobufToMap(protoBytes []byte) (map[string]any, error) {
	// protobuf反序列化
	winDetailsResp := &pb.WinDetailsResponse{}
	if err := proto.Unmarshal(protoBytes, winDetailsResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal protobuf: %v", err)
	}

	// 转换为map[string]any格式
	data := make(map[string]any)

	// 基础字段
	if winDetailsResp.BaseBet != nil {
		data["baseBet"] = *winDetailsResp.BaseBet
	}
	if winDetailsResp.BetAmount != nil {
		data["betAmount"] = *winDetailsResp.BetAmount
	}
	if winDetailsResp.Multiplier != nil {
		data["multiplier"] = *winDetailsResp.Multiplier
	}
	data["isFreeRound"] = winDetailsResp.IsFreeRound
	if winDetailsResp.Balance != nil {
		data["balance"] = *winDetailsResp.Balance
	}
	if winDetailsResp.CurrentBalance != nil {
		data["currentBalance"] = *winDetailsResp.CurrentBalance
	}
	if winDetailsResp.IsFree != nil {
		data["isFree"] = *winDetailsResp.IsFree
	}
	data["isRoundOver"] = winDetailsResp.IsRoundOver
	data["isSpinOver"] = winDetailsResp.IsSpinOver
	if winDetailsResp.Review != nil {
		data["review"] = *winDetailsResp.Review
	}
	if winDetailsResp.NewFreeTimes != nil {
		data["newFreeTimes"] = *winDetailsResp.NewFreeTimes
	}
	if winDetailsResp.RemainingFreeTimes != nil {
		data["remainingFreeTimes"] = *winDetailsResp.RemainingFreeTimes
	}
	if winDetailsResp.TotalFreeTime != nil {
		data["totalFreeTime"] = *winDetailsResp.TotalFreeTime
	}
	if winDetailsResp.CurWin != nil {
		data["curWin"] = *winDetailsResp.CurWin
	}
	if winDetailsResp.RoundWin != nil {
		data["roundWin"] = *winDetailsResp.RoundWin
	}
	if winDetailsResp.TotalWin != nil {
		data["totalWin"] = *winDetailsResp.TotalWin
	}
	if winDetailsResp.OrderSN != nil {
		data["orderSN"] = *winDetailsResp.OrderSN
	}
	if winDetailsResp.ExtraLv != nil {
		data["extraLv"] = *winDetailsResp.ExtraLv
	}
	if winDetailsResp.FreeTotalWin != nil {
		data["freeTotalWin"] = *winDetailsResp.FreeTotalWin
	}

	// 数组字段
	if len(winDetailsResp.ExtraSpinCount) > 0 {
		extraSpinCount := make([]map[string]any, len(winDetailsResp.ExtraSpinCount))
		for i, item := range winDetailsResp.ExtraSpinCount {
			extraSpinCount[i] = map[string]any{
				"k": item.K,
				"v": item.V,
			}
		}
		data["extraSpinCount"] = extraSpinCount
	}

	if len(winDetailsResp.SpinRespList) > 0 {
		spinRespList := make([]map[string]any, len(winDetailsResp.SpinRespList))
		for i, item := range winDetailsResp.SpinRespList {
			spinRespList[i] = convertGameInfoToMap(item)
		}
		data["spinRespList"] = spinRespList
	}

	if len(winDetailsResp.BetFreeNum) > 0 {
		betFreeNum := make([]map[string]any, len(winDetailsResp.BetFreeNum))
		for i, item := range winDetailsResp.BetFreeNum {
			betFreeNum[i] = map[string]any{
				"k": item.K,
				"v": item.V,
			}
		}
		data["betFreeNum"] = betFreeNum
	}

	if len(winDetailsResp.AddBonusNum) > 0 {
		addBonusNum := make([]map[string]any, len(winDetailsResp.AddBonusNum))
		for i, item := range winDetailsResp.AddBonusNum {
			addBonusNum[i] = map[string]any{
				"k": item.K,
				"v": item.V,
			}
		}
		data["addBonusNum"] = addBonusNum
	}

	// 嵌套对象
	if winDetailsResp.GetFreeGrid != nil {
		data["getFreeGrid"] = convertGetFreeGridInfoToMap(winDetailsResp.GetFreeGrid)
	}

	return data, nil
}

// convertGameInfoToMap 将GameInfo转换为map
func convertGameInfoToMap(info *pb.GameInfo) map[string]any {
	data := make(map[string]any)

	if info.SymbolGrid != nil {
		data["symbolGrid"] = convertBoardToMap(info.SymbolGrid)
	}
	if info.WinGrid != nil {
		data["winGrid"] = convertBoardToMap(info.WinGrid)
	}

	if len(info.WinResults) > 0 {
		winResults := make([]map[string]any, len(info.WinResults))
		for i, item := range info.WinResults {
			winResults[i] = map[string]any{
				"symbol":             item.Symbol,
				"symbolCount":        item.SymbolCount,
				"lineNumber":         item.GetLineNumber(),
				"baseLineMultiplier": item.BaseLineMultiplier,
				"winPositions":       item.WinPositions,
			}
		}
		data["winResults"] = winResults
	}

	if info.ExtraMultiplier != nil {
		data["extraMultiplier"] = *info.ExtraMultiplier
	}

	if len(info.BonusMultiplier) > 0 {
		bonusMultiplier := make([]map[string]any, len(info.BonusMultiplier))
		for i, item := range info.BonusMultiplier {
			bonusMultiplier[i] = map[string]any{
				"pos":        item.Pos,
				"multiplier": item.Multiplier,
				"money":      item.Money,
			}
		}
		data["bonusMultiplier"] = bonusMultiplier
	}

	if info.PromoteBonusMul != nil {
		data["promoteBonusMul"] = map[string]any{
			"pos":        info.PromoteBonusMul.Pos,
			"multiplier": info.PromoteBonusMul.Multiplier,
			"money":      info.PromoteBonusMul.Money,
		}
	}

	if len(info.JpBonusIndex) > 0 {
		jpBonusIndex := make([]map[string]any, len(info.JpBonusIndex))
		for i, item := range info.JpBonusIndex {
			jpBonusIndex[i] = map[string]any{
				"jpPos":   item.JpPos,
				"jpId":    item.JpId,
				"jpType":  item.JpType,
				"jpMoney": item.JpMoney,
			}
		}
		data["jpBonusIndex"] = jpBonusIndex
	}

	data["isGetGrand"] = info.IsGetGrand

	if info.GrandMoney != nil {
		data["grandMoney"] = *info.GrandMoney
	}

	if len(info.PromoteGrid) > 0 {
		data["promoteGrid"] = info.PromoteGrid
	}

	return data
}

// convertGetFreeGridInfoToMap 将GetFreeGridInfo转换为map
func convertGetFreeGridInfoToMap(info *pb.GetFreeGridInfo) map[string]any {
	data := make(map[string]any)

	if info.Grid != nil {
		data["grid"] = convertBoardToMap(info.Grid)
	}

	if len(info.BonusMultiplier) > 0 {
		bonusMultiplier := make([]map[string]any, len(info.BonusMultiplier))
		for i, item := range info.BonusMultiplier {
			bonusMultiplier[i] = map[string]any{
				"pos":        item.Pos,
				"multiplier": item.Multiplier,
				"money":      item.Money,
			}
		}
		data["bonusMultiplier"] = bonusMultiplier
	}

	return data
}

// convertBoardToMap 将Board转换为map
func convertBoardToMap(board *pb.Board) map[string]any {
	return map[string]any{
		"elements": board.Elements,
	}
}
