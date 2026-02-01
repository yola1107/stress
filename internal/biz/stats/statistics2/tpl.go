package statistics

const chartTpl = `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<title>游戏数据统计 - %s</title>
<script src="https://cdn.plot.ly/plotly-2.27.0.min.js"></script>
<style>body{font-family:'Microsoft YaHei';margin:0;padding:20px;background:#f5f5f5}.container{background:#fff;padding:20px;border-radius:8px;box-shadow:0 2px 4px rgba(0,0,0,.1)}.xtick,.xtick text,.xaxislayer-above,.xaxislayer-above text{visibility:visible!important;display:block!important;opacity:1!important;fill:#000!important;color:#000!important}</style>
</head>
<body>
<div class="container"><h1>商户: %s, 游戏: %s, 模式: %s</h1><div id="chart"></div></div>
<script>
var xData=%s,yData=%s,timeData=%s,xMax=%f,yMin=%f,yMax=%f;
var trace1={x:xData,y:yData,mode:'lines',name:'平台盈利率',line:{color:'#F00',width:2,shape:'spline'},customdata:timeData,hovertemplate:'订单: %%{x:.2f}万<br>盈利率: %%{y:.2%%}<br>%%{customdata}<extra></extra>'};
var trace2={x:[0,xMax],y:[0.02,0.02],mode:'lines',name:'2%%',line:{color:'blue',dash:'dashdot'}};
var trace3={x:[0,xMax],y:[0.04,0.04],mode:'lines',name:'4%%',line:{color:'green',dash:'dashdot'}};
var annotations=[];
for(var m=50;m<=Math.ceil(xMax);m+=50){
  var idx=0,minD=1e9;
  for(var i=0;i<xData.length;i++) if(xData[i]>=m-10&&Math.abs(xData[i]-m)<minD){minD=Math.abs(xData[i]-m);idx=i;}
  if(minD<1e9) annotations.push({x:xData[idx],y:yData[idx],text:'<b>'+m+'万</b><br>'+(yData[idx]*100).toFixed(2)+'%%',showarrow:true,ax:0,ay:-45,bgcolor:'rgba(255,255,255,.7)'});
}
var pad=Math.max(0.005,(yMax-yMin)*0.08);
var yRMin=yMin-pad,yRMax=yMax+pad;
if(yMax<0.08){yRMin=Math.max(0.01,yMin-0.005);yRMax=Math.max(yRMax,yMin+0.04);}
else{yRMin=Math.max(-0.05,yRMin);yRMax=Math.min(1,yRMax);}
var r=yRMax-yRMin;if(r<0.04){yRMin=yMin-0.01;yRMax=yMax+0.02;r=yRMax-yRMin;}
var step=r<=0.1?0.01:r<=0.2?0.02:0.05;
var yTicks=[];for(var y=Math.floor(yRMin/step)*step;y<=yRMax+1e-6;y+=step)yTicks.push(parseFloat(y.toFixed(2)));
var xTicks=[],xTickText=[];
var xStep=Math.ceil(xMax/15);
if(xStep<10)xStep=10;
else if(xStep>100)xStep=100;
for(var x=0;x<=Math.ceil(xMax);x+=xStep){
  xTicks.push(x);
  xTickText.push(x+'万');
}
if(xTicks.length>0&&xTicks[xTicks.length-1]<Math.ceil(xMax)){
  xTicks.push(Math.ceil(xMax));
  xTickText.push(Math.ceil(xMax)+'万');
}
var layout={title:'商户: %s, 游戏: %s, 模式: %s',annotations:annotations,
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
