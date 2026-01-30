package game

import (
	"stress/internal/biz/game/base"
	"stress/internal/biz/game/g18890"
	"stress/internal/biz/game/g18891"
	"stress/internal/biz/game/g18892"
	"stress/internal/biz/game/g18894"
	"stress/internal/biz/game/g18895"
	"stress/internal/biz/game/g18896"
	"stress/internal/biz/game/g18897"
	"stress/internal/biz/game/g18898"
	"stress/internal/biz/game/g18899"
	"stress/internal/biz/game/g18900"
	"stress/internal/biz/game/g18901"
	"stress/internal/biz/game/g18902"
	"stress/internal/biz/game/g18903"
	"stress/internal/biz/game/g18904"
	"stress/internal/biz/game/g18905"
	"stress/internal/biz/game/g18906"
	"stress/internal/biz/game/g18907"
	"stress/internal/biz/game/g18910"
	"stress/internal/biz/game/g18912"
	"stress/internal/biz/game/g18913"
	"stress/internal/biz/game/g18914"
	"stress/internal/biz/game/g18920"
	"stress/internal/biz/game/g18922"
	"stress/internal/biz/game/g18923"
	"stress/internal/biz/game/g18925"
	"stress/internal/biz/game/g18931"
	"stress/internal/biz/game/g18933"
	"stress/internal/biz/game/g18935"
	"stress/internal/biz/game/g18936"
	"stress/internal/biz/game/g18937"
	"stress/internal/biz/game/g18939"
	"stress/internal/biz/game/g18940"
	"stress/internal/biz/game/g18943"
	"stress/internal/biz/game/g18945"
	"stress/internal/biz/game/g18946"
	"stress/internal/biz/game/g18947"
	"stress/internal/biz/game/g18949"
	"stress/internal/biz/game/g18954"
	"stress/internal/biz/game/g18958"
	"stress/internal/biz/game/g18961"
	"stress/internal/biz/game/g18965"
	"stress/internal/biz/game/g18971"
)

var registry = map[int64]base.IGame{
	g18890.ID: g18890.Register, // 战火西岐
	g18891.ID: g18891.Register, // 吸血鬼
	g18892.ID: g18892.Register, // 血色浪漫
	g18894.ID: g18894.Register, // 水果盛宴
	g18895.ID: g18895.Register, // 金字塔的秘密
	g18896.ID: g18896.Register, // 哪吒无极限
	g18897.ID: g18897.Register, // 僵尸冲冲冲
	g18898.ID: g18898.Register, // 埃及女王
	g18899.ID: g18899.Register, // 功夫
	g18900.ID: g18900.Register, // 炸弹甜妞
	g18901.ID: g18901.Register, // 清爽夏日
	g18902.ID: g18902.Register, // 波塞冬之力
	g18903.ID: g18903.Register, // 大富翁
	g18904.ID: g18904.Register, // 法老归来
	g18905.ID: g18905.Register, // 加勒比海盗
	g18906.ID: g18906.Register, // 疯狂动物城
	g18907.ID: g18907.Register, // 英雄联盟
	g18910.ID: g18910.Register, // 甜蜜传奇
	g18912.ID: g18912.Register, // 金钱虎
	g18913.ID: g18913.Register, // vip欲望派对
	g18914.ID: g18914.Register, // 贪吃蛇
	g18920.ID: g18920.Register, // 战神雅典娜
	g18922.ID: g18922.Register, // 金钱兔
	g18923.ID: g18923.Register, // 巨龙传说
	g18925.ID: g18925.Register, // 牌九
	g18931.ID: g18931.Register, // 赏金女王
	g18933.ID: g18933.Register, // 金龙送宝2
	g18935.ID: g18935.Register, // 赏金船长
	g18936.ID: g18936.Register, // 赏金大对决
	g18937.ID: g18937.Register, // 亡灵大盗
	g18939.ID: g18939.Register, // 斗鸡
	g18940.ID: g18940.Register, // 寻宝黄金城
	g18943.ID: g18943.Register, // 麻将胡了
	g18945.ID: g18945.Register, // 加拿大28
	g18946.ID: g18946.Register, // 庆余年
	g18947.ID: g18947.Register, // 五福临门
	g18949.ID: g18949.Register, // 霍比特人
	g18954.ID: g18954.Register, // 王牌
	g18958.ID: g18958.Register, // 冰河世纪
	g18961.ID: g18961.Register, // 幸运熊猫
	g18965.ID: g18965.Register, // 巴西狂欢
	g18971.ID: g18971.Register, // 哪吒之魔童闹海
}
