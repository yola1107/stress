package chart

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"stress/internal/conf"

	"github.com/google/wire"
	jsoniter "github.com/json-iterator/go"
)

var ProviderSet = wire.NewSet(NewGenerator)

var chromeCache string

const OutputDir = "./rtp_charts"

// IGenerator 图表生成接口
type IGenerator interface {
	Generate(pts []Point, taskId, gameName, merchant string, saveLocal bool) (*GenerateResult, error)
}

// Point 图表数据点
type Point struct {
	X    float64 // 订单数（万）
	Y    float64 // 盈利率
	Time string  // 时间
}

// GenerateResult 生成结果
type GenerateResult struct {
	HTMLContent string // HTML 内容
	FilePath    string // 文件路径（saveLocal=false 时为空）
}

// Generator 图表生成器
type Generator struct {
	outputDir string
}

// NewGenerator 创建图表生成器（使用默认输出目录）
func NewGenerator(_ *conf.Stress) IGenerator {
	return &Generator{outputDir: OutputDir}
}

// Generate 生成图表
// saveLocal: 是否保存本地文件（HTML/PNG）
func (g *Generator) Generate(pts []Point, taskId, gameName, merchant string, saveLocal bool) (*GenerateResult, error) {
	if len(pts) == 0 {
		return nil, fmt.Errorf("no data")
	}

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

	xJ, _ := jsoniter.Marshal(x)
	yJ, _ := jsoniter.Marshal(y)
	tJ, _ := jsoniter.Marshal(t)

	html := fmt.Sprintf(chartTpl, gameName, merchant, gameName, "普通", taskId, string(xJ), string(yJ), string(tJ), xMax, yMin, yMax, merchant, gameName, "普通", taskId)

	result := &GenerateResult{
		HTMLContent: html,
	}

	if !saveLocal {
		return result, nil
	}

	// 保存本地文件
	if err := os.MkdirAll(g.outputDir, 0755); err != nil {
		return nil, err
	}

	path := filepath.Join(g.outputDir, fmt.Sprintf("%s.html", taskId))
	if err := os.WriteFile(path, []byte(html), 0644); err != nil {
		return nil, err
	}

	renderPNG(path)
	result.FilePath = path
	return result, nil
}

func toPng(html string) string {
	return strings.TrimSuffix(html, ".html") + ".png"
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
		args[0] = "--headless"
		_ = exec.Command(chrome, args...).Run()
	}
}

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

const chartTpl = `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<title>游戏数据统计 - %s</title>
<script src="https://cdn.plot.ly/plotly-2.27.0.min.js"></script>
<style>body{font-family:'Microsoft YaHei';margin:0;padding:20px;background:#f5f5f5}.container{background:#fff;padding:20px;border-radius:8px;box-shadow:0 2px 4px rgba(0,0,0,.1)}.xtick,.xtick text,.xaxislayer-above,.xaxislayer-above text{visibility:visible!important;display:block!important;opacity:1!important;fill:#000!important;color:#000!important}</style>
</head>
<body>
<div class="container"><h1>商户: %s, 游戏: %s, 模式: %s, Task: %s</h1><div id="chart"></div></div>
<script>
var xData=%s,yData=%s,timeData=%s,xMax=%f,yMin=%f,yMax=%f;
var trace1={x:xData,y:yData,mode:'lines',name:'平台盈利率',line:{color:'#F00',width:2,shape:'spline'},customdata:timeData,hovertemplate:'订单: %%{x:.2f}万<br>盈利率: %%{y:.2%%}<br>%%{customdata}<extra></extra>'};
var trace2={x:[0,xMax],y:[0.02,0.02],mode:'lines',name:'2%%',line:{color:'blue',dash:'dashdot'}};
var trace3={x:[0,xMax],y:[0.04,0.04],mode:'lines',name:'4%%',line:{color:'green',dash:'dashdot'}};
var annotations=[];
var maxAnno=xMax>=1000?15:12;
var targetStep=Math.ceil(xMax/maxAnno);
var annoStep=targetStep<=50?50:targetStep<=100?100:targetStep<=200?200:targetStep<=500?500:targetStep<=1000?1000:2000;
for(var m=annoStep,count=0;m<=Math.ceil(xMax)&&count<maxAnno;m+=annoStep,count++){
  var idx=0,minD=1e9;
  for(var i=0;i<xData.length;i++) if(xData[i]>=m-10&&Math.abs(xData[i]-m)<minD){minD=Math.abs(xData[i]-m);idx=i;}
  if(minD<1e9) annotations.push({x:xData[idx],y:yData[idx],text:'<b>'+m+'万</b><br>'+(yData[idx]*100).toFixed(2)+'%%',showarrow:true,ax:0,ay:-45,bgcolor:'rgba(255,255,255,.7)'});
}
var yRMin=-0.05,yRMax=1,step=0.05;
var yTicks=[];for(var y=yRMin;y<=yRMax+1e-6;y+=step)yTicks.push(parseFloat(y.toFixed(2)));
var xTicks=[],xTickText=[];
var xStep=xMax<500?50:100;
for(var x=0;x<=Math.ceil(xMax);x+=xStep){
  xTicks.push(x);
  xTickText.push(x+'万');
}
if(xTicks.length>0&&xTicks[xTicks.length-1]<Math.ceil(xMax)){
  xTicks.push(Math.ceil(xMax));
  xTickText.push(Math.ceil(xMax)+'万');
}
var layout={title:'商户: %s, 游戏: %s, 模式: %s, Task: %s',annotations:annotations,
  xaxis:{title:'总订单数(万)',tickmode:'array',tickvals:xTicks,ticktext:xTickText,tickangle:0,showgrid:true,showticklabels:true,side:'bottom',ticklen:5,tickwidth:1,tickfont:{size:12,color:'#000'},titlefont:{size:14},fixedrange:false,automargin:true,zeroline:false},
  yaxis:{title:'平台盈利率',tickvals:yTicks,tickformat:'.0%%',range:[yRMin,yRMax],showgrid:true,showticklabels:true},
  font:{size:14},plot_bgcolor:'#E8F8FF',height:800,width:1600,hovermode:'closest',
  legend:{x:0.99,y:0.99,xanchor:'right'}};
Plotly.newPlot('chart',[trace1,trace2,trace3],layout,{displayModeBar:false}).then(function(){
  setTimeout(function(){
    Plotly.redraw('chart');
    setTimeout(function(){
      var allXTicks=document.querySelectorAll('.xtick');
      for(var i=0;i<allXTicks.length;i++){
        allXTicks[i].style.visibility='visible';
        allXTicks[i].style.display='block';
        allXTicks[i].style.opacity='1';
        var text=allXTicks[i].querySelector('text');
        if(text){
          text.style.visibility='visible';
          text.style.display='block';
          text.style.opacity='1';
          text.style.fill='#000';
        }
      }
      var xAxisLayer=document.querySelector('.xaxislayer-above');
      if(xAxisLayer){
        xAxisLayer.style.visibility='visible';
        xAxisLayer.style.display='block';
      }
    },1000);
  },2000);
});
</script>
</body>
</html>`
