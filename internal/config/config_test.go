package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	// Create a temporary config file
	content := `rateLimit:
  requestsPerSecond: 4
  burst: 4
concurrency: 4
urls:
  articleURLsFile: "endg-urls"
  wordBankURL: "https://raw.githubusercontent.com/dwyl/english-words/master/words.txt"
httpClient:
  timeout: 30
  maxRetries: 3
  retryDelay: 5
  userAgent: "WordCount-Service/1.0"
output:
  topWordsCount: 10
  includeStats: true
  format: "json"
  prettyPrint: true
wordProcessing:
  minWordLength: 3
  convertToLower: true
  removeSpecialChars: true`

	if err := os.WriteFile("config.yaml", []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}
	defer os.Remove("config.yaml")

	// Create a temporary URLs file with actual Engadget URLs
	urlsContent := `https://www.engadget.com/2019/08/25/sony-and-yamaha-sc-1-sociable-cart/
https://www.engadget.com/2019/08/24/trump-tries-to-overturn-ruling-stopping-him-from-blocking-twitte/
https://www.engadget.com/2019/08/24/crime-allegation-in-space/`

	if err := os.WriteFile("endg-urls", []byte(urlsContent), 0644); err != nil {
		t.Fatalf("Failed to create test URLs file: %v", err)
	}
	defer os.Remove("endg-urls")

	// Test loading configuration
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify loaded values
	if cfg.RateLimit.RequestsPerSecond != 4 {
		t.Errorf("Expected RequestsPerSecond = 4, got %d", cfg.RateLimit.RequestsPerSecond)
	}
	if cfg.RateLimit.Burst != 4 {
		t.Errorf("Expected Burst = 4, got %d", cfg.RateLimit.Burst)
	}
	if cfg.Concurrency != 4 {
		t.Errorf("Expected Concurrency = 4, got %d", cfg.Concurrency)
	}
	if cfg.URLs.WordBankURL != "https://raw.githubusercontent.com/dwyl/english-words/master/words.txt" {
		t.Errorf("Expected WordBankURL = https://raw.githubusercontent.com/dwyl/english-words/master/words.txt, got %s", cfg.URLs.WordBankURL)
	}
	if len(cfg.ArticleURLs) != 3 {
		t.Errorf("Expected 3 article URLs, got %d", len(cfg.ArticleURLs))
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: &Config{
				RateLimit: struct {
					RequestsPerSecond int "yaml:\"requestsPerSecond\""
					Burst             int "yaml:\"burst\""
				}{
					RequestsPerSecond: 4,
					Burst:             4,
				},
				Concurrency: 4,
				URLs: struct {
					ArticleURLsFile string "yaml:\"articleURLsFile\""
					WordBankURL     string "yaml:\"wordBankURL\""
				}{
					WordBankURL: "https://raw.githubusercontent.com/dwyl/english-words/master/words.txt",
				},
				ArticleURLs: []string{"https://www.engadget.com/article1"},
			},
			wantErr: false,
		},
		{
			name: "missing word bank URL",
			cfg: &Config{
				RateLimit: struct {
					RequestsPerSecond int "yaml:\"requestsPerSecond\""
					Burst             int "yaml:\"burst\""
				}{
					RequestsPerSecond: 4,
					Burst:             4,
				},
				Concurrency: 4,
				URLs: struct {
					ArticleURLsFile string "yaml:\"articleURLsFile\""
					WordBankURL     string "yaml:\"wordBankURL\""
				}{
					WordBankURL: "",
				},
				ArticleURLs: []string{"https://www.engadget.com/article1"},
			},
			wantErr: true,
		},
		{
			name: "no article URLs",
			cfg: &Config{
				RateLimit: struct {
					RequestsPerSecond int "yaml:\"requestsPerSecond\""
					Burst             int "yaml:\"burst\""
				}{
					RequestsPerSecond: 4,
					Burst:             4,
				},
				Concurrency: 4,
				URLs: struct {
					ArticleURLsFile string "yaml:\"articleURLsFile\""
					WordBankURL     string "yaml:\"wordBankURL\""
				}{
					WordBankURL: "https://raw.githubusercontent.com/dwyl/english-words/master/words.txt",
				},
				ArticleURLs: []string{},
			},
			wantErr: true,
		},
		{
			name: "invalid rate limit",
			cfg: &Config{
				RateLimit: struct {
					RequestsPerSecond int "yaml:\"requestsPerSecond\""
					Burst             int "yaml:\"burst\""
				}{
					RequestsPerSecond: 0,
					Burst:             4,
				},
				Concurrency: 4,
				URLs: struct {
					ArticleURLsFile string "yaml:\"articleURLsFile\""
					WordBankURL     string "yaml:\"wordBankURL\""
				}{
					WordBankURL: "https://raw.githubusercontent.com/dwyl/english-words/master/words.txt",
				},
				ArticleURLs: []string{"https://www.engadget.com/article1"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
