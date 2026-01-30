package chart

import (
	"fmt"
	"testing"
)

const sampleMax = 5000 // 最大采样数

func TestGenerator(t *testing.T) {
	pts := []Point{
		{X: 1.0, Y: 0.1, Time: "2024-01-01 00:00:00"},
		{X: 2.0, Y: 0.15, Time: "2024-01-01 00:01:00"},
		{X: 3.0, Y: 0.2, Time: "2024-01-01 00:02:00"},
	}

	// 测试采样
	sampled := sample(pts)
	if len(sampled) != len(pts) {
		t.Logf("小数据集不应被采样: %d -> %d", len(pts), len(sampled))
	}

	// 测试大数据集采样
	largePts := make([]Point, 10000)
	for i := 0; i < 10000; i++ {
		largePts[i] = Point{X: float64(i), Y: float64(i%100) / 1000.0, Time: "2024-01-01 00:00:00"}
	}
	sampledLarge := sample(largePts)
	if len(sampledLarge) > sampleMax {
		t.Errorf("采样后不应超过%d，实际: %d", sampleMax, len(sampledLarge))
	}

	t.Logf("采样测试通过: %d -> %d", len(largePts), len(sampledLarge))
}

// sample 等间距采样
func sample(pts []Point) []Point {
	n := len(pts)
	if n <= sampleMax {
		return pts
	}
	step := (n - 1) / (sampleMax - 1)
	if step < 1 {
		step = 1
	}
	out := make([]Point, 0, sampleMax)
	for i := 0; i < n && len(out) < sampleMax-1; i += step {
		out = append(out, pts[i])
	}
	out = append(out, pts[n-1])
	fmt.Printf("采样: 原%d后%d", n, len(out))
	return out
}
