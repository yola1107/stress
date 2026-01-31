package notify

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"stress/internal/conf"
	"strings"
	"time"

	"github.com/google/wire"
	jsoniter "github.com/json-iterator/go"
)

var ProviderSet = wire.NewSet(NewFeishu)

// Feishu 飞书 Webhook
type Feishu struct {
	WebhookURL string
	Prefix     string
	Client     *http.Client
}

// NewFeishu 根据配置创建 Notifier
func NewFeishu(c *conf.Notify) Notifier {
	if c == nil || !c.Enabled || strings.TrimSpace(c.WebhookUrl) == "" {
		return Noop{}
	}
	return &Feishu{
		WebhookURL: strings.TrimSpace(c.WebhookUrl),
		Prefix:     strings.TrimSpace(c.Prefix),
		Client:     &http.Client{Timeout: 10 * time.Second},
	}
}

// Send 实现 Notifier，Content 支持 Markdown
func (f *Feishu) Send(ctx context.Context, msg *Message) error {
	if f.WebhookURL == "" || msg == nil {
		return nil
	}
	content := f.buildText(msg)
	body, _ := jsoniter.Marshal(map[string]any{
		"msg_type": "interactive",
		"card": map[string]any{
			"config":   map[string]bool{"wide_screen_mode": true},
			"header":   f.buildHeader(msg),
			"elements": []map[string]any{{"tag": "div", "text": map[string]string{"tag": "lark_md", "content": content}}},
		},
	})

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
		Code int `json:"code"`
	}
	_ = jsoniter.NewDecoder(resp.Body).Decode(&r)
	if r.Code != 0 {
		return fmt.Errorf("feishu: code=%d", r.Code)
	}
	return nil
}

func (f *Feishu) buildHeader(msg *Message) map[string]any {
	title := msg.Title
	if title == "" {
		title = "通知"
	}
	if p := strings.TrimSpace(f.Prefix); p != "" {
		title = p + " " + title
	}
	return map[string]any{
		"title":    map[string]string{"tag": "plain_text", "content": title},
		"template": "blue",
	}
}

func (f *Feishu) buildText(msg *Message) string {
	content := msg.Content
	if content == "" {
		content = msg.Title
	}
	return content
}
