# Word Count Service

A service that fetches articles from provided URLs and counts the top 10 most frequent words that match specific criteria.

## Requirements

- Go 1.20 or higher
- Internet connection to fetch articles

## Installation

1. Clone the repository:
```bash
git clone https://github.com/NivBraz/wordcount-service.git
cd wordcount-service
```

2. Install dependencies:
```bash
go mod tidy
```

3. Configure the service:
- Edit `config.yaml` to set your URLs and preferences
- Ensure the word bank URL is correctly set

## Usage

Run the service:
```bash
go run cmd/wordcount/main.go
```

The service will:
1. Load the word bank
2. Fetch all articles concurrently (with rate limiting)
3. Process words according to the criteria:
    - At least 3 characters
    - Only alphabetic characters
    - Present in the word bank
4. Output the top 10 most frequent words in JSON format

## Project Structure

- `cmd/wordcount/`: Main application entry point
- `internal/`: Internal application code
    - `app/`: Application logic
    - `config/`: Configuration handling
    - `models/`: Data models
    - `services/`: Business logic services
- `pkg/`: Reusable packages
    - `fetcher/`: HTTP fetching with rate limiting
    - `parser/`: HTML and text parsing
    - `wordbank/`: Word bank management

## Configuration

The `config.yaml` file allows you to configure:
- Rate limiting parameters
- Word bank URL
- Article URLs to process
- Concurrency level

## License

MIT License