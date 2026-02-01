package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	v1 "stress/api/stress/v1"
	"stress/internal/conf"

	"github.com/google/wire"
	jsoniter "github.com/json-iterator/go"
)

var ProviderSet = wire.NewSet(NewFeishu)

type Feishu struct {
	WebhookURL    string
	SigningSecret string
	Prefix        string
	Client        *http.Client
}

func NewFeishu(c *conf.Notify) Notifier {
	if c == nil || !c.Enabled || strings.TrimSpace(c.GetWebhookUrl()) == "" {
		return Noop{}
	}
	return &Feishu{
		WebhookURL:    strings.TrimSpace(c.GetWebhookUrl()),
		SigningSecret: strings.TrimSpace(c.GetSigningSecret()),
		Prefix:        strings.TrimSpace(c.GetPrefix()),
		Client:        &http.Client{Timeout: 10 * time.Second},
	}
}

func (f *Feishu) Send(ctx context.Context, msg *Message) error {
	if f.WebhookURL == "" || msg == nil {
		return nil
	}

	content := msg.Content
	if content == "" {
		content = msg.Title
	}
	title := msg.Title
	if title == "" {
		title = "通知"
	}
	if p := strings.TrimSpace(f.Prefix); p != "" {
		title = p + " " + title
	}

	payload := map[string]any{
		"msg_type": "interactive",
		"card": map[string]any{
			"config":   map[string]bool{"wide_screen_mode": true},
			"header":   map[string]any{"title": map[string]string{"tag": "plain_text", "content": title}, "template": "blue"},
			"elements": []map[string]any{{"tag": "div", "text": map[string]string{"tag": "lark_md", "content": content}}},
		},
	}
	if f.SigningSecret != "" {
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		payload["timestamp"] = ts
		payload["sign"] = f.sign(ts)
	}

	body, _ := jsoniter.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, f.WebhookURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	client := f.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("feishu: status %d", resp.StatusCode)
	}
	var r struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	_ = jsoniter.NewDecoder(resp.Body).Decode(&r)
	if r.Code != 0 {
		return fmt.Errorf("feishu: code=%d msg=%s", r.Code, r.Msg)
	}
	return nil
}

// sign 飞书加签，与 scripts/feishu-test.sh 一致：HMAC-SHA256(key=timestamp+\n+secret, message="")
func (f *Feishu) sign(ts string) string {
	key := ts + "\n" + f.SigningSecret
	h := hmac.New(sha256.New, []byte(key))
	h.Write(nil)
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// BuildTaskCompletionMessage 根据 proto TaskCompletionReport 构建任务结束的 Markdown 消息
func BuildTaskCompletionMessage(r *v1.TaskCompletionReport) *Message {
	if r == nil {
		return &Message{Title: "压测任务结束", Content: ""}
	}
	lines := []string{
		fmt.Sprintf("**任务ID**：%s", r.TaskId),
		fmt.Sprintf("**游戏ID**：%d", r.GameId),
		fmt.Sprintf("**进度**：%d / %d (%.1f%%)", r.Process, r.Target, r.ProgressPct),
		fmt.Sprintf("**总步数**：%d", r.Step),
		fmt.Sprintf("**耗时**：%s", r.Duration),
		fmt.Sprintf("**QPS**：%.2f", r.Qps),
		fmt.Sprintf("**平均延迟**：%s", r.AvgLatency),
		fmt.Sprintf("**订单数**：%d", r.OrderCount),
		fmt.Sprintf("**总下注**：%.2f (×1e4)", float64(r.TotalBet)),
		fmt.Sprintf("**总赢**：%.2f (×1e4)", float64(r.TotalWin)),
		fmt.Sprintf("**RTP**：%.2f%%", r.RtpPct),
		fmt.Sprintf("**活跃成员**：%d", r.ActiveMembers),
		fmt.Sprintf("**完成成员**：%d", r.Completed),
		fmt.Sprintf("**失败成员**：%d", r.Failed),
		fmt.Sprintf("**失败请求**：%d", r.FailedReqs),
	}
	return &Message{Title: "压测任务结束", Content: strings.Join(lines, "\n")}
}
