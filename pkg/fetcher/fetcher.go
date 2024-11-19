// pkg/fetcher/fetcher.go
package fetcher

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type GeonodeProxy struct {
	IP             string   `json:"ip"`
	Port           string   `json:"port"`
	Protocols      []string `json:"protocols"`
	UpTime         float64  `json:"upTime"`
	Speed          int      `json:"speed"`
	AnonymityLevel string   `json:"anonymityLevel"` // Changed from bool to string
	LastChecked    float64  `json:"lastChecked"`
}

type GeonodeResponse struct {
	Data []GeonodeProxy `json:"data"`
}

type Fetcher struct {
	client          *http.Client
	limiter         *rate.Limiter
	config          FetcherConfig
	lastRequestTime time.Time
	mu              sync.Mutex
	userAgents      []string
	currentUAIndex  int
	proxyList       []string
	lastProxyUpdate time.Time
}

type FetcherConfig struct {
	RequestsPerSecond    int
	Burst                int
	MinRequestInterval   time.Duration
	MaxRequestInterval   time.Duration
	Timeout              time.Duration
	UserAgent            string
	MaxRetries           int
	InitialBackoff       time.Duration
	MaxBackoff           time.Duration
	ProxyRefreshInterval time.Duration // How often to refresh proxy list
}

var defaultUserAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:89.0) Gecko/20100101 Firefox/89.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.1.1 Safari/605.1.15",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Edge/91.0.864.59",
}

func New(config FetcherConfig) (*Fetcher, error) {
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.InitialBackoff == 0 {
		config.InitialBackoff = 1 * time.Second
	}
	if config.MaxBackoff == 0 {
		config.MaxBackoff = 30 * time.Second
	}
	if config.ProxyRefreshInterval == 0 {
		config.ProxyRefreshInterval = 1 * time.Minute
	}

	jar, _ := cookiejar.New(nil)
	f := &Fetcher{
		client: &http.Client{
			Timeout: config.Timeout,
			Jar:     jar,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return nil // Allow all redirects
			},
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		limiter:         rate.NewLimiter(rate.Limit(config.RequestsPerSecond), config.Burst),
		config:          config,
		lastRequestTime: time.Now().Add(-10 * time.Second),
		userAgents:      defaultUserAgents,
		proxyList:       make([]string, 0),
	}

	// Initial proxy refresh
	if err := f.refreshProxyList(); err != nil {
		return nil, fmt.Errorf("failed to initialize proxy list: %w", err)
	}

	return f, nil
}

func (f *Fetcher) refreshProxyList() error {
	if time.Since(f.lastProxyUpdate) < f.config.ProxyRefreshInterval {
		return nil
	}

	proxyClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
			DisableKeepAlives:   true,
		},
	}

	resp, err := proxyClient.Get("https://proxylist.geonode.com/api/proxy-list?limit=100&page=1&sort_by=lastChecked&sort_type=desc&protocols=http%2Chttps&filterUpTime=90")
	if err != nil {
		return fmt.Errorf("failed to fetch proxy list: %w", err)
	}
	defer resp.Body.Close()

	var geonodeResp GeonodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&geonodeResp); err != nil {
		return fmt.Errorf("failed to decode proxy list: %w", err)
	}

	var validProxies []string
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Validate proxies concurrently
	semaphore := make(chan struct{}, 10) // Limit concurrent validations
	for _, p := range geonodeResp.Data {
		if len(p.Protocols) == 0 {
			continue
		}

		wg.Add(1)
		go func(proxy GeonodeProxy) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire semaphore
			defer func() { <-semaphore }() // Release semaphore

			proxyURL := fmt.Sprintf("%s://%s:%s", proxy.Protocols[0], proxy.IP, proxy.Port)
			if f.validateProxy(proxyURL) {
				mu.Lock()
				validProxies = append(validProxies, proxyURL)
				mu.Unlock()
			}
		}(p)
	}

	wg.Wait()

	if len(validProxies) == 0 {
		return fmt.Errorf("no valid proxies found")
	}

	f.mu.Lock()
	f.proxyList = validProxies
	f.lastProxyUpdate = time.Now()
	f.mu.Unlock()

	return nil
}

func (f *Fetcher) updateTransport(proxyURL string) error {
	transport := &http.Transport{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableKeepAlives:     true, // Don't reuse connections
		ResponseHeaderTimeout: 30 * time.Second,
		// Add timeouts to prevent hanging connections
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}

	if proxyURL != "" {
		proxyURLParsed, err := url.Parse(proxyURL)
		if err != nil {
			return fmt.Errorf("invalid proxy URL: %w", err)
		}

		transport.Proxy = http.ProxyURL(proxyURLParsed)
		// Add proxy-specific timeouts
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true, // Sometimes needed for proxies
		}
	}

	f.client.Transport = transport
	return nil
}

func (f *Fetcher) validateProxy(proxyURL string) bool {
	testClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Proxy: func(_ *http.Request) (*url.URL, error) {
				return url.Parse(proxyURL)
			},
			DisableKeepAlives: true,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			DialContext: (&net.Dialer{
				Timeout: 5 * time.Second,
			}).DialContext,
		},
	}

	req, err := http.NewRequest("GET", "https://api.ipify.org", nil)
	if err != nil {
		return false
	}

	resp, err := testClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

func (f *Fetcher) getNextProxy() string {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.proxyList) == 0 {
		return ""
	}

	// Get a random proxy from the list
	return f.proxyList[rand.Intn(len(f.proxyList))]
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

	// Refresh proxy list if needed
	if err := f.refreshProxyList(); err != nil {
		fmt.Printf("Warning: failed to refresh proxy list: %v\n", err)
	}

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

		proxy := f.getNextProxy()
		if err := f.updateTransport(proxy); err != nil {
			continue // Try next proxy if this one fails
		}

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
			// If we get EOF or timeout, try another proxy
			if strings.Contains(err.Error(), "EOF") ||
				strings.Contains(err.Error(), "timeout") ||
				strings.Contains(err.Error(), "connection refused") ||
				strings.Contains(err.Error(), "no such host") {
				continue
			}
			lastErr = fmt.Errorf("request error (attempt %d): %w", attempt+1, err)
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
