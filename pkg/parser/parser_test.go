package parser

import (
	"github.com/NivBraz/wordcount-service/internal/models"
	"reflect"
	"testing"
)

func TestParseWords(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		expected []string
		wantErr  bool
	}{
		{
			name:     "Simple HTML",
			content:  []byte("<html><body>Hello World</body></html>"),
			expected: []string{"hello", "world"},
			wantErr:  false,
		},
		{
			name:     "HTML with Script and Style",
			content:  []byte("<html><script>var x = 'test';</script><style>.test{color:red;}</style><body>Hello World</body></html>"),
			expected: []string{"hello", "world"},
			wantErr:  false,
		},
		{
			name:     "HTML with Special Characters",
			content:  []byte("<html><body>Hello! World? Test123</body></html>"),
			expected: []string{"hello", "world", "test"},
			wantErr:  false,
		},
		{
			name:     "Invalid HTML",
			content:  []byte("<html><body>Hello World</body>"),
			expected: []string{"hello", "world"},
			wantErr:  false, // html.Parse is quite forgiving
		},
		{
			name:     "Empty HTML",
			content:  []byte(""),
			expected: make([]string, 0), // Initialize as empty slice rather than nil
			wantErr:  false,
		},
	}

	p := New()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := p.ParseWords(tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseWords() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Handle nil case
			if got == nil {
				got = make([]string, 0)
			}
			if tt.expected == nil {
				tt.expected = make([]string, 0)
			}

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ParseWords() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseWordBank(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		expected []string
		wantErr  bool
	}{
		{
			name:     "Simple Word List",
			content:  []byte("hello\nworld\ntest"),
			expected: []string{"hello", "world", "test"},
			wantErr:  false,
		},
		{
			name:     "Word List with Special Characters",
			content:  []byte("hello!\nworld?\ntest123"),
			expected: []string{"hello", "world", "test"},
			wantErr:  false,
		},
		{
			name:     "Empty Lines",
			content:  []byte("hello\n\nworld"),
			expected: []string{"hello", "world"},
			wantErr:  false,
		},
		{
			name:     "Empty Content",
			content:  []byte(""),
			expected: make([]string, 0), // Initialize as empty slice rather than nil
			wantErr:  false,
		},
	}

	p := New()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := p.ParseWordBank(tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseWordBank() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Handle nil case
			if got == nil {
				got = make([]string, 0)
			}
			if tt.expected == nil {
				tt.expected = make([]string, 0)
			}

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ParseWordBank() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCleanWord(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple Word",
			input:    "Hello",
			expected: "hello",
		},
		{
			name:     "Word with Special Characters",
			input:    "Hello!@#$%",
			expected: "hello",
		},
		{
			name:     "Word with Numbers",
			input:    "Hello123",
			expected: "hello",
		},
		{
			name:     "Empty String",
			input:    "",
			expected: "",
		},
		{
			name:     "Only Special Characters",
			input:    "!@#$%",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cleanWord(tt.input); got != tt.expected {
				t.Errorf("cleanWord() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsAlphabetic(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "Simple Word",
			input:    "hello",
			expected: true,
		},
		{
			name:     "Word with Numbers",
			input:    "hello123",
			expected: false,
		},
		{
			name:     "Word with Special Characters",
			input:    "hello!",
			expected: false,
		},
		{
			name:     "Empty String",
			input:    "",
			expected: true,
		},
		{
			name:     "Mixed Characters",
			input:    "Hello World!",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAlphabetic(tt.input); got != tt.expected {
				t.Errorf("IsAlphabetic() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSortWordCounts(t *testing.T) {
	tests := []struct {
		name     string
		input    []models.WordCount
		expected []models.WordCount
	}{
		{
			name: "Different Counts",
			input: []models.WordCount{
				{Word: "hello", Count: 1},
				{Word: "world", Count: 3},
				{Word: "test", Count: 2},
			},
			expected: []models.WordCount{
				{Word: "world", Count: 3},
				{Word: "test", Count: 2},
				{Word: "hello", Count: 1},
			},
		},
		{
			name: "Same Counts",
			input: []models.WordCount{
				{Word: "zebra", Count: 2},
				{Word: "apple", Count: 2},
				{Word: "banana", Count: 2},
			},
			expected: []models.WordCount{
				{Word: "apple", Count: 2},
				{Word: "banana", Count: 2},
				{Word: "zebra", Count: 2},
			},
		},
		{
			name:     "Empty Slice",
			input:    make([]models.WordCount, 0),
			expected: make([]models.WordCount, 0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SortWordCounts(tt.input)
			if !reflect.DeepEqual(tt.input, tt.expected) {
				t.Errorf("SortWordCounts() = %v, want %v", tt.input, tt.expected)
			}
		})
	}
}
