// pkg/parser/parser.go
package parser

import (
	"bytes"
	"golang.org/x/net/html"
	"sort"
	"strings"
	"unicode"

	"github.com/NivBraz/wordcount-service/internal/models"
)

type Parser struct{}

func New() *Parser {
	return &Parser{}
}

// ParseWords extracts words from HTML content
func (p *Parser) ParseWords(content []byte) ([]string, error) {
	doc, err := html.Parse(bytes.NewReader(content))
	if err != nil {
		return nil, err
	}

	var words []string
	var extractText func(*html.Node)
	extractText = func(n *html.Node) {
		// if its script or style ignore
		if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style") {
			return
		}
		if n.Type == html.TextNode {
			// Split text into words
			text := strings.Fields(n.Data)
			for _, word := range text {
				// Clean and normalize the word
				word = cleanWord(word)
				if word != "" {
					words = append(words, word)
				}
			}
		}
		// Recursively process child nodes
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extractText(c)
		}
	}

	extractText(doc)
	return words, nil
}

// ParseWordBank extracts words from the word bank content
func (p *Parser) ParseWordBank(content []byte) ([]string, error) {
	// Split content into lines
	lines := strings.Split(string(content), "\n")
	var words []string

	for _, line := range lines {
		// Clean and normalize the word
		word := cleanWord(line)
		if word != "" {
			words = append(words, word)
		}
	}

	return words, nil
}

// cleanWord normalizes and cleans a word
func cleanWord(word string) string {
	// Convert to lowercase
	word = strings.ToLower(word)

	// Remove any remaining non-letter characters
	word = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) {
			return r
		}
		return -1
	}, word)

	return strings.TrimSpace(word)
}

// IsAlphabetic checks if a string contains only alphabetic characters
func IsAlphabetic(word string) bool {
	for _, r := range word {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

// SortWordCounts sorts word counts by frequency (descending) and alphabetically for ties
func SortWordCounts(words []models.WordCount) {
	sort.Slice(words, func(i, j int) bool {
		if words[i].Count == words[j].Count {
			return words[i].Word < words[j].Word
		}
		return words[i].Count > words[j].Count
	})
}
