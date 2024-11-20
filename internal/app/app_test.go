package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/NivBraz/wordcount-service/internal/config"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: &config.Config{
				RateLimit: struct {
					RequestsPerSecond int "yaml:\"requestsPerSecond\""
					Burst             int "yaml:\"burst\""
				}{
					RequestsPerSecond: 5,
					Burst:             10,
				},
				Concurrency: 4,
				URLs: struct {
					ArticleURLsFile string "yaml:\"articleURLsFile\""
					WordBankURL     string "yaml:\"wordBankURL\""
				}{
					WordBankURL: "https://raw.githubusercontent.com/dwyl/english-words/master/words.txt",
				},
				HTTPClient: struct {
					Timeout    int    "yaml:\"timeout\""
					MaxRetries int    "yaml:\"maxRetries\""
					RetryDelay int    "yaml:\"retryDelay\""
					UserAgent  string "yaml:\"userAgent\""
				}{
					Timeout:   30,
					UserAgent: "test-agent",
				},
				ArticleURLs: []string{"http://example.com/article1"},
			},
			wantErr: false,
		},
		{
			name: "invalid config - missing word bank URL",
			cfg: &config.Config{
				RateLimit: struct {
					RequestsPerSecond int "yaml:\"requestsPerSecond\""
					Burst             int "yaml:\"burst\""
				}{
					RequestsPerSecond: 5,
					Burst:             10,
				},
				Concurrency: 4,
				URLs: struct {
					ArticleURLsFile string "yaml:\"articleURLsFile\""
					WordBankURL     string "yaml:\"wordBankURL\""
				}{
					WordBankURL: "",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsValidWord(t *testing.T) {
	tests := []struct {
		name string
		word string
		want bool
	}{
		{"valid word", "test", true},
		{"short word", "ab", false},
		{"with numbers", "test123", false},
		{"with special chars", "test!", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidWord(tt.word); got != tt.want {
				t.Errorf("isValidWord() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApp_Run(t *testing.T) {
	// Create a test server that serves a simple word bank and article
	wordBank := `test
word
bank
common`

	article := `This is a test article with some common words.
The test contains multiple test words that should be counted.
Some words appear more frequently than others in this test.`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/wordbank":
			w.Write([]byte(wordBank))
		case "/article":
			w.Write([]byte(article))
		}
	}))
	defer server.Close()

	// Create test configuration
	cfg := &config.Config{
		RateLimit: struct {
			RequestsPerSecond int "yaml:\"requestsPerSecond\""
			Burst             int "yaml:\"burst\""
		}{
			RequestsPerSecond: 10,
			Burst:             20,
		},
		Concurrency: 4,
		URLs: struct {
			ArticleURLsFile string "yaml:\"articleURLsFile\""
			WordBankURL     string "yaml:\"wordBankURL\""
		}{
			WordBankURL: server.URL + "/wordbank",
		},
		HTTPClient: struct {
			Timeout    int    "yaml:\"timeout\""
			MaxRetries int    "yaml:\"maxRetries\""
			RetryDelay int    "yaml:\"retryDelay\""
			UserAgent  string "yaml:\"userAgent\""
		}{
			Timeout:   30,
			UserAgent: "test-agent",
		},
		ArticleURLs: []string{server.URL + "/article"},
	}

	// Create and run the application
	app, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := app.Run(ctx)
	if err != nil {
		t.Fatalf("Failed to run app: %v", err)
	}

	// Verify results
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Check if "test" is one of the top words (it appears 4 times)
	foundTest := false
	for _, wc := range result.TopWords {
		if wc.Word == "test" {
			foundTest = true
			if wc.Count != 4 {
				t.Errorf("Expected word 'test' to appear 4 times, got %d", wc.Count)
			}
			break
		}
	}

	if !foundTest {
		t.Error("Expected 'test' to be in top words")
	}
}
