// internal/config/config.go
package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

type Config struct {
	RateLimit struct {
		RequestsPerSecond int `yaml:"requestsPerSecond"`
		Burst             int `yaml:"burst"`
	} `yaml:"rateLimit"`

	Concurrency int `yaml:"concurrency"`

	URLs struct {
		ArticleURLsFile string `yaml:"articleURLsFile"`
		WordBankURL     string `yaml:"wordBankURL"`
	} `yaml:"urls"`

	HTTPClient struct {
		Timeout    int    `yaml:"timeout"`
		MaxRetries int    `yaml:"maxRetries"`
		RetryDelay int    `yaml:"retryDelay"`
		UserAgent  string `yaml:"userAgent"`
	} `yaml:"httpClient"`

	Output struct {
		TopWordsCount int    `yaml:"topWordsCount"`
		IncludeStats  bool   `yaml:"includeStats"`
		Format        string `yaml:"format"`
		PrettyPrint   bool   `yaml:"prettyPrint"`
	} `yaml:"output"`

	WordProcessing struct {
		MinWordLength      int  `yaml:"minWordLength"`
		ConvertToLower     bool `yaml:"convertToLower"`
		RemoveSpecialChars bool `yaml:"removeSpecialChars"`
	} `yaml:"wordProcessing"`

	// This will be populated from the file
	ArticleURLs []string `yaml:"-"`
}

// Load reads and parses the configuration
func Load() (*Config, error) {
	// Load YAML config
	f, err := os.Open("config.yaml")
	if err != nil {
		return nil, fmt.Errorf("error opening config file: %w", err)
	}
	defer f.Close()

	var cfg Config
	decoder := yaml.NewDecoder(f)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("error decoding config: %w", err)
	}

	// Load URLs from file
	urls, err := loadURLsFromFile(cfg.URLs.ArticleURLsFile)
	if err != nil {
		return nil, fmt.Errorf("error loading URLs from file: %w", err)
	}
	cfg.ArticleURLs = urls

	// Set default values
	setDefaults(&cfg)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// loadURLsFromFile reads URLs from the specified file
func loadURLsFromFile(filepath string) ([]string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("error opening URLs file: %w", err)
	}
	defer file.Close()

	var urls []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		// Get the line and trim spaces
		url := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if url != "" && !strings.HasPrefix(url, "#") {
			urls = append(urls, url)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading URLs file: %w", err)
	}

	if len(urls) == 0 {
		return nil, fmt.Errorf("no URLs found in file %s", filepath)
	}

	return urls, nil
}

// setDefaults sets default values for configuration
func setDefaults(cfg *Config) {
	if cfg.RateLimit.RequestsPerSecond == 0 {
		cfg.RateLimit.RequestsPerSecond = 5
	}
	if cfg.RateLimit.Burst == 0 {
		cfg.RateLimit.Burst = 10
	}
	if cfg.Concurrency == 0 {
		cfg.Concurrency = 4
	}
	if cfg.HTTPClient.Timeout == 0 {
		cfg.HTTPClient.Timeout = 30
	}
	if cfg.WordProcessing.MinWordLength == 0 {
		cfg.WordProcessing.MinWordLength = 3
	}
	if cfg.Output.TopWordsCount == 0 {
		cfg.Output.TopWordsCount = 10
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.URLs.WordBankURL == "" {
		return fmt.Errorf("wordBankURL is required")
	}
	if len(c.ArticleURLs) == 0 {
		return fmt.Errorf("no article URLs loaded from file")
	}
	if c.RateLimit.RequestsPerSecond <= 0 {
		return fmt.Errorf("requestsPerSecond must be positive")
	}
	if c.Concurrency <= 0 {
		return fmt.Errorf("concurrency must be positive")
	}
	return nil
}
