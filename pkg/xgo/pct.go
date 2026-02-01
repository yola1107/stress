package xgo

// Pct 百分比 num/denom*100，denom<=0 返回 0
func Pct(num, denom int64) float64 {
	if denom <= 0 {
		return 0
	}
	return float64(num) / float64(denom) * 100
}

// PctCap100 同 Pct，结果上限 100
func PctCap100(num, denom int64) float64 {
	p := Pct(num, denom)
	if p > 100 {
		return 100
	}
	return p
}
