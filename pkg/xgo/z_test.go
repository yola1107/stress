package xgo

import (
	"testing"

	v1 "stress/api/stress/v1"
)

func TestJSONPretty(t *testing.T) {
	in := &v1.TaskConfig{
		GameId:         18888,
		Description:    "",
		MemberCount:    100,
		TimesPerMember: 100,
		BetOrder: &v1.BetOrderConfig{
			BaseMoney: 0.1,
			Multiple:  10,
			Purchase:  0,
			BonusNum:  []int64{1, 2, 3},
		},
	}

	t.Logf("%s", ToJSONPretty(in))
}
