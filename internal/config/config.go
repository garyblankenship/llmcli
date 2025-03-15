package config

import (
	"os"
	"path/filepath"
)

// Config holds the application configuration
type Config struct {
	ModelsDir    string
	DBPath       string
	LlamaServer  string
	LlamaCLI     string
	DefaultPort  int
	APIURL       string
	Temperature  float64
	TopK         int
	TopP         float64
	NPredictMax  int
}

// Load creates a Config with values from environment or defaults
func Load() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	cacheDir := filepath.Join(homeDir, ".cache", "llm-cli")
	modelsDir := filepath.Join(cacheDir, "models")
	dbPath := filepath.Join(cacheDir, "llm-cli.db")

	// Create directories if they don't exist
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		return nil, err
	}

	// Default values
	defaultPort := 1966
	
	// Server path (prefer env vars if set)
	llamaServer := os.Getenv("LLAMA_SERVER")
	if llamaServer == "" {
		llamaServer = "/opt/homebrew/bin/llama-server"
	}
	
	llamaCLI := os.Getenv("LLAMA_CLI")
	if llamaCLI == "" {
		llamaCLI = "/opt/homebrew/bin/llama-cli"
	}
	
	// API URL (prefer env var if set)
	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:1966"
	}

	return &Config{
		ModelsDir:    modelsDir,
		DBPath:       dbPath,
		LlamaServer:  llamaServer,
		LlamaCLI:     llamaCLI,
		DefaultPort:  defaultPort,
		APIURL:       apiURL,
		Temperature:  0.7,
		TopK:         40,
		TopP:         0.5,
		NPredictMax:  256,
	}, nil
}