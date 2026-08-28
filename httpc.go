package main

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

type Fetcher struct {
	cli  *http.Client
	cfg  *Config
	last time.Time
}

func NewFetcher(cfg *Config) *Fetcher {
	return &Fetcher{
		cli: &http.Client{
			Timeout: time.Duration(cfg.TimeoutSec) * time.Second,
		},
		cfg: cfg,
	}
}

// throttle は同時1接続・間隔ジッター付きで待機する。
// 相手サーバに余計な負荷をかけないための下限であり、短くしないこと。
func (f *Fetcher) throttle() {
	min := f.cfg.MinDelayMS
	max := f.cfg.MaxDelayMS
	if max < min {
		max = min
	}
	wait := time.Duration(min+rand.Intn(max-min+1)) * time.Millisecond
	elapsed := time.Since(f.last)
	if elapsed < wait {
		time.Sleep(wait - elapsed)
	}
	f.last = time.Now()
}

// Get は1ページ取得する。429/503 は Retry-After を尊重して指数バックオフ。
func (f *Fetcher) Get(url string) (string, error) {
	var lastErr error
	backoff := 5 * time.Second

	for attempt := 0; attempt <= f.cfg.MaxRetries; attempt++ {
		f.throttle()

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", f.cfg.UserAgent)
		req.Header.Set("Accept", "text/html,application/xhtml+xml")
		req.Header.Set("Accept-Language", "ja,en-US;q=0.9")

		resp, err := f.cli.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(backoff)
			backoff *= 2
			continue
		}

		switch {
		case resp.StatusCode == 200:
			b, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				return "", err
			}
			return string(b), nil

		case resp.StatusCode == 429 || resp.StatusCode >= 500:
			ra := resp.Header.Get("Retry-After")
			resp.Body.Close()
			wait := backoff
			if ra != "" {
				if s, e := strconv.Atoi(ra); e == nil {
					wait = time.Duration(s) * time.Second
				}
			}
			fmt.Printf("  [warn] HTTP %d — %v 待機して再試行 (%d/%d)\n",
				resp.StatusCode, wait, attempt+1, f.cfg.MaxRetries)
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			time.Sleep(wait)
			backoff *= 2

		case resp.StatusCode == 404:
			resp.Body.Close()
			return "", ErrNotFound

		default:
			resp.Body.Close()
			return "", fmt.Errorf("HTTP %d", resp.StatusCode)
		}
	}
	return "", fmt.Errorf("再試行上限に到達: %w", lastErr)
}

var ErrNotFound = fmt.Errorf("404 not found")
