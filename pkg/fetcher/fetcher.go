// pkg/fetcher/fetcher.go
package fetcher

import (
	"context"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type Fetcher struct {
	client          *http.Client
	limiter         *rate.Limiter
	config          FetcherConfig
	lastRequestTime time.Time
	mu              sync.Mutex
	userAgents      []string
	currentUAIndex  int
}

type FetcherConfig struct {
	RequestsPerSecond  int
	Burst              int
	MinRequestInterval time.Duration
	MaxRequestInterval time.Duration
	Timeout            time.Duration
	UserAgent          string
	MaxRetries         int
	InitialBackoff     time.Duration
	MaxBackoff         time.Duration
}

var defaultUserAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:89.0) Gecko/20100101 Firefox/89.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.1.1 Safari/605.1.15",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Edge/91.0.864.59",
}

func New(config FetcherConfig) *Fetcher {
	if config.MaxRetries == 0 {
		config.MaxRetries = 1
	}
	if config.InitialBackoff == 0 {
		config.InitialBackoff = 1 * time.Second
	}
	if config.MaxBackoff == 0 {
		config.MaxBackoff = 30 * time.Second
	}

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
		userAgents:      defaultUserAgents,
	}
}

func (f *Fetcher) rotateUserAgent() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.currentUAIndex = (f.currentUAIndex + 1) % len(f.userAgents)
	return f.userAgents[f.currentUAIndex]
}

func (f *Fetcher) addRandomDelay() {
	jitter := time.Duration(rand.Float64() * float64(500*time.Millisecond))
	time.Sleep(jitter)
}

func (f *Fetcher) calculateBackoff(attempt int) time.Duration {
	backoff := float64(f.config.InitialBackoff)
	max := float64(f.config.MaxBackoff)
	calculated := math.Min(backoff*math.Pow(2, float64(attempt)), max)

	// Add jitter (±20%)
	jitter := calculated * (0.8 + rand.Float64()*0.4)
	return time.Duration(jitter)
}

func (f *Fetcher) BasicFetch(ctx context.Context, urlStr string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching URL: %w", err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}
	return body, nil
}

func (f *Fetcher) Fetch(ctx context.Context, urlStr string) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt <= f.config.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := f.calculateBackoff(attempt - 1)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		// Wait for rate limiter
		if err := f.limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limiter error: %w", err)
		}

		f.addRandomDelay()

		parsedURL, err := url.Parse(urlStr)
		if err != nil {
			return nil, fmt.Errorf("invalid URL: %w", err)
		}

		// Set up cookies
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

		if strings.Contains(parsedURL.Host, "engadget.com") {
			f.client.Jar.SetCookies(parsedURL, append(commonCookies, engadgetCookies...))
		} else if strings.Contains(parsedURL.Host, "yahoo.com") {
			f.client.Jar.SetCookies(parsedURL, append(commonCookies, yahooCookies...))
		}

		req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
		if err != nil {
			return nil, fmt.Errorf("error creating request: %w", err)
		}

		// Rotate and set User-Agent
		userAgent := f.rotateUserAgent()
		req.Header.Set("User-Agent", userAgent)

		// Set realistic browser headers
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		req.Header.Set("Connection", "keep-alive")
		req.Header.Set("Upgrade-Insecure-Requests", "1")
		req.Header.Set("Sec-Fetch-Dest", "document")
		req.Header.Set("Sec-Fetch-Mode", "navigate")
		req.Header.Set("Sec-Fetch-Site", "none")
		req.Header.Set("Sec-Fetch-User", "?1")
		req.Header.Set("Cache-Control", "max-age=0")

		resp, err := f.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("error fetching URL (attempt %d): %w", attempt+1, err)
			continue
		}

		// Handle different status codes
		switch resp.StatusCode {
		case http.StatusOK:
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				lastErr = fmt.Errorf("error reading response body: %w", err)
				continue
			}
			return body, nil

		case http.StatusTooManyRequests, 999: // Rate limit cases
			resp.Body.Close()
			if attempt == f.config.MaxRetries {
				return nil, fmt.Errorf("rate limit exceeded after %d retries", attempt+1)
			}
			lastErr = fmt.Errorf("rate limit exceeded (status %d), retrying...", resp.StatusCode)
			continue

		default:
			resp.Body.Close()
			if attempt == f.config.MaxRetries {
				return nil, fmt.Errorf("unexpected status code %d after %d retries", resp.StatusCode, attempt+1)
			}
			lastErr = fmt.Errorf("unexpected status code: %d, retrying...", resp.StatusCode)
			continue
		}
	}

	return nil, lastErr
}
