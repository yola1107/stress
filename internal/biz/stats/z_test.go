package stats

import "testing"

// TestQueryGameStatistics 单游戏查询流程（不依赖真实DB）
func TestQueryGameStatistics(t *testing.T) {
	c := New()
	points := []Point{{X: 1, Y: 0.1, Time: "2024-01-01 00:00:00"}}
	r, err := c.BuildChart(points, "幸运熊猫", "xfl123")
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if r == nil {
		t.Fatalf("结果为空")
	}
}
