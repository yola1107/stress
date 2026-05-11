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
	"stress/internal/biz/game/g18915"
	"stress/internal/biz/game/g18916"
	"stress/internal/biz/game/g18917"
	"stress/internal/biz/game/g18920"
	"stress/internal/biz/game/g18921"
	"stress/internal/biz/game/g18922"
	"stress/internal/biz/game/g18923"
	"stress/internal/biz/game/g18925"
	"stress/internal/biz/game/g18926"
	"stress/internal/biz/game/g18928"
	"stress/internal/biz/game/g18929"
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
	"stress/internal/biz/game/g18955"
	"stress/internal/biz/game/g18956"
	"stress/internal/biz/game/g18958"
	"stress/internal/biz/game/g18959"
	"stress/internal/biz/game/g18960"
	"stress/internal/biz/game/g18961"
	"stress/internal/biz/game/g18964"
	"stress/internal/biz/game/g18965"
	"stress/internal/biz/game/g18968"
	"stress/internal/biz/game/g18969"
	"stress/internal/biz/game/g18971"
	"stress/internal/biz/game/g18972"
	"stress/internal/biz/game/g18974"
	"stress/internal/biz/game/g18977"
	"stress/internal/biz/game/g18979"
	"stress/internal/biz/game/g18983"
	"stress/internal/biz/game/g18984"
	"stress/internal/biz/game/g18986"
	"stress/internal/biz/game/g18987"
	"stress/internal/biz/game/g18988"
	"stress/internal/biz/game/g18989"
	"stress/internal/biz/game/g18992"
	"stress/internal/biz/game/g19004"
	"stress/internal/biz/game/g8917"
)

var registry = map[int64]base.IGame{
	g18890.ID: g18890.New(), // 战火西岐
	g18891.ID: g18891.New(), // 吸血鬼
	g18892.ID: g18892.New(), // 血色浪漫
	g18894.ID: g18894.New(), // 水果盛宴
	g18895.ID: g18895.New(), // 金字塔的秘密
	g18896.ID: g18896.New(), // 哪吒无极限
	g18897.ID: g18897.New(), // 僵尸冲冲冲
	g18898.ID: g18898.New(), // 埃及女王
	g18899.ID: g18899.New(), // 功夫
	g18900.ID: g18900.New(), // 炸弹甜妞
	g18901.ID: g18901.New(), // 清爽夏日
	g18902.ID: g18902.New(), // 波塞冬之力
	g18903.ID: g18903.New(), // 大富翁
	g18904.ID: g18904.New(), // 法老归来
	g18905.ID: g18905.New(), // 加勒比海盗
	g18906.ID: g18906.New(), // 疯狂动物城
	g18907.ID: g18907.New(), // 英雄联盟
	g18910.ID: g18910.New(), // 甜蜜传奇
	g18912.ID: g18912.New(), // 金钱虎
	g18913.ID: g18913.New(), // vip欲望派对
	g18914.ID: g18914.New(), // 贪吃蛇
	g18920.ID: g18920.New(), // 战神雅典娜
	g18922.ID: g18922.New(), // 金钱兔
	g18923.ID: g18923.New(), // 巨龙传说
	g18925.ID: g18925.New(), // 牌九
	g18931.ID: g18931.New(), // 赏金女王
	g18933.ID: g18933.New(), // 金龙送宝2
	g18935.ID: g18935.New(), // 赏金船长
	g18936.ID: g18936.New(), // 赏金大对决
	g18937.ID: g18937.New(), // 亡灵大盗
	g18939.ID: g18939.New(), // 斗鸡
	g18940.ID: g18940.New(), // 寻宝黄金城
	g18943.ID: g18943.New(), // 麻将胡了
	g18945.ID: g18945.New(), // 加拿大28
	g18946.ID: g18946.New(), // 庆余年
	g18947.ID: g18947.New(), // 五福临门
	g18949.ID: g18949.New(), // 霍比特人
	g18954.ID: g18954.New(), // 王牌
	g18958.ID: g18958.New(), // 冰河世纪
	g18961.ID: g18961.New(), // 幸运熊猫
	g18965.ID: g18965.New(), // 巴西狂欢
	g18971.ID: g18971.New(), // 哪吒之魔童闹海
	g18921.ID: g18921.New(), // 三国志
	g18968.ID: g18968.New(), // 玛雅迷城
	g18979.ID: g18979.New(), // 美杜莎的宝藏
	g18986.ID: g18986.New(), // 欢乐钓鱼佬
	g18988.ID: g18988.New(), // 金钱牛
	g18964.ID: g18964.New(), // 马行大运
	g18974.ID: g18974.New(), // 金钱牛
	g18916.ID: g18916.New(), // 地产大亨
	g18928.ID: g18928.New(), // 夺宝黄金岛
	g18955.ID: g18955.New(), // 马里奥世界
	g18929.ID: g18929.New(), // 生化危机
	g18917.ID: g18917.New(), // 齐鲁佳肴
	g18977.ID: g18977.New(), // 速度激情
	g18983.ID: g18983.New(), // 财神降福
	g18969.ID: g18969.New(), // 世界小姐
	g19004.ID: g19004.New(), // 埃及探秘
	g18956.ID: g18956.New(), // 横财三千亿
	g18972.ID: g18972.New(), // 金钱鼠
	g18984.ID: g18984.New(), // 破茧成蝶
	g18926.ID: g18926.New(), // 印钞机
	g18959.ID: g18959.New(), // 樱花物语
	g8917.ID:  g8917.New(),  // 桑巴与亚马逊
	g18960.ID: g18960.New(), // 大航海家
	g18987.ID: g18987.New(), // 聚宝盆
	g18989.ID: g18989.New(), // 鬼吹灯
	g18915.ID: g18915.New(), // 鼠来宝
	g18992.ID: g18992.New(), // 谍战风云
}
