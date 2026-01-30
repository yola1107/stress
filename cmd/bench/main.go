package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type requestPayload struct {
	Config map[string]any `json:"config"`
}

type options struct {
	baseURL        string
	memberCount    int
	timesPerMember int
	baseMoney      float64
	multiple       int
	purchase       int
	ids            []int64
	concurrency    int
	betSizes       map[int64][]float64
}

func main() {
	baseURL := flag.String("base-url", "http://192.168.10.72:8001", "")
	memberCount := flag.Int("member-count", 1000, "")
	timesPerMember := flag.Int("times-per-member", 5000, "")
	baseMoney := flag.Float64("base-money", 0.1, "")
	multiple := flag.Int("multiple", 1, "")
	purchase := flag.Int("purchase", 0, "")
	concurrency := flag.Int("concurrency", 6, "")
	flag.Parse()

	opts := options{
		baseURL:        *baseURL,
		memberCount:    *memberCount,
		timesPerMember: *timesPerMember,
		baseMoney:      *baseMoney,
		multiple:       *multiple,
		purchase:       *purchase,
		concurrency:    *concurrency,
	}
	if opts.concurrency < 1 {
		opts.concurrency = 1
	}

	endpoint := strings.TrimRight(opts.baseURL, "/") + "/stress/CreateTask"
	client := &http.Client{Timeout: 30 * time.Second}

	betSizes, listErr := fetchGameBetSizes(client, opts.baseURL)
	if listErr != nil {
		fmt.Printf("fetch list games failed: %v\n", listErr)
	}
	if len(betSizes) == 0 {
		fmt.Println("no game ids found")
		return
	}
	opts.ids = make([]int64, 0, len(betSizes))
	for id := range betSizes {
		opts.ids = append(opts.ids, id)
	}
	sort.Slice(opts.ids, func(i, j int) bool { return opts.ids[i] < opts.ids[j] })
	if len(opts.ids) > 10 {
		opts.ids = opts.ids[:10]
	}
	opts.betSizes = betSizes

	runConcurrent(client, endpoint, opts)
}

func buildPayload(gameID int64, opts options) requestPayload {
	baseMoney := opts.baseMoney
	if sizes, ok := opts.betSizes[gameID]; ok {
		if picked := pickBaseMoney(sizes); picked > 0 {
			baseMoney = picked
		}
	}
	config := map[string]any{
		"game_id":          gameID,
		"member_count":     opts.memberCount,
		"times_per_member": opts.timesPerMember,
		"bet_order": map[string]any{
			"base_money": baseMoney,
			"multiple":   opts.multiple,
			"purchase":   opts.purchase,
		},
	}
	if needBetBonus(gameID) {
		config["betBonus"] = map[string]any{
			"enable":        true,
			"bonusNum":      0,
			"randomNums":    []int64{},
			"bonusSequence": []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
		}
	}
	return requestPayload{Config: config}
}

func needBetBonus(gameID int64) bool {
	switch gameID {
	case 18931, 18930, 18902:
		return true
	default:
		return false
	}
}

func runConcurrent(client *http.Client, endpoint string, opts options) {
	jobs := make(chan int64)
	var wg sync.WaitGroup
	for i := 0; i < opts.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				payload := buildPayload(id, opts)
				if err := postJSON(client, endpoint, payload); err != nil {
					fmt.Printf("game %d request failed: %v\n", id, err)
					continue
				}
				fmt.Printf("game %d request ok\n", id)
			}
		}()
	}
	for _, id := range opts.ids {
		jobs <- id
	}
	close(jobs)
	wg.Wait()
}

func fetchGameBetSizes(client *http.Client, baseURL string) (map[int64][]float64, error) {
	endpoint := strings.TrimRight(baseURL, "/") + "/stress/ListGames"
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader([]byte(`{}`)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	var data struct {
		Games []struct {
			GameID  string    `json:"gameId"`
			BetSize []float64 `json:"betSize"`
		} `json:"games"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	out := make(map[int64][]float64, len(data.Games))
	for _, g := range data.Games {
		id, err := strconv.ParseInt(g.GameID, 10, 64)
		if err != nil {
			continue
		}
		out[id] = g.BetSize
	}
	return out, nil
}

func pickBaseMoney(betSizes []float64) float64 {
	if len(betSizes) >= 2 {
		return betSizes[1]
	}
	if len(betSizes) == 1 {
		return betSizes[0]
	}
	return 0
}

func postJSON(client *http.Client, url string, payload requestPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}
