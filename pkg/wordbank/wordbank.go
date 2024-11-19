package wordbank

import (
	"strings"
	"sync"
)

type WordBank struct {
	words map[string]struct{}
	mu    sync.RWMutex
}

func New() *WordBank {
	return &WordBank{
		words: make(map[string]struct{}),
	}
}

func (wb *WordBank) Add(word string) {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	wb.words[strings.ToLower(word)] = struct{}{}
}

func (wb *WordBank) Contains(word string) bool {
	wb.mu.RLock()
	defer wb.mu.RUnlock()
	_, exists := wb.words[strings.ToLower(word)]
	return exists
}
