package data

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

//-- 2. 插入游戏配置信息
//INSERT IGNORE INTO `egame`.`game_setting` (`game_id`, `api_url`, `api_port`, `rebate_rate`, `bet_size`, `bet_twice`, `win_amt`) VALUES (18923, 'www.baidu.com', 102, 100, '0.02,0.2,0.2', '1,2,3,4,5,6,7,8,9,10', ' ');

// GetGameBetSize 批量获取游戏下注金额数组
func (r *dataRepo) GetGameBetSize(ctx context.Context, gameIDs []int64) (map[int64][]float64, error) {
	if len(gameIDs) == 0 {
		return map[int64][]float64{}, nil
	}

	type GameSetting struct {
		GameID  int64  `xorm:"'game_id'"`
		BetSize string `xorm:"'bet_size'"`
	}

	var list []GameSetting

	err := r.data.db.
		Context(ctx).
		In("game_id", gameIDs).
		Find(&list)
	if err != nil {
		return nil, err
	}

	result := make(map[int64][]float64, len(list))

	for _, item := range list {
		arrStr := strings.Split(item.BetSize, ",")
		var arrFloat []float64
		for _, s := range arrStr {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			f, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return nil, fmt.Errorf("game_id %d: invalid bet_size value %q: %w", item.GameID, s, err)
			}
			arrFloat = append(arrFloat, f)
		}
		result[item.GameID] = arrFloat
	}

	return result, nil
}
