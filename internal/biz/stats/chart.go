package stats

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// OutputDir 输出目录（可配置）
var OutputDir = "./rtp_charts" //defaultOutputDir()

// Result 查询结果
type Result struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// Point 图表数据点
type Point struct {
	X    float64 // 订单数（万）
	Y    float64 // 盈利率
	Time string  // 时间
}

// Component 图表生成组件
type Component struct{}

// New 创建图表组件
func New() *Component {
	return &Component{}
}

// BuildChart 生成单个游戏图表
func (c *Component) BuildChart(pts []Point, gameName, merchant string) (*Result, error) {
	if len(pts) == 0 {
		return &Result{Message: "无数据"}, nil
	}

	path, err := generateChart(pts, gameName, merchant)
	if err != nil {
		return &Result{Message: fmt.Sprintf("%s 生成失败", gameName)}, nil
	}

	ext := "HTML"
	if _, e := os.Stat(toPng(path)); e == nil {
		ext = "HTML+PNG"
	}
	slog.Info("完成", "game", gameName)
	return &Result{Success: true, Message: fmt.Sprintf("%s(%s)", gameName, ext)}, nil
}

// generateChart 生成图表
func generateChart(pts []Point, gameName, merchant string) (string, error) {
	if err := os.MkdirAll(OutputDir, 0755); err != nil {
		return "", err
	}

	html, name, err := generateChartBytes(pts, gameName, merchant)
	if err != nil {
		return "", err
	}

	path := filepath.Join(OutputDir, name)
	if err := os.WriteFile(path, html, 0644); err != nil {
		return "", err
	}

	renderPNG(path)
	return path, nil
}

// generateChartBytes 生成图表 HTML 字节
func generateChartBytes(pts []Point, gameName, merchant string) ([]byte, string, error) {
	x, y, t := make([]float64, len(pts)), make([]float64, len(pts)), make([]string, len(pts))
	xMax, yMin, yMax := 0.0, pts[0].Y, pts[0].Y
	for i, p := range pts {
		x[i], y[i], t[i] = p.X, p.Y, p.Time
		if p.X > xMax {
			xMax = p.X
		}
		if p.Y < yMin {
			yMin = p.Y
		}
		if p.Y > yMax {
			yMax = p.Y
		}
	}

	xJ, _ := json.Marshal(x)
	yJ, _ := json.Marshal(y)
	tJ, _ := json.Marshal(t)

	name := fmt.Sprintf("%s_%s_cdn.html", gameName, time.Now().Format("20060102_1504"))
	html := fmt.Sprintf(chartTpl, gameName, merchant, gameName, "普通", string(xJ), string(yJ), string(tJ), xMax, yMin, yMax, merchant, gameName, "普通")
	return []byte(html), name, nil
}

func toPng(html string) string {
	return strings.TrimSuffix(strings.TrimSuffix(html, "_cdn.html"), ".html") + ".png"
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
