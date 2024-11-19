// pkg/fetcher/fetcher.go
package fetcher

import (
	"context"
	"fmt"
	"golang.org/x/time/rate"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Fetcher struct {
	client          *http.Client
	limiter         *rate.Limiter
	config          FetcherConfig
	lastRequestTime time.Time
	mu              sync.Mutex
}

type FetcherConfig struct {
	RequestsPerSecond  int
	Burst              int
	MinRequestInterval time.Duration
	MaxRequestInterval time.Duration
	Timeout            time.Duration
	UserAgent          string
}

func NewFetcherConfig() FetcherConfig {
	return FetcherConfig{
		RequestsPerSecond:  2,
		Burst:              1,
		MinRequestInterval: 2 * time.Second,
		MaxRequestInterval: 5 * time.Second,
		Timeout:            30 * time.Second,
		UserAgent:          "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	}
}

func New(config FetcherConfig) *Fetcher {
	jar, _ := cookiejar.New(nil)
	return &Fetcher{
		client: &http.Client{
			Timeout: config.Timeout,
			Jar:     jar,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return nil // Allow all redirects
			},
		},
		limiter:         rate.NewLimiter(rate.Limit(config.RequestsPerSecond), config.Burst),
		config:          config,
		lastRequestTime: time.Now().Add(-10 * time.Second),
	}
}

func (f *Fetcher) Fetch(ctx context.Context, urlStr string) ([]byte, error) {
	// First, wait for the rate limiter
	if err := f.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter error: %w", err)
	}

	// Then, ensure minimum time between requests
	//f.mu.Lock()
	//interval := f.config.MinRequestInterval +
	//	time.Duration(rand.Float64()*float64(f.config.MaxRequestInterval-f.config.MinRequestInterval))
	//
	//timeSinceLastRequest := time.Since(f.lastRequestTime)
	//if timeSinceLastRequest < interval {
	//	waitTime := interval - timeSinceLastRequest
	//	f.mu.Unlock()
	//
	//	select {
	//	case <-ctx.Done():
	//		return nil, ctx.Err()
	//	case <-time.After(waitTime):
	//	}
	//} else {
	//	f.mu.Unlock()
	//}
	//
	//// Update last request time
	//f.mu.Lock()
	//f.lastRequestTime = time.Now()
	//f.mu.Unlock()

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Set up cookies for both domains
	engadgetCookies := []*http.Cookie{
		{
			Name:   "A1",
			Value:  "d=AQABBDaRPGcCECIr_BTsFjo-rt9hBQhVcjMFEgEBAQHiPWdGZ15ByyMA_eMAAA&S=AQAAAkGPCWY_SwOkFmp6nfvyuSs",
			Domain: ".engadget.com",
			Path:   "/",
		},
		{
			Name:   "A1S",
			Value:  "d=AQABBEeRPGcCEKEF7YNrzldMkTQGH51Ng8YFEgABCAHhPWdpZ15Ub2UBAiAAAAcINpE8Z_NTlpQ&S=AQAAAsFwKw09qbLuNohmBPdak3o",
			Domain: ".engadget.com",
			Path:   "/",
		},
	}

	yahooCookies := []*http.Cookie{
		{
			Name:   "A1",
			Value:  "d=AQABBDaRPGcCECIr_BTsFjo-rt9hBQhVcjMFEgEBAQHiPWdGZ15ByyMA_eMAAA&S=AQAAAkGPCWY_SwOkFmp6nfvyuSs",
			Domain: ".yahoo.com",
			Path:   "/",
		},
		{
			Name:   "A1S",
			Value:  "d=AQABBEeRPGcCEKEF7YNrzldMkTQGH51Ng8YFEgABCAHhPWdpZ15Ub2UBAiAAAAcINpE8Z_NTlpQ&S=AQAAAsFwKw09qbLuNohmBPdak3o",
			Domain: ".yahoo.com",
			Path:   "/",
		},
	}

	// Set common cookies
	commonCookies := []*http.Cookie{
		{
			Name:   "euconsent-v2",
			Value:  "CPyicIAPyicIAAHABBENCmCsAP_AAH_AAB6YJLNf_X__b2_r-_7_f_t0eY1P9_7__-0zjhfdl-8N3f_X_L8X52M7vF36tq4KuR4ku3bBIQdtHOncTUmx6olVryxPVk2_r93V-ww-9Y3v-_7___Z_3_v__97________7-3f3__5_3_--_e_V_99zbv9____39nP___9v-_9_34IrgakxLgA9kCAMNQhgAIEhWxJAKIAUBxQDCQGGsCSoKqKAEACgLRIYQAkmASCFyQICFBAMAkEAAACAQBIREBIAeCARAEQCAAEAKEBYAAQABAtCQsQCsqEsIEvlZAAuBDKS5YAAA",
			Domain: parsedURL.Host,
			Path:   "/",
		},
		{
			Name:   "guce",
			Value:  "1",
			Domain: parsedURL.Host,
			Path:   "/",
		},
		{
			Name:   "cookie_consent",
			Value:  "accepted",
			Domain: parsedURL.Host,
			Path:   "/",
		},
	}

	// Set all cookies for both domains
	if strings.Contains(parsedURL.Host, "engadget.com") {
		f.client.Jar.SetCookies(parsedURL, append(commonCookies, engadgetCookies...))
	} else if strings.Contains(parsedURL.Host, "yahoo.com") {
		f.client.Jar.SetCookies(parsedURL, append(commonCookies, yahooCookies...))
	}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Set headers to look more like a browser
	req.Header.Set("User-Agent", f.config.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Cache-Control", "max-age=0")

	// Perform the request
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching URL: %w", err)
	}
	defer resp.Body.Close()

	// Handle rate limiting responses
	if resp.StatusCode == 429 || resp.StatusCode == 999 {
		return nil, fmt.Errorf("rate limit exceeded (status %d), consider increasing delay between requests", resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
