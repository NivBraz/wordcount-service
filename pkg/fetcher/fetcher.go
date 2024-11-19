package fetcher

import (
	"context"
	"io"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

type Fetcher struct {
	client  *http.Client
	limiter *rate.Limiter
}

func New(requestsPerSecond, burst int) *Fetcher {
	return &Fetcher{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		limiter: rate.NewLimiter(rate.Limit(requestsPerSecond), burst),
	}
}

func (f *Fetcher) Fetch(ctx context.Context, url string) ([]byte, error) {
	err := f.limiter.Wait(ctx)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}
