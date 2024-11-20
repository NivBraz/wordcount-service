package fetcher

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name   string
		config FetcherConfig
	}{
		{
			name: "Default Configuration",
			config: FetcherConfig{
				RequestsPerSecond: 1,
				Burst:             1,
				Timeout:           30 * time.Second,
			},
		},
		{
			name: "Custom Configuration",
			config: FetcherConfig{
				RequestsPerSecond:  5,
				Burst:              3,
				MinRequestInterval: 100 * time.Millisecond,
				MaxRequestInterval: 1 * time.Second,
				Timeout:            10 * time.Second,
				MaxRetries:         3,
				InitialBackoff:     2 * time.Second,
				MaxBackoff:         60 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := New(tt.config)
			if f == nil {
				t.Error("New() returned nil")
			}
			if f.client == nil {
				t.Error("HTTP client is nil")
			}
			if f.limiter == nil {
				t.Error("Rate limiter is nil")
			}
			if len(f.userAgents) == 0 {
				t.Error("User agents list is empty")
			}
		})
	}
}

func TestRotateUserAgent(t *testing.T) {
	f := New(FetcherConfig{})
	_ = f.rotateUserAgent()

	// Test that we rotate through all user agents
	seen := make(map[string]bool)
	for i := 0; i < len(defaultUserAgents)*2; i++ {
		ua := f.rotateUserAgent()
		if ua == "" {
			t.Error("Got empty user agent")
		}
		seen[ua] = true
	}

	// Verify we've seen all user agents
	if len(seen) != len(defaultUserAgents) {
		t.Errorf("Expected to see %d unique user agents, got %d", len(defaultUserAgents), len(seen))
	}
}

func TestCalculateBackoff(t *testing.T) {
	config := FetcherConfig{
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     30 * time.Second,
	}
	f := New(config)

	tests := []struct {
		name        string
		attempt     int
		minExpected time.Duration
		maxExpected time.Duration
	}{
		{
			name:        "First Attempt",
			attempt:     0,
			minExpected: 800 * time.Millisecond,  // 80% of 1s
			maxExpected: 1400 * time.Millisecond, // 140% of 1s
		},
		{
			name:        "Second Attempt",
			attempt:     1,
			minExpected: 1600 * time.Millisecond, // 80% of 2s
			maxExpected: 2800 * time.Millisecond, // 140% of 2s
		},
		{
			name:        "Max Backoff",
			attempt:     10,               // Should hit max
			minExpected: 24 * time.Second, // 80% of max
			maxExpected: 42 * time.Second, // 140% of max
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run multiple times to account for randomness
			for i := 0; i < 10; i++ {
				backoff := f.calculateBackoff(tt.attempt)
				if backoff < tt.minExpected || backoff > tt.maxExpected {
					t.Errorf("Expected backoff between %v and %v, got %v",
						tt.minExpected, tt.maxExpected, backoff)
				}
			}
		})
	}
}

func TestFetch(t *testing.T) {
	tests := []struct {
		name          string
		statusCodes   []int
		responseBody  string
		expectedError bool
		config        FetcherConfig
	}{
		{
			name:          "Successful Request",
			statusCodes:   []int{http.StatusOK},
			responseBody:  "Hello, World!",
			expectedError: false,
			config: FetcherConfig{
				MaxRetries:        1,
				RequestsPerSecond: 10,
				Burst:             5,
				InitialBackoff:    100 * time.Millisecond,
			},
		},
		{
			name:          "Rate Limited Then Success",
			statusCodes:   []int{http.StatusTooManyRequests, http.StatusOK},
			responseBody:  "Success after retry",
			expectedError: false,
			config: FetcherConfig{
				MaxRetries:        2,
				RequestsPerSecond: 10,
				Burst:             5,
				InitialBackoff:    100 * time.Millisecond,
			},
		},
		{
			name:          "All Requests Fail",
			statusCodes:   []int{http.StatusInternalServerError, http.StatusInternalServerError},
			responseBody:  "Error",
			expectedError: true,
			config: FetcherConfig{
				MaxRetries:        1,
				RequestsPerSecond: 10,
				Burst:             5,
				InitialBackoff:    100 * time.Millisecond,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			currentResponse := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify headers
				if ua := r.Header.Get("User-Agent"); ua == "" {
					t.Error("User-Agent header not set")
				}
				if accept := r.Header.Get("Accept"); accept == "" {
					t.Error("Accept header not set")
				}

				// Return configured status code and increment counter
				w.WriteHeader(tt.statusCodes[currentResponse])
				if tt.statusCodes[currentResponse] == http.StatusOK {
					io.WriteString(w, tt.responseBody)
				}
				if currentResponse < len(tt.statusCodes)-1 {
					currentResponse++
				}
			}))
			defer server.Close()

			f := New(tt.config)
			ctx := context.Background()
			body, err := f.Fetch(ctx, server.URL)

			if tt.expectedError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if string(body) != tt.responseBody {
					t.Errorf("Expected body %q, got %q", tt.responseBody, string(body))
				}
			}
		})
	}
}

func TestBasicFetch(t *testing.T) {
	tests := []struct {
		name          string
		responseBody  string
		statusCode    int
		expectedError bool
	}{
		{
			name:          "Successful Request",
			responseBody:  "Basic fetch response",
			statusCode:    http.StatusOK,
			expectedError: false,
		},
		{
			name:          "Server Error",
			responseBody:  "Internal Server Error",
			statusCode:    http.StatusInternalServerError,
			expectedError: false, // BasicFetch doesn't check status codes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				io.WriteString(w, tt.responseBody)
			}))
			defer server.Close()

			f := New(FetcherConfig{})
			ctx := context.Background()
			body, err := f.BasicFetch(ctx, server.URL)

			if tt.expectedError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if string(body) != tt.responseBody {
					t.Errorf("Expected body %q, got %q", tt.responseBody, string(body))
				}
			}
		})
	}
}

func TestFetchWithContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond) // Increased delay
		io.WriteString(w, "Delayed response")
	}))
	defer server.Close()

	f := New(FetcherConfig{
		RequestsPerSecond: 10,
		Burst:             5,
		InitialBackoff:    100 * time.Millisecond,
	})

	// Test context cancellation
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond) // Shorter timeout
	defer cancel()

	_, err := f.Fetch(ctx, server.URL)
	if err == nil || !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("Expected context deadline exceeded error, got: %v", err)
	}
}
