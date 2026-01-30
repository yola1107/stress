package flgl

import (
	"fmt"

	"stress/api/game/common"

	"google.golang.org/protobuf/proto"
)

// ConvertProtobufToMap 将protobuf消息转换为map[string]any格式
func ConvertProtobufToMap(protoBytes []byte) (map[string]any, error) {
	// protobuf反序列化
	winDetailsResp := &WinDetailsResponse{}
	if err := proto.Unmarshal(protoBytes, winDetailsResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal protobuf: %v", err)
	}

	// 转换为map[string]any格式
	data := make(map[string]any)

	// 基础字段
	data["baseBet"] = winDetailsResp.BaseBet
	data["betAmount"] = winDetailsResp.BetAmount
	data["multiplier"] = winDetailsResp.Multiplier
	data["isFreeRound"] = winDetailsResp.IsFreeRound
	data["balance"] = winDetailsResp.Balance
	data["currentBalance"] = winDetailsResp.CurrentBalance
	data["isFree"] = winDetailsResp.IsFree
	data["isRoundOver"] = winDetailsResp.IsRoundOver
	data["isSpinOver"] = winDetailsResp.IsSpinOver
	data["review"] = winDetailsResp.Review
	data["newFreeTimes"] = winDetailsResp.NewFreeTimes
	data["remainingFreeTimes"] = winDetailsResp.RemainingFreeTimes
	data["totalFreeTime"] = winDetailsResp.TotalFreeTime
	data["curWin"] = winDetailsResp.CurWin
	data["roundWin"] = winDetailsResp.RoundWin
	data["totalWin"] = winDetailsResp.TotalWin
	data["orderSN"] = winDetailsResp.OrderSN
	data["extraLv"] = winDetailsResp.ExtraLv
	data["freeTotalWin"] = winDetailsResp.FreeTotalWin

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
func convertGameInfoToMap(info *GameInfo) map[string]any {
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
				"lineNumber":         item.LineNumber,
				"baseLineMultiplier": item.BaseLineMultiplier,
				"winPositions":       item.WinPositions,
			}
		}
		data["winResults"] = winResults
	}

	data["extraMultiplier"] = info.ExtraMultiplier

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

	data["grandMoney"] = info.GrandMoney

	if len(info.PromoteGrid) > 0 {
		data["promoteGrid"] = info.PromoteGrid
	}

	return data
}

// convertGetFreeGridInfoToMap 将GetFreeGridInfo转换为map
func convertGetFreeGridInfoToMap(info *GetFreeGridInfo) map[string]any {
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
func convertBoardToMap(board *common.Board) map[string]any {
	return map[string]any{
		"elements": board.Elements,
	}
}
