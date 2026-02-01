package statistics

import (
	"context"
	"testing"
	"time"
)

// TestQueryGameStatistics 与 main 相同功能：ID 解析、流程串联
func TestQueryGameStatistics(t *testing.T) {
	ctx := context.Background()
	today := time.Now().Format("2006-01-02")
	r, err := QueryGameStatistics(ctx,
		&Config{
			Host:     "192.168.10.83",
			User:     "root",
			Password: "Aa12345!@#",
			Database: "egame_order",
		},
		"18961",
		"幸运熊猫",
		"xfl123",
		"",
		today+" 00:00:00",
		today+" 23:59:59",
	)
	if err != nil {
		t.Fatalf("集成测试失败: %v", err)
	}
	if r == nil || !r.Success {
		t.Fatalf("集成测试失败: %+v", r)
	}
}
