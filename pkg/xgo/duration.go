package xgo

import (
	"fmt"
	"time"
)

var durationUnits = []struct {
	div float64
	sym string
}{
	{60 * 60 * 24, "d"},
	{60 * 60, "h"},
	{60, "m"},
	{1, "s"},
	{1e-3, "ms"},
	{1e-6, "µs"},
	{1e-9, "ns"},
}

// ShortDuration 格式化时长为最合适单位，如 1d、2.5h、12.34ms
func ShortDuration(d time.Duration) string {
	if d == 0 {
		return "0"
	}
	sec := d.Seconds()
	for _, u := range durationUnits {
		if sec >= u.div {
			val := sec / u.div
			if val >= 100 {
				return fmt.Sprintf("%.0f%s", val, u.sym)
			}
			if val >= 10 {
				return fmt.Sprintf("%.1f%s", val, u.sym)
			}
			return fmt.Sprintf("%.2f%s", val, u.sym)
		}
	}
	return "0"
}

// AvgDuration 计算平均时长 (d/step)，如单次请求平均延迟
func AvgDuration(d time.Duration, step int64) string {
	if step <= 0 {
		return "0"
	}
	return ShortDuration(time.Duration(int64(d) / step))
}

// FormatDuration 格式化时长为可读串，如 1.5h、2.3m、3.4s
func FormatDuration(d time.Duration) string {
	if d <= 0 {
		return "0"
	}
	sec := d.Seconds()
	if sec >= 3600 {
		return fmt.Sprintf("%.1fh", sec/3600)
	}
	if sec >= 60 {
		return fmt.Sprintf("%.1fm", sec/60)
	}
	return fmt.Sprintf("%.1fs", sec)
}
