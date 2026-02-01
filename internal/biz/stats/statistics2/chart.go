package statistics

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"xorm.io/xorm"
)

const (
	sampleMax  = 5000 // 最大采样数
	orderUnit  = 1e4  // 订单单位（万）
	excludeAmt = 0.01 // 排除金额
	timeLayout = "2006-01-02 15:04:05"
)

var locSH, _ = time.LoadLocation("Asia/Shanghai")

// OutputDir 输出目录（可配置）
var OutputDir = "./rtp_charts" //defaultOutputDir()

// Config 数据库配置
type Config struct {
	Host, User, Password, Database string
}

// Result 查询结果
type Result struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// point 图表数据点
type point struct {
	x    float64 // 订单数（万）
	y    float64 // 盈利率
	time string  // 时间
}

// QueryGameStatistics 查询游戏统计并生成图表
func QueryGameStatistics(ctx context.Context, cfg *Config, gameIds, gameNames, merchant, member, startTime, endTime string) (*Result, error) {
	ids, names := parseGameIDs(gameIds, gameNames)
	if len(ids) == 0 {
		return &Result{Message: "游戏ID为空"}, nil
	}

	eng, err := newEngine(cfg)
	if err != nil {
		return &Result{Message: "连接失败: " + err.Error()}, nil
	}
	defer eng.Close()

	var ok, fail []string
	for i, id := range ids {
		name := names[i]
		pts, err := queryAndProcess(eng, id, merchant, member, startTime, endTime)
		if err != nil {
			fail = append(fail, fmt.Sprintf("%s 查询失败", name))
			continue
		}
		if len(pts) == 0 {
			fail = append(fail, fmt.Sprintf("%s 无数据", name))
			continue
		}

		path, err := generateChart(sample(pts), name, merchant)
		if err != nil {
			fail = append(fail, fmt.Sprintf("%s 生成失败", name))
			continue
		}

		ext := "HTML"
		if _, e := os.Stat(toPng(path)); e == nil {
			ext = "HTML+PNG"
		}
		ok = append(ok, fmt.Sprintf("%s(%s)", name, ext))
		slog.Info("完成", "game", name, "id", id)
	}

	return buildResult(ok, fail), nil
}

func parseGameIDs(ids, names string) ([]int, []string) {
	idList := strings.Split(strings.TrimSpace(ids), ",")
	nameList := strings.Split(strings.TrimSpace(names), ",")
	var ri []int
	var rn []string
	for i, s := range idList {
		if id, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			ri = append(ri, id)
			name := strconv.Itoa(id)
			if i < len(nameList) && strings.TrimSpace(nameList[i]) != "" {
				name = strings.TrimSpace(nameList[i])
			}
			rn = append(rn, name)
		}
	}
	return ri, rn
}

func newEngine(cfg *Config) (*xorm.Engine, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.User, cfg.Password, cfg.Host, cfg.Database)
	eng, err := xorm.NewEngine("mysql", dsn)
	if err != nil {
		return nil, err
	}
	eng.SetMaxOpenConns(5)
	eng.SetMaxIdleConns(5)
	return eng, eng.Ping()
}

func buildResult(ok, fail []string) *Result {
	var msg string
	if len(ok) > 0 {
		msg = "成功: " + strings.Join(ok, ", ")
	}
	if len(fail) > 0 {
		if msg != "" {
			msg += "\n"
		}
		msg += "失败: " + strings.Join(fail, ", ")
	}
	if msg == "" {
		msg = "无游戏处理"
	}
	return &Result{Success: len(ok) > 0, Message: msg}
}

func toPng(html string) string {
	return strings.TrimSuffix(strings.TrimSuffix(html, "_cdn.html"), ".html") + ".png"
}

// queryAndProcess 查询并处理数据，返回图表数据点
func queryAndProcess(eng *xorm.Engine, gameID int, merchant, member, start, end string) ([]point, error) {
	// 构建查询
	where, args := "game_id = ? AND amount != ?", []any{gameID, excludeAmt}
	if merchant != "" {
		where, args = where+" AND merchant = ?", append(args, merchant)
	}
	if member != "" {
		where, args = where+" AND member = ?", append(args, member)
	}
	if start != "" && end != "" {
		where += " AND created_at BETWEEN UNIX_TIMESTAMP(?) AND UNIX_TIMESTAMP(?)"
		args = append(args, start, end)
	}

	type rec struct {
		Amount, BonusAmount float64
		CreatedAt           int64
	}
	var recs []rec
	if err := eng.SQL("SELECT amount, bonus_amount, created_at FROM game_order WHERE "+where+" ORDER BY id", args...).Find(&recs); err != nil {
		return nil, err
	}

	// 合并订单：Amount>0 开始新局，Amount==0 累加 bonus
	var pts []point
	var bet, win, cumBet, cumWin float64
	var t string
	var orders int

	flush := func() {
		if bet > 0 || win > 0 {
			cumBet += bet
			cumWin += win
			rate := 0.0
			if cumBet > 0 {
				rate = (cumBet - cumWin) / cumBet
			}
			pts = append(pts, point{x: float64(orders) / orderUnit, y: rate, time: t})
		}
	}

	for _, r := range recs {
		orders++
		if r.Amount > 0 {
			flush()
			bet, win = r.Amount, r.BonusAmount
			t = time.Unix(r.CreatedAt, 0).In(locSH).Format(timeLayout)
		} else {
			win += r.BonusAmount
		}
	}
	flush()

	return pts, nil
}

// sample 等间距采样，保留首尾
func sample(pts []point) []point {
	n := len(pts)
	if n <= sampleMax {
		return pts
	}
	step := (n - 1) / (sampleMax - 1)
	if step < 1 {
		step = 1
	}
	out := make([]point, 0, sampleMax)
	for i := 0; i < n && len(out) < sampleMax-1; i += step {
		out = append(out, pts[i])
	}
	out = append(out, pts[n-1]) // 保留最后一条
	slog.Info("采样", "原", n, "后", len(out))
	return out
}

// generateChart 生成图表
func generateChart(pts []point, gameName, merchant string) (string, error) {
	if err := os.MkdirAll(OutputDir, 0755); err != nil {
		return "", err
	}

	x, y, t := make([]float64, len(pts)), make([]float64, len(pts)), make([]string, len(pts))
	xMax, yMin, yMax := 0.0, pts[0].y, pts[0].y
	for i, p := range pts {
		x[i], y[i], t[i] = p.x, p.y, p.time
		if p.x > xMax {
			xMax = p.x
		}
		if p.y < yMin {
			yMin = p.y
		}
		if p.y > yMax {
			yMax = p.y
		}
	}

	xJ, _ := json.Marshal(x)
	yJ, _ := json.Marshal(y)
	tJ, _ := json.Marshal(t)

	path := filepath.Join(OutputDir, fmt.Sprintf("%s_%s_cdn.html", gameName, time.Now().Format("20060102_1504")))
	html := fmt.Sprintf(chartTpl, gameName, merchant, gameName, "普通", string(xJ), string(yJ), string(tJ), xMax, yMin, yMax, merchant, gameName, "普通")
	if err := os.WriteFile(path, []byte(html), 0644); err != nil {
		return "", err
	}

	renderPNG(path)
	return path, nil
}

func renderPNG(htmlPath string) {
	chrome := findChrome()
	if chrome == "" {
		return
	}
	absH, _ := filepath.Abs(htmlPath)
	absP, _ := filepath.Abs(toPng(htmlPath))
	args := []string{
		"--headless=new", "--disable-gpu", "--hide-scrollbars",
		"--window-size=1720,920", "--force-device-scale-factor=2",
		"--run-all-compositor-stages-before-draw", "--virtual-time-budget=8000",
		"--disable-web-security", "--no-sandbox",
		"--screenshot=" + absP, "file://" + absH,
	}
	if exec.Command(chrome, args...).Run() != nil {
		args[0] = "--headless" // 旧版
		_ = exec.Command(chrome, args...).Run()
	}
}

var chromeCache string

func findChrome() string {
	if chromeCache != "" {
		return chromeCache
	}
	for _, p := range []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/usr/bin/google-chrome",
		"/usr/bin/chromium",
		"/usr/bin/chromium-browser",
	} {
		if _, err := os.Stat(p); err == nil {
			chromeCache = p
			return p
		}
	}
	for _, name := range []string{"google-chrome", "chromium"} {
		if out, _ := exec.Command("which", name).Output(); len(out) > 0 {
			chromeCache = strings.TrimSpace(string(out))
			return chromeCache
		}
	}
	return ""
}
