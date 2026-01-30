package user

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
	"sync/atomic"
	"time"

	jsoniter "github.com/json-iterator/go"

	v1 "stress/api/stress/v1"
	"stress/internal/biz/game/base"
)

var (
	defaultHTTPOnce sync.Once
	defaultHTTP     *http.Client
	jsonBufferPool  = sync.Pool{
		New: func() any {
			return &bytes.Buffer{}
		},
	}
)

func DefaultHTTPClient() *http.Client {
	defaultHTTPOnce.Do(func() {
		defaultHTTP = &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				Proxy:               http.ProxyFromEnvironment,
				TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
				MaxIdleConns:        50000,            // 增加总空闲连接数，支持更高并发
				MaxIdleConnsPerHost: 5000,             // 增加单个主机空闲连接数
				MaxConnsPerHost:     0,                // 0表示不限制并发连接数
				IdleConnTimeout:     90 * time.Second, // 延长空闲超时，减少重连
				DisableKeepAlives:   false,
				TLSHandshakeTimeout: 5 * time.Second, // 减少 TLS 握手超时
				DialContext: (&net.Dialer{
					Timeout:   10 * time.Second,
					KeepAlive: 60 * time.Second,
					DualStack: true,
				}).DialContext,
				// 不设置 TLSNextProto，使用 Go 默认行为：HTTPS 自动协商 HTTP/2，多路复用可提升 QPS
				ResponseHeaderTimeout: 10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
		}
	})
	return defaultHTTP
}

type APIClient struct {
	http                *http.Client
	secret              base.SecretProvider
	game                base.IGame
	protobufChecker     func(gameID int64) bool
	protoConverterCache atomic.Value
}

func NewAPIClient(httpClient *http.Client, secretProvider base.SecretProvider, game base.IGame, protobufChecker func(gameID int64) bool) *APIClient {
	if httpClient == nil {
		httpClient = DefaultHTTPClient()
	}
	return &APIClient{
		http:            httpClient,
		secret:          secretProvider,
		game:            game,
		protobufChecker: protobufChecker,
	}
}

func (c *APIClient) requireProtobuf(gameID int64) bool {
	if c.protobufChecker != nil && c.protobufChecker(gameID) {
		return true
	}
	if c.game != nil && c.game.GetProtobufConverter() != nil {
		return true
	}
	return false
}

type APIError struct {
	Op   string
	Code int
	Msg  string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s error: %d %s", e.Op, e.Code, e.Msg)
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

func (c *APIClient) request(ctx context.Context, method, apiURL string, body any, token string, sign bool, cfg *v1.TaskConfig) (*apiResponse, error) {
	var bodyReader io.Reader
	if body != nil {
		buf := jsonBufferPool.Get().(*bytes.Buffer)
		defer func() {
			buf.Reset()
			jsonBufferPool.Put(buf)
		}()

		encoder := jsoniter.NewEncoder(buf)
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
	req.Header.Set("Connection", "keep-alive") // 明确设置 keep-alive
	if token != "" && !strings.Contains(apiURL, "/v1/game/launch") {
		req.Header.Set("x-token", token)
	}

	if sign && cfg != nil {
		if lp, ok := body.(launchParams); ok {
			if c.secret == nil {
				return nil, errors.New("sign_required=true but secret provider is nil")
			}
			secret, ok := c.secret(cfg.Merchant)
			if !ok || secret == "" {
				return nil, fmt.Errorf("sign_required=true but no secret for merchant=%s", cfg.Merchant)
			}
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
	if err := jsoniter.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *APIClient) Launch(ctx context.Context, cfg *v1.TaskConfig, member string) (string, error) {
	params := launchParams{
		GameID:    cfg.GameId,
		Merchant:  cfg.Merchant,
		Member:    member,
		Timestamp: time.Now().Unix(),
	}

	launchUrl := strings.TrimRight(cfg.LaunchUrl, "/")
	apiURL := fmt.Sprintf("%s/v1/game/launch", launchUrl)

	res, err := c.request(ctx, http.MethodPost, apiURL, params, "", cfg.SignRequired, cfg)
	if err != nil {
		return "", err
	}
	if res.Code != 0 {
		return "", &APIError{Op: "launch", Code: res.Code, Msg: res.Msg}
	}

	var data struct {
		LaunchUrl string `json:"launchUrl"`
	}
	decoder := jsoniter.NewDecoder(bytes.NewReader(res.Data))
	_ = decoder.Decode(&data)

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
	apiURL := fmt.Sprintf("%s/api/member/login", strings.TrimRight(cfg.ApiUrl, "/"))
	res, err := c.request(ctx, http.MethodPost, apiURL, map[string]any{"token": token}, "", false, nil)
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
	decoder := jsoniter.NewDecoder(bytes.NewReader(res.Data))
	_ = decoder.Decode(&data)

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

func (c *APIClient) getConverter(cfg *v1.TaskConfig) (base.ProtobufConverter, error) {
	if c.game == nil {
		return nil, fmt.Errorf("no game instance, game=%d", cfg.GameId)
	}

	if cached := c.protoConverterCache.Load(); cached != nil {
		if conv, ok := cached.(base.ProtobufConverter); ok {
			return conv, nil
		}
	}

	conv := c.game.GetProtobufConverter()
	if conv == nil {
		return nil, fmt.Errorf("no converter for game %d", cfg.GameId)
	}

	c.protoConverterCache.Store(conv)
	return conv, nil
}

func (c *APIClient) decodeProtobuf(cfg *v1.TaskConfig, bytesData string) (map[string]any, error) {
	conv, err := c.getConverter(cfg)
	if err != nil {
		return nil, err
	}

	bytesTrimmed := strings.TrimSpace(bytesData)
	if bytesTrimmed == "" {
		return nil, fmt.Errorf("betorder api response bytes is empty for game %d", cfg.GameId)
	}

	protoBytes, err := base64.StdEncoding.DecodeString(bytesTrimmed)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 bytes: %v", err)
	}

	result, err := conv(protoBytes)
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

	apiURL := fmt.Sprintf("%s/api/game/betorder", strings.TrimRight(cfg.ApiUrl, "/"))
	res, err := c.request(ctx, http.MethodPost, apiURL, params, token, false, nil)
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

	needPB := c.requireProtobuf(cfg.GameId)
	if needPB {
		return c.decodeProtobuf(cfg, res.Bytes)
	}

	if strings.TrimSpace(res.Bytes) != "" {
		return c.decodeProtobuf(cfg, res.Bytes)
	}

	var data map[string]any
	decoder := jsoniter.NewDecoder(bytes.NewReader(res.Data))
	_ = decoder.Decode(&data)
	return data, nil
}

type BetBonusResult struct {
	Data         map[string]any
	NeedContinue bool
}

func (c *APIClient) BetBonus(ctx context.Context, cfg *v1.TaskConfig, token string, bonusNum int64) (*BetBonusResult, error) {
	params := map[string]any{"gameId": cfg.GameId, "bonusNum": bonusNum}
	apiURL := fmt.Sprintf("%s/api/game/betbonus", strings.TrimRight(cfg.ApiUrl, "/"))
	res, err := c.request(ctx, http.MethodPost, apiURL, params, token, false, nil)
	if err != nil {
		return nil, err
	}
	if res.Code != 0 {
		return nil, fmt.Errorf("betbonus error: %d %s", res.Code, res.Msg)
	}

	var data map[string]any
	decoder := jsoniter.NewDecoder(bytes.NewReader(res.Data))
	_ = decoder.Decode(&data)
	result := &BetBonusResult{Data: data}
	if c.game != nil {
		if bi := c.game.AsBonusInterface(); bi != nil {
			result.NeedContinue = bi.BonusNextState(data)
		}
	}
	return result, nil
}
