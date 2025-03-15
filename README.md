# llmcli

A modern, Go-based command-line interface for managing, downloading, and interacting with Large Language Models, specifically optimized for GGUF format models.

## 🚀 Features

- Download models from Hugging Face
- Run models and start server instances
- Interactive chat sessions with models
- Manage a local database of model information
- Generate embeddings
- Tokenize and detokenize text
- Monitor running model servers
- Fetch recent and trending GGUF models from Hugging Face
- Beautiful, colorized terminal output

## 📋 Prerequisites

Before you begin, ensure you have the following installed:

- `llama-server` command (macOS: `brew install llama.cpp`)
- `huggingface-cli` command (macOS: `brew install huggingface-cli`)
- `sqlite3` (usually pre-installed on macOS)
- Go 1.21 or newer

## 🛠 Installation

1. Clone this repository:
   ```
   git clone https://github.com/garyblankenship/llmcli.git
   cd llmcli
   ```

2. Build the application:
   ```
   go build -o llmcli cmd/llm-cli/main.go
   ```

3. Optionally, add the binary to your PATH for easier access:
   ```
   cp llmcli /usr/local/bin/
   ```

## 🎮 Usage Examples

### Browsing Models

```bash
# Get recent GGUF models from Hugging Face
llmcli recent

# Get trending GGUF models from Hugging Face
llmcli trending
```

### Managing Models

```bash
# Download a new model
llmcli pull bartowski/Qwen2.5-Math-1.5B-Instruct-GGUF

# List all downloaded models
llmcli ls
```

### Using Models

```bash
# Start a chat session with a model
llmcli chat model-slug

# Generate embeddings
llmcli embed model-slug "Your text here"

# Tokenize text
llmcli tokenize model-slug "Your text here"
```

### Server Management

```bash
# Check server health
llmcli health

# Show running processes
llmcli ps

# Start a server
llmcli server model-slug
```

For a full list of commands, run:

```bash
llmcli
```

For help with a specific command, use:

```bash
llmcli <command> --help
```

## 🧑‍💻 Development

To build the project from source:

```bash
# Clone the repository
git clone https://github.com/garyblankenship/llmcli.git
cd llmcli

# Install dependencies
go mod download

# Build the binary
go build -o llmcli cmd/llm-cli/main.go

# Run tests
go test ./...

# Format code
go fmt ./...
```

## 📜 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 🙏 Acknowledgements

- [llama.cpp](https://github.com/ggerganov/llama.cpp) for the underlying model server
- [Hugging Face](https://huggingface.co/) for hosting the models