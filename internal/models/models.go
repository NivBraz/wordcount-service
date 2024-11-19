package models

type WordCount struct {
	Word  string `json:"word"`
	Count int    `json:"count"`
}

type Result struct {
	TopWords []WordCount `json:"topWords"`
	Stats    struct {
		TotalProcessed int `json:"totalProcessed"`
		TimeElapsed    int `json:"timeElapsedMs"`
	} `json:"stats"`
}
