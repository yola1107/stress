package task

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	v1 "stress/api/stress/v1"
	"stress/internal/biz/game/base"
	"stress/internal/conf"

	jsoniter "github.com/json-iterator/go"
)

const (
	maxConnsCap = 5000
)

var (
	jsonAPI = jsoniter.ConfigFastest

	jsonBufferPool = sync.Pool{
		New: func() any {
			return &bytes.Buffer{}
		},
	}

	// NoopSecretProvider 不提供 secret，用于压测
	NoopSecretProvider base.SecretProvider = func(string) (string, bool) { return "", false }
)

type APIError struct {
	Op   string
	Code int
	Msg  string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("error: [op=%q, code=%d， msg=%s]", e.Op, e.Code, e.Msg)
}

type APIClient struct {
	http         *http.Client
	secret       base.SecretProvider
	launchURL    string
	loginURL     string
	betOrderURL  string
	betBonusURL  string
	merchant     string
	signRequired bool

	env *SessionEnv
}

type SessionEnv struct {
	ctx             context.Context
	cfg             *v1.TaskConfig
	target          int32
	bonusNum        int64
	randomNums      []int64
	bonusSeq        []int64
	isSpinOver      func(map[string]any) bool
	needBonus       func(map[string]any) bool
	protobuf        base.ProtobufConverter
	bonusNext       func(map[string]any) bool
	addBetOrder     func(time.Duration, bool)
	addBetBonus     func(time.Duration)
	addError        func()
	isTaskCancelled func() bool
}

func NewAPIClient(capacity int, secretProvider base.SecretProvider, launchCfg *conf.Stress_Launch) *APIClient {
	if capacity <= 0 {
		capacity = 100
	}
	if capacity > maxConnsCap {
		capacity = maxConnsCap
	}

	baseApiURL := strings.TrimRight(launchCfg.GetApiUrl(), "/")
	baseLaunchURL := strings.TrimRight(launchCfg.GetLaunchUrl(), "/")

	return &APIClient{
		http: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				Proxy:               http.ProxyFromEnvironment,
				TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
				MaxIdleConns:        capacity,
				MaxIdleConnsPerHost: capacity,
				MaxConnsPerHost:     capacity,
				IdleConnTimeout:     30 * time.Second,
				DisableKeepAlives:   false,
				ForceAttemptHTTP2:   true,
				TLSHandshakeTimeout: 5 * time.Second,
				DialContext: (&net.Dialer{
					Timeout:   10 * time.Second,
					KeepAlive: 30 * time.Second,
					DualStack: true,
				}).DialContext,
				ResponseHeaderTimeout: 10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
		},
		secret:       secretProvider,
		launchURL:    baseLaunchURL + "/v1/game/launch",
		loginURL:     baseApiURL + "/api/member/login",
		betOrderURL:  baseApiURL + "/api/game/betorder",
		betBonusURL:  baseApiURL + "/api/game/betbonus",
		merchant:     launchCfg.Merchant,
		signRequired: launchCfg.SignRequired,
	}
}

func (c *APIClient) BindSessionEnv(ctx context.Context, task *Task) error {
	if task == nil {
		return errors.New("task is nil")
	}
	cfg := task.GetConfig()
	if cfg == nil {
		return errors.New("task config is nil")
	}
	env := &SessionEnv{
		ctx:    ctx,
		cfg:    cfg,
		target: cfg.TimesPerMember,
	}
	if bonusCfg := cfg.BetBonus; bonusCfg != nil {
		env.bonusNum = bonusCfg.BonusNum
		env.randomNums = bonusCfg.RandomNums
		env.bonusSeq = bonusCfg.BonusSequence
	}
	if game := task.GetGame(); game != nil {
		env.isSpinOver = game.IsSpinOver
		env.needBonus = game.NeedBetBonus
		env.protobuf = game.GetProtobufConverter()
		if bi := game.AsBonusInterface(); bi != nil {
			env.bonusNext = bi.BonusNextState
		}
	}
	env.addBetOrder = task.AddBetOrder
	env.addBetBonus = task.AddBetBonus
	env.addError = task.AddError
	env.isTaskCancelled = func() bool {
		return task.GetStatus() == v1.TaskStatus_TASK_CANCELLED
	}
	c.env = env
	return nil
}

func (c *APIClient) Env() *SessionEnv {
	return c.env
}

type launchParams struct {
	GameID    int64  `json:"gameId"`
	Merchant  string `json:"merchant"`
	Member    string `json:"member"`
	Timestamp int64  `json:"timestamp"`
}

func signForLaunch(params launchParams, secret string) string {
	sign := fmt.Sprintf("%d%s%s%d%s", params.Timestamp, params.Merchant, params.Member, params.GameID, secret)
	h := md5.New()
	h.Write([]byte(sign))
	return fmt.Sprintf("%x", h.Sum(nil))
}

type apiResponse struct {
	Code  int                 `json:"code"`
	Msg   string              `json:"msg"`
	Data  jsoniter.RawMessage `json:"data"`
	Bytes string              `json:"bytes,omitempty"`
	Type  int                 `json:"type,omitempty"`
}

func (c *APIClient) request(ctx context.Context, method, apiURL string, body any, token string, sign bool) (*apiResponse, error) {
	var bodyReader io.Reader
	if body != nil {
		buf := jsonBufferPool.Get().(*bytes.Buffer)
		defer func() {
			buf.Reset()
			jsonBufferPool.Put(buf)
		}()
		encoder := jsonAPI.NewEncoder(buf)
		if err := encoder.Encode(body); err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(buf.Bytes())
	}

	req, err := http.NewRequestWithContext(ctx, method, apiURL, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connection", "keep-alive")
	if token != "" {
		req.Header.Set("x-token", token)
	}

	if sign {
		if c.secret == nil {
			return nil, errors.New("sign_required=true but secret provider is nil")
		}
		secret, ok := c.secret(c.merchant)
		if !ok || secret == "" {
			return nil, fmt.Errorf("sign_required=true but no secret for merchant=%s", c.merchant)
		}
		if lp, ok := body.(launchParams); ok {
			req.Header.Set("Sign", signForLaunch(lp, secret))
		}
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.CopyN(io.Discard, resp.Body, 1024) // 只丢弃前1KB，避免大响应体阻塞
		return nil, fmt.Errorf("http status %d", resp.StatusCode)
	}

	var res apiResponse
	if err := jsonAPI.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *APIClient) Launch(ctx context.Context, cfg *v1.TaskConfig, member string) (string, error) {
	params := launchParams{
		GameID:    cfg.GameId,
		Merchant:  c.merchant,
		Member:    member,
		Timestamp: time.Now().Unix(),
	}

	res, err := c.request(ctx, http.MethodPost, c.launchURL, params, "", c.signRequired)
	if err != nil {
		return "", err
	}
	if res.Code != 0 {
		return "", &APIError{Op: "launch", Code: res.Code, Msg: res.Msg}
	}

	var data struct {
		LaunchUrl string `json:"launchUrl"`
	}
	_ = jsonAPI.Unmarshal(res.Data, &data)

	path, _ := url.QueryUnescape(data.LaunchUrl)
	parsed, err := url.Parse(path)
	if err != nil {
		return "", err
	}
	tk := parsed.Query().Get("token")
	if tk == "" {
		return "", errors.New("empty token")
	}
	return strings.ReplaceAll(tk, " ", "+"), nil
}

func (c *APIClient) Login(ctx context.Context, cfg *v1.TaskConfig, token string) (string, map[string]any, error) {
	res, err := c.request(ctx, http.MethodPost, c.loginURL, map[string]any{"token": token}, "", false)
	if err != nil {
		return "", nil, err
	}
	if res.Code != 0 {
		return "", nil, fmt.Errorf("login error: %s", res.Msg)
	}

	var data struct {
		Token    string         `json:"token"`
		FreeData map[string]any `json:"freeData"`
	}
	_ = jsonAPI.Unmarshal(res.Data, &data)
	return strings.ReplaceAll(data.Token, " ", "+"), data.FreeData, nil
}

type BetOrderError struct {
	Code          int
	Msg           string
	NeedRelogin   bool
	NeedRelaunch  bool
	SleepDuration time.Duration
}

func (e *BetOrderError) Error() string {
	return fmt.Sprintf("betorder error: code=%d msg=%s", e.Code, e.Msg)
}

func (c *APIClient) decodeProtobuf(cfg *v1.TaskConfig, bytesData string) (map[string]any, error) {
	bytesTrimmed := strings.TrimSpace(bytesData)
	if bytesTrimmed == "" {
		return nil, fmt.Errorf("betorder api response bytes is empty for game %d", cfg.GameId)
	}

	if c.env == nil || c.env.protobuf == nil {
		return nil, fmt.Errorf("protobuf converter is nil for game %d", cfg.GameId)
	}
	protoBytes, err := base64.StdEncoding.DecodeString(bytesTrimmed)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 bytes: %v", err)
	}

	result, err := c.env.protobuf(protoBytes)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *APIClient) BetOrder(ctx context.Context, cfg *v1.TaskConfig, token string) (map[string]any, error) {
	params := map[string]any{"gameId": cfg.GameId}
	if cfg.BetOrder != nil {
		params["baseMoney"] = cfg.BetOrder.BaseMoney
		params["multiple"] = cfg.BetOrder.Multiple
		params["purchase"] = cfg.BetOrder.Purchase
	}

	//apiURL := fmt.Sprintf("%s/api/game/betorder", strings.TrimRight(c.launchCfg.GetApiUrl(), "/"))
	res, err := c.request(ctx, http.MethodPost, c.betOrderURL, params, token, false)
	if err != nil {
		return nil, err
	}

	if res.Code != 0 {
		msg := strings.TrimSpace(res.Msg)
		e := &BetOrderError{Code: res.Code, Msg: msg}
		lmsg := strings.ToLower(msg)

		relaunchKeywords := []string{"连接失效", "internal error", "invalid token", "token expired", "unauthorized"}
		for _, kw := range relaunchKeywords {
			if strings.Contains(lmsg, kw) {
				e.NeedRelaunch = true
				if kw == "internal error" {
					e.SleepDuration = time.Second
				}
				return nil, e
			}
		}

		if strings.Contains(lmsg, "limit") {
			e.NeedRelogin = true
			e.SleepDuration = 3 * time.Second
			return nil, e
		}

		return nil, e
	}

	if c.env != nil && c.env.protobuf != nil {
		return c.decodeProtobuf(cfg, res.Bytes)
	}

	var data map[string]any
	err = jsonAPI.Unmarshal(res.Data, &data)
	return data, err
}

type BetBonusResult struct {
	Data         map[string]any
	NeedContinue bool
}

func (c *APIClient) BetBonus(ctx context.Context, cfg *v1.TaskConfig, token string, bonusNum int64) (*BetBonusResult, error) {
	params := map[string]any{"gameId": cfg.GameId, "bonusNum": bonusNum}
	//apiURL := fmt.Sprintf("%s/api/game/betbonus", strings.TrimRight(c.launchCfg.GetApiUrl(), "/"))
	res, err := c.request(ctx, http.MethodPost, c.betBonusURL, params, token, false)
	if err != nil {
		return nil, err
	}
	if res.Code != 0 {
		return nil, fmt.Errorf("betbonus error: %d %s", res.Code, res.Msg)
	}

	var data map[string]any
	_ = jsonAPI.Unmarshal(res.Data, &data)
	result := &BetBonusResult{Data: data}
	if c.env != nil && c.env.bonusNext != nil {
		result.NeedContinue = c.env.bonusNext(data)
	}
	return result, nil
}

// Close 释放APIClient占用的资源
func (c *APIClient) Close() {
	if c.http != nil {
		c.http.CloseIdleConnections()
		time.Sleep(200 * time.Millisecond)
		if transport, ok := c.http.Transport.(*http.Transport); ok {
			transport.CloseIdleConnections()
			c.http.Transport = nil // 解除引用，让GC回收
		}
		c.http = nil
	}
	c.secret = nil
	c.env = nil
	c.launchURL = ""
	c.loginURL = ""
	c.betOrderURL = ""
	c.betBonusURL = ""
}
