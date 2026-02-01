package biz

import "time"

// OrderScope 订单范围（与 statistics 查询口径一致）
type OrderScope struct {
	GameID     int64
	Merchant   string
	StartTime  time.Time
	EndTime    time.Time
	ExcludeAmt float64 // 0 表示 0.01（= base_money）
}
