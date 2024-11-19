// internal/app/app.go
package app

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/NivBraz/wordcount-service/internal/config"
	"github.com/NivBraz/wordcount-service/internal/models"
	"github.com/NivBraz/wordcount-service/pkg/fetcher"
	"github.com/NivBraz/wordcount-service/pkg/parser"
	"github.com/NivBraz/wordcount-service/pkg/wordbank"
)

// App represents the main application
type App struct {
	config   *config.Config
	fetcher  *fetcher.Fetcher
	parser   *parser.Parser
	wordBank *wordbank.WordBank
}

// New creates a new instance of the application
func New(cfg *config.Config) (*App, error) {
	// Validate config
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Initialize components
	f := fetcher.New(cfg.RateLimit.RequestsPerSecond, cfg.RateLimit.Burst)
	p := parser.New()
	wb := wordbank.New()

	// Initialize word bank
	if err := initializeWordBank(context.Background(), f, p, wb, cfg.URLs.WordBankURL); err != nil {
		return nil, fmt.Errorf("failed to initialize word bank: %w", err)
	}

	return &App{
		config:   cfg,
		fetcher:  f,
		parser:   p,
		wordBank: wb,
	}, nil
}

// Run executes the main application logic
func (a *App) Run(ctx context.Context) (*models.Result, error) {
	startTime := time.Now()

	// Create channels for word processing
	wordChan := make(chan string, 1000)
	errChan := make(chan error, len(a.config.ArticleURLs))

	// Create wait groups for goroutines
	var fetchWg sync.WaitGroup
	var processWg sync.WaitGroup

	// Create word frequency map with mutex
	frequencies := make(map[string]int)
	var freqMutex sync.RWMutex

	// Start word processing goroutine
	processWg.Add(1)
	go func() {
		defer processWg.Done()
		for word := range wordChan {
			if isValidWord(word) && a.wordBank.Contains(word) {
				freqMutex.Lock()
				fmt.Println("word: ", word)
				frequencies[word]++
				freqMutex.Unlock()
			}
		}
	}()

	// Start fetching articles
	semaphore := make(chan struct{}, a.config.Concurrency)
	for _, url := range a.config.ArticleURLs {
		fetchWg.Add(1)
		go func(url string) {
			defer fetchWg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Fetch and process article
			if err := a.processArticle(ctx, url, wordChan); err != nil {
				log.Printf("Error processing article %s: %v", url, err)
				errChan <- fmt.Errorf("failed to process %s: %w", url, err)
			}
		}(url)
	}

	// Wait for all fetches to complete and close channels
	go func() {
		fetchWg.Wait()
		close(wordChan)
		close(errChan)
	}()

	// Wait for word processing to complete
	processWg.Wait()

	// Check for errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	// Prepare results
	result := &models.Result{
		TopWords: getTopWords(frequencies, 10),
		Stats: struct {
			TotalProcessed int `json:"totalProcessed"`
			TimeElapsed    int `json:"timeElapsedMs"`
		}{
			TotalProcessed: len(frequencies),
			TimeElapsed:    int(time.Since(startTime).Milliseconds()),
		},
	}

	if len(errs) > 0 {
		return result, fmt.Errorf("encountered %d errors during processing", len(errs))
	}

	return result, nil
}

// processArticle fetches and processes a single article
func (a *App) processArticle(ctx context.Context, url string, wordChan chan<- string) error {
	// Fetch article content
	fmt.Println("fetching article: ", url)
	content, err := a.fetcher.Fetch(ctx, url)
	if err != nil {
		return fmt.Errorf("failed to fetch article: %w", err)
	}

	// Parse words from content
	fmt.Println("parsing words")
	words, err := a.parser.ParseWords(content)
	if err != nil {
		return fmt.Errorf("failed to parse article: %w", err)
	}

	// Send words to processing channel
	for _, word := range words {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case wordChan <- word:
		}
	}

	return nil
}

// Helper functions

func validateConfig(cfg *config.Config) error {
	if cfg.RateLimit.RequestsPerSecond <= 0 {
		return fmt.Errorf("invalid rate limit: requests per second must be positive")
	}
	if cfg.Concurrency <= 0 {
		return fmt.Errorf("invalid concurrency: must be positive")
	}
	if len(cfg.ArticleURLs) == 0 {
		return fmt.Errorf("no article URLs provided")
	}
	if cfg.URLs.WordBankURL == "" {
		return fmt.Errorf("word bank URL is required")
	}
	return nil
}

func initializeWordBank(ctx context.Context, f *fetcher.Fetcher, p *parser.Parser, wb *wordbank.WordBank, url string) error {
	// Fetch word bank content
	content, err := f.Fetch(ctx, url)
	if err != nil {
		return fmt.Errorf("failed to fetch word bank: %w", err)
	}

	// Parse words and add to word bank
	words, err := p.ParseWordBank(content)
	if err != nil {
		return fmt.Errorf("failed to parse word bank: %w", err)
	}

	for _, word := range words {
		wb.Add(word)
	}

	return nil
}

func isValidWord(word string) bool {
	return len(word) >= 3 && parser.IsAlphabetic(word)
}

func getTopWords(frequencies map[string]int, n int) []models.WordCount {
	// Convert map to slice for sorting
	var words []models.WordCount
	for word, count := range frequencies {
		words = append(words, models.WordCount{
			Word:  word,
			Count: count,
		})
	}

	// Sort by frequency (descending) and alphabetically for ties
	parser.SortWordCounts(words)

	// Return top N words
	if len(words) > n {
		return words[:n]
	}
	return words
}
